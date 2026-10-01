package server_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/facturacion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/pdf/pdfprueba"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// correoQueFalla falla las primeras n veces (el servidor SMTP caído).
type correoQueFalla struct {
	mu     sync.Mutex
	fallas int
	mail.Memory
}

func (c *correoQueFalla) Send(ctx context.Context, m mail.Message) error {
	c.mu.Lock()
	if c.fallas > 0 {
		c.fallas--
		c.mu.Unlock()
		return errors.New("smtp: conexión rechazada para " + m.To)
	}
	c.mu.Unlock()
	return c.Memory.Send(ctx, m)
}

// F5-11: autorizada la factura, el comprador recibe el RIDE en PDF y el XML autorizado.
func TestCorreoAlCompradorConRIDEyXML(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "correo@w.ec")
	n := e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	var cfg facturacion.Config
	c.do("PUT", "/v1/cajas/"+cajas[0].ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 200, &cfg)
	ahora := time.Now()
	c.subirP12(p12De(t, "1790011674001", ahora.Add(-time.Hour), ahora.AddDate(1, 0, 0)), claveP12, 201)
	conCorreo := func(d *sri.DatosFactura) {
		d.Comprador = sri.Comprador{TipoIdentificacion: "05", Identificacion: "1710034065", RazonSocial: "María Pérez"}
		d.Adicionales = []sri.CampoAdicional{{Nombre: "Email", Valor: "maria@example.com"}}
	}
	f := n.comprobanteCon("1790011674001", *cfg.Cajas[0].PuntoID, 21, false, conCorreo)
	sinCorreo := n.comprobante("1790011674001", *cfg.Cajas[0].PuntoID, 22, false)
	drenar(t, e.worker("fiscal@correo"))

	smtp := &correoQueFalla{fallas: 1}
	correos := &facturacion.Correos{DB: e.tdb.App, Mail: smtp, Log: slog.Default()}
	ctx := context.Background()
	// Primer intento: el servidor de correo falla; queda registrado y se reintenta después.
	if n, err := correos.EnviarPendientes(ctx); err != nil || n != 0 {
		t.Fatalf("primer intento: %d %v", n, err)
	}
	if n, _ := correos.EnviarPendientes(ctx); n != 0 {
		t.Fatal("el reintento espera (2 min tras la primera falla)")
	}
	if _, err := e.tdb.Admin.Exec(ctx, `ALTER TABLE comprobante_correos DISABLE TRIGGER comprobante_correos_inmutable`); err != nil {
		t.Fatal(err)
	}
	_, _ = e.tdb.Admin.Exec(ctx, `UPDATE comprobante_correos SET created_at = now() - interval '3 minutes'`)
	_, _ = e.tdb.Admin.Exec(ctx, `ALTER TABLE comprobante_correos ENABLE TRIGGER comprobante_correos_inmutable`)
	if n, err := correos.EnviarPendientes(ctx); err != nil || n != 1 {
		t.Fatalf("reintento: %d %v", n, err)
	}
	if n, _ := correos.EnviarPendientes(ctx); n != 0 {
		t.Fatal("una factura se envía una sola vez")
	}

	msg, ok := smtp.Last()
	if !ok || msg.To != "maria@example.com" || !strings.Contains(msg.Subject, "001-001-000000021") || !strings.HasPrefix(msg.Subject, "[PRUEBAS]") {
		t.Fatalf("correo: %+v", msg)
	}
	if len(msg.Attachments) != 2 || msg.Attachments[0].Name != "FACTURA-001-001-000000021.pdf" || msg.Attachments[1].Name != "FACTURA-001-001-000000021.xml" {
		t.Fatalf("adjuntos: %+v", msg.Attachments)
	}
	pdfB, xmlB := msg.Attachments[0].Data, msg.Attachments[1].Data
	if !bytes.HasPrefix(pdfB, []byte("%PDF")) || !bytes.Contains(xmlB, []byte("<estado>AUTORIZADO</estado>")) ||
		!bytes.Contains(xmlB, []byte("<numeroAutorizacion>"+f.ClaveAcceso+"</numeroAutorizacion>")) || !bytes.Contains(xmlB, []byte("<ds:Signature")) {
		t.Fatalf("documentos: %s", xmlB[:200])
	}
	if txt := pdfprueba.Texto(t, pdfB); txt != "" && (!strings.Contains(txt, "María Pérez") || strings.Contains(txt, "Pendiente de autorización")) {
		t.Fatalf("el RIDE del correo es el autorizado:\n%s", txt)
	}

	var intentos, exitosos int
	var comprador, nombre string
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE ok) FROM comprobante_correos WHERE tenant_id = $1`, r.TenantID).Scan(&intentos, &exitosos)
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT comprador_identificacion, comprador_nombre FROM comprobantes WHERE id = $1`, f.ID).Scan(&comprador, &nombre)
	if intentos != 2 || exitosos != 1 || comprador != "1710034065" || nombre != "María Pérez" {
		t.Fatalf("intentos %d, ok %d, comprador %s %s", intentos, exitosos, comprador, nombre)
	}
	var errorGuardado string
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT error FROM comprobante_correos WHERE NOT ok`).Scan(&errorGuardado)
	if strings.Contains(errorGuardado, "maria@example.com") {
		t.Fatal("el error no repite el correo del comprador")
	}

	// Sin correo no se envía; reenviar a mano exige un destino.
	if err := correos.Enviar(ctx, r.TenantID, sinCorreo.ID, "", facturacion.CorreoReenvio, nil); err == nil {
		t.Fatal("sin correo del comprador no hay a quién enviar")
	}
	if err := correos.Enviar(ctx, r.TenantID, ids.New(), "x@example.com", facturacion.CorreoReenvio, nil); err == nil {
		t.Fatal("comprobante inexistente")
	}
}
