package app

import (
	"errors"
	"fmt"
	"sort"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// TarifaTotales es la base y el IVA de una tarifa (clave: porcentaje de IVA).
type TarifaTotales struct {
	Base money.Money `json:"base"`
	IVA  money.Money `json:"iva"`
}

// lineaReparto es un plato con su importe final (tras descuentos) y su tarifa de IVA.
type lineaReparto struct {
	ID    ids.ID
	Pct   string
	Final money.Money
}

// cuentaReparto es una cuenta de la división. Las pagadas traen lo que ya pagaron (congelado);
// las abiertas, los pesos con que se llevan cada plato.
type cuentaReparto struct {
	ID            ids.ID
	Numero        int
	Pagada        bool
	Pesos         map[ids.ID]int64 // abiertas: plato → peso (pizza entre 3: peso 1 en cada una)
	PagadoLineas  map[ids.ID]money.Money
	PagadoTarifas map[string]TarifaTotales
	PagadoPropina money.Money
}

// CuentaTotales es el resultado de una cuenta.
type CuentaTotales struct {
	ID        ids.ID                   `json:"id"`
	Numero    int                      `json:"numero"`
	Pagada    bool                     `json:"pagada"`
	Lineas    map[ids.ID]money.Money   `json:"lineas"` // lo que la cuenta lleva de cada plato (con IVA si los precios lo incluyen)
	PorTarifa map[string]TarifaTotales `json:"porTarifa"`
	Subtotal  money.Money              `json:"subtotal"`
	IVA       money.Money              `json:"iva"`
	Propina   money.Money              `json:"propina"`
	Total     money.Money              `json:"total"`
}

var errPagadoSupera = errors.New("lo pagado supera lo que queda de la orden")

// errSinCuenta lista los platos con saldo que no están en ninguna cuenta abierta.
type errSinCuenta struct{ lineas []ids.ID }

func (e errSinCuenta) Error() string { return "hay platos sin cuenta" }

func pesosDe(xs []money.Money) []decimal.Decimal {
	out := make([]decimal.Decimal, len(xs))
	for i, x := range xs {
		out[i] = x.Decimal()
	}
	return out
}

// repartirEnCuentas divide la orden entre sus cuentas (RF-04-06, F4-08) de modo que la suma
// de las cuentas es exactamente el total de la orden, al centavo:
//   - cada plato reparte lo que le queda por pagar entre sus cuentas abiertas según los
//     pesos, con el residuo de centavos en la última (AllocateUltimo);
//   - por tarifa, lo que queda de base se reparte según lo que cada cuenta consume en esa
//     tarifa, y el IVA es la diferencia (si los precios lo incluyen) o se reparte igual;
//   - el servicio pendiente se reparte según la base de cada cuenta.
//
// Las cuentas pagadas quedan congeladas con lo que pagaron.
func repartirEnCuentas(lineas []lineaReparto, orden map[string]TarifaTotales, propina money.Money, incluyeIVA bool, cuentas []cuentaReparto) ([]CuentaTotales, error) {
	sort.SliceStable(cuentas, func(i, j int) bool { return cuentas[i].Numero < cuentas[j].Numero })
	out := make([]CuentaTotales, len(cuentas))
	var abiertas []int
	resto := map[string]TarifaTotales{}
	for k, v := range orden {
		resto[k] = v
	}
	restoPropina := propina
	for i, c := range cuentas {
		out[i] = CuentaTotales{ID: c.ID, Numero: c.Numero, Pagada: c.Pagada, Lineas: map[ids.ID]money.Money{}, PorTarifa: map[string]TarifaTotales{}}
		if !c.Pagada {
			abiertas = append(abiertas, i)
			continue
		}
		for k, v := range c.PagadoLineas {
			out[i].Lineas[k] = v
		}
		for t, v := range c.PagadoTarifas {
			out[i].PorTarifa[t] = v
			r := resto[t]
			resto[t] = TarifaTotales{Base: r.Base.Sub(v.Base), IVA: r.IVA.Sub(v.IVA)}
		}
		out[i].Propina = c.PagadoPropina
		restoPropina = restoPropina.Sub(c.PagadoPropina)
	}
	// 1) Platos: lo pendiente de cada uno entre sus cuentas abiertas. Los platos que van a las
	// mismas cuentas con los mismos pesos forman un grupo que se reparte como un todo (partes
	// iguales de $24.00 entre 3 da $8.00 a cada una, no $7.99/$8.00/$8.01).
	type grupo struct {
		idx    []int
		pesos  []decimal.Decimal
		lineas []int // índices en lineas
		pend   []money.Money
	}
	var grupos []*grupo
	porFirma := map[string]*grupo{}
	var sinCuenta []ids.ID
	for n, l := range lineas {
		pendiente := l.Final
		for _, c := range cuentas {
			if c.Pagada {
				pendiente = pendiente.Sub(c.PagadoLineas[l.ID])
			}
		}
		if pendiente.IsNegative() {
			return nil, errPagadoSupera
		}
		var idx []int
		var pesos []decimal.Decimal
		firma := ""
		for _, i := range abiertas {
			if p := cuentas[i].Pesos[l.ID]; p > 0 {
				idx = append(idx, i)
				pesos = append(pesos, decimal.NewFromInt(p))
				firma += fmt.Sprintf("%d:%d,", i, p)
			}
		}
		if len(idx) == 0 {
			if !pendiente.IsZero() {
				sinCuenta = append(sinCuenta, l.ID)
			}
			continue
		}
		g := porFirma[firma]
		if g == nil {
			g = &grupo{idx: idx, pesos: pesos}
			porFirma[firma] = g
			grupos = append(grupos, g)
		}
		g.lineas = append(g.lineas, n)
		g.pend = append(g.pend, pendiente)
	}
	gruesoTarifa := map[string]map[int]money.Money{} // tarifa → cuenta → consumo
	exacto := map[int]decimal.Decimal{}              // cuenta → consumo sin redondear (guía del total)
	for _, g := range grupos {
		sumaPesos := decimal.Zero
		for _, p := range g.pesos {
			sumaPesos = sumaPesos.Add(p)
		}
		for _, p := range g.pend {
			for k, i := range g.idx {
				exacto[i] = exacto[i].Add(p.Decimal().Mul(g.pesos[k]).DivRound(sumaPesos, 16))
			}
		}
		partes, err := repartirGrupo(g.pend, g.pesos)
		if err != nil {
			return nil, err
		}
		for n, li := range g.lineas {
			l := lineas[li]
			if gruesoTarifa[l.Pct] == nil {
				gruesoTarifa[l.Pct] = map[int]money.Money{}
			}
			for k, i := range g.idx {
				out[i].Lineas[l.ID] = partes[n][k]
				gruesoTarifa[l.Pct][i] = gruesoTarifa[l.Pct][i].Add(partes[n][k])
			}
		}
	}
	if len(sinCuenta) > 0 {
		return nil, errSinCuenta{sinCuenta}
	}
	if len(abiertas) == 0 {
		for _, r := range resto {
			if !r.Base.IsZero() || !r.IVA.IsZero() {
				return nil, errPagadoSupera
			}
		}
		return totalizar(out), nil
	}
	// 2) Base e IVA pendientes de cada tarifa según el consumo de cada cuenta en esa tarifa.
	tarifas := make([]string, 0, len(resto))
	for t := range resto {
		tarifas = append(tarifas, t)
	}
	sort.Strings(tarifas)
	for _, t := range tarifas {
		r := resto[t]
		if r.Base.IsNegative() || r.IVA.IsNegative() {
			return nil, errPagadoSupera
		}
		consumo := make([]money.Money, len(abiertas))
		suma := money.Money{}
		for k, i := range abiertas {
			consumo[k] = gruesoTarifa[t][i]
			suma = suma.Add(consumo[k])
		}
		if suma.IsZero() {
			if r.Base.IsZero() && r.IVA.IsZero() {
				continue
			}
			consumo[len(consumo)-1] = money.MustParse("1") // centavos de redondeo sin consumo: a la última
		}
		bases, err := money.AllocateUltimo(r.Base, pesosDe(consumo))
		if err != nil {
			return nil, err
		}
		ivas := make([]money.Money, len(abiertas))
		if incluyeIVA && !suma.IsZero() && suma.Equal(r.Base.Add(r.IVA)) {
			for k := range abiertas {
				ivas[k] = consumo[k].Sub(bases[k]) // base + IVA = lo que consume la cuenta
			}
		} else if ivas, err = money.AllocateUltimo(r.IVA, pesosDe(consumo)); err != nil {
			return nil, err
		}
		for k, i := range abiertas {
			if bases[k].IsZero() && ivas[k].IsZero() {
				continue
			}
			out[i].PorTarifa[t] = TarifaTotales{Base: bases[k], IVA: ivas[k]}
		}
	}
	// 3) Servicio pendiente según la base de cada cuenta.
	if restoPropina.IsNegative() {
		return nil, errPagadoSupera
	}
	if !restoPropina.IsZero() {
		bases := make([]money.Money, len(abiertas))
		suma := money.Money{}
		for k, i := range abiertas {
			for _, v := range out[i].PorTarifa {
				bases[k] = bases[k].Add(v.Base)
			}
			suma = suma.Add(bases[k])
		}
		if suma.IsZero() {
			bases[len(bases)-1] = money.MustParse("1")
		}
		partes, err := money.AllocateUltimo(restoPropina, pesosDe(bases))
		if err != nil {
			return nil, err
		}
		for k, i := range abiertas {
			out[i].Propina = partes[k]
		}
	}
	return cuadrarTotales(totalizar(out), abiertas, exacto, incluyeIVA)
}

// cuadrarTotales evita que los centavos de cada rubro (consumo, servicio, IVA aparte) caigan
// todos en la misma cuenta: lo que queda de la orden se reparte una vez según el consumo sin
// redondear (residuo en la última) y se mueven centavos del servicio —y del IVA cuando los
// precios no lo incluyen— hasta que cada cuenta llegue a su parte. Lo de cada plato no cambia.
func cuadrarTotales(out []CuentaTotales, abiertas []int, exacto map[int]decimal.Decimal, incluyeIVA bool) ([]CuentaTotales, error) {
	if len(abiertas) < 2 {
		return out, nil
	}
	resto := money.Money{}
	pesos := make([]decimal.Decimal, len(abiertas))
	suma := decimal.Zero
	for k, i := range abiertas {
		resto = resto.Add(out[i].Total)
		pesos[k] = exacto[i]
		suma = suma.Add(exacto[i])
	}
	if suma.IsZero() {
		return out, nil
	}
	meta, err := money.AllocateUltimo(resto, pesos)
	if err != nil {
		return nil, err
	}
	centavo := money.MustParse("0.01")
	// pasar un centavo de un rubro ajustable de la cuenta i a la j (primero el servicio).
	pasar := func(i, j int) bool {
		if out[i].Propina.Decimal().IsPositive() {
			out[i].Propina, out[j].Propina = out[i].Propina.Sub(centavo), out[j].Propina.Add(centavo)
			return true
		}
		if incluyeIVA {
			return false // el IVA es el que llevan los platos: no se mueve
		}
		tarifas := make([]string, 0, len(out[i].PorTarifa))
		for t := range out[i].PorTarifa {
			tarifas = append(tarifas, t)
		}
		sort.Strings(tarifas)
		for _, t := range tarifas {
			if v, w := out[i].PorTarifa[t], out[j].PorTarifa[t]; v.IVA.Decimal().IsPositive() {
				out[i].PorTarifa[t] = TarifaTotales{Base: v.Base, IVA: v.IVA.Sub(centavo)}
				out[j].PorTarifa[t] = TarifaTotales{Base: w.Base, IVA: w.IVA.Add(centavo)}
				return true
			}
		}
		return false
	}
	for {
		movido := false
		for k, i := range abiertas {
			if !out[i].Total.GreaterThan(meta[k]) {
				continue
			}
			for q, j := range abiertas {
				if out[j].Total.LessThan(meta[q]) && pasar(i, j) {
					out = totalizar(out)
					movido = true
					break
				}
			}
			if movido {
				break
			}
		}
		if !movido {
			return out, nil
		}
	}
}

// repartirGrupo reparte varios platos entre las mismas cuentas y pesos. Lo que lleva cada
// cuenta del grupo es el total del grupo repartido una sola vez (residuo en la última); cada
// plato parte de su propio reparto y se corrigen los centavos moviéndolos, desde el último
// plato, de las cuentas que llevan de más a las que llevan de menos. Así cada plato suma su
// importe, ninguna parte queda negativa y el total de cada cuenta no depende del orden.
func repartirGrupo(pend []money.Money, pesos []decimal.Decimal) ([][]money.Money, error) {
	partes := make([][]money.Money, len(pend))
	suma := money.Money{}
	actual := make([]money.Money, len(pesos))
	for n, p := range pend {
		ps, err := money.AllocateUltimo(p, pesos)
		if err != nil {
			return nil, err
		}
		partes[n] = ps
		suma = suma.Add(p)
		for k := range ps {
			actual[k] = actual[k].Add(ps[k])
		}
	}
	meta, err := money.AllocateUltimo(suma, pesos)
	if err != nil {
		return nil, err
	}
	centavo := money.MustParse("0.01")
	for {
		falta, sobra := -1, -1
		for k := range meta {
			switch d := meta[k].Sub(actual[k]); {
			case d.Decimal().IsPositive() && falta < 0:
				falta = k
			case d.IsNegative() && sobra < 0:
				sobra = k
			}
		}
		if falta < 0 || sobra < 0 {
			return partes, nil
		}
		for n := len(partes) - 1; n >= 0; n-- {
			if partes[n][sobra].Decimal().IsPositive() {
				partes[n][sobra] = partes[n][sobra].Sub(centavo)
				partes[n][falta] = partes[n][falta].Add(centavo)
				actual[sobra], actual[falta] = actual[sobra].Sub(centavo), actual[falta].Add(centavo)
				break
			}
		}
	}
}

func totalizar(cs []CuentaTotales) []CuentaTotales {
	for i := range cs {
		c := &cs[i]
		c.Subtotal, c.IVA = money.Money{}, money.Money{}
		for _, v := range c.PorTarifa {
			c.Subtotal, c.IVA = c.Subtotal.Add(v.Base), c.IVA.Add(v.IVA)
		}
		c.Total = c.Subtotal.Add(c.IVA).Add(c.Propina)
	}
	return cs
}
