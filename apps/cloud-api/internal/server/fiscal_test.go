package server_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/facturacion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/fiscal"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/sriws"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/tools/sri-stub/stub"
)

// worker fiscal contra el stub del SRI (uno por prueba, compartido por sus workers), sin
// esperas entre pasos y con reintentos lejanos (un error transitorio deja el comprobante
// quieto hasta la hora siguiente).
func (e *env) worker(proceso string) *fiscal.Worker {
	e.t.Helper()
	if e.sri == nil {
		e.sri = httptest.NewServer(stub.NewStub(time.Millisecond, slog.Default()).Routes())
		e.t.Cleanup(e.sri.Close)
	}
	return &fiscal.Worker{DB: e.tdb.App, KEK: e.kek, Host: e.sri.URL, Now: time.Now, Log: slog.Default(), Proceso: proceso,
		Politica: sriws.Politica{Reintentos: []time.Duration{time.Hour}, Alerta: 24 * time.Hour}}
}

func drenar(t *testing.T, w *fiscal.Worker) {
	t.Helper()
	for range 10 {
		n, err := w.Procesar(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return
		}
	}
}

type estadoComprobante struct {
	estado, xmlFirmado, numero string
	intentos                   int
	error                      *string
}

func (e *env) estadoDe(id ids.ID) estadoComprobante {
	e.t.Helper()
	var s estadoComprobante
	var firmado, numero *string
	if err := e.tdb.Admin.QueryRow(context.Background(), `SELECT estado, xml_firmado, numero_autorizacion, intentos, ultimo_error FROM comprobantes WHERE id = $1`, id).
		Scan(&s.estado, &firmado, &numero, &s.intentos, &s.error); err != nil {
		e.t.Fatal(err)
	}
	if firmado != nil {
		s.xmlFirmado = *firmado
	}
	if numero != nil {
		s.numero = *numero
	}
	return s
}

