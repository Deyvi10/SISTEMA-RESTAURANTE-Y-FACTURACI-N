// Package fiscal es el worker fiscal (ADR-0006): toma los comprobantes que emitieron los nodos,
// los firma con el certificado del restaurante (el único proceso que puede descifrarlo, F5-07)
// y los lleva por recepción y autorización del SRI (F5-10) hasta AUTORIZADO.
//
// La cola es la propia tabla de comprobantes (ADR-0007): comprobantes_pendientes() da los que
// tocan, y cada uno se reclama con FOR UPDATE SKIP LOCKED y una concesión corta en
// proximo_intento_at, así varios workers no toman el mismo y nada queda bloqueado si uno muere.
package fiscal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/certificados"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/firma"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/sriws"
)

// Estados del comprobante en la nube (db/cloud/migrations/…_comprobantes.sql).
const (
	EnNube       = "EN_NUBE"
	Firmado      = "FIRMADO"
	Recibido     = "RECIBIDO"
	Autorizado   = "AUTORIZADO"
	NoAutorizado = "NO_AUTORIZADO"
	Devuelto     = "DEVUELTO"
)

// Concesion es cuánto tiempo un worker se reserva un comprobante mientras habla con el SRI.
const Concesion = 2 * time.Minute

// EsperaSinFirma: sin certificado (o vencido) se vuelve a mirar cada hora; subir uno nuevo
// reactiva la cola al instante.
const EsperaSinFirma = time.Hour

type Worker struct {
	DB  *db.DB
	KEK certificados.Desenvolvedor
	// Host reemplaza el del SRI (el stub en local y en pruebas); vacío: el del ambiente.
	Host     string
	Politica sriws.Politica
	Now      func() time.Time
	Log      *slog.Logger
	// Proceso identifica a este worker en el log de accesos al certificado.
	Proceso string
}

type pendiente struct{ tenant, id ids.ID }

type comprobante struct {
	id                 ids.ID
	estado             string
	ambiente           sri.Ambiente
	clave              string
	xml                string
	xmlFirmado         *string
	intentos           int
	zona               string
	certID             *ids.ID
	numeroAutorizacion *string
	sustento           *ids.ID // factura que modifica una nota de crédito
	revierteTodo       bool
}

