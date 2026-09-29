package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// F4-10: el 10 % de servicio sobre la base sin IVA; el cajero lo retira si el cliente lo
// rechaza (auditado) y la propina cobrada queda registrada por orden y mesero.
func TestPropinaLegal(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	tel := c.emparejar(t, "Tablet")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "0"})
	totales := func(orden string) map[string]any {
		_, o := c.pos.req("GET", "/v1/ordenes/"+orden, nil)
		return o["totales"].(map[string]any)
	}
	propina := func(orden string, retirar bool, motivo string) (int, map[string]any) {
		return c.pos.req("POST", "/v1/ordenes/"+orden+"/propina", map[string]any{"retirar": retirar, "motivo": motivo})
	}
	auditorias := func(accion string) (n int) {
		_ = c.a.Store.Read().QueryRow(`SELECT count(*) FROM auditoria WHERE accion = ?`, accion).Scan(&n)
		return
	}

	// 18.50 con IVA 15 % incluido → base 16.09, IVA 2.41; servicio 10 % de la base = 1.609 → 1.60
	// (hacia abajo: no puede superar el 10 % del subtotal, Tabla 21 de la ficha del SRI).
	orden, _ := c.ordenEnMesa(t, tel, "orden-propina", c.mesa1, plato(c.cerveza, "2"), plato(c.ceviche, "1"))
	if tt := totales(orden); tt["subtotal"] != "16.09" || tt["iva"] != "2.41" || tt["propina"] != "1.60" || tt["total"] != "20.10" || tt["propinaActiva"] != true {
		t.Fatalf("totales: %v", tt)
	}

	// El cliente la rechaza: se retira, auditado; repetir no audita dos veces; se puede reponer.
	if st, tt := propina(orden, true, "El cliente no quiere pagar servicio"); st != 200 || tt["propina"] != "0.00" || tt["total"] != "18.50" || tt["propinaRetirada"] != true {
		t.Fatalf("retirar: %d %v", st, tt)
	}
	propina(orden, true, "")
	if n := auditorias("PROPINA_RETIRADA"); n != 1 {
		t.Fatalf("auditorías de retiro: %d", n)
	}
	var detalle string
	_ = c.a.Store.Read().QueryRow(`SELECT detalle FROM auditoria WHERE accion = 'PROPINA_RETIRADA'`).Scan(&detalle)
	if detalle == "" || !strings.Contains(detalle, "1.60") || !strings.Contains(detalle, "no quiere pagar servicio") {
		t.Fatalf("detalle de la auditoría: %s", detalle)
	}
	if st, tt := propina(orden, false, ""); st != 200 || tt["total"] != "20.10" || auditorias("PROPINA_REPUESTA") != 1 {
		t.Fatalf("reponer: %d %v", st, tt)
	}
	propina(orden, true, "")

	// Se cobra sin servicio: el documento no lo lleva y no hay propina que repartir.
	st, out, raw := c.cobrar(orden, c.efectivo.String(), "", "cobro-sin-propina")
	if st != 200 || out.Documento.Totales.Propina != "0.00" || out.Documento.Totales.Total != "18.50" {
		t.Fatalf("cobro sin propina: %d %v", st, raw)
	}
	var n int
	_ = c.a.Store.Read().QueryRow(`SELECT count(*) FROM propinas`).Scan(&n)
	if n != 0 {
		t.Fatalf("propinas: %d", n)
	}

	// Con servicio: queda por orden y mesero (Carlos tomó la orden aunque cobre la caja).
	orden2, _ := c.ordenEnMesa(t, tel, "orden-con-propina", c.mesa2, plato(c.cerveza, "2"), plato(c.ceviche, "1"))
	c.cobrar(orden2, c.efectivo.String(), "", "cobro-con-propina")
	var mesero, monto, fecha string
	if err := c.a.Store.Read().QueryRow(`SELECT mesero_nombre, monto, fecha_negocio FROM propinas WHERE orden_id = ?`, orden2).Scan(&mesero, &monto, &fecha); err != nil {
		t.Fatal(err)
	}
	if mesero != "Carlos M." || monto != "1.60" || fecha != "2026-09-25" {
		t.Fatalf("propina registrada: %s %s %s", mesero, monto, fecha)
	}
	if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error { _, err := tx.Exec(`UPDATE propinas SET monto = '0'`); return err }); err == nil {
		t.Fatal("propinas se pudo modificar")
	}

	// Si el local no cobra servicio no hay nada que retirar.
	if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error { _, err := tx.Exec(`UPDATE locales SET propina_legal_activa = 0`); return err }); err != nil {
		t.Fatal(err)
	}
	orden3, _ := c.ordenEnMesa(t, tel, "orden-sin-servicio", c.mesa3, plato(c.cerveza, "1"))
	if st, out := propina(orden3, true, ""); st != 409 || out["code"] != "SIN_SERVICIO" {
		t.Fatalf("sin servicio: %d %v", st, out)
	}
}

// El servicio es por atender en el local: no se cobra en Para llevar.
func TestSinServicioParaLlevar(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	for tipo, conServicio := range map[string]bool{"LLEVAR": false, "BARRA": true} {
		orden := ids.New()
		st, out := c.pos.req("POST", "/v1/ordenes/enviar", map[string]any{"idempotencyKey": "srv-" + tipo, "ordenId": orden, "tipo": tipo, "lineas": []any{plato(c.ceviche, "1")}})
		if st != 200 {
			t.Fatalf("%s: %d %v", tipo, st, out)
		}
		_, o := c.pos.req("GET", "/v1/ordenes/"+orden.String(), nil)
		tt := o["totales"].(map[string]any)
		if (tt["propina"] != "0.00") != conServicio || tt["propinaActiva"] != conServicio {
			t.Fatalf("%s: %v", tipo, tt)
		}
	}
}
