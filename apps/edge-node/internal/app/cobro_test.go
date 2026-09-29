package app

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/cierrez"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// ordenEnMesa: un mesero envía una orden a la mesa y la caja lee su total.
func (c *cajaF4) ordenEnMesa(t *testing.T, tel *telefono, clave string, mesa ids.ID, lineas ...map[string]any) (string, money.Money) {
	t.Helper()
	st, out := tel.enviar(clave, mesa, lineas...)
	if st != 200 {
		t.Fatalf("enviar: %d %v", st, out)
	}
	id := out["orden"].(map[string]any)["id"].(string)
	_, o := c.pos.req("GET", "/v1/ordenes/"+id, nil)
	return id, money.MustParse(o["totales"].(map[string]any)["total"].(string))
}

func (c *cajaF4) cobrar(orden, metodo, recibido, clave string) (int, CobroOut, map[string]any) {
	st, raw := c.pos.req("POST", "/v1/ordenes/"+orden+"/cobrar", map[string]any{
		"cajaId": c.caja1, "metodoId": metodo, "recibido": recibido, "consumidorFinal": true, "idempotencyKey": clave})
	var out CobroOut
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &out)
	return st, out, raw
}

func TestCobroZeroClick(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	var tarjeta string
	if err := c.a.Store.Read().QueryRow(`SELECT id FROM metodos_pago WHERE tipo = 'TARJETA_CREDITO'`).Scan(&tarjeta); err != nil {
		t.Fatal(err)
	}
	efectivo := c.efectivo.String()
	tel := c.emparejar(t, "Tablet de Carlos")
	tel.entrar(c.carlos, pinCarlos)
	orden, total := c.ordenEnMesa(t, tel, "orden-mesa-1", c.mesa1, plato(c.cerveza, "2"), plato(c.ceviche, "1"))
	if total.String() != "20.10" { // 18.50 con IVA 15 % incluido + 10 % de servicio sobre la base (1.609 → 1.60)
		t.Fatalf("total: %s", total)
	}

	// Sin turno no se cobra.
	if st, _, raw := c.cobrar(orden, efectivo, "", "cobro-sin-turno"); st != 409 || raw["code"] != "SIN_TURNO" {
		t.Fatalf("sin turno: %d %v", st, raw)
	}
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "20"})

	// Un mesero editando la mesa la bloquea también para la caja.
	if st, out := tel.req("POST", "/v1/mesas/"+c.mesa1.String()+"/bloqueo", nil); st != 200 {
		t.Fatalf("bloqueo: %d %v", st, out)
	}
	if st, _, raw := c.cobrar(orden, efectivo, "", "cobro-bloqueada"); st != 409 || raw["code"] != "LOCKED_BY" {
		t.Fatalf("mesa bloqueada: %d %v", st, raw)
	}
	tel.req("DELETE", "/v1/mesas/"+c.mesa1.String()+"/bloqueo", nil)

	// Límite de consumidor final (parámetro global): por encima hacen falta datos del comprador.
	param := func(v string) {
		if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
			_, err := tx.Exec(`UPDATE parametros_globales SET valor = ? WHERE clave = 'consumidor_final_maximo'`, v)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	param("20.00")
	if st, _, raw := c.cobrar(orden, efectivo, "", "cobro-limite"); st != 422 || raw["code"] != "CONSUMIDOR_FINAL_EXCEDIDO" || !strings.Contains(raw["detail"].(string), "$20.00") {
		t.Fatalf("límite: %d %v", st, raw)
	}
	param("50.00")
	if st, _, _ := c.cobrar(orden, efectivo, "20.09", "cobro-no-alcanza"); st != 422 {
		t.Fatalf("recibido insuficiente: %d", st)
	}
	if st, raw := c.pos.req("POST", "/v1/ordenes/"+orden+"/cobrar", map[string]any{"cajaId": c.caja1, "metodoId": efectivo, "consumidorFinal": false, "idempotencyKey": "cobro-con-datos"}); st != 422 {
		t.Fatalf("sin consumidor final: %d %v", st, raw)
	}

	// Un toque en «$50»: vuelto, documento, cajón, impresión y mesa libre.
	st, out, raw := c.cobrar(orden, efectivo, "50", "cobro-mesa-1")
	if st != 200 {
		t.Fatalf("cobro: %d %v", st, raw)
	}
	d := out.Documento
	if d.Codigo != "INT-000001" || d.Vuelto != "29.90" || d.Recibido != "50.00" || !d.AbreCajon || d.Totales.Total != "20.10" || d.Comprador != "CONSUMIDOR FINAL" {
		t.Fatalf("documento: %+v", d)
	}
	// Doble toque o reintento: el mismo documento; otra clave: ya está cobrada.
	if st, again, _ := c.cobrar(orden, efectivo, "50", "cobro-mesa-1"); st != 200 || again.Documento.ID != d.ID {
		t.Fatalf("reintento: %d %+v", st, again.Documento)
	}
	if st, _, raw := c.cobrar(orden, efectivo, "50", "cobro-mesa-1-otra"); st != 409 || raw["code"] != "ORDEN_CERRADA" {
		t.Fatalf("doble cobro: %d %v", st, raw)
	}
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool {
			return strings.Contains(s, "[abrir cajón]") && strings.Contains(s, "INT-000001") && strings.Contains(s, "Vuelto")
		})
	})
	_, salon := c.pos.req("GET", "/v1/salon", nil)
	for _, m := range salon["mesas"].([]any) {
		if mm := m.(map[string]any); mm["id"] == c.mesa1.String() && (mm["estado"] != "LIBRE" || mm["ordenId"] != nil) {
			t.Fatalf("la mesa no quedó libre: %v", mm)
		}
	}

	// Tarjeta: el total exacto, sin vuelto ni cajón.
	orden2, total2 := c.ordenEnMesa(t, tel, "orden-mesa-2", c.mesa2, plato(c.ceviche, "1"))
	if st, _, _ := c.cobrar(orden2, tarjeta, "30", "cobro-tarjeta-mal"); st != 422 {
		t.Fatalf("tarjeta con monto distinto: %d", st)
	}
	st, out2, raw := c.cobrar(orden2, tarjeta, "", "cobro-mesa-2")
	if st != 200 || out2.Documento.Codigo != "INT-000002" || out2.Documento.Vuelto != "0.00" || out2.Documento.AbreCajon {
		t.Fatalf("cobro con tarjeta: %d %v", st, raw)
	}

	// Inmutables (QA-10) y en el outbox para la nube, uno por venta.
	for _, q := range []string{`UPDATE documentos_venta SET total = '0'`, `DELETE FROM documentos_venta`} {
		if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error { _, err := tx.Exec(q); return err }); err == nil {
			t.Fatalf("%s no falló", q)
		}
	}
	var eventos int
	_ = c.a.Store.Read().QueryRow(`SELECT count(*) FROM outbox WHERE tipo = ?`, EventoVentaCobrada).Scan(&eventos)
	if eventos != 2 {
		t.Fatalf("eventos de venta: %d", eventos)
	}

	// El Cierre Z cuadra con la suma de los pagos (F4-16): fondo + efectivo y el voucher.
	// Efectivo esperado: 20 + 20.10 = 40.10 = 2×$20 + 10¢.
	st, z, raw := c.cerrar("cierre-cobros", []cierrez.Conteo{{Clave: "B20", Cantidad: 2}, {Clave: "M0.10", Cantidad: 1}},
		map[string]any{"metodoId": tarjeta, "monto": total2.String()})
	if st != 200 || z.Cierre.Resultado != cierrez.Cuadrado {
		t.Fatalf("cierre: %d %v", st, raw)
	}
	for _, l := range z.Cierre.Lineas {
		if l.Tipo == "EFECTIVO" && (l.Cobrado.String() != "20.10" || l.Esperado.String() != "40.10") {
			t.Fatalf("efectivo en el Z: %+v", l)
		}
	}
}

