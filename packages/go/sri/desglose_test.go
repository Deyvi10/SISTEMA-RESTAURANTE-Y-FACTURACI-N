package sri

import (
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

var (
	iva15 = Tarifa{Porcentaje: "15", Codigo: "4"}
	iva0  = Tarifa{Porcentaje: "0", Codigo: "0"}
)

func m(s string) money.Money          { return money.MustParse(s) }
func d(s string) decimal.Decimal      { return decimal.RequireFromString(s) }
func tasaDe(t Tarifa) decimal.Decimal { return d(t.Porcentaje).Div(d("100")) }

// totalesComoElNodo reproduce calcularTotales del nodo: por tarifa, con IVA incluido la base es
// redondeo2(monto / (1 + tasa)) y el IVA el resto; sin IVA incluido, el IVA es redondeo2(base ×
// tasa). La propina es el 10 % de la base hacia abajo.
func totalesComoElNodo(lineas []LineaVenta, incluye bool, conPropina bool) Venta {
	v := Venta{Lineas: lineas, PorTarifa: map[string]TotalTarifa{}, IncluyeIVA: incluye}
	montos := map[string]money.Money{}
	tarifas := map[string]Tarifa{}
	for _, l := range lineas {
		montos[l.Tarifa.Porcentaje] = montos[l.Tarifa.Porcentaje].Add(l.Final)
		tarifas[l.Tarifa.Porcentaje] = l.Tarifa
	}
	base := money.Money{}
	for p, monto := range montos {
		tasa := tasaDe(tarifas[p])
		var tt TotalTarifa
		if incluye {
			b := money.FromDecimal(monto.Decimal().Div(d("1").Add(tasa))).Round2()
			tt = TotalTarifa{Base: b, IVA: monto.Sub(b)}
		} else {
			tt = TotalTarifa{Base: monto, IVA: monto.Mul(tasa).Round2()}
		}
		v.PorTarifa[p] = tt
		base = base.Add(tt.Base)
		v.Total = v.Total.Add(tt.Base).Add(tt.IVA)
	}
	if conPropina {
		v.Propina = money.FromDecimal(base.Decimal().Mul(d("0.10")).RoundFloor(2))
		v.Total = v.Total.Add(v.Propina)
	}
	return v
}

func linea(desc, cant, bruto, final string, t Tarifa) LineaVenta {
	return LineaVenta{Codigo: desc[:3], Descripcion: desc, Cantidad: d(cant), Bruto: m(bruto), Final: m(final), Tarifa: t}
}

// El ejemplo de F5-04: un plato de $15,00 con IVA incluido factura exactamente $15,00.
func TestPlatoDeQuinceDolares(t *testing.T) {
	v := totalesComoElNodo([]LineaVenta{linea("Ceviche", "1", "15.00", "15.00", iva15)}, true, false)
	f, err := Desglosar(v)
	if err != nil {
		t.Fatal(err)
	}
	l := f.Lineas[0]
	if l.PrecioUnitario.String() != "13.04" || l.PrecioTotalSinImpuesto.String() != "13.04" || l.IVA.String() != "1.96" || f.ImporteTotal.String() != "15.00" {
		t.Fatalf("detalle: pu %s base %s iva %s total %s", l.PrecioUnitario, l.PrecioTotalSinImpuesto, l.IVA, f.ImporteTotal)
	}
	if f.Ajustes != 0 {
		t.Fatalf("13.04 × 15 %% = 1.956 → 1.96: no debía haber ajuste, hubo %d", f.Ajustes)
	}
}

// Precio unitario con 6 decimales: 3 cervezas de $1.00 con IVA dan una base de 2.61 que no
// se divide en tres centavos exactos; 0.87 × 3 = 2.61 sí.
func TestPrecioUnitarioSeisDecimales(t *testing.T) {
	v := totalesComoElNodo([]LineaVenta{
		linea("Cerveza", "3", "3.00", "3.00", iva15),
		linea("Jugo por peso", "0.375", "1.20", "1.20", iva15), // 375 g a $3.20/kg
	}, true, true)
	f, err := Desglosar(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range f.Lineas {
		if !money.FromDecimal(l.Cantidad.Mul(l.PrecioUnitario)).Round2().Sub(l.Descuento).Equal(l.PrecioTotalSinImpuesto) {
			t.Fatalf("%s: %s × %s − %s ≠ %s", l.Descripcion, l.Cantidad, l.PrecioUnitario, l.Descuento, l.PrecioTotalSinImpuesto)
		}
		if l.PrecioUnitario.Exponent() < -6 {
			t.Fatalf("%s: precio con más de 6 decimales %s", l.Descripcion, l.PrecioUnitario)
		}
	}
	if !f.ImporteTotal.Equal(v.Total) {
		t.Fatalf("total %s, se cobró %s", f.ImporteTotal, v.Total)
	}
}

// Descuento en una línea y dos tarifas (15 % y 0 %): el descuento va sin IVA en su línea.
func TestDescuentoYDosTarifas(t *testing.T) {
	v := totalesComoElNodo([]LineaVenta{
		linea("Parrillada", "1", "12.00", "10.80", iva15), // 10 % de descuento
		linea("Agua", "2", "2.00", "2.00", iva0),
	}, true, true)
	f, err := Desglosar(v)
	if err != nil {
		t.Fatal(err)
	}
	p := f.Lineas[0]
	// 12.00 / 1.15 = 10.43 antes del descuento; 10.80 / 1.15 = 9.39 después → descuento 1.04.
	if p.PrecioUnitario.String() != "10.43" || p.Descuento.String() != "1.04" || p.PrecioTotalSinImpuesto.String() != "9.39" {
		t.Fatalf("parrillada: pu %s desc %s base %s", p.PrecioUnitario, p.Descuento, p.PrecioTotalSinImpuesto)
	}
	if len(f.Impuestos) != 2 || f.Impuestos[0].Tarifa.Codigo != "0" || f.Impuestos[1].Tarifa.Codigo != "4" {
		t.Fatalf("impuestos: %+v", f.Impuestos)
	}
	if f.TotalDescuento.String() != "1.04" || !f.ImporteTotal.Equal(v.Total) {
		t.Fatalf("descuento %s, total %s de %s", f.TotalDescuento, f.ImporteTotal, v.Total)
	}
}

func TestRechazaLoQueNoCuadra(t *testing.T) {
	v := totalesComoElNodo([]LineaVenta{linea("Ceviche", "1", "15.00", "15.00", iva15)}, true, false)
	v.Total = v.Total.Add(m("0.01"))
	if _, err := Desglosar(v); !errors.Is(err, ErrDesglose) {
		t.Fatalf("un total distinto de lo cobrado debe fallar: %v", err)
	}
	v = totalesComoElNodo([]LineaVenta{linea("Ceviche", "1", "10.05", "10.05", iva0)}, true, false)
	v.Propina, v.Total = m("1.01"), v.Total.Add(m("1.01")) // 10 % de 10.05 = 1.005
	if _, err := Desglosar(v); !errors.Is(err, ErrDesglose) {
		t.Fatal("una propina mayor al 10 % del subtotal debe fallar (Tabla 21)")
	}
}

// QA-05: 10 000 ventas al azar (precios con y sin IVA incluido, cantidades enteras y por peso,
// descuentos, dos tarifas y propina) cuadran al centavo con lo cobrado.
func TestQA05DiezMilVentas(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 2026))
	const n = 10_000
	lineasTotales, ajustes, tarifasTotales, ajustesTarifa := 0, 0, 0, 0
	for i := range n {
		var lineas []LineaVenta
		for k := range 1 + r.IntN(8) {
			cant := decimal.NewFromInt(int64(1 + r.IntN(4)))
			if r.IntN(6) == 0 { // por peso
				cant = decimal.New(int64(50+r.IntN(1500)), -3)
			}
			precio := decimal.New(int64(25+r.IntN(6000)), -2)
			bruto := money.FromDecimal(precio.Mul(cant)).Round2()
			final := bruto
			if r.IntN(4) == 0 {
				final = money.FromDecimal(bruto.Decimal().Mul(decimal.New(int64(50+r.IntN(50)), -2))).Round2()
			}
			if r.IntN(20) == 0 {
				final = money.Money{} // cortesía
			}
			tarifa := iva15
			if r.IntN(5) == 0 {
				tarifa = iva0
			}
			lineas = append(lineas, LineaVenta{Codigo: "P", Descripcion: "plato", Cantidad: cant, Bruto: bruto, Final: final, Tarifa: tarifa})
			_ = k
		}
		v := totalesComoElNodo(lineas, r.IntN(4) != 0, r.IntN(2) == 0)
		f, err := Desglosar(v)
		if err != nil {
			t.Fatalf("venta %d: %v", i, err)
		}
		suma := money.Money{}
		for _, l := range f.Lineas {
			if !money.FromDecimal(l.Cantidad.Mul(l.PrecioUnitario)).Round2().Sub(l.Descuento).Equal(l.PrecioTotalSinImpuesto) || l.Descuento.IsNegative() {
				t.Fatalf("venta %d: línea %+v no cumple cantidad × precio − descuento", i, l)
			}
			suma = suma.Add(l.PrecioTotalSinImpuesto)
		}
		if !suma.Equal(f.TotalSinImpuestos) || !f.ImporteTotal.Equal(v.Total) {
			t.Fatalf("venta %d: líneas %s, subtotal %s, total %s de %s", i, suma, f.TotalSinImpuestos, f.ImporteTotal, v.Total)
		}
		lineasTotales += len(f.Lineas)
		ajustes += f.Ajustes
		for _, imp := range f.Impuestos {
			tarifasTotales++
			if !imp.Valor.Equal(imp.BaseImponible.Mul(tasaDe(imp.Tarifa)).Round2()) {
				ajustesTarifa++
			}
		}
	}
	// El punto 🔎: líneas cuyo IVA no es redondeo2(base × tarifa). Se mide para confirmarlo
	// con el SRI; hoy solo se informa.
	t.Logf("%d ventas, %d líneas; IVA de línea distinto de redondeo2(base × tarifa) en %d (%.2f %%)", n, lineasTotales, ajustes, 100*float64(ajustes)/float64(lineasTotales))
	t.Logf("totales por tarifa: %d de %d con IVA distinto de redondeo2(base × tarifa) (%.2f %%)", ajustesTarifa, tarifasTotales, 100*float64(ajustesTarifa)/float64(tarifasTotales))
}