// ADR-0006 / F5-07 / F5-10: el worker firma con el certificado descifrado en memoria y lleva
// cada comprobante hasta la respuesta del SRI; lo que falla por la red espera y reintenta.
func TestWorkerFiscalHastaAutorizado(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "w@w.ec")
	n := e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	var cfg facturacion.Config
	c.do("PUT", "/v1/cajas/"+cajas[0].ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 200, &cfg)
	punto := *cfg.Cajas[0].PuntoID
	w := e.worker("fiscal@prueba")

	// Sin firma electrónica el comprobante espera en la nube, con el motivo a la vista.
	ok := n.comprobante("1790011674001", punto, 1, false)
	drenar(t, w)
	if s := e.estadoDe(ok.ID); s.estado != fiscal.EnNube || s.error == nil || !strings.Contains(*s.error, "firma electrónica") {
		t.Fatalf("sin firma: %+v", s)
	}

	// Con la firma, lo pendiente sale solo.
	ahora := time.Now()
	c.subirP12(p12De(t, "1790011674001", ahora.Add(-time.Hour), ahora.AddDate(1, 0, 0)), claveP12, 201)
	red := n.comprobante("1790011674001", punto, 903, false)       // el SRI responde 503
	rechazado := n.comprobante("1790011674001", punto, 906, false) // NO AUTORIZADO
	devuelto := n.comprobante("1790011674001", punto, 901, false)  // DEVUELTA por estructura
	alterado := n.comprobante("1790011674001", punto, 2, true)     // hash que no coincide
	drenar(t, w)

	s := e.estadoDe(ok.ID)
	if s.estado != fiscal.Autorizado || s.numero != ok.ClaveAcceso || s.error != nil {
		t.Fatalf("autorizado: %+v", s)
	}
	// Lo firmado es el XML del nodo, intacto, con la firma XAdES dentro de la raíz.
	sinFirma := strings.TrimSuffix(strings.TrimSpace(ok.XML), "</factura>")
	if !strings.HasPrefix(s.xmlFirmado, sinFirma) || !strings.Contains(s.xmlFirmado, "<ds:Signature") || !strings.Contains(s.xmlFirmado, "<etsi:SigningTime>") {
		t.Fatalf("firmado:\n%s", s.xmlFirmado)
	}
	if s := e.estadoDe(red.ID); s.estado != fiscal.Firmado || s.intentos != 1 || s.error == nil {
		t.Fatalf("503: %+v", s)
	}
	if s := e.estadoDe(rechazado.ID); s.estado != fiscal.NoAutorizado || s.numero != "" {
		t.Fatalf("no autorizado: %+v", s)
	}
	if s := e.estadoDe(devuelto.ID); s.estado != fiscal.Devuelto {
		t.Fatalf("devuelto: %+v", s)
	}
	if s := e.estadoDe(alterado.ID); s.estado != "REQUIERE_ATENCION" || s.xmlFirmado != "" {
		t.Fatalf("un hash alterado no se firma: %+v", s)
	}

	ctx := context.Background()
	// Cada paso quedó en la historia del comprobante.
	var pasos string
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT string_agg(estado, ' → ' ORDER BY created_at) FROM comprobante_eventos WHERE comprobante_id = $1`, ok.ID).Scan(&pasos)
	if pasos != "EN_NUBE → FIRMADO → RECIBIDO → AUTORIZADO" {
		t.Fatalf("historia: %s", pasos)
	}
	var mensajes string
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT mensajes_sri::text FROM comprobantes WHERE id = $1`, devuelto.ID).Scan(&mensajes)
	if !strings.Contains(mensajes, "ARCHIVO NO CUMPLE ESTRUCTURA XML") {
		t.Fatalf("mensajes del SRI: %s", mensajes)
	}
	// Un descifrado por comprobante firmado, cada uno registrado con su proceso.
	var accesos, firmados int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*) FILTER (WHERE ok AND proceso = 'fiscal@prueba'), (SELECT count(*) FROM comprobantes WHERE tenant_id = $1 AND xml_firmado IS NOT NULL)
		FROM certificados_accesos WHERE tenant_id = $1`, r.TenantID).Scan(&accesos, &firmados)
	if accesos != 4 || firmados != 4 {
		t.Fatalf("accesos %d, firmados %d", accesos, firmados)
	}
	// F5-11: cada estado vuelve al nodo por la réplica, sin el XML; el volcado trae el último.
	estados := map[string][]string{}
	var autorizado map[string]any
	for _, cam := range n.pull(1, 0).Cambios {
		if cam.Tabla != "estados_comprobante" {
			continue
		}
		var d map[string]any
		_ = json.Unmarshal(cam.Datos, &d)
		if _, hay := d["xml"]; hay {
			t.Fatal("el estado no lleva el XML")
		}
		estados[d["clave_acceso"].(string)] = append(estados[d["clave_acceso"].(string)], d["estado"].(string))
		if d["estado"] == "AUTORIZADO" {
			autorizado = d
		}
	}
	if got := strings.Join(estados[ok.ClaveAcceso], " → "); got != "EN_NUBE → FIRMADO → RECIBIDO → AUTORIZADO" {
		t.Fatalf("estados al nodo: %s", got)
	}
	if autorizado["numero_autorizacion"] != ok.ClaveAcceso || autorizado["fecha_autorizacion"] == nil {
		t.Fatalf("autorizado al nodo: %v", autorizado)
	}
	if got := estados[devuelto.ClaveAcceso]; got[len(got)-1] != "DEVUELTO" {
		t.Fatalf("devuelto al nodo: %v", got)
	}
	ultimo := map[string]string{}
	for _, cam := range n.pull(0, 0).Cambios {
		if cam.Tabla == "estados_comprobante" {
			var d map[string]any
			_ = json.Unmarshal(cam.Datos, &d)
			ultimo[d["clave_acceso"].(string)] = d["estado"].(string)
		}
	}
	if len(ultimo) != 5 || ultimo[ok.ClaveAcceso] != "AUTORIZADO" || ultimo[alterado.ClaveAcceso] != "REQUIERE_ATENCION" {
		t.Fatalf("volcado de estados: %v", ultimo)
	}

	// Lo firmado y autorizado ya no cambia.
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE comprobantes SET xml_firmado = 'x' WHERE id = $1`, ok.ID); err == nil {
		t.Fatal("el XML firmado no se reescribe")
	}
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE comprobantes SET numero_autorizacion = NULL WHERE id = $1`, ok.ID); err == nil {
		t.Fatal("la autorización no se borra")
	}

	// El reintento tras el 503 manda exactamente el mismo documento firmado.
	antes := e.estadoDe(red.ID).xmlFirmado
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE comprobantes SET proximo_intento_at = now() - interval '1 second' WHERE id = $1`, red.ID); err != nil {
		t.Fatal(err)
	}
	drenar(t, w)
	if s := e.estadoDe(red.ID); s.xmlFirmado != antes || s.intentos != 2 {
		t.Fatalf("reintento: %+v", s)
	}

	// Con la factura de prueba autorizada ya se puede pasar a producción.
	datos := map[string]any{"ambiente": 2, "razonSocial": "Distribuidora", "direccionMatriz": "Quito", "regimen": "GENERAL", "facturacionActiva": false}
	c.do("PUT", "/v1/facturacion", datos, 201, &cfg)
	if cfg.Ambiente != 2 || !cfg.PruebaAprobada || cfg.Certificado == nil || len(cfg.Pendientes) != 0 || len(cfg.Avisos) != 0 {
		t.Fatalf("producción: %+v", cfg)
	}
}

