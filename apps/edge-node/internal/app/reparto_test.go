package app

import (
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// totalesDeOrden reproduce el cálculo de la orden (precios con IVA incluido y servicio 10 %).
func totalesDeOrden(lineas []lineaReparto) (map[string]TarifaTotales, money.Money, money.Money) {
	monto := map[string]money.Money{}
	for _, l := range lineas {
		monto[l.Pct] = monto[l.Pct].Add(l.Final)
	}
	out := map[string]TarifaTotales{}
	base := money.Money{}
	for t, m := range monto {
		f := decimal.RequireFromString(t).Div(decimal.NewFromInt(100))
		b := money.FromDecimal(m.Decimal().Div(decimal.NewFromInt(1).Add(f))).Round2()
		out[t] = TarifaTotales{Base: b, IVA: m.Sub(b)}
		base = base.Add(b)
	}
	propina := base.Mul(decimal.RequireFromString("0.10")).Round2()
	total := propina
	for _, v := range out {
		total = total.Add(v.Base).Add(v.IVA)
	}
	return out, propina, total
}

// Propiedad (F4-08, QA-05): en 1 000 escenarios al azar —con cuentas que se pagan a medio
// camino y el resto que se reagrupa— la suma de las cuentas es el total de la orden al
// centavo, nada es negativo y cada cuenta cumple base + IVA = lo que consume.
func TestRepartoEnCuentasCuadraAlCentavo(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 5))
	for escenario := range 1000 {
		var lineas []lineaReparto
		for range 1 + r.IntN(7) {
			lineas = append(lineas, lineaReparto{ID: ids.New(), Pct: []string{"15", "0"}[r.IntN(3)/2], Final: money.FromCents(1 + r.Int64N(4000))})
		}
		orden, propina, total := totalesDeOrden(lineas)
		var cuentas []cuentaReparto
		for n := range 2 + r.IntN(5) {
			cuentas = append(cuentas, cuentaReparto{ID: ids.New(), Numero: n + 1})
		}
		asignar := func() {
			for i := range cuentas {
				if !cuentas[i].Pagada {
					cuentas[i].Pesos = map[ids.ID]int64{}
				}
			}
			var abiertas []int
			for i, c := range cuentas {
				if !c.Pagada {
					abiertas = append(abiertas, i)
				}
			}
			for _, l := range lineas {
				// Cada plato en al menos una cuenta abierta, a veces en varias (fracción).
				for k, i := range abiertas {
					if k == r.IntN(len(abiertas)) || r.IntN(3) == 0 {
						cuentas[i].Pesos[l.ID] = 1 + r.Int64N(3)
					}
				}
				if !algunaTiene(cuentas, abiertas, l.ID) {
					cuentas[abiertas[len(abiertas)-1]].Pesos[l.ID] = 1
				}
			}
		}
		asignar()
		// Se pagan algunas cuentas (queda al menos una abierta) y se reagrupa lo que queda.
		for pagos := r.IntN(len(cuentas)); pagos > 0; pagos-- {
			res, err := repartirEnCuentas(lineas, orden, propina, true, cuentas)
			if err != nil {
				t.Fatalf("escenario %d: %v", escenario, err)
			}
			i := r.IntN(len(cuentas))
			if cuentas[i].Pagada || contarAbiertas(cuentas) == 1 {
				continue
			}
			for _, c := range res {
				if c.ID == cuentas[i].ID {
					cuentas[i].Pagada, cuentas[i].PagadoLineas, cuentas[i].PagadoTarifas, cuentas[i].PagadoPropina = true, c.Lineas, c.PorTarifa, c.Propina
				}
			}
			asignar()
		}
		res, err := repartirEnCuentas(lineas, orden, propina, true, cuentas)
		if err != nil {
			t.Fatalf("escenario %d: %v", escenario, err)
		}
		suma := money.Money{}
		for _, c := range res {
			consumo := money.Money{}
			for _, v := range c.Lineas {
				if v.IsNegative() {
					t.Fatalf("escenario %d: plato negativo", escenario)
				}
				consumo = consumo.Add(v)
			}
			if c.Subtotal.IsNegative() || c.IVA.IsNegative() || c.Propina.IsNegative() {
				t.Fatalf("escenario %d: cuenta negativa %+v", escenario, c)
			}
			if !c.Subtotal.Add(c.IVA).Equal(consumo) {
				t.Fatalf("escenario %d: base + IVA %s ≠ consumo %s", escenario, c.Subtotal.Add(c.IVA), consumo)
			}
			suma = suma.Add(c.Total)
		}
		if !suma.Equal(total) {
			t.Fatalf("escenario %d: cuentas %s ≠ orden %s", escenario, suma, total)
		}
	}
}

