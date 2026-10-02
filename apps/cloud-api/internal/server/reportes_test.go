package server_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/facturacion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/reportes"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// venta sube una venta cobrada como la emite el nodo.
func (n *nodoSim) venta(fechaNegocio, total, metodo string, extra ...map[string]any) ids.ID {
	n.e.t.Helper()
	id := ids.New()
	pagos := []map[string]any{{"metodo": metodo, "monto": total}}
	if len(extra) > 0 {
		pagos = append(pagos, extra...)
	}
	ev := n.evento(reportes.EventoVentaCobrada)
	ev.Payload, _ = json.Marshal(map[string]any{"documento": map[string]any{"id": id, "tipo": "INTERNO", "codigo": "INT-000001", "mesa": "Mesa 1",
		"cajero": "Luis P.", "comprador": "CONSUMIDOR FINAL", "emitidoAt": time.Now(), "fechaNegocio": fechaNegocio, "pagos": pagos,
		"totales": map[string]any{"subtotal": "10.00", "iva": "1.50", "propina": "1.00", "descuento": "0.00", "total": total}}})
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: []edgesync.Event{ev}}, true, 200, nil)
	return id
}

// F5-16: ventas por jornada, por método, Cierres Z con PDF y Excel para el contador.
func TestReportesDeVentas(t *testing.T) {
	e := newEnv(t)
	c, _ := e.restaurante("1790011674001", "rep@w.ec")
	n := e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	n.venta("2026-09-29", "12.50", "Efectivo")
	n.venta("2026-09-30", "20.00", "Efectivo")
	n.venta("2026-09-30", "30.00", "Tarjeta crédito", map[string]any{"metodo": "Efectivo", "monto": "0.00"})
	repetida := n.venta("2026-09-30", "5.00", "Efectivo")
	_ = repetida
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	n.cierreZ(cajas[0].ID, 1, "", "0")

	// Una venta anterior al pago mixto: un solo «metodo» y sin lista de pagos.
	ev := n.evento(reportes.EventoVentaCobrada)
	ev.Payload, _ = json.Marshal(map[string]any{"documento": map[string]any{"id": ids.New(), "tipo": "INTERNO", "codigo": "INT-000000", "metodo": "Efectivo",
		"emitidoAt": time.Date(2026, 9, 28, 4, 0, 0, 0, time.UTC), "totales": map[string]any{"subtotal": "8.00", "iva": "1.20", "propina": "0.80", "total": "10.00"}}})
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: []edgesync.Event{ev}}, true, 200, nil)
	var viejo reportes.Resumen
	c.do("GET", "/v1/reportes/ventas?desde=2026-09-27", nil, 200, &viejo) // 23:00 del 27 en Ecuador
	if viejo.Total.Total != "10.00" || len(viejo.PorMetodo) != 1 || viejo.PorMetodo[0].Monto != "10.00" {
		t.Fatalf("venta antigua: %+v", viejo)
	}

	var r reportes.Resumen
	c.do("GET", "/v1/reportes/ventas?desde=2026-09-30", nil, 200, &r)
	if r.Total.Documentos != 3 || r.Total.Total != "55.00" || r.TicketPromedio != "18.33" || len(r.Dias) != 1 || r.Neto != "55.00" {
		t.Fatalf("jornada del 30: %+v", r)
	}
	if len(r.PorMetodo) != 2 || r.PorMetodo[0].Metodo != "Tarjeta crédito" || r.PorMetodo[0].Monto != "30.00" || r.PorMetodo[1].Monto != "25.00" {
		t.Fatalf("por método: %+v", r.PorMetodo)
	}
	c.do("GET", "/v1/reportes/ventas?desde=2026-09-29&hasta=2026-09-30", nil, 200, &r)
	if r.Total.Documentos != 4 || r.Total.Total != "67.50" || len(r.Dias) != 2 || r.Dias[0].Fecha != "2026-09-29" {
		t.Fatalf("periodo: %+v", r)
	}
	c.do("GET", "/v1/reportes/ventas?desde=2026-10-01&hasta=2026-09-01", nil, 422, nil)

	// Cierres Z de la jornada, con su PDF.
	var cierres []reportes.CierreFila
	c.do("GET", "/v1/reportes/cierres?desde=2026-09-25", nil, 200, &cierres)
	if len(cierres) != 1 || cierres[0].Numero != 1 || cierres[0].Resultado != "CUADRADO" {
		t.Fatalf("cierres: %+v", cierres)
	}
	pdfB, disp := c.descargar("/v1/reportes/cierres/"+cierres[0].ID.String()+"/pdf", 200)
	if !bytes.HasPrefix(pdfB, []byte("%PDF")) || !strings.Contains(disp, "cierre-z-0001-2026-09-25.pdf") {
		t.Fatalf("pdf del cierre: %s", disp)
	}

	// Excel: una fila por venta, números como números.
	xl, disp := c.descargar("/v1/reportes/ventas.xlsx?desde=2026-09-29&hasta=2026-09-30", 200)
	if !strings.Contains(disp, "ventas-2026-09-29-a-2026-09-30.xlsx") {
		t.Fatalf("excel: %s", disp)
	}
	if _, err := zip.NewReader(bytes.NewReader(xl), int64(len(xl))); err != nil {
		t.Fatalf("el excel no es un zip: %v", err)
	}
	if exec.Command("python3", "-c", "import openpyxl").Run() == nil {
		f := filepath.Join(t.TempDir(), "v.xlsx")
		_ = os.WriteFile(f, xl, 0o600)
		out, err := exec.Command("python3", "-c", `import sys, openpyxl
ws = openpyxl.load_workbook(sys.argv[1]).active
filas = list(ws.iter_rows(values_only=True))
print(len(filas) - 1, round(sum(f[11] for f in filas[1:]), 2), filas[0][11])`, f).CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != "4 67.5 Total" {
			t.Fatalf("openpyxl: %s %v", out, err)
		}
	}
	// Otro restaurante no ve nada.
	otro, _ := e.restaurante("0992339411001", "rep@b.ec")
	otro.do("GET", "/v1/reportes/ventas?desde=2026-09-30", nil, 200, &r)
	if r.Total.Documentos != 0 {
		t.Fatal("RLS")
	}
	otro.descargar("/v1/reportes/cierres/"+cierres[0].ID.String()+"/pdf", 404)
}

