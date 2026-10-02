package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// RetencionLocal: cuánto guarda el nodo la copia de un comprobante autorizado (F5-18).
const RetencionLocal = 90 * 24 * time.Hour

var (
	zstdUna sync.Once
	zEnc    *zstd.Encoder
	zDec    *zstd.Decoder
)

func zstdListo() {
	zstdUna.Do(func() {
		zEnc, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
		zDec, _ = zstd.NewReader(nil)
	})
}

type autorizadoNube struct {
	ID                 ids.ID    `json:"id"`
	NumeroAutorizacion string    `json:"numeroAutorizacion"`
	FechaAutorizacion  time.Time `json:"fechaAutorizacion"`
	XML                string    `json:"xml"`
}

// copiarAutorizados pide a la nube los XML autorizados que el nodo todavía no tiene (los que la
// réplica marcó AUTORIZADO o ANULADO) y los guarda comprimidos. Sin internet no hace nada:
// lo intenta en la siguiente vuelta. Una vez al día purga lo de más de 90 días.
func (a *App) copiarAutorizados(ctx context.Context) {
	now := a.Clock.Now()
	limite := now.Add(-RetencionLocal).Format(time.RFC3339Nano)
	rows, err := a.Store.Read().QueryContext(ctx, `SELECT c.id FROM comprobantes c JOIN estados_comprobante e ON e.id = c.id
		WHERE e.estado IN ('AUTORIZADO', 'ANULADO') AND c.created_at > ?
		  AND NOT EXISTS (SELECT 1 FROM autorizados_locales l WHERE l.comprobante_id = c.id)
		ORDER BY c.created_at LIMIT 50`, limite)
	if err != nil {
		return
	}
	var faltan []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			faltan = append(faltan, id)
		}
	}
	_ = rows.Close()
	if len(faltan) > 0 {
		var res []autorizadoNube
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := a.nube.Do(cctx, http.MethodPost, "/v1/nodos/comprobantes/autorizados", map[string]any{"ids": faltan}, &res, true)
		cancel()
		if err != nil {
			a.Log.Warn("copia local: no se pudieron traer los autorizados", "err", err)
		} else if err := a.guardarAutorizados(ctx, res, now); err != nil {
			a.Log.Error("copia local: no se pudieron guardar los autorizados", "err", err)
		}
	}
	if now.Sub(a.ultimaPurga) > 24*time.Hour {
		a.ultimaPurga = now
		if n, err := a.purgarAutorizados(ctx, now); err != nil {
			a.Log.Error("copia local: purga fallida", "err", err)
		} else if n > 0 {
			a.Log.Info("copia local: purgados los autorizados de más de 90 días", "comprobantes", n)
		}
	}
}

func (a *App) guardarAutorizados(ctx context.Context, res []autorizadoNube, now time.Time) error {
	if len(res) == 0 {
		return nil
	}
	zstdListo()
	return a.Store.Write(ctx, func(tx *store.Tx) error {
		for _, r := range res {
			// Solo se guarda si el comprobante autorizado es el que este nodo emitió.
			var clave string
			if err := tx.QueryRowContext(ctx, `SELECT clave_acceso FROM comprobantes WHERE id = ?`, r.ID.String()).Scan(&clave); err != nil {
				continue
			}
			if r.NumeroAutorizacion != clave || !strings.Contains(r.XML, "<claveAcceso>"+clave+"</claveAcceso>") {
				continue
			}
			suma := sha256.Sum256([]byte(r.XML))
			if _, err := tx.ExecContext(ctx, `INSERT INTO autorizados_locales (comprobante_id, numero_autorizacion, fecha_autorizacion, xml_zst, sha256, guardado_at)
				VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`, r.ID.String(), r.NumeroAutorizacion, r.FechaAutorizacion.UTC().Format(time.RFC3339),
				zEnc.EncodeAll([]byte(r.XML), nil), hex.EncodeToString(suma[:]), now.Format(time.RFC3339Nano)); err != nil {
				return err
			}
		}
		return nil
	})
}