// Procesar hace una pasada por la cola y devuelve cuántos comprobantes avanzó.
func (w *Worker) Procesar(ctx context.Context) (int, error) {
	var ps []pendiente
	if err := w.DB.Global(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT tenant_id, comprobante_id FROM comprobantes_pendientes(100)`)
		if err != nil {
			return err
		}
		ps, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (pendiente, error) {
			var p pendiente
			return p, r.Scan(&p.tenant, &p.id)
		})
		return err
	}); err != nil {
		return 0, err
	}
	n := 0
	for _, p := range ps {
		if ctx.Err() != nil {
			break
		}
		ok, err := w.procesar(ctx, p)
		if err != nil {
			w.Log.Error("fiscal: no se pudo procesar el comprobante", "comprobante", p.id, "err", err)
			continue
		}
		if ok {
			n++
		}
	}
	return n, nil
}

// reclamar toma el comprobante si sigue pendiente y nadie más lo tiene.
func (w *Worker) reclamar(ctx context.Context, p pendiente) (*comprobante, error) {
	var c *comprobante
	err := w.DB.InTenant(ctx, p.tenant, func(tx db.Tx) error {
		var x comprobante
		var amb int16
		err := tx.QueryRow(ctx, `SELECT c.id, c.estado, c.ambiente, c.clave_acceso, c.xml, c.xml_firmado, c.intentos, l.zona_horaria,
				(SELECT f.id FROM certificados_firma f WHERE f.activo), c.numero_autorizacion, c.sustento_id, c.revierte_todo
			FROM comprobantes c JOIN locales l ON l.id = c.local_id
			WHERE c.id = $1 AND c.estado IN ('EN_NUBE', 'FIRMADO', 'RECIBIDO')
			  AND (c.proximo_intento_at IS NULL OR c.proximo_intento_at <= $2)
			FOR UPDATE OF c SKIP LOCKED`, p.id, w.Now()).
			Scan(&x.id, &x.estado, &amb, &x.clave, &x.xml, &x.xmlFirmado, &x.intentos, &x.zona, &x.certID, &x.numeroAutorizacion, &x.sustento, &x.revierteTodo)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		x.ambiente = sri.Ambiente(amb)
		if _, err := tx.Exec(ctx, `UPDATE comprobantes SET proximo_intento_at = $2 WHERE id = $1`, x.id, w.Now().Add(Concesion)); err != nil {
			return err
		}
		c = &x
		return nil
	})
	return c, err
}

func (w *Worker) procesar(ctx context.Context, p pendiente) (bool, error) {
	c, err := w.reclamar(ctx, p)
	if err != nil || c == nil {
		return false, err
	}
	if c.estado == EnNube {
		firmado, motivo := w.firmar(ctx, p.tenant, c)
		if firmado == nil {
			return false, w.guardar(ctx, p.tenant, c, cambio{estado: EnNube, proximo: w.Now().Add(EsperaSinFirma), error: &motivo})
		}
		s := string(firmado)
		c.xmlFirmado = &s
		if err := w.guardar(ctx, p.tenant, c, cambio{estado: Firmado, xmlFirmado: &s, proximo: w.Now().Add(Concesion)}); err != nil {
			return false, err
		}
		c.estado, c.intentos = Firmado, 0
	}
	clave, err := sri.ParseClaveAcceso(c.clave)
	if err != nil {
		return false, err
	}
	e := sriws.Envio{Clave: clave, Fase: sriws.PorEnviar, Intentos: c.intentos}
	if c.estado == Recibido {
		e.Fase = sriws.Recibido
	}
	cli := sriws.NuevoCliente(c.ambiente, nil)
	if w.Host != "" {
		cli.Host = w.Host
	}
	nuevo := cli.Avanzar(ctx, e, []byte(*c.xmlFirmado), w.Now(), w.Politica)
	ch := cambio{estado: estadoDe(nuevo.Fase), intentos: nuevo.Intentos, proximo: nuevo.Proximo, mensajes: nuevo.Mensajes}
	if nuevo.UltimoError != "" {
		ch.error = &nuevo.UltimoError
	}
	if nuevo.Fase == sriws.Autorizada {
		ch.numero, ch.fecha = &nuevo.NumeroAutorizacion, &nuevo.FechaAutorizacion
	}
	return ch.estado != c.estado, w.guardar(ctx, p.tenant, c, ch)
}

// firmar descifra el certificado en memoria, firma y lo borra. Cada descifrado queda en
// certificados_accesos, salga bien o mal.
func (w *Worker) firmar(ctx context.Context, tenant ids.ID, c *comprobante) ([]byte, string) {
	if c.certID == nil {
		return nil, "Sin firma electrónica: sube el certificado .p12 en Facturación SRI."
	}
	var p12c, passc, dek []byte
	if err := w.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT p12_cifrado, password_cifrada, dek_cifrada FROM certificados_firma WHERE id = $1`, *c.certID).
			Scan(&p12c, &passc, &dek)
	}); err != nil {
		return nil, "No se pudo leer el certificado."
	}
	partes, err := certificados.Abrir(w.KEK, certificados.Contexto(tenant, *c.certID), dek, p12c, passc)
	w.registrarAcceso(ctx, tenant, *c.certID, c.id, err == nil)
	if err != nil {
		w.Log.Error("fiscal: no se pudo abrir el certificado", "certificado", *c.certID, "err", err)
		return nil, "No se pudo abrir el certificado guardado."
	}
	defer certificados.Borrar(partes...)
	f, err := firma.CargarP12(partes[0], string(partes[1]), w.Now())
	if err != nil {
		return nil, "El certificado no se puede usar: " + err.Error()
	}
	zona, err := time.LoadLocation(c.zona)
	if err != nil {
		zona = clock.Guayaquil
	}
	f.Ahora, f.Zona = w.Now, zona
	firmado, err := f.Firmar([]byte(c.xml))
	if err != nil {
		return nil, "No se pudo firmar: " + err.Error()
	}
	return firmado, ""
}

