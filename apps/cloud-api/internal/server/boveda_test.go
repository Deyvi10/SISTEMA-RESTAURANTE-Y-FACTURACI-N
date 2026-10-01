package server_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/facturacion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/pdf/pdfprueba"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// descargar pide un archivo y devuelve su cuerpo y su Content-Disposition.
func (c *cliente) descargar(path string, want int) ([]byte, string) {
	c.e.t.Helper()
	req, _ := http.NewRequest("GET", c.e.srv.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		c.e.t.Fatalf("GET %s = %d: %s", path, res.StatusCode, b)
	}
	return b, res.Header.Get("Content-Disposition")
}

// F5-12: la bóveda encuentra cualquier factura, la descarga, la reenvía y reintenta lo pendiente.
func TestBovedaDeComprobantes(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "boveda@w.ec")
	n := e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	var cfg facturacion.Config
	c.do("PUT", "/v1/cajas/"+cajas[0].ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 200, &cfg)
	punto := *cfg.Cajas[0].PuntoID
	maria := func(d *sri.DatosFactura) {
		d.Comprador = sri.Comprador{TipoIdentificacion: "05", Identificacion: "1710034065", RazonSocial: "María Pérez"}
		d.Adicionales = []sri.CampoAdicional{{Nombre: "Email", Valor: "maria@example.com"}}
	}
	// Sin firma todavía: espera en la nube; así se ve el «por qué» en la lista.
	espera := n.comprobanteCon("1790011674001", punto, 1, false, maria)
	w := e.worker("fiscal@boveda")
	drenar(t, w)
	var l facturacion.Lista
	c.do("GET", "/v1/comprobantes", nil, 200, &l)
	if len(l.Filas) != 1 || l.Filas[0].Grupo != "ENVIADO" || l.Filas[0].Explicacion == nil || !strings.Contains(l.Filas[0].Explicacion.Que, "firma") || l.Pendientes["ENVIADO"] != 1 {
		t.Fatalf("pendiente sin firma: %+v", l)
	}
	// Antes de autorizar: RIDE «pendiente» sí, XML autorizado no.
	pdfB, disp := c.descargar("/v1/comprobantes/"+espera.ID.String()+"/pdf", 200)
	if !bytes.HasPrefix(pdfB, []byte("%PDF")) || !strings.Contains(disp, "FACTURA-001-001-000000001.pdf") {
		t.Fatalf("pdf pendiente: %s", disp)
	}
	if txt := pdfprueba.Texto(t, pdfB); txt != "" && !strings.Contains(txt, "Pendiente de autorización") {
		t.Fatal("el RIDE pendiente lo dice")
	}
	c.descargar("/v1/comprobantes/"+espera.ID.String()+"/xml", 409)

	ahora := time.Now()
	c.subirP12(p12De(t, "1790011674001", ahora.Add(-time.Hour), ahora.AddDate(1, 0, 0)), claveP12, 201)
	n.comprobante("1790011674001", punto, 2, false)
	devuelto := n.comprobanteCon("1790011674001", punto, 901, false, maria) // DEVUELTA por estructura
	alterado := n.comprobante("1790011674001", punto, 3, true)
	drenar(t, w)

	// Filtros: estado, texto (nombre, cédula, número), fechas, tipo y punto.
	lista := func(q url.Values) facturacion.Lista {
		var out facturacion.Lista
		c.do("GET", "/v1/comprobantes?"+q.Encode(), nil, 200, &out)
		return out
	}
	if l := lista(url.Values{"estado": {"AUTORIZADO"}}); len(l.Filas) != 2 {
		t.Fatalf("autorizados: %d", len(l.Filas))
	}
	l = lista(url.Values{"estado": {"REQUIERE_ATENCION"}})
	if len(l.Filas) != 2 || l.Pendientes["REQUIERE_ATENCION"] != 2 {
		t.Fatalf("requieren atención: %+v", l)
	}
	for _, f := range l.Filas {
		if f.Explicacion == nil || f.Explicacion.Que == "" || f.Explicacion.Accion == "" {
			t.Fatalf("sin explicación en lenguaje claro: %+v", f)
		}
	}
	for _, q := range []string{"maría", "17100", "000000901", "001-001-000000901"} {
		if l := lista(url.Values{"q": {q}}); len(l.Filas) == 0 || l.Filas[0].ID != devuelto.ID && l.Filas[len(l.Filas)-1].ID != devuelto.ID {
			t.Errorf("buscar %q: %+v", q, l.Filas)
		}
	}
	if l := lista(url.Values{"q": {devuelto.ClaveAcceso}}); len(l.Filas) != 1 {
		t.Fatal("por clave de acceso")
	}
	if l := lista(url.Values{"desde": {"2026-09-30"}}); len(l.Filas) != 0 {
		t.Fatal("filtro de fecha")
	}
	if l := lista(url.Values{"hasta": {"2026-09-29"}, "tipo": {"01"}, "punto": {punto.String()}}); len(l.Filas) != 4 {
		t.Fatalf("fecha + tipo + punto: %d", len(l.Filas))
	}
	c.do("GET", "/v1/comprobantes?desde=ayer", nil, 422, nil)
	c.do("GET", "/v1/comprobantes?estado=RARO", nil, 422, nil)

	// Detalle con historia y XML autorizado.
	var autorizado facturacion.FilaComprobante
	for _, f := range lista(url.Values{"estado": {"AUTORIZADO"}}).Filas {
		if f.Numero == "001-001-000000001" {
			autorizado = f
		}
	}
	var d facturacion.Detalle
	c.do("GET", "/v1/comprobantes/"+autorizado.ID.String(), nil, 200, &d)
	if len(d.Eventos) < 4 || d.Correo == nil || *d.Correo != "maria@example.com" {
		t.Fatalf("detalle: %+v", d)
	}
	xmlB, disp := c.descargar("/v1/comprobantes/"+autorizado.ID.String()+"/xml", 200)
	if !bytes.Contains(xmlB, []byte("<estado>AUTORIZADO</estado>")) || !strings.Contains(disp, "FACTURA-001-001-000000001.xml") {
		t.Fatalf("xml: %s", disp)
	}
	c.descargar("/v1/comprobantes/"+alterado.ID.String()+"/pdf", 409)

	// Reenviar: al comprador o a otro correo; con un correo malo, error de campo.
	antes := len(e.mail.Sent)
	c.do("POST", "/v1/comprobantes/"+autorizado.ID.String()+"/reenviar", map[string]string{}, 204, nil)
	c.do("POST", "/v1/comprobantes/"+autorizado.ID.String()+"/reenviar", map[string]string{"correo": "contador@example.com"}, 204, nil)
	c.do("POST", "/v1/comprobantes/"+autorizado.ID.String()+"/reenviar", map[string]string{"correo": "a@b.ec, c@d.ec"}, 422, nil)
	if len(e.mail.Sent) != antes+2 || e.mail.Sent[len(e.mail.Sent)-1].To != "contador@example.com" {
		t.Fatalf("reenvíos: %d", len(e.mail.Sent)-antes)
	}
	c.do("POST", "/v1/comprobantes/"+devuelto.ID.String()+"/reenviar", map[string]string{}, 409, nil) // aún no autorizado

	// Reintentar: lo alterado no se envía nunca; lo devuelto se vuelve a firmar y enviar.
	c.do("POST", "/v1/comprobantes/"+alterado.ID.String()+"/reintentar", map[string]string{}, 409, nil)
	c.do("POST", "/v1/comprobantes/"+autorizado.ID.String()+"/reintentar", map[string]string{}, 409, nil)
	var fila facturacion.FilaComprobante
	c.do("POST", "/v1/comprobantes/"+devuelto.ID.String()+"/reintentar", map[string]string{}, 200, &fila)
	if fila.Estado != "EN_NUBE" {
		t.Fatalf("reintento: %+v", fila)
	}
	var firmado *string
	_ = e.tdb.Admin.QueryRow(context.Background(), `SELECT xml_firmado FROM comprobantes WHERE id = $1`, devuelto.ID).Scan(&firmado)
	if firmado != nil {
		t.Fatal("se vuelve a firmar con la firma vigente")
	}
	drenar(t, w)
	var accesos, auditados int
	_ = e.tdb.Admin.QueryRow(context.Background(), `SELECT count(*) FROM certificados_accesos WHERE comprobante_id = $1`, devuelto.ID).Scan(&accesos)
	_ = e.tdb.Admin.QueryRow(context.Background(), `SELECT count(*) FROM auditoria WHERE tenant_id = $1 AND accion = 'COMPROBANTE_REINTENTADO'`, r.TenantID).Scan(&auditados)
	if accesos != 2 || auditados != 1 {
		t.Fatalf("re-firma: %d accesos, %d auditados", accesos, auditados)
	}
	// Lo autorizado no vuelve atrás ni siquiera como dueño de la base.
	if _, err := e.tdb.Admin.Exec(context.Background(), `UPDATE comprobantes SET estado = 'EN_NUBE' WHERE id = $1`, autorizado.ID); err == nil {
		t.Fatal("un autorizado no retrocede")
	}

	// Paginación estable.
	for i := range 5 {
		n.comprobante("1790011674001", punto, int64(10+i), false)
	}
	var vistos []string
	paginas := 0
	cursor := ""
	for range 10 {
		paginas++
		q := url.Values{}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var pag facturacion.Lista
		c.do("GET", "/v1/comprobantes?"+q.Encode()+"&limite=3", nil, 200, &pag)
		for _, f := range pag.Filas {
			vistos = append(vistos, f.Numero)
		}
		if pag.Siguiente == nil {
			break
		}
		cursor = *pag.Siguiente
	}
	unicos := map[string]bool{}
	for _, v := range vistos {
		unicos[v] = true
	}
	if len(vistos) != 9 || len(unicos) != 9 || paginas != 3 {
		t.Fatalf("paginación: %d páginas, %v", paginas, vistos)
	}

	// Otro restaurante no ve nada.
	otro, _ := e.restaurante("0992339411001", "boveda@b.ec")
	var vacia facturacion.Lista
	otro.do("GET", "/v1/comprobantes", nil, 200, &vacia)
	otro.do("GET", "/v1/comprobantes/"+autorizado.ID.String(), nil, 404, nil)
	otro.descargar("/v1/comprobantes/"+autorizado.ID.String()+"/xml", 404)
	if len(vacia.Filas) != 0 {
		t.Fatal("RLS")
	}
}