// purgarAutorizados borra las copias locales de más de 90 días (la nube conserva el original).
func (a *App) purgarAutorizados(ctx context.Context, now time.Time) (int64, error) {
	var n int64
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		r, err := tx.ExecContext(ctx, `DELETE FROM autorizados_locales WHERE comprobante_id IN (
			SELECT c.id FROM comprobantes c WHERE c.created_at < ?)`, now.Add(-RetencionLocal).Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		n, err = r.RowsAffected()
		return err
	})
	return n, err
}

// XMLAutorizadoLocal devuelve la copia local del XML autorizado, verificada.
func (a *App) XMLAutorizadoLocal(ctx context.Context, id ids.ID) ([]byte, error) {
	var z []byte
	var suma string
	err := a.Store.Read().QueryRowContext(ctx, `SELECT xml_zst, sha256 FROM autorizados_locales WHERE comprobante_id = ?`, id.String()).Scan(&z, &suma)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, problema(http.StatusNotFound, "SIN_COPIA", "Este comprobante todavía no tiene su copia autorizada en el local.")
	}
	if err != nil {
		return nil, err
	}
	zstdListo()
	doc, err := zDec.DecodeAll(z, nil)
	if err != nil {
		return nil, err
	}
	if s := sha256.Sum256(doc); hex.EncodeToString(s[:]) != suma {
		return nil, errors.New("copia local alterada")
	}
	return doc, nil
}

// ReimprimirComprobanteIn: en qué caja se reimprime.
type ReimprimirComprobanteIn struct {
	CajaID ids.ID `json:"cajaId"`
}

