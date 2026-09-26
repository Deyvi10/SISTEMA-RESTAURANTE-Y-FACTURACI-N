package app

import "testing"

func TestLineasCuentaJuntaElMismoPlato(t *testing.T) {
	hb := []ModLinea{{Nombre: "Término medio"}}
	got := lineasCuenta([]LineaOrden{
		{Producto: "Empanadas de verde", Cantidad: "2", Total: "9.00", Estado: "ENVIADA"},
		{Producto: "Hamburguesa", Cantidad: "1", Total: "10.25", Modificadores: hb, Estado: "ENVIADA"},
		{Producto: "Empanadas de verde", Cantidad: "1", Total: "4.50", Estado: "ENVIADA"},
		{Producto: "Hamburguesa", Cantidad: "1", Total: "9.50", Estado: "ENVIADA"},        // sin modificador
		{Producto: "Empanadas de verde", Cantidad: "1", Total: "5.00", Estado: "ENVIADA"}, // otro precio
		{Producto: "Alitas", Cantidad: "1", Total: "7.50", Estado: "ANULADA"},
	})
	want := []struct{ cant, prod, total string }{
		{"3", "Empanadas de verde", "13.50"},
		{"1", "Hamburguesa + Término medio", "10.25"},
		{"1", "Hamburguesa", "9.50"},
		{"1", "Empanadas de verde", "5.00"},
	}
	if len(got) != len(want) {
		t.Fatalf("líneas = %d, quiero %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Cantidad != w.cant || got[i].Producto != w.prod || got[i].Total.String() != w.total {
			t.Errorf("línea %d = %s %s %s, quiero %s %s %s", i, got[i].Cantidad, got[i].Producto, got[i].Total, w.cant, w.prod, w.total)
		}
	}
}
