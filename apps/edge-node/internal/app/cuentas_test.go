package app

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

type divisionT struct {
	Cuentas   []CuentaVista `json:"cuentas"`
	SinCuenta []string      `json:"sinCuenta"`
	Total     string        `json:"total"`
}

func (c *cajaF4) dividir(orden string, cuentas ...[]AsignacionIn) (int, divisionT, map[string]any) {
	in := DivisionIn{}
	for _, as := range cuentas {
		in.Cuentas = append(in.Cuentas, CuentaIn{Asignaciones: as})
	}
	st, raw := c.pos.req("PUT", "/v1/ordenes/"+orden+"/cuentas", in)
	var out divisionT
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &out)
	return st, out, raw
}

func (c *cajaF4) cobrarCuenta(orden, cuenta, clave string) (int, CobroOut, map[string]any) {
	st, raw := c.pos.req("POST", "/v1/ordenes/"+orden+"/cobrar", map[string]any{"cajaId": c.caja1, "metodoId": c.efectivo, "consumidorFinal": true, "cuentaId": cuenta, "idempotencyKey": clave})
	var out CobroOut
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &out)
	return st, out, raw
}

func sumar(cs []CuentaVista) money.Money {
	t := money.Money{}
	for _, c := range cs {
		t = t.Add(money.MustParse(c.Total))
	}
	return t
}

// F4-08: una mesa de amigos que paga por separado: la cerveza de cada uno, el ceviche
// compartido entre tres, cobro por cuenta y la orden que se cierra con la última.
func TestDivisionDeCuenta(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	tel := c.emparejar(t, "Tablet")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "0"})
	orden, total := c.ordenEnMesa(t, tel, "orden-amigos", c.mesa1, plato(c.cerveza, "1"), plato(c.cerveza, "1"), plato(c.ceviche, "1"))
	_, o := c.pos.req("GET", "/v1/ordenes/"+orden, nil)
	ls := o["orden"].(map[string]any)["lineas"].([]any)
	id := func(i int) string { return ls[i].(map[string]any)["id"].(string) }
	as := func(pares ...any) []AsignacionIn {
		var out []AsignacionIn
		for i := 0; i < len(pares); i += 2 {
			out = append(out, AsignacionIn{LineaID: mustID(pares[i].(string)), Peso: int64(pares[i+1].(int))})
		}
		return out
	}

	// Un plato sin cuenta no deja confirmar; la división es atómica.
	if st, _, raw := c.dividir(orden, as(id(0), 1), as(id(1), 1)); st != 422 || raw["code"] != "PLATOS_SIN_CUENTA" || !strings.Contains(raw["detail"].(string), "Ceviche") {
		t.Fatalf("sin cuenta: %d %v", st, raw)
	}
	if _, d, _ := c.dividir(orden); len(d.Cuentas) != 0 {
		t.Fatal("un intento fallido dejó cuentas")
	}

	// Tres cuentas: cada uno su cerveza y el ceviche entre los tres.
	st, d, raw := c.dividir(orden, as(id(0), 1, id(2), 1), as(id(1), 1, id(2), 1), as(id(2), 1))
	if st != 200 || len(d.Cuentas) != 3 || !sumar(d.Cuentas).Equal(total) {
		t.Fatalf("división: %d %v", st, raw)
	}
	// El ceviche de 12.50 entre tres: 4.16, 4.17, 4.17 (el ajuste a las últimas).
	var partes []string
	for _, cu := range d.Cuentas {
		partes = append(partes, cu.Lineas[mustID(id(2))])
	}
	if !slices.Equal(partes, []string{"4.16", "4.17", "4.17"}) {
		t.Fatalf("ceviche entre tres: %v", partes)
	}

	// Sin elegir cuenta no se cobra; se cobra la 1 y la orden sigue abierta.
	if st, _, raw := c.cobrar(orden, c.efectivo.String(), "", "cobro-sin-cuenta"); st != 409 || raw["code"] != "ORDEN_DIVIDIDA" {
		t.Fatalf("sin cuenta elegida: %d %v", st, raw)
	}
	st, uno, raw := c.cobrarCuenta(orden, d.Cuentas[0].ID.String(), "cobro-cuenta-1")
	if st != 200 || uno.Cerrada || uno.Documento.Cuenta != 1 || uno.Documento.Totales.Total != d.Cuentas[0].Total {
		t.Fatalf("cuenta 1: %d %v", st, raw)
	}
	if st, _, raw := c.cobrarCuenta(orden, d.Cuentas[0].ID.String(), "cobro-cuenta-1-otra"); st != 409 || raw["code"] != "CUENTA_PAGADA" {
		t.Fatalf("cobrar dos veces: %d %v", st, raw)
	}
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool { return strings.Contains(s, "1/3 Ceviche") && strings.Contains(s, "$4.16") })
	})

	// Con una cuenta pagada: no se tocan sus platos ni lo de toda la cuenta; se reagrupa el resto.
	if st, out := c.pos.req("POST", "/v1/ordenes/"+orden+"/propina", map[string]any{"retirar": true}); st != 409 || out["code"] != "CUENTAS_PAGADAS" {
		t.Fatalf("servicio con cuentas pagadas: %d %v", st, out)
	}
	if st, _, raw := c.dividir(orden, as(id(1), 1, id(2), 2)); st != 200 {
		t.Fatalf("reagrupar: %d %v", st, raw)
	}
	_, d, _ = c.dividir(orden, as(id(1), 1, id(2), 1)) // lo que queda en una sola cuenta
	if len(d.Cuentas) != 2 || d.Cuentas[0].Estado != "PAGADA" || !sumar(d.Cuentas).Equal(total) {
		t.Fatalf("tras reagrupar: %+v", d.Cuentas)
	}

	// La última cuenta cierra la orden y libera la mesa; todo cuadra con el total.
	st, dos, raw := c.cobrarCuenta(orden, d.Cuentas[1].ID.String(), "cobro-cuenta-2")
	if st != 200 || !dos.Cerrada {
		t.Fatalf("última cuenta: %d %v", st, raw)
	}
	cobrado := money.MustParse(uno.Documento.Totales.Total).Add(money.MustParse(dos.Documento.Totales.Total))
	if !cobrado.Equal(total) {
		t.Fatalf("cobrado %s ≠ total %s", cobrado, total)
	}
	_, salon := c.pos.req("GET", "/v1/salon", nil)
	for _, m := range salon["mesas"].([]any) {
		if mm := m.(map[string]any); mm["id"] == c.mesa1.String() && mm["estado"] != "LIBRE" {
			t.Fatalf("mesa: %v", mm)
		}
	}
	var pagos int
	_ = c.a.Store.Read().QueryRow(`SELECT count(DISTINCT cuenta_id) FROM pagos WHERE orden_id = ?`, orden).Scan(&pagos)
	if pagos != 2 {
		t.Fatalf("pagos por cuenta: %d", pagos)
	}
}

func mustID(s string) ids.ID { return ids.MustParse(s) }
