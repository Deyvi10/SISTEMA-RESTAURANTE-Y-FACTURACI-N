package caja

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/pdf"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/cierrez"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// EventoCierreZ es el evento del nodo con el Cierre Z completo (F4-12).
const EventoCierreZ = "caja.cierre_z"

// canalCierres despierta al notificador cuando llega un cierre (NOTIFY al confirmar el push).
const canalCierres = "cierres_z"

// RegistrarCierre guarda el Cierre Z que envió el nodo, dentro de la transacción del push.
// Idempotente por id. Un payload ilegible queda solo en la bitácora de eventos: no detiene la
// sincronización del nodo.
func RegistrarCierre(ctx context.Context, tx pgx.Tx, tenant, local ids.ID, payload []byte) error {
	var c cierrez.Cierre
	if err := json.Unmarshal(payload, &c); err != nil || c.ID == ids.Nil || c.TurnoID == ids.Nil || c.CajaID == ids.Nil || c.Numero < 1 {
		return nil
	}
	fecha, err := time.Parse("2006-01-02", c.FechaNegocio)
	if err != nil {
		fecha = c.CerradoAt
	}
	switch c.Resultado {
	case cierrez.Cuadrado, cierrez.Sobrante, cierrez.Faltante:
	default:
		return nil
	}
	tag, err := tx.Exec(ctx, `INSERT INTO cierres_z (id, tenant_id, local_id, caja_id, turno_id, numero, resultado, cajero, fecha_negocio, cerrado_at, datos, hash, hash_anterior, hash_valido)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) ON CONFLICT (id) DO NOTHING`,
		c.ID, tenant, local, c.CajaID, c.TurnoID, c.Numero, c.Resultado, c.Cajero, fecha, c.CerradoAt, payload, c.Hash, c.HashAnterior, c.Verificar())
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	_, err = tx.Exec(ctx, `SELECT pg_notify($1, $2)`, canalCierres, c.ID.String())
	return err
}

// Notificador envía cada Cierre Z por correo, con el PDF, a los dueños del restaurante; si una
// diferencia supera el umbral del local, el correo es una alerta crítica (RF-04-10.5-6).
// Entrega al menos una vez: si el proceso cae entre el envío y el registro, se reenvía.
type Notificador struct {
	DB   *db.DB
	Mail mail.Sender
	Log  *slog.Logger
}

// Pendiente es un cierre sin correo enviado.
type pendiente struct{ tenant, cierre ids.ID }

