package facturacion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/ride"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/sriws"
)

// Correos manda al comprador su factura autorizada: el RIDE en PDF (generado al vuelo) y el
// XML autorizado (F5-11). Si falla, reintenta hasta 5 veces con espera creciente.
type Correos struct {
	DB   *db.DB
	Mail mail.Sender
	Log  *slog.Logger
}

// Motivos de envío.
const (
	CorreoAutorizado = "AUTORIZADO"
	CorreoReenvio    = "REENVIO"
)

// Documentos es lo que se entrega de una factura autorizada.
type Documentos struct {
	Numero, Nombre, Correo string
	Archivo                string // FACTURA-001-002-000000067 o NOTA-CREDITO-001-002-000000004
	Ambiente               sri.Ambiente
	PDF, XML               []byte
	Leida                  sri.FacturaLeida
}

// LeerDocumentos arma el RIDE y el XML autorizado de un comprobante, dentro de su tenant.
func LeerDocumentos(ctx context.Context, tx db.Tx, comprobante ids.ID) (Documentos, error) {
	var firmado, numero, estado, correo *string
	var fecha *time.Time
	var amb int16
	var d Documentos
	err := tx.QueryRow(ctx, `SELECT c.estado, c.xml_firmado, c.numero_autorizacion, c.fecha_autorizacion, c.ambiente, c.correo_comprador, t.nombre_comercial
		FROM comprobantes c JOIN tenants t ON t.id = c.tenant_id WHERE c.id = $1`, comprobante).
		Scan(&estado, &firmado, &numero, &fecha, &amb, &correo, &d.Nombre)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, apperr.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	if (*estado != "AUTORIZADO" && *estado != "ANULADO") || firmado == nil || numero == nil || fecha == nil {
		return d, apperr.New(apperr.Conflict, "NO_AUTORIZADO", "La factura todavía no está autorizada por el SRI.")
	}
	d.Ambiente = sri.Ambiente(amb)
	if correo != nil {
		d.Correo = *correo
	}
	if d.Leida, err = sri.LeerComprobante([]byte(*firmado)); err != nil {
		return d, err
	}
	d.Numero = d.Leida.Numero()
	d.Archivo = NombreArchivo(d.Leida.CodDoc, d.Numero)
	if d.PDF, err = ride.PDF([]byte(*firmado), ride.Autorizacion{Fecha: fecha}); err != nil {
		return d, err
	}
	d.XML, err = sriws.XMLAutorizado([]byte(*firmado), *numero, *fecha, d.Ambiente)
	return d, err
}

// Mensaje es el correo con los dos adjuntos.
func (d Documentos) Mensaje() (mail.Message, error) {
	f := d.Leida
	que, titulo := "factura", "Factura"
	if f.EsNotaCredito() {
		que, titulo = "nota de crédito", "Nota de crédito"
	}
	asunto := fmt.Sprintf("Tu %s %s de %s", que, d.Numero, d.Nombre)
	parrafos := []string{
		fmt.Sprintf("Hola, %s. Te enviamos tu %s electrónica autorizada por el SRI.", f.RazonSocialComprador, que),
		fmt.Sprintf("%s %s del %s por $%s.", titulo, d.Numero, f.FechaEmision, f.ImporteTotal),
	}
	if f.EsNotaCredito() {
		parrafos = append(parrafos, fmt.Sprintf("Modifica la factura %s del %s. Motivo: %s.", f.NumDocModificado, f.FechaEmisionDocSustento, f.Motivo))
	}
	parrafos = append(parrafos, "Número de autorización: "+f.ClaveAcceso,
		"Adjuntamos el RIDE (PDF) y el comprobante en XML; con la clave de acceso también puedes consultarlo en el portal del SRI.")
	if d.Ambiente == sri.AmbientePruebas {
		asunto = "[PRUEBAS] " + asunto
		parrafos = append(parrafos, "Este comprobante es del ambiente de pruebas del SRI y no tiene validez tributaria.")
	}
	text, html, err := mail.Render(mail.Contenido{Titulo: titulo + " " + d.Numero, Parrafos: parrafos, Pie: d.Nombre})
	if err != nil {
		return mail.Message{}, err
	}
	base := d.Archivo
	return mail.Message{Subject: asunto, Text: text, HTML: html, Attachments: []mail.Attachment{
		{Name: base + ".pdf", ContentType: "application/pdf", Data: d.PDF},
		{Name: base + ".xml", ContentType: "application/xml", Data: d.XML},
	}}, nil
}

// Enviar manda el correo y deja el intento registrado (salga bien o mal).
func (c *Correos) Enviar(ctx context.Context, tenant, comprobante ids.ID, destino, motivo string, por *ids.ID) error {
	var d Documentos
	if err := c.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		var err error
		d, err = LeerDocumentos(ctx, tx, comprobante)
		return err
	}); err != nil {
		return err
	}
	if destino == "" {
		destino = d.Correo
	}
	if destino == "" {
		return apperr.New(apperr.Invalid, "SIN_CORREO", "Esta factura no tiene un correo del comprador.")
	}
	msg, err := d.Mensaje()
	if err != nil {
		return err
	}
	msg.To = destino
	envio := c.Mail.Send(ctx, msg)
	var detalle *string
	if envio != nil {
		e := strings.ReplaceAll(envio.Error(), destino, "…")
		detalle = &e
	}
	if err := c.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO comprobante_correos (id, tenant_id, comprobante_id, destino, motivo, ok, error, solicitado_por)
			VALUES ($1, app_tenant(), $2, $3, $4, $5, $6, $7)`, ids.New(), comprobante, destino, motivo, envio == nil, detalle, por)
		return err
	}); err != nil {
		return err
	}
	if envio != nil {
		return apperr.New(apperr.Unavailable, "CORREO_FALLO", "No se pudo enviar el correo; se volverá a intentar.")
	}
	return nil
}

// EnviarPendientes manda los correos de las facturas recién autorizadas.
func (c *Correos) EnviarPendientes(ctx context.Context) (int, error) {
	type pendiente struct{ tenant, id ids.ID }
	var ps []pendiente
	if err := c.DB.Global(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT tenant_id, comprobante_id FROM comprobantes_por_correo(50)`)
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
		if err := c.Enviar(ctx, p.tenant, p.id, "", CorreoAutorizado, nil); err != nil {
			c.Log.Warn("factura: no se pudo enviar el correo al comprador", "comprobante", p.id, "err", err)
			continue
		}
		n++
	}
	return n, nil
}

// Correr envía al arrancar, al autorizarse una factura (LISTEN) y cada minuto (reintentos).
func (c *Correos) Correr(ctx context.Context, pool *pgxpool.Pool) {
	despertar := make(chan struct{}, 1)
	go db.Escuchar(ctx, pool, "comprobantes_autorizados", despertar, c.Log)
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		if _, err := c.EnviarPendientes(ctx); err != nil && ctx.Err() == nil {
			c.Log.Warn("factura: no se pudieron leer los correos pendientes", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-despertar:
		}
	}
}

// NombreArchivo del comprobante descargado o adjunto.
func NombreArchivo(tipo, numero string) string {
	if tipo == sri.TipoNotaCredito {
		return "NOTA-CREDITO-" + numero
	}
	return "FACTURA-" + numero
}
