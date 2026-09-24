package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

func clave(t *testing.T, secuencial int64) string {
	t.Helper()
	c, err := sri.NuevaClaveAcceso(sri.ClaveAccesoInput{
		FechaEmision: time.Date(2026, 9, 24, 0, 0, 0, 0, clock.Guayaquil), TipoComprobante: sri.TipoFactura,
		RUC: "1790011674001", Ambiente: sri.AmbientePruebas, Establecimiento: "001", PuntoEmision: "002", Secuencial: secuencial,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c.String()
}

func post(t *testing.T, url, body string) (int, string) {
	t.Helper()
	res, err := http.Post(url, "text/xml", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func enviar(t *testing.T, base, c string) string {
	xmlComp := fmt.Sprintf(`<factura id="comprobante"><infoTributaria><claveAcceso>%s</claveAcceso></infoTributaria></factura>`, c)
	body := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ec="http://ec.gob.sri.ws.recepcion"><soapenv:Body><ec:validarComprobante><xml>` +
		base64.StdEncoding.EncodeToString([]byte(xmlComp)) + `</xml></ec:validarComprobante></soapenv:Body></soapenv:Envelope>`
	_, out := post(t, base+rutaRecepcion, body)
	return out
}

func consultar(t *testing.T, base, c string) string {
	_, out := post(t, base+rutaAutorizacion, `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body><ec:autorizacionComprobante xmlns:ec="http://ec.gob.sri.ws.autorizacion"><claveAccesoComprobante>`+c+`</claveAccesoComprobante></ec:autorizacionComprobante></soapenv:Body></soapenv:Envelope>`)
	return elemento([]byte(out), "estado")
}

func server(t *testing.T) string {
	srv := httptest.NewServer(NewStub(50*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes())
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestFlujoAutorizado(t *testing.T) {
	base := server(t)
	c := clave(t, 123)
	if got := elemento([]byte(enviar(t, base, c)), "estado"); got != "RECIBIDA" {
		t.Fatalf("recepción: %s", got)
	}
	if got := consultar(t, base, c); got != "AUTORIZADO" {
		t.Fatalf("autorización: %s", got)
	}
	// Reenviar la misma clave nunca crea otro comprobante: el SRI responde «clave registrada».
	if out := enviar(t, base, c); elemento([]byte(out), "identificador") != "43" {
		t.Fatalf("reenvío: %s", out)
	}
}

func TestEscenarios(t *testing.T) {
	base := server(t)
	casos := []struct {
		sec           int64
		recepcion, id string
		autorizacion  []string
	}{
		{901, "DEVUELTA", "35", []string{""}},
		{904, "RECIBIDA", "", []string{"EN PROCESO", "EN PROCESO", "AUTORIZADO"}},
		{905, "DEVUELTA", "43", []string{"AUTORIZADO"}},
		{906, "RECIBIDA", "", []string{"NO AUTORIZADO"}},
		{907, "DEVUELTA", "70", []string{"AUTORIZADO"}},
	}
	for _, cs := range casos {
		c := clave(t, cs.sec)
		out := []byte(enviar(t, base, c))
		if got := elemento(out, "estado"); got != cs.recepcion {
			t.Errorf("%d recepción = %s, se esperaba %s", cs.sec, got, cs.recepcion)
		}
		if got := elemento(out, "identificador"); got != cs.id {
			t.Errorf("%d identificador = %s, se esperaba %s", cs.sec, got, cs.id)
		}
		for i, want := range cs.autorizacion {
			if got := consultar(t, base, c); got != want {
				t.Errorf("%d consulta %d = %q, se esperaba %q", cs.sec, i+1, got, want)
			}
		}
	}
}

func TestErroresTransitorios(t *testing.T) {
	base := server(t)
	if code, _ := post(t, base+rutaRecepcion, `<x><xml>`+base64.StdEncoding.EncodeToString([]byte(`<f><claveAcceso>`+clave(t, 903)+`</claveAcceso></f>`))+`</xml></x>`); code != 503 {
		t.Errorf("903 debería responder 503, respondió %d", code)
	}
	client := &http.Client{Timeout: 20 * time.Millisecond}
	res, err := client.Post(base+rutaRecepcion, "text/xml", strings.NewReader(`<x><xml>`+base64.StdEncoding.EncodeToString([]byte(`<f><claveAcceso>`+clave(t, 902)+`</claveAcceso></f>`))+`</xml></x>`))
	if err == nil {
		_ = res.Body.Close()
		t.Error("902 debería provocar timeout en el cliente")
	}
}

func TestRechazaBasura(t *testing.T) {
	base := server(t)
	for _, body := range []string{`<x><xml>no-es-base64</xml></x>`, `<x/>`, `<x><xml>` + base64.StdEncoding.EncodeToString([]byte(`<f><claveAcceso>123</claveAcceso></f>`)) + `</xml></x>`} {
		_, out := post(t, base+rutaRecepcion, body)
		if elemento([]byte(out), "estado") != "DEVUELTA" {
			t.Errorf("%s debería ser DEVUELTA: %s", body, out)
		}
	}
	if got := consultar(t, base, clave(t, 1)); got != "" {
		t.Errorf("una clave desconocida no tiene autorizaciones: %q", got)
	}
}
