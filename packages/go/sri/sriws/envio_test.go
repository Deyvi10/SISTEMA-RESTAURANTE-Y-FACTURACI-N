package sriws

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/tools/sri-stub/stub"
)

// factura arma una factura real (desglose + XML 1.1.0) con el secuencial que elige el
// escenario del stub (901…908 en los 3 últimos dígitos).
func factura(t *testing.T, secuencial int64) (sri.ClaveAcceso, []byte) {
	t.Helper()
	iva := sri.Tarifa{Porcentaje: "15", Codigo: "4"}
	v := sri.Venta{
		Lineas:     []sri.LineaVenta{{Codigo: "CEV", Descripcion: "Ceviche", Cantidad: decimal.NewFromInt(1), Bruto: money.MustParse("15.00"), Final: money.MustParse("15.00"), Tarifa: iva}},
		PorTarifa:  map[string]sri.TotalTarifa{"15": {Base: money.MustParse("13.04"), IVA: money.MustParse("1.96")}},
		IncluyeIVA: true, Total: money.MustParse("15.00"),
	}
	f, err := sri.Desglosar(v)
	if err != nil {
		t.Fatal(err)
	}
	fecha := time.Date(2026, 9, 29, 13, 0, 0, 0, clock.Guayaquil)
	clave, err := sri.NuevaClaveAcceso(sri.ClaveAccesoInput{FechaEmision: fecha, TipoComprobante: sri.TipoFactura, RUC: "1710034065001",
		Ambiente: sri.AmbientePruebas, Establecimiento: "001", PuntoEmision: "002", Secuencial: secuencial, CodigoNumerico: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := sri.FacturaXML(sri.DatosFactura{Ambiente: sri.AmbientePruebas, ClaveAcceso: clave, Secuencial: secuencial, Fecha: fecha,
		Emisor:    sri.Emisor{RUC: "1710034065001", RazonSocial: "PÉREZ JOSÉ", DirMatriz: "Guayaquil", Establecimiento: "001", PuntoEmision: "002"},
		Comprador: sri.Comprador{TipoIdentificacion: "07", Identificacion: "9999999999999", RazonSocial: "CONSUMIDOR FINAL"},
		Pagos:     []sri.Pago{{FormaPago: "01", Total: f.ImporteTotal}}}, f)
	if err != nil {
		t.Fatal(err)
	}
	return clave, doc
}

func contraStub(t *testing.T) *Cliente {
	t.Helper()
	srv := httptest.NewServer(stub.NewStub(300*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes())
	t.Cleanup(srv.Close)
	return &Cliente{Host: srv.URL, HTTP: &http.Client{Timeout: 150 * time.Millisecond}}
}

// recorrer avanza hasta una fase final (o hasta 10 pasos), saltando las esperas.
func recorrer(t *testing.T, c *Cliente, clave sri.ClaveAcceso, doc []byte) Envio {
	t.Helper()
	e := Envio{Clave: clave}
	ahora := time.Date(2026, 9, 29, 13, 0, 0, 0, time.UTC)
	for range 10 {
		if e.Fase.Final() {
			break
		}
		e = c.Avanzar(context.Background(), e, doc, ahora, PoliticaPorDefecto)
		ahora = e.Proximo
	}
	return e
}

func TestFlujoAutorizado(t *testing.T) {
	c := contraStub(t)
	clave, doc := factura(t, 67)
	e := recorrer(t, c, clave, doc)
	if e.Fase != Autorizada || e.NumeroAutorizacion != clave.String() || e.FechaAutorizacion.IsZero() || !strings.Contains(e.XMLAutorizado, clave.String()) {
		t.Fatalf("envío: %+v", e)
	}
}

func TestEscenariosDelSRI(t *testing.T) {
	c := contraStub(t)
	for sec, quiero := range map[int64]struct {
		fase    Fase
		mensaje string
	}{
		901: {Devuelto, "35"},  // error de estructura
		904: {Autorizada, ""},  // EN PROCESO dos veces y luego autorizado
		905: {Autorizada, ""},  // «clave registrada»: ya lo tenía, se consulta
		906: {Rechazada, "56"}, // NO AUTORIZADO
		907: {Autorizada, ""},  // «en procesamiento»: se consulta
		908: {Rechazada, "56"}, // RECHAZADO, como el ejemplo oficial
	} {
		clave, doc := factura(t, 1000+sec)
		e := recorrer(t, c, clave, doc)
		if e.Fase != quiero.fase {
			t.Errorf("%d: fase %s, se esperaba %s (%+v)", sec, e.Fase, quiero.fase, e)
			continue
		}
		if quiero.mensaje != "" && (len(e.Mensajes) == 0 || e.Mensajes[0].Identificador != quiero.mensaje) {
			t.Errorf("%d: mensajes %+v, se esperaba el %s", sec, e.Mensajes, quiero.mensaje)
		}
	}
}

// Timeout y 503: no se pierde nada, se reintenta con la misma clave y espera creciente.
func TestErroresTransitoriosReintentan(t *testing.T) {
	c := contraStub(t)
	ahora := time.Date(2026, 9, 29, 13, 0, 0, 0, time.UTC)
	for _, sec := range []int64{1902, 1903} {
		clave, doc := factura(t, sec)
		e := Envio{Clave: clave}
		var esperas []time.Duration
		for range 3 {
			e = c.Avanzar(context.Background(), e, doc, ahora, PoliticaPorDefecto)
			esperas = append(esperas, e.Proximo.Sub(ahora))
		}
		if e.Fase != PorEnviar || e.Intentos != 3 || e.UltimoError == "" {
			t.Fatalf("%d: %+v", sec, e)
		}
		if esperas[0] != 5*time.Second || esperas[1] != 15*time.Second || esperas[2] != time.Minute {
			t.Fatalf("%d: esperas %v", sec, esperas)
		}
	}
}

// La respuesta de recepción se perdió: al reenviar, el SRI dice «clave registrada» (43) y el
// envío sigue a la consulta en vez de quedar como error.
func TestRespuestaPerdidaNoDuplica(t *testing.T) {
	c := contraStub(t)
	clave, doc := factura(t, 77)
	if _, err := c.Enviar(context.Background(), doc); err != nil { // llegó, pero «se perdió»
		t.Fatal(err)
	}
	e := recorrer(t, c, clave, doc)
	if e.Fase != Autorizada {
		t.Fatalf("reenvío tras respuesta perdida: %+v", e)
	}
}

// Las respuestas literales de la ficha técnica v2.34 (§7.2.3): con saltos de línea dentro
// de los valores y el estado RECHAZADO del ejemplo de no autorizado.
func TestRespuestasDeLaFichaOficial(t *testing.T) {
	const devuelta = `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
<soap:Body>
<ns2:validarComprobanteResponse xmlns:ns2="http://ec.gob.sri.ws.recepcion">
<RespuestaRecepcionComprobante>
<estado>DEVUELTA</estado>
<comprobantes><comprobante>
<claveAcceso>1702201205176001321000110010030001000011234567816</claveAcceso>
<mensajes><mensaje>
<identificador>35</identificador>
<mensaje>DOCUMENTO INVÁLIDO</mensaje>
<informacionAdicional>Se encontró el siguiente error en la estructura del comprobante: cvc-
complex-type.2.4.a: Invalid content was found starting with element 'totalSinImpuestos'.</informacionAdicional>
<tipo>ERROR</tipo>
</mensaje></mensajes>
</comprobante></comprobantes>
</RespuestaRecepcionComprobante>
</ns2:validarComprobanteResponse>
</soap:Body>
</soap:Envelope>`
	const rechazado = `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
<soap:Body>
<ns2:autorizacionComprobanteResponse
xmlns:ns2="http://ec.gob.sri.ws.autorizacion">
<RespuestaAutorizacionComprobante>
<claveAccesoConsultada>
1302201201176001321000120010030000050431234567814
</claveAccesoConsultada>
<numeroComprobantes>1</numeroComprobantes>
<autorizaciones><autorizacion>
<estado>RECHAZADO</estado>
<fechaAutorizacion>2012-02-13T16:34:48.997-05:00</fechaAutorizacion>
<ambiente>PRUEBAS</ambiente>
<comprobante><![CDATA[<?xml version="1.0" encoding="UTF-8"?><factura id="comprobante" version="1.0.0"></factura>]]></comprobante>
<mensajes><mensaje>
<identificador>46</identificador>
<mensaje> RUC no existe </mensaje>
<tipo>ERROR</tipo>
</mensaje></mensajes>
</autorizacion></autorizaciones>
</RespuestaAutorizacionComprobante>
</ns2:autorizacionComprobanteResponse>
</soap:Body>
</soap:Envelope>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cuerpo, _ := io.ReadAll(r.Body)
		// La petición lleva la operación en su namespace y los hijos sin namespace (WSDL).
		switch {
		case strings.Contains(string(cuerpo), `<ec:validarComprobante xmlns:ec="http://ec.gob.sri.ws.recepcion"><xml>`):
			_, _ = io.WriteString(w, devuelta)
		case strings.Contains(string(cuerpo), `<ec:autorizacionComprobante xmlns:ec="http://ec.gob.sri.ws.autorizacion"><claveAccesoComprobante>`):
			_, _ = io.WriteString(w, rechazado)
		default:
			http.Error(w, "petición con otra forma: "+string(cuerpo), http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	c := &Cliente{Host: srv.URL, HTTP: srv.Client()}
	r, err := c.Enviar(context.Background(), []byte("<factura/>"))
	if err != nil || r.Estado != Devuelta || !r.Tiene("35") || !strings.HasPrefix(r.Mensajes[0].InformacionAdicional, "Se encontró") {
		t.Fatalf("recepción: %+v %v", r, err)
	}
	a, err := c.Consultar(context.Background(), sri.ClaveAcceso("1302201201176001321000120010030000050431234567814"))
	if err != nil || !a.Hay || a.Estado != Rechazado || a.Mensajes[0].Mensaje != "RUC no existe" || a.FechaAutorizacion.IsZero() || !strings.Contains(a.Comprobante, "<factura") {
		t.Fatalf("autorización: %+v %v", a, err)
	}
}

func TestComprobanteDemasiadoGrande(t *testing.T) {
	c := &Cliente{Host: "http://127.0.0.1:1", HTTP: http.DefaultClient}
	_, err := c.Enviar(context.Background(), make([]byte, MaxComprobante+1))
	if err == nil || errors.Is(err, ErrTransitorio) {
		t.Fatalf("un comprobante de más de 320 KB no se reintenta: %v", err)
	}
}

func TestAlertaTrasVeinticuatroHoras(t *testing.T) {
	recibido := time.Date(2026, 9, 29, 13, 0, 0, 0, time.UTC)
	e := Envio{Fase: Recibido, RecibidoAt: recibido}
	if e.Retrasado(recibido.Add(23*time.Hour), PoliticaPorDefecto) || !e.Retrasado(recibido.Add(25*time.Hour), PoliticaPorDefecto) {
		t.Fatal("la alerta fiscal salta pasadas 24 h sin respuesta")
	}
}