// Sin la prueba autorizada, producción sigue cerrada aunque haya firma.
func TestProduccionEsperaUnaFacturaAutorizada(t *testing.T) {
	e := newEnv(t)
	c, _ := e.restaurante("1790011674001", "prod@a.ec")
	ahora := time.Now()
	c.subirP12(p12De(t, "1790011674001", ahora.Add(-time.Hour), ahora.AddDate(1, 0, 0)), claveP12, 201)
	datos := map[string]any{"ambiente": 2, "razonSocial": "Distribuidora", "direccionMatriz": "Quito", "regimen": "GENERAL", "facturacionActiva": false}
	var p map[string]any
	c.do("PUT", "/v1/facturacion", datos, 409, &p)
	if p["code"] != "PRODUCCION_SIN_FIRMA" {
		t.Fatalf("%v", p)
	}
}

// Dos workers a la vez (réplicas del contenedor) no firman ni envían dos veces lo mismo.
func TestDosWorkersNoRepitenTrabajo(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "dos@w.ec")
	n := e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	var cfg facturacion.Config
	c.do("PUT", "/v1/cajas/"+cajas[0].ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 200, &cfg)
	ahora := time.Now()
	c.subirP12(p12De(t, "1790011674001", ahora.Add(-time.Hour), ahora.AddDate(1, 0, 0)), claveP12, 201)
	const total = 12
	for i := range total {
		n.comprobante("1790011674001", *cfg.Cajas[0].PuntoID, int64(10+i), false)
	}
	a, b := e.worker("fiscal@a"), e.worker("fiscal@b")
	var wg sync.WaitGroup
	for _, w := range []*fiscal.Worker{a, b} {
		wg.Go(func() { drenar(t, w) })
	}
	wg.Wait()
	drenar(t, a)
	ctx := context.Background()
	var autorizados, accesos, eventosFirmado int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*) FILTER (WHERE estado = 'AUTORIZADO'),
			(SELECT count(*) FROM certificados_accesos WHERE tenant_id = $1),
			(SELECT count(*) FROM comprobante_eventos WHERE tenant_id = $1 AND estado = 'FIRMADO')
		FROM comprobantes WHERE tenant_id = $1`, r.TenantID).Scan(&autorizados, &accesos, &eventosFirmado)
	if autorizados != total || accesos != total || eventosFirmado != total {
		t.Fatalf("autorizados %d, descifrados %d, firmados %d de %d", autorizados, accesos, eventosFirmado, total)
	}
}