// RNF-05: de «cobrar» a caja libre, p95 ≤ 2 s (aquí sin red real: el trabajo del nodo).
func TestCobroP95(t *testing.T) {
	c := nuevaCaja(t, time.Now())
	tel := c.emparejar(t, "Tablet")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "0"})
	var tiempos []time.Duration
	for i := range 30 {
		orden, _ := c.ordenEnMesa(t, tel, "p95-orden-"+string(rune('a'+i)), c.mesa3, plato(c.cerveza, "1"))
		t0 := time.Now()
		if st, _, raw := c.cobrar(orden, c.efectivo.String(), "20", "p95-cobro-"+string(rune('a'+i))); st != 200 {
			t.Fatalf("cobro %d: %d %v", i, st, raw)
		}
		tiempos = append(tiempos, time.Since(t0))
	}
	sort.Slice(tiempos, func(i, j int) bool { return tiempos[i] < tiempos[j] })
	p95 := tiempos[len(tiempos)*95/100]
	t.Logf("cobro p95 = %s", p95)
	if p95 > 2*time.Second {
		t.Fatalf("p95 = %s", p95)
	}
}

// F4-06: una cuenta pagada con varios métodos ($10 en efectivo + el resto con tarjeta).
func TestPagoMixto(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	var tarjeta string
	if err := c.a.Store.Read().QueryRow(`SELECT id FROM metodos_pago WHERE tipo = 'TARJETA_CREDITO'`).Scan(&tarjeta); err != nil {
		t.Fatal(err)
	}
	efectivo := c.efectivo.String()
	tel := c.emparejar(t, "Tablet")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "0"})
	orden, total := c.ordenEnMesa(t, tel, "orden-mixta", c.mesa1, plato(c.cerveza, "2"), plato(c.ceviche, "1")) // 20.10
	pagar := func(clave string, pagos ...map[string]any) (int, CobroOut, map[string]any) {
		st, raw := c.pos.req("POST", "/v1/ordenes/"+orden+"/cobrar", map[string]any{"cajaId": c.caja1, "consumidorFinal": true, "idempotencyKey": clave, "pagos": pagos})
		var out CobroOut
		b, _ := json.Marshal(raw)
		_ = json.Unmarshal(b, &out)
		return st, out, raw
	}
	ef := func(monto, recibido string) map[string]any {
		return map[string]any{"metodoId": efectivo, "monto": monto, "recibido": recibido}
	}
	tj := func(monto, ultimos4 string) map[string]any {
		return map[string]any{"metodoId": tarjeta, "monto": monto, "ultimos4": ultimos4, "lote": "L-017", "referencia": "123456"}
	}
	for nombre, pagos := range map[string][]map[string]any{
		"no suman el total":       {ef("10", ""), tj("10", "")},
		"se pasan del total":      {ef("10", ""), tj("10.11", "")},
		"dos pagos en efectivo":   {ef("10", ""), ef("10.10", "")},
		"monto en cero":           {ef("20.10", ""), tj("0", "")},
		"últimos 4 inválidos":     {ef("10", ""), tj("10.10", "12a4")},
		"efectivo que no alcanza": {ef("10", "5"), tj("10.10", "")},
		"más de dos decimales":    {ef("10.005", ""), tj("10.105", "")},
	} {
		if st, _, raw := pagar("mixto-malo-"+strings.ReplaceAll(nombre, " ", "-"), pagos...); st != 422 {
			t.Fatalf("%s: %d %v", nombre, st, raw)
		}
	}

	st, out, raw := pagar("mixto-bueno", ef("10", "20"), tj("10.10", "4821"))
	if st != 200 {
		t.Fatalf("pago mixto: %d %v", st, raw)
	}
	d := out.Documento
	if d.Metodo != "Efectivo + Tarjeta crédito" || len(d.Pagos) != 2 || d.Recibido != "20.00" || d.Vuelto != "10.00" || !d.AbreCajon || d.Totales.Total != total.String() {
		t.Fatalf("documento: %+v", d)
	}
	if p := d.Pagos[1]; p.CodigoSRI != "19" || p.Ultimos4 != "4821" || p.Lote != "L-017" || p.Referencia != "123456" || p.Monto != "10.10" {
		t.Fatalf("pago con tarjeta: %+v", p)
	}
	// Dos filas en pagos, con el voucher para el cuadre.
	var n int
	var ultimos string
	_ = c.a.Store.Read().QueryRow(`SELECT count(*), max(coalesce(ultimos4, '')) FROM pagos WHERE documento_id = ?`, d.ID.String()).Scan(&n, &ultimos)
	if n != 2 || ultimos != "4821" {
		t.Fatalf("pagos guardados: %d %q", n, ultimos)
	}
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool { return strings.Contains(s, "Tarjeta crédito ****4821") })
	})

	// El Cierre Z separa lo cobrado por método: $10 en efectivo y $10.10 en tarjeta.
	st, z, raw := c.cerrar("cierre-mixto", []cierrez.Conteo{{Clave: "B10", Cantidad: 1}}, map[string]any{"metodoId": tarjeta, "monto": "10.10"})
	if st != 200 || z.Cierre.Resultado != cierrez.Cuadrado {
		t.Fatalf("cierre: %d %v", st, raw)
	}
}