// ReimprimirComprobante vuelve a imprimir el RIDE de una factura o NC con la marca REIMPRESIÓN y, si ya
// está autorizada, la leyenda con la fecha de autorización (F5-18, auditado como F2-13).
func (a *App) ReimprimirComprobante(ctx context.Context, u Usuario, id ids.ID, in ReimprimirComprobanteIn) (map[string]any, error) {
	if !u.Puede(rbac.Cobrar) && !u.Puede(rbac.EmitirNC) {
		return nil, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para reimprimir comprobantes.")
	}
	now := a.Clock.Now()
	var impresoras []string
	var despertar []ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		var doc, creado string
		var fechaAut sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT c.xml, c.created_at, coalesce(l.fecha_autorizacion, e.fecha_autorizacion) FROM comprobantes c
			LEFT JOIN autorizados_locales l ON l.comprobante_id = c.id LEFT JOIN estados_comprobante e ON e.id = c.id WHERE c.id = ?`, id.String()).
			Scan(&doc, &creado, &fechaAut)
		if errors.Is(err, sql.ErrNoRows) {
			return problema(http.StatusNotFound, "NO_ENCONTRADO", "Ese comprobante no existe en este local.")
		}
		if err != nil {
			return err
		}
		cfg, err := leerConfigFiscalSiempre(ctx, tx)
		if err != nil || cfg == nil {
			return problema(http.StatusConflict, "SIN_FACTURACION", "Este local no tiene la facturación electrónica configurada.")
		}
		l, err := sri.LeerComprobante([]byte(doc))
		if err != nil {
			return err
		}
		emitido, _ := time.Parse(time.RFC3339Nano, creado)
		caja, err := cajaExiste(ctx, tx, in.CajaID)
		if err != nil {
			return err
		}
		r := rideDesdeXML(l, emitido.In(a.zonaLocal(ctx)), caja, u.Nombre)
		r.Reimpresion = true
		if fechaAut.Valid && fechaAut.String != "" {
			if f, err := time.Parse(time.RFC3339, fechaAut.String); err == nil {
				t := f.In(a.zonaLocal(ctx))
				r.FechaAutorizacion = &t
			}
		}
		var estacion sql.NullString
		_ = tx.QueryRowContext(ctx, `SELECT estacion_id FROM cajas WHERE id = ?`, in.CajaID.String()).Scan(&estacion)
		var est *ids.ID
		if e, err := ids.Parse(estacion.String); err == nil {
			est = &e
		}
		imps, _, err := a.impresorasDeCaja(ctx, tx, est)
		if err != nil {
			return err
		}
		if len(imps) == 0 {
			return problema(http.StatusConflict, "SIN_IMPRESORA", "No hay impresora para reimprimir.")
		}
		imp := imps[0]
		if err := impresion.Encolar(ctx, tx, impresion.Trabajo{ID: ids.New(), ImpresoraID: imp.ID, Tipo: "REIMPRESION"}, nil, escpos.ImprimirRide(imp.Ancho, r), nil, now); err != nil {
			return err
		}
		impresoras, despertar = []string{imp.Nombre}, []ids.ID{imp.ID}
		uid := u.ID
		return auditar(ctx, tx, "COMPROBANTE_REIMPRESO", "comprobante", id, &uid, map[string]any{"numero": l.Numero(), "tipo": l.CodDoc,
			"autorizado": r.FechaAutorizacion != nil}, now)
	})
	if err != nil {
		return nil, err
	}
	a.motor.Despertar(despertar...)
	a.notificarPush()
	return map[string]any{"impresoras": impresoras}, nil
}

// rideDesdeXML arma el ticket RIDE desde el XML del comprobante (para reimprimir).
func rideDesdeXML(l sri.FacturaLeida, emitido time.Time, caja, cajero string) escpos.Ride {
	m := func(s string) money.Money { v, _ := money.Parse(s); return v }
	r := escpos.Ride{
		NombreComercial: l.NombreComercial, RazonSocial: l.RazonSocial, RUC: l.RUC, DirMatriz: l.DirMatriz, DirEstablecimiento: l.DirEstablecimiento,
		ContribuyenteEspecial: l.ContribuyenteEspecial, ObligadoContabilidad: l.ObligadoContabilidad == "SI", AgenteRetencion: l.AgenteRetencion,
		RIMPE: l.ContribuyenteRimpe != "", Numero: l.Numero(), ClaveAcceso: l.ClaveAcceso, Pruebas: l.Ambiente == "1", Emision: emitido,
		Mesa: l.Adicional("Mesa"), Cajero: cajero, Comprador: l.RazonSocialComprador, SubtotalSinImpuestos: m(l.TotalSinImpuestos),
		TotalDescuento: m(l.TotalDescuento), Propina: m(l.Propina), Total: m(l.ImporteTotal),
		NotaCredito: l.EsNotaCredito(), Sustento: l.NumDocModificado, FechaSustento: l.FechaEmisionDocSustento, Motivo: l.Motivo,
	}
	if r.Mesa == "" {
		r.Mesa = caja
	}
	if l.TipoIdComprador != string(sri.IdConsumidorFinal) {
		r.CompradorID = l.IdComprador
	}
	for _, d := range l.Detalles {
		r.Lineas = append(r.Lineas, escpos.LineaRide{Cantidad: strings.TrimRight(strings.TrimRight(d.Cantidad, "0"), "."), Descripcion: d.Descripcion,
			PrecioUnitario: precioDesdeXML(d.PrecioUnitario), Descuento: m(d.Descuento), Total: m(d.PrecioTotalSinImpuesto)})
	}
	for _, t := range l.Impuestos {
		pct := strings.TrimSuffix(strings.TrimRight(t.Tarifa, "0"), ".")
		if pct == "" {
			pct = "0"
		}
		if b := m(t.BaseImponible); !b.IsZero() {
			r.Subtotales = append(r.Subtotales, escpos.ValorRide{Etiqueta: "SUBTOTAL " + pct + "%", Valor: b})
		}
		if v := m(t.Valor); !v.IsZero() {
			r.IVAs = append(r.IVAs, escpos.ValorRide{Etiqueta: "IVA " + pct + "%", Valor: v})
		}
	}
	for _, p := range l.Pagos {
		r.Pagos = append(r.Pagos, escpos.PagoRide{Codigo: p.FormaPago, Monto: m(p.Total)})
	}
	for _, c := range l.Adicionales {
		if c.Nombre != "Mesa" && strings.TrimSpace(c.Valor) != "" {
			r.Adicionales = append(r.Adicionales, escpos.ValorTexto{Nombre: c.Nombre, Valor: c.Valor})
		}
	}
	return r
}

// precioDesdeXML: «13.043478» se deja, «2.000000» → «2.00».
func precioDesdeXML(s string) string {
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return s
	}
	dec := strings.TrimRight(s[i+1:], "0")
	for len(dec) < 2 {
		dec += "0"
	}
	return s[:i] + "." + dec
}