func algunaTiene(cs []cuentaReparto, idx []int, l ids.ID) bool {
	for _, i := range idx {
		if cs[i].Pesos[l] > 0 {
			return true
		}
	}
	return false
}

func contarAbiertas(cs []cuentaReparto) (n int) {
	for _, c := range cs {
		if !c.Pagada {
			n++
		}
	}
	return
}

// Partes iguales: $10.00 entre 3 → 3.33, 3.33, 3.34 (residuo en la última) y la pizza de
// $12.50 entre 3 con el ajuste de centavos también en la última.
func TestPartesIgualesResiduoEnLaUltima(t *testing.T) {
	pizza := lineaReparto{ID: ids.New(), Pct: "0", Final: money.MustParse("10.00")}
	orden, _, _ := totalesDeOrden([]lineaReparto{pizza})
	var cs []cuentaReparto
	for n := range 3 {
		cs = append(cs, cuentaReparto{ID: ids.New(), Numero: n + 1, Pesos: map[ids.ID]int64{pizza.ID: 1}})
	}
	res, err := repartirEnCuentas([]lineaReparto{pizza}, orden, money.Money{}, true, cs)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range res {
		got = append(got, c.Total.String())
	}
	if got[0] != "3.33" || got[1] != "3.33" || got[2] != "3.34" {
		t.Fatalf("partes iguales: %v", got)
	}
	// Un plato sin cuenta se informa.
	otra := lineaReparto{ID: ids.New(), Pct: "0", Final: money.MustParse("2")}
	_, err = repartirEnCuentas([]lineaReparto{pizza, otra}, orden, money.Money{}, true, cs)
	var sc errSinCuenta
	if !errors.As(err, &sc) || len(sc.lineas) != 1 || sc.lineas[0] != otra.ID {
		t.Fatalf("sin cuenta: %v", err)
	}
}

func TestPartesIgualesDeVariosPlatosCuadranPorCuenta(t *testing.T) {
	lineas := []lineaReparto{
		{ID: ids.New(), Pct: "15", Final: money.MustParse("12.50")},
		{ID: ids.New(), Pct: "15", Final: money.MustParse("10.00")},
		{ID: ids.New(), Pct: "15", Final: money.MustParse("1.50")},
	}
	var cuentas []cuentaReparto
	for n := 1; n <= 3; n++ {
		p := map[ids.ID]int64{}
		for _, l := range lineas {
			p[l.ID] = 1
		}
		cuentas = append(cuentas, cuentaReparto{ID: ids.New(), Numero: n, Pesos: p})
	}
	orden := map[string]TarifaTotales{"15": {Base: money.MustParse("20.87"), IVA: money.MustParse("3.13")}}
	out, err := repartirEnCuentas(lineas, orden, money.Money{}, true, cuentas)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range out {
		if c.Total.String() != "8.00" {
			t.Fatalf("cuenta %d: %s, se esperaba 8.00", c.Numero, c.Total)
		}
	}
	for _, l := range lineas {
		suma := money.Money{}
		for _, c := range out {
			if c.Lineas[l.ID].IsNegative() {
				t.Fatalf("parte negativa en %s", l.ID)
			}
			suma = suma.Add(c.Lineas[l.ID])
		}
		if !suma.Equal(l.Final) {
			t.Fatalf("el plato suma %s y vale %s", suma, l.Final)
		}
	}
}

func TestPartesIgualesConServicioNoAcumulanCentavos(t *testing.T) {
	// Pasta 10.50 + café 1.75 = 12.25 con IVA incluido y 10 % de servicio: 13.32 entre 3.
	lineas := []lineaReparto{
		{ID: ids.New(), Pct: "15", Final: money.MustParse("10.50")},
		{ID: ids.New(), Pct: "15", Final: money.MustParse("1.75")},
	}
	var cuentas []cuentaReparto
	for n := 1; n <= 3; n++ {
		cuentas = append(cuentas, cuentaReparto{ID: ids.New(), Numero: n, Pesos: map[ids.ID]int64{lineas[0].ID: 1, lineas[1].ID: 1}})
	}
	orden := map[string]TarifaTotales{"15": {Base: money.MustParse("10.65"), IVA: money.MustParse("1.60")}}
	out, err := repartirEnCuentas(lineas, orden, money.MustParse("1.07"), true, cuentas)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range out {
		if c.Total.String() != "4.44" {
			t.Fatalf("cuenta %d: %s (servicio %s), se esperaba 4.44", c.Numero, c.Total, c.Propina)
		}
		consumo := money.Money{}
		for _, v := range c.Lineas {
			consumo = consumo.Add(v)
		}
		if !consumo.Equal(c.Subtotal.Add(c.IVA)) {
			t.Fatalf("cuenta %d: los platos suman %s y base + IVA %s", c.Numero, consumo, c.Subtotal.Add(c.IVA))
		}
	}
}