func (w *Worker) registrarAcceso(ctx context.Context, tenant, cert, comp ids.ID, ok bool) {
	err := w.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO certificados_accesos (id, tenant_id, certificado_id, proceso, motivo, comprobante_id, ok, created_at)
			VALUES ($1, app_tenant(), $2, $3, 'FIRMAR_COMPROBANTE', $4, $5, $6)`, ids.New(), cert, w.Proceso, comp, ok, w.Now())
		return err
	})
	if err != nil {
		w.Log.Error("fiscal: no se pudo registrar el acceso al certificado", "certificado", cert, "err", err)
	}
}

type cambio struct {
	estado     string
	intentos   int
	proximo    time.Time
	mensajes   []sriws.Mensaje
	error      *string
	xmlFirmado *string
	numero     *string
	fecha      *time.Time
}

// guardar persiste el paso y, si cambió el estado, lo deja en comprobante_eventos.
func (w *Worker) guardar(ctx context.Context, tenant ids.ID, c *comprobante, ch cambio) error {
	var proximo *time.Time
	if !ch.proximo.IsZero() {
		proximo = &ch.proximo
	}
	mensajes, err := json.Marshal(nonil(ch.mensajes))
	if err != nil {
		return err
	}
	return w.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE comprobantes SET estado = $2, intentos = $3, proximo_intento_at = $4,
				mensajes_sri = CASE WHEN $5::jsonb = '[]' THEN mensajes_sri ELSE $5::jsonb END, ultimo_error = $6,
				xml_firmado = coalesce(xml_firmado, $7), numero_autorizacion = coalesce(numero_autorizacion, $8),
				fecha_autorizacion = coalesce(fecha_autorizacion, $9), updated_at = $10
			WHERE id = $1`, c.id, ch.estado, ch.intentos, proximo, mensajes, ch.error, ch.xmlFirmado, ch.numero, ch.fecha, w.Now()); err != nil {
			return fmt.Errorf("guardar comprobante: %w", err)
		}
		if ch.estado == c.estado {
			return nil
		}
		detalle, _ := json.Marshal(map[string]any{"mensajes": nonil(ch.mensajes), "error": ch.error, "numeroAutorizacion": ch.numero})
		if _, err := tx.Exec(ctx, `INSERT INTO comprobante_eventos (id, tenant_id, comprobante_id, estado, detalle, created_at)
			VALUES ($1, app_tenant(), $2, $3, $4, $5)`, ids.New(), c.id, ch.estado, detalle, w.Now()); err != nil {
			return err
		}
		// Autorizada la nota de crédito que revierte todo lo que quedaba: la factura queda
		// anulada por NC (F5-13).
		if ch.estado != Autorizado || c.sustento == nil || !c.revierteTodo {
			return nil
		}
		tag, err := tx.Exec(ctx, `UPDATE comprobantes SET estado = 'ANULADO', updated_at = $2 WHERE id = $1 AND estado = 'AUTORIZADO'`, *c.sustento, w.Now())
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		anulada, _ := json.Marshal(map[string]any{"notaCredito": c.id, "claveNotaCredito": c.clave})
		_, err = tx.Exec(ctx, `INSERT INTO comprobante_eventos (id, tenant_id, comprobante_id, estado, detalle, created_at)
			VALUES ($1, app_tenant(), $2, 'ANULADO', $3, $4)`, ids.New(), *c.sustento, anulada, w.Now())
		return err
	})
}

func nonil(m []sriws.Mensaje) []sriws.Mensaje {
	if m == nil {
		return []sriws.Mensaje{}
	}
	return m
}

func estadoDe(f sriws.Fase) string {
	switch f {
	case sriws.Recibido:
		return Recibido
	case sriws.Autorizada:
		return Autorizado
	case sriws.Rechazada:
		return NoAutorizado
	case sriws.Devuelto:
		return Devuelto
	default:
		return Firmado
	}
}

// Correr procesa la cola al arrancar, cuando llega un comprobante (LISTEN comprobantes) y cada
// pocos segundos para las consultas y reintentos programados.
func (w *Worker) Correr(ctx context.Context, pool *pgxpool.Pool, cada time.Duration) {
	despertar := make(chan struct{}, 1)
	go db.Escuchar(ctx, pool, "comprobantes", despertar, w.Log)
	tick := time.NewTicker(cada)
	defer tick.Stop()
	for {
		for {
			n, err := w.Procesar(ctx)
			if err != nil && ctx.Err() == nil {
				w.Log.Warn("fiscal: no se pudo leer la cola", "err", err)
			}
			if n == 0 || ctx.Err() != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-despertar:
		}
	}
}
