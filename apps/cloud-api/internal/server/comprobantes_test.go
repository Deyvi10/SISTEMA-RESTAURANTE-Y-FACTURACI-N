package server_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/facturacion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// comprobante envía a la nube una factura como la emite el nodo al cobrar (F5-05).
func (n *nodoSim) comprobante(ruc string, punto ids.ID, sec int64, alterarHash bool) facturacion.ComprobanteEmitido {
	n.e.t.Helper()
	iva := sri.Tarifa{Porcentaje: "15", Codigo: "4"}
	f, err := sri.Desglosar(sri.Venta{
		Lineas:     []sri.LineaVenta{{Codigo: "CEV", Descripcion: "Ceviche", Cantidad: decimal.NewFromInt(1), Bruto: money.MustParse("15.00"), Final: money.MustParse("15.00"), Tarifa: iva}},
		PorTarifa:  map[string]sri.TotalTarifa{"15": {Base: money.MustParse("13.04"), IVA: money.MustParse("1.96")}},
		IncluyeIVA: true, Total: money.MustParse("15.00"),
	})
	if err != nil {
		n.e.t.Fatal(err)
	}
	fecha := time.Date(2026, 9, 29, 13, 0, 0, 0, clock.Guayaquil)
	clave, err := sri.NuevaClaveAcceso(sri.ClaveAccesoInput{FechaEmision: fecha, TipoComprobante: "01", RUC: ruc, Ambiente: 1,
		Establecimiento: "001", PuntoEmision: "001", Secuencial: sec})
	if err != nil {
		n.e.t.Fatal(err)
	}
	doc, err := sri.FacturaXML(sri.DatosFactura{Ambiente: 1, ClaveAcceso: clave, Secuencial: sec, Fecha: fecha,
		Emisor:    sri.Emisor{RUC: ruc, RazonSocial: "Emisor", DirMatriz: "Quito", Establecimiento: "001", PuntoEmision: "001"},
		Comprador: sri.Comprador{TipoIdentificacion: "07", Identificacion: "9999999999999", RazonSocial: "CONSUMIDOR FINAL"}}, f)
	if err != nil {
		n.e.t.Fatal(err)
	}
	suma := sha256.Sum256(doc)
	c := facturacion.ComprobanteEmitido{ID: ids.New(), DocumentoID: ids.New(), Tipo: "01", Ambiente: 1, PuntoEmisionID: punto, Serie: "001001",
		Secuencial: sec, ClaveAcceso: clave.String(), FechaEmision: "2026-09-29", ImporteTotal: "15.00", XML: string(doc), Hash: hex.EncodeToString(suma[:])}
	if alterarHash {
		c.Hash = hex.EncodeToString(make([]byte, 32))
	}
	n.enviarComprobante(c)
	return c
}

func (n *nodoSim) enviarComprobante(c facturacion.ComprobanteEmitido) {
	n.e.t.Helper()
	ev := n.evento(facturacion.EventoComprobanteEmitido)
	ev.Payload, _ = json.Marshal(c)
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: []edgesync.Event{ev}}, true, 200, nil)
}

// F5-05: la nube recibe los comprobantes del nodo sin alterar su contenido.
func TestNubeRecibeComprobantes(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "h@h.ec")
	n := e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	var cfg facturacion.Config
	c.do("PUT", "/v1/cajas/"+cajas[0].ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 200, &cfg)
	punto := *cfg.Cajas[0].PuntoID

	uno := n.comprobante("1790011674001", punto, 7, false)
	n.enviarComprobante(uno) // el nodo reintenta el mismo evento: no se duplica
	// Mismo secuencial con otra clave (dos nodos numerando): no se guarda ni detiene la sync.
	n.comprobante("1790011674001", punto, 7, false)
	alterado := n.comprobante("1790011674001", punto, 8, true)

	ctx := context.Background()
	estado := func(id ids.ID) (string, bool) {
		var est string
		var valido bool
		if err := e.tdb.Admin.QueryRow(ctx, `SELECT estado, hash_valido FROM comprobantes WHERE id = $1`, id).Scan(&est, &valido); err != nil {
			t.Fatal(err)
		}
		return est, valido
	}
	if est, ok := estado(uno.ID); est != "EN_NUBE" || !ok {
		t.Fatalf("comprobante sano: %s %v", est, ok)
	}
	if est, ok := estado(alterado.ID); est != "REQUIERE_ATENCION" || ok {
		t.Fatalf("hash alterado: %s %v", est, ok)
	}
	var total, eventos int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*), (SELECT count(*) FROM comprobante_eventos WHERE tenant_id = $1) FROM comprobantes WHERE tenant_id = $1`, r.TenantID).Scan(&total, &eventos)
	if total != 2 || eventos != 2 {
		t.Fatalf("comprobantes %d, eventos %d", total, eventos)
	}
	// La nube recuerda el último secuencial del punto (un nodo que reemplace a este sigue desde ahí).
	var ultimos string
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT ultimos_secuenciales::text FROM puntos_emision WHERE id = $1`, punto).Scan(&ultimos)
	if ultimos != `{"01-1": 8}` {
		t.Fatalf("últimos secuenciales: %s", ultimos)
	}
	// El contenido tributario no se modifica, ni siquiera como dueño de la base.
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE comprobantes SET importe_total = 1 WHERE id = $1`, uno.ID); err == nil {
		t.Fatal("el importe de un comprobante no se puede cambiar")
	}
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE comprobantes SET estado = 'FIRMADO' WHERE id = $1`, uno.ID); err != nil {
		t.Fatalf("el estado sí avanza: %v", err)
	}
	if _, err := e.tdb.Admin.Exec(ctx, `DELETE FROM comprobantes WHERE id = $1`, uno.ID); err == nil {
		t.Fatal("un comprobante no se borra")
	}
}