// F5-16: alertas fiscales en el panel y por correo, una vez por alerta y día.
func TestAlertasFiscales(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "alerta@w.ec")
	n := e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	var cfg facturacion.Config
	c.do("PUT", "/v1/cajas/"+cajas[0].ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 200, &cfg)
	c.do("PUT", "/v1/facturacion", map[string]any{"ambiente": 1, "razonSocial": "Distribuidora", "direccionMatriz": "Quito", "regimen": "GENERAL", "facturacionActiva": true}, 201, &cfg)
	alertas := func() map[string]facturacion.Alerta {
		var l []facturacion.Alerta
		c.do("GET", "/v1/alertas/fiscales", nil, 200, &l)
		m := map[string]facturacion.Alerta{}
		for _, a := range l {
			m[a.Clave] = a
		}
		return m
	}
	// Facturando sin firma: crítica.
	if a := alertas(); a["sin-firma"].Nivel != facturacion.Critica {
		t.Fatalf("sin firma: %+v", a)
	}
	// Firma que vence en 15 días y un comprobante devuelto por el SRI.
	ahora := time.Now()
	c.subirP12(p12De(t, "1790011674001", ahora.Add(-time.Hour), ahora.AddDate(0, 0, 15).Add(time.Hour)), claveP12, 201)
	n.comprobante("1790011674001", *cfg.Cajas[0].PuntoID, 901, false)
	viejo := n.comprobante("1790011674001", *cfg.Cajas[0].PuntoID, 3, false)
	drenar(t, e.worker("fiscal@alertas"))
	ctx := context.Background()
	// Uno que lleva 13 h esperando (el SRI no responde).
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE comprobantes SET estado = 'RECIBIDO', recibido_at = now() - interval '13 hours' WHERE id = $1`, viejo.ID); err == nil {
		t.Fatal("un autorizado no retrocede")
	}
	espera := n.comprobante("1790011674001", *cfg.Cajas[0].PuntoID, 4, false) // sin pasar por el worker: queda en la nube
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE comprobantes SET recibido_at = now() - interval '13 hours' WHERE id = $1`, espera.ID); err != nil {
		t.Fatal(err)
	}
	a := alertas()
	if a["firma-15"].Nivel != facturacion.Advertencia || a["errores-sri"].Nivel != facturacion.Critica || a["sin-autorizar"].Nivel != facturacion.Advertencia || len(a) != 3 {
		t.Fatalf("alertas: %+v", a)
	}
	// Con plazo legal configurado (DP-07) y 13 h de 15: crítica.
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE parametros_globales SET valor = '15' WHERE clave = 'plazo_envio_comprobante_horas'`); err != nil {
		t.Fatal(err)
	}
	if a := alertas(); a["sin-autorizar"].Nivel != facturacion.Critica {
		t.Fatalf("cerca del plazo: %+v", a["sin-autorizar"])
	}
	_, _ = e.tdb.Admin.Exec(ctx, `UPDATE parametros_globales SET valor = '' WHERE clave = 'plazo_envio_comprobante_horas'`)

	// Correo: las críticas y la firma a 15 días; una sola vez por día.
	antes := len(e.mail.Sent)
	notif := &facturacion.NotificadorAlertas{DB: e.tdb.App, Mail: e.mail, Log: slog.Default(), Now: time.Now}
	enviados, err := notif.Enviar(ctx)
	if err != nil || enviados != 2 {
		t.Fatalf("correos: %d %v", enviados, err)
	}
	asuntos := []string{}
	for _, m := range e.mail.Sent[antes:] {
		asuntos = append(asuntos, m.Subject)
	}
	todo := strings.Join(asuntos, " | ")
	if !strings.Contains(todo, "vence en 15 días") || !strings.Contains(todo, "⚠️ Alerta") || strings.Contains(todo, "sin autorización") {
		t.Fatalf("asuntos: %s", todo)
	}
	if enviados, _ := notif.Enviar(ctx); enviados != 0 {
		t.Fatal("una alerta se envía una vez al día")
	}
	var guardadas int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*) FROM alertas_enviadas WHERE tenant_id = $1`, r.TenantID).Scan(&guardadas)
	if guardadas != 2 {
		t.Fatalf("alertas enviadas: %d", guardadas)
	}
}
