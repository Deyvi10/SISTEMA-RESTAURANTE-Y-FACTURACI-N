package server_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/facturacion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// notaCredito envía a la nube una NC como la emite el nodo (F5-13) sobre una factura ya subida.
func (n *nodoSim) notaCredito(ruc string, punto ids.ID, sec int64, factura facturacion.ComprobanteEmitido, todo bool) facturacion.ComprobanteEmitido {
	n.e.t.Helper()
	f, err := sri.LeerFactura([]byte(factura.XML))
	if err != nil {
		n.e.t.Fatal(err)
	}
	nc, err := sri.CalcularNC(f, nil, nil, true)
	if err != nil {
		n.e.t.Fatal(err)
	}
	fecha := time.Date(2026, 9, 30, 10, 0, 0, 0, clock.Guayaquil)
	clave, err := sri.NuevaClaveAcceso(sri.ClaveAccesoInput{FechaEmision: fecha, TipoComprobante: "04", RUC: ruc, Ambiente: 1,
		Establecimiento: "001", PuntoEmision: "001", Secuencial: sec})
	if err != nil {
		n.e.t.Fatal(err)
	}
	doc, err := sri.NotaCreditoXML(sri.DatosNC{Ambiente: 1, ClaveAcceso: clave, Secuencial: sec, Fecha: fecha,
		Emisor:    sri.Emisor{RUC: ruc, RazonSocial: "Emisor", DirMatriz: "Quito", Establecimiento: "001", PuntoEmision: "001"},
		Comprador: sri.Comprador{TipoIdentificacion: "05", Identificacion: "1710034065", RazonSocial: "María Pérez"},
		Sustento:  f, Motivo: "Mesa cancelada", Adicionales: []sri.CampoAdicional{{Nombre: "Email", Valor: "maria@example.com"}}}, nc)
	if err != nil {
		n.e.t.Fatal(err)
	}
	suma := sha256.Sum256(doc)
	sustento := factura.ID
	c := facturacion.ComprobanteEmitido{ID: ids.New(), Tipo: "04", Ambiente: 1, PuntoEmisionID: punto, Serie: "001001", Secuencial: sec,
		ClaveAcceso: clave.String(), FechaEmision: "2026-09-30", ImporteTotal: nc.ValorModificacion.String(), XML: string(doc),
		Hash: hex.EncodeToString(suma[:]), SustentoID: &sustento, SustentoClave: factura.ClaveAcceso, Total: todo}
	n.enviarComprobante(c)
	return c
}

// F5-13: la NC espera a que su factura se autorice, se firma y se envía; al autorizarse la que
// revierte todo, la factura queda «anulada por NC»; el comprador recibe la NC por correo.
func TestNotaDeCreditoEnLaNube(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "nc@w.ec")
	n := e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	var cfg facturacion.Config
	c.do("PUT", "/v1/cajas/"+cajas[0].ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 200, &cfg)
	punto := *cfg.Cajas[0].PuntoID
	// La NC llega junto con su factura; no puede salir antes que ella.
	factura := n.comprobante("1790011674001", punto, 7, false)
	nc := n.notaCredito("1790011674001", punto, 1, factura, true)
	ctx := context.Background()
	estado := func(id ids.ID) string {
		var s string
		_ = e.tdb.Admin.QueryRow(ctx, `SELECT estado FROM comprobantes WHERE id = $1`, id).Scan(&s)
		return s
	}
	if estado(nc.ID) != "EN_NUBE" {
		t.Fatal("la NC se registra en la nube")
	}
	ahora := time.Now()
	c.subirP12(p12De(t, "1790011674001", ahora.Add(-time.Hour), ahora.AddDate(1, 0, 0)), claveP12, 201)
	w := e.worker("fiscal@nc")
	// Primera pasada: la factura se firma y el SRI la recibe; la NC todavía espera.
	if _, err := w.Procesar(ctx); err != nil {
		t.Fatal(err)
	}
	if estado(factura.ID) != "RECIBIDO" || estado(nc.ID) != "EN_NUBE" {
		t.Fatalf("la NC espera a su factura: factura %s, nc %s", estado(factura.ID), estado(nc.ID))
	}
	var l facturacion.Lista
	c.do("GET", "/v1/comprobantes?"+url.Values{"q": {"000000001"}}.Encode(), nil, 200, &l)
	if len(l.Filas) != 1 || l.Filas[0].Tipo != "04" || l.Filas[0].Sustento == nil || *l.Filas[0].Sustento != "001-001-000000007" ||
		l.Filas[0].Explicacion == nil || !strings.Contains(l.Filas[0].Explicacion.Que, "001-001-000000007") {
		t.Fatalf("NC en la bóveda: %+v", l.Filas)
	}
	// La factura se autoriza y la NC sale detrás.
	drenar(t, w)
	if estado(nc.ID) != "AUTORIZADO" || estado(factura.ID) != "ANULADO" {
		t.Fatalf("nc %s, factura %s", estado(nc.ID), estado(factura.ID))
	}
	var pasos string
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT string_agg(estado, ' → ' ORDER BY created_at) FROM comprobante_eventos WHERE comprobante_id = $1`, factura.ID).Scan(&pasos)
	if !strings.HasSuffix(pasos, "AUTORIZADO → ANULADO") {
		t.Fatalf("historia de la factura: %s", pasos)
	}
	c.do("GET", "/v1/comprobantes?estado=ANULADO", nil, 200, &l)
	if len(l.Filas) != 1 || l.Filas[0].ID != factura.ID {
		t.Fatalf("anulada por NC: %+v", l.Filas)
	}
	// Al nodo le llega que su factura quedó anulada.
	var anuladoAlNodo bool
	for _, cam := range n.pull(1, 0).Cambios {
		var d map[string]any
		_ = json.Unmarshal(cam.Datos, &d)
		if cam.Tabla == "estados_comprobante" && d["id"] == factura.ID.String() && d["estado"] == "ANULADO" {
			anuladoAlNodo = true
		}
	}
	if !anuladoAlNodo {
		t.Fatal("el nodo no supo que la factura quedó anulada")
	}
	// Correo de la NC al cliente, con su RIDE y XML.
	correos := &facturacion.Correos{DB: e.tdb.App, Mail: e.mail, Log: slog.Default()}
	if n, err := correos.EnviarPendientes(ctx); err != nil || n != 1 {
		t.Fatalf("correo de la NC: %d %v", n, err)
	}
	msg, _ := e.mail.Last()
	if !strings.Contains(msg.Subject, "nota de crédito 001-001-000000001") || msg.Attachments[0].Name != "NOTA-CREDITO-001-001-000000001.pdf" ||
		!strings.Contains(msg.Text, "001-001-000000007") {
		t.Fatalf("correo: %s %v", msg.Subject, msg.Attachments[0].Name)
	}
	// Una NC de una factura que la nube no tiene no se registra (ni detiene la sincronización).
	desconocida := factura
	desconocida.ID = ids.New()
	if huerfana := n.notaCredito("1790011674001", punto, 2, desconocida, false); estado(huerfana.ID) != "" {
		t.Fatal("una NC sin su factura en la nube no se registra")
	}
	var notas int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*) FROM comprobantes WHERE tenant_id = $1 AND tipo = '04'`, r.TenantID).Scan(&notas)
	if notas != 1 {
		t.Fatalf("notas: %d", notas)
	}
}