// EnviarPendientes procesa los cierres que aún no se enviaron y devuelve cuántos envió.
func (n *Notificador) EnviarPendientes(ctx context.Context) (int, error) {
	var ps []pendiente
	if err := n.DB.Global(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT tenant_id, cierre_id FROM cierres_z_pendientes(50)`)
		if err != nil {
			return err
		}
		ps, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (pendiente, error) {
			var p pendiente
			return p, r.Scan(&p.tenant, &p.cierre)
		})
		return err
	}); err != nil {
		return 0, err
	}
	enviados := 0
	for _, p := range ps {
		if err := n.enviar(ctx, p); err != nil {
			n.Log.Error("cierre z: no se pudo enviar el correo", "cierre", p.cierre, "err", err)
			continue
		}
		enviados++
	}
	return enviados, nil
}

type datosCorreo struct {
	cierre   cierrez.Cierre
	valido   bool
	negocio  string
	zona     string
	umbral   money.Money
	destinos []string
}

func (n *Notificador) enviar(ctx context.Context, p pendiente) error {
	var d datosCorreo
	var raw []byte
	var umbral string
	err := n.DB.InTenant(ctx, p.tenant, func(tx db.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT c.datos, c.hash_valido, l.zona_horaria, l.umbral_alerta_cierre::text, t.nombre_comercial
			FROM cierres_z c JOIN locales l ON l.id = c.local_id JOIN tenants t ON t.id = c.tenant_id WHERE c.id = $1`, p.cierre).
			Scan(&raw, &d.valido, &d.zona, &umbral, &d.negocio); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT email::text FROM usuarios WHERE es_dueno AND activo AND email IS NOT NULL ORDER BY email`)
		if err != nil {
			return err
		}
		d.destinos, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &d.cierre); err != nil {
		return err
	}
	if d.umbral, err = money.Parse(umbral); err != nil {
		return err
	}
	msg, alerta, err := correoCierre(d)
	if err != nil {
		return err
	}
	for _, to := range d.destinos {
		msg.To = to
		if err := n.Mail.Send(ctx, msg); err != nil {
			return err
		}
	}
	return n.DB.InTenant(ctx, p.tenant, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO cierres_z_envios (cierre_id, tenant_id, destinatarios, alerta) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
			p.cierre, p.tenant, len(d.destinos), alerta)
		return err
	})
}

// mayorDiferencia: el método con la diferencia más grande en valor absoluto.
func mayorDiferencia(c cierrez.Cierre) (cierrez.Linea, bool) {
	var peor cierrez.Linea
	hay := false
	for _, l := range c.Lineas {
		abs := l.Diferencia
		if abs.IsNegative() {
			abs = abs.Neg()
		}
		prev := peor.Diferencia
		if prev.IsNegative() {
			prev = prev.Neg()
		}
		if !abs.IsZero() && (!hay || abs.GreaterThan(prev)) {
			peor, hay = l, true
		}
	}
	return peor, hay
}

func correoCierre(d datosCorreo) (mail.Message, bool, error) {
	c := d.cierre
	loc, err := time.LoadLocation(d.zona)
	if err != nil {
		loc = time.UTC
	}
	peor, hayDif := mayorDiferencia(c)
	abs := peor.Diferencia
	if abs.IsNegative() {
		abs = abs.Neg()
	}
	alerta := hayDif && abs.GreaterThan(d.umbral)
	titulo := fmt.Sprintf("Cierre Z %04d · %s", c.Numero, c.Caja)
	resumen := map[string]string{cierrez.Cuadrado: "La caja cuadró.", cierrez.Sobrante: "Hubo un sobrante.", cierrez.Faltante: "Hubo un faltante."}[c.Resultado]
	asunto := titulo + " · " + strings.ToLower(c.Resultado[:1]) + strings.ToLower(c.Resultado[1:])
	quien := fmt.Sprintf("%s cerró su turno", c.Cajero)
	if c.CerradoPor != "" && c.CerradoPor != c.Cajero {
		quien = fmt.Sprintf("%s cerró el turno de %s", c.CerradoPor, c.Cajero)
	}
	parrafos := []string{
		fmt.Sprintf("%s en %s (jornada del %s, %s).", quien, c.Caja, c.FechaNegocio, c.CerradoAt.In(loc).Format("15:04")),
		resumen,
	}
	for _, l := range c.Lineas {
		if !l.Diferencia.IsZero() {
			parrafos = append(parrafos, fmt.Sprintf("%s: esperado $%s, declarado $%s, %s de $%s.", l.Metodo, l.Esperado, l.Declarado, strings.ToLower(l.Resultado), absoluto(l.Diferencia)))
		}
	}
	if alerta {
		asunto = fmt.Sprintf("⚠️ Alerta: %s de $%s en %s (Cierre Z %04d)", strings.ToLower(peor.Resultado), abs, c.Caja, c.Numero)
		parrafos = append([]string{fmt.Sprintf("La diferencia en %s ($%s) supera el umbral de $%s que configuraste para este local.", peor.Metodo, abs, d.umbral)}, parrafos...)
	}
	if !d.valido {
		parrafos = append(parrafos, "Atención: el código de integridad de este cierre no coincide con su contenido. Revisa el Nodo Local.")
	}
	text, html, err := mail.Render(mail.Contenido{Titulo: titulo, Parrafos: parrafos, Pie: d.negocio + " · El Cierre Z completo va adjunto en PDF. No se puede modificar."})
	if err != nil {
		return mail.Message{}, false, err
	}
	return mail.Message{Subject: asunto, Text: text, HTML: html, Attachments: []mail.Attachment{{
		Name: fmt.Sprintf("cierre-z-%04d-%s.pdf", c.Numero, c.FechaNegocio), ContentType: "application/pdf", Data: PDFCierre(c, loc),
	}}}, alerta, nil
}

func absoluto(m money.Money) money.Money {
	if m.IsNegative() {
		return m.Neg()
	}
	return m
}

// PDFCierre es el mismo Cierre Z que se imprimió en la caja, en una página A4.
func PDFCierre(c cierrez.Cierre, loc *time.Location) []byte {
	var d pdf.Doc
	texto := escpos.Decode(escpos.ImprimirCierreZ(escpos.Paper80, c, loc)).Text()
	for i, l := range strings.Split(strings.TrimRight(texto, "\n"), "\n") {
		if strings.Contains(l, "corte --") {
			continue
		}
		d.Linea(l, 10, i < 2 || l == c.Resultado)
	}
	if len(c.Hash) > 16 {
		d.Linea("", 10, false)
		d.Linea("Hash completo: "+c.Hash, 7, false)
	}
	return d.Bytes()
}

// Correr envía lo pendiente al arrancar, cada vez que llega un cierre (LISTEN) y, por si se
// perdió un aviso, cada minuto.
func (n *Notificador) Correr(ctx context.Context, pool *pgxpool.Pool) {
	despertar := make(chan struct{}, 1)
	go db.Escuchar(ctx, pool, canalCierres, despertar, n.Log)
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		if _, err := n.EnviarPendientes(ctx); err != nil && ctx.Err() == nil {
			n.Log.Warn("cierre z: no se pudieron leer los pendientes", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-despertar:
		}
	}
}
