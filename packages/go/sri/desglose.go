package sri

import (
	"errors"
	"fmt"
	"sort"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// Desglose fiscal de una venta (F5-04): convierte lo que se cobró en la caja en el detalle
// que exige la factura 1.1.0 (ficha técnica offline v2.34, docs/fuentes/sri/): por línea,
// cantidad y precio unitario sin IVA con hasta 6 decimales, descuento, precio total sin
// impuestos, base e IVA; y los totales por tarifa, la propina y el importe total.
//
// La verdad son los totales por tarifa que ya calculó el nodo al cobrar (base e IVA): el
// desglose los reparte entre las líneas sin cambiar un centavo, de modo que el comprobante
// suma exactamente lo cobrado. Se garantizan, de forma exacta:
//
//   - Σ precioTotalSinImpuesto de las líneas = totalSinImpuestos;
//   - por tarifa, base imponible y valor = Σ de sus líneas;
//   - precioTotalSinImpuesto = redondeo2(cantidad × precioUnitario) − descuento;
//   - importeTotal = totalSinImpuestos + Σ IVA + propina = lo cobrado;
//   - propina ≤ 10 % del subtotal (Tabla 21).
//
// 🔎 Punto abierto (docs/fuentes/sri/README.md): la ficha no publica la tolerancia de la
// validación de diferencias (error 52). Con precios que incluyen IVA hay montos para los que
// no existe una base cuyo IVA redondeado dé justo el total; entonces el IVA de esa línea
// difiere en un centavo de redondeo2(base × tarifa). Se cuentan en Ajustes para medirlos y
// confirmarlos en el ambiente de pruebas del SRI.

// Tarifa de IVA de una línea (Tabla 17): porcentaje y código.
type Tarifa struct {
	Porcentaje string // "15"
	Codigo     string // "4"
}

// LineaVenta es una línea cobrada. Bruto y Final son el total de la línea antes y después
// de descuentos, con IVA si los precios del local lo incluyen.
type LineaVenta struct {
	Codigo      string
	Descripcion string
	Cantidad    decimal.Decimal
	Bruto       money.Money
	Final       money.Money
	Tarifa      Tarifa
}

// TotalTarifa es la base y el IVA de una tarifa tal como se cobraron.
type TotalTarifa struct {
	Base money.Money
	IVA  money.Money
}

// Venta es lo que el nodo cobró.
type Venta struct {
	Lineas     []LineaVenta
	PorTarifa  map[string]TotalTarifa // clave: porcentaje ("15")
	Propina    money.Money
	IncluyeIVA bool
	Total      money.Money // lo cobrado: el desglose debe dar exactamente esto
}

// LineaFactura es un <detalle> de la factura.
type LineaFactura struct {
	Codigo                 string
	Descripcion            string
	Cantidad               decimal.Decimal // hasta 6 decimales
	PrecioUnitario         decimal.Decimal // sin IVA, hasta 6 decimales
	Descuento              money.Money
	PrecioTotalSinImpuesto money.Money
	Tarifa                 Tarifa
	BaseImponible          money.Money
	IVA                    money.Money
}

// ImpuestoTotal es un <totalImpuesto> por tarifa.
type ImpuestoTotal struct {
	Tarifa        Tarifa
	BaseImponible money.Money
	Valor         money.Money
}

// Factura es el desglose listo para el XML.
type Factura struct {
	Lineas            []LineaFactura
	TotalSinImpuestos money.Money
	TotalDescuento    money.Money
	Impuestos         []ImpuestoTotal // ordenados por porcentaje
	Propina           money.Money
	ImporteTotal      money.Money
	// Ajustes cuenta las líneas cuyo IVA no es exactamente redondeo2(base × tarifa) (🔎).
	Ajustes int
}

// CodigoImpuestoIVA es el código del impuesto IVA (Tabla 16 de la ficha).
const CodigoImpuestoIVA = "2"

var (
	ErrDesglose        = errors.New("sri: el desglose no cuadra")
	cien               = decimal.NewFromInt(100)
	maxDecimalesPrecio = int32(6)
)

// Desglosar reparte los totales cobrados entre las líneas.
func Desglosar(v Venta) (Factura, error) {
	var f Factura
	grupos := map[string][]int{}
	for i, l := range v.Lineas {
		if l.Cantidad.Sign() <= 0 {
			return f, fmt.Errorf("%w: la línea %d no tiene cantidad", ErrDesglose, i+1)
		}
		if l.Final.IsNegative() || l.Final.GreaterThan(l.Bruto) {
			return f, fmt.Errorf("%w: la línea %d vale %s tras descuentos y %s antes", ErrDesglose, i+1, l.Final, l.Bruto)
		}
		grupos[l.Tarifa.Porcentaje] = append(grupos[l.Tarifa.Porcentaje], i)
	}
	pcts := make([]string, 0, len(grupos))
	for p := range grupos {
		pcts = append(pcts, p)
	}
	sort.Strings(pcts)
	if len(pcts) != len(v.PorTarifa) {
		return f, fmt.Errorf("%w: hay %d tarifas en las líneas y %d en los totales", ErrDesglose, len(pcts), len(v.PorTarifa))
	}
	f.Lineas = make([]LineaFactura, len(v.Lineas))
	for _, pct := range pcts {
		tot, ok := v.PorTarifa[pct]
		if !ok {
			return f, fmt.Errorf("%w: falta el total de la tarifa %s %%", ErrDesglose, pct)
		}
		p, err := decimal.NewFromString(pct)
		if err != nil {
			return f, fmt.Errorf("%w: tarifa %q", ErrDesglose, pct)
		}
		tasa := p.Div(cien)
		idx := grupos[pct]
		// Bases: la base de la tarifa, según lo que cada línea cobró (mayor residuo).
		pesos := make([]decimal.Decimal, len(idx))
		for k, i := range idx {
			pesos[k] = v.Lineas[i].Final.Decimal()
		}
		bases, err := repartir(tot.Base, pesos)
		if err != nil {
			return f, err
		}
		ivas, err := ivasDeLineas(v, idx, bases, tot.IVA, tasa)
		if err != nil {
			return f, err
		}
		imp := ImpuestoTotal{Tarifa: v.Lineas[idx[0]].Tarifa}
		for k, i := range idx {
			l := v.Lineas[i]
			lf, err := lineaFactura(l, bases[k], ivas[k], tasa, v.IncluyeIVA)
			if err != nil {
				return f, fmt.Errorf("%w: línea %d: %v", ErrDesglose, i+1, err)
			}
			if !lf.IVA.Equal(lf.BaseImponible.Mul(tasa).Round2()) {
				f.Ajustes++
			}
			f.Lineas[i] = lf
			imp.BaseImponible = imp.BaseImponible.Add(lf.BaseImponible)
			imp.Valor = imp.Valor.Add(lf.IVA)
			f.TotalSinImpuestos = f.TotalSinImpuestos.Add(lf.PrecioTotalSinImpuesto)
			f.TotalDescuento = f.TotalDescuento.Add(lf.Descuento)
		}
		if !imp.BaseImponible.Equal(tot.Base) || !imp.Valor.Equal(tot.IVA) {
			return f, fmt.Errorf("%w: la tarifa %s %% suma %s + %s y se cobró %s + %s", ErrDesglose, pct, imp.BaseImponible, imp.Valor, tot.Base, tot.IVA)
		}
		f.Impuestos = append(f.Impuestos, imp)
	}
	f.Propina = v.Propina
	if f.Propina.IsNegative() || f.Propina.GreaterThan(f.TotalSinImpuestos.Mul(decimal.NewFromInt(10)).Mul(decimal.New(1, -2))) {
		return f, fmt.Errorf("%w: la propina %s supera el 10 %% del subtotal %s", ErrDesglose, f.Propina, f.TotalSinImpuestos)
	}
	f.ImporteTotal = f.TotalSinImpuestos.Add(f.Propina)
	for _, imp := range f.Impuestos {
		f.ImporteTotal = f.ImporteTotal.Add(imp.Valor)
	}
	if !f.ImporteTotal.Equal(v.Total) {
		return f, fmt.Errorf("%w: el comprobante suma %s y se cobró %s", ErrDesglose, f.ImporteTotal, v.Total)
	}
	return f, nil
}

// lineaFactura arma el <detalle>: el descuento es lo que la línea bajó sin IVA y el precio
// unitario (6 decimales) reproduce al centavo la base antes del descuento.
func lineaFactura(l LineaVenta, base, iva money.Money, tasa decimal.Decimal, incluyeIVA bool) (LineaFactura, error) {
	bruta := base // sin descuento: el precio sale de la base
	if l.Final.LessThan(l.Bruto) {
		bruta = l.Bruto
		if incluyeIVA {
			bruta = money.FromDecimal(l.Bruto.Decimal().Div(decimal.NewFromInt(1).Add(tasa))).Round2()
		}
		if bruta.LessThan(base) { // un descuento de menos de un centavo sin IVA
			bruta = base
		}
	}
	pu := bruta.Decimal().DivRound(l.Cantidad, maxDecimalesPrecio)
	if !money.FromDecimal(l.Cantidad.Mul(pu)).Round2().Equal(bruta) {
		return LineaFactura{}, fmt.Errorf("el precio unitario %s × %s no reproduce %s", pu, l.Cantidad, bruta)
	}
	if l.Cantidad.Exponent() < -maxDecimalesPrecio {
		return LineaFactura{}, fmt.Errorf("la cantidad %s tiene más de 6 decimales", l.Cantidad)
	}
	desc := bruta.Sub(base)
	return LineaFactura{
		Codigo: l.Codigo, Descripcion: l.Descripcion, Cantidad: l.Cantidad, PrecioUnitario: pu,
		Descuento: desc, PrecioTotalSinImpuesto: base, Tarifa: l.Tarifa, BaseImponible: base, IVA: iva,
	}, nil
}

// ivasDeLineas da a cada línea su IVA de modo que sumen exactamente el IVA cobrado de la tarifa.
//   - Con IVA incluido, el de cada línea es lo que cobró menos su base: la línea suma lo que marcó
//     la caja y la tarifa cuadra sola (Σ final − Σ base = IVA de la tarifa).
//   - Sin IVA incluido, cada línea lleva redondeo2(base × tasa) y los centavos que falten o sobren
//     frente al IVA de la tarifa van a las líneas cuyo redondeo quedó más cerca del límite.
func ivasDeLineas(v Venta, idx []int, bases []money.Money, ivaTarifa money.Money, tasa decimal.Decimal) ([]money.Money, error) {
	ivas := make([]money.Money, len(idx))
	if v.IncluyeIVA {
		suma := money.Money{}
		for k, i := range idx {
			ivas[k] = v.Lineas[i].Final.Sub(bases[k])
			if ivas[k].IsNegative() {
				return nil, fmt.Errorf("%w: la línea %d quedaría con IVA negativo", ErrDesglose, i+1)
			}
			suma = suma.Add(ivas[k])
		}
		if !suma.Equal(ivaTarifa) {
			return nil, fmt.Errorf("%w: el IVA de las líneas suma %s y el de la tarifa es %s", ErrDesglose, suma, ivaTarifa)
		}
		return ivas, nil
	}
	exactos := make([]decimal.Decimal, len(idx))
	suma := money.Money{}
	for k := range idx {
		exactos[k] = bases[k].Decimal().Mul(tasa)
		ivas[k] = money.FromDecimal(exactos[k]).Round2()
		suma = suma.Add(ivas[k])
	}
	faltan := ivaTarifa.Sub(suma).Cents()
	centavo := money.MustParse("0.01")
	for faltan != 0 {
		// La línea cuyo redondeo quedó más lejos a favor del ajuste (y que no quede negativa).
		mejor, dist := -1, decimal.Zero
		for k := range idx {
			diff := exactos[k].Sub(ivas[k].Decimal()) // > 0: se redondeó hacia abajo
			if faltan < 0 {
				diff = diff.Neg()
				if ivas[k].IsZero() {
					continue
				}
			}
			if mejor < 0 || diff.GreaterThan(dist) {
				mejor, dist = k, diff
			}
		}
		if mejor < 0 {
			return nil, fmt.Errorf("%w: no se puede ajustar el IVA de la tarifa", ErrDesglose)
		}
		if faltan > 0 {
			ivas[mejor], faltan = ivas[mejor].Add(centavo), faltan-1
		} else {
			ivas[mejor], faltan = ivas[mejor].Sub(centavo), faltan+1
		}
	}
	return ivas, nil
}

// repartir es el mayor residuo; si todos los pesos son cero (líneas en cero) va a la última.
func repartir(total money.Money, pesos []decimal.Decimal) ([]money.Money, error) {
	suma := decimal.Zero
	for _, p := range pesos {
		suma = suma.Add(p)
	}
	if suma.IsZero() {
		pesos = append([]decimal.Decimal(nil), pesos...)
		pesos[len(pesos)-1] = decimal.NewFromInt(1)
	}
	return money.Allocate(total, pesos)
}
