package money

import (
	"sort"

	"github.com/shopspring/decimal"
)

// SplitEqual divide un total en n partes iguales a centavos.
// El residuo de centavos se asigna a la última parte (RF-04-06.3, CU-02).
// El total se redondea primero a centavos; la suma de las partes es exactamente ese total.
//
//	SplitEqual(10.00, 3) → [3.33, 3.33, 3.34]
func SplitEqual(total Money, n int) ([]Money, error) {
	if n <= 0 {
		return nil, ErrInvalidParts
	}
	cents := total.Round2().Cents()
	base := cents / int64(n) // trunca hacia cero, también para importes negativos (notas de crédito)
	parts := make([]Money, n)
	for i := range parts {
		parts[i] = FromCents(base)
	}
	parts[n-1] = FromCents(cents - base*int64(n-1))
	return parts, nil
}

// Allocate reparte un total entre pesos no negativos con el método del mayor residuo
// (largest remainder). Cada parte recibe la parte entera de su cuota en centavos y los
// centavos sobrantes van, uno por uno, a las cuotas con mayor fracción descartada.
// Los empates se resuelven a favor del índice menor, así el resultado es determinístico.
//
// Se usa para prorratear descuentos, propinas o IVA entre líneas sin perder centavos.
func Allocate(total Money, weights []decimal.Decimal) ([]Money, error) {
	if len(weights) == 0 {
		return nil, ErrInvalidWeights
	}
	sumW := decimal.Zero
	for _, w := range weights {
		if w.IsNegative() {
			return nil, ErrInvalidWeights
		}
		sumW = sumW.Add(w)
	}
	if sumW.IsZero() {
		return nil, ErrInvalidWeights
	}

	cents := total.Round2().Cents()
	sign := int64(1)
	if cents < 0 {
		sign, cents = -1, -cents
	}
	totalD := decimal.NewFromInt(cents)

	type share struct {
		idx  int
		base int64
		rem  decimal.Decimal
	}
	shares := make([]share, len(weights))
	assigned := int64(0)
	for i, w := range weights {
		// cuota exacta = total × w / Σw, calculada sin redondeo intermedio
		exact := totalD.Mul(w).DivRound(sumW, 16)
		base := exact.Truncate(0)
		shares[i] = share{idx: i, base: base.IntPart(), rem: exact.Sub(base)}
		assigned += base.IntPart()
	}

	left := cents - assigned
	order := make([]share, len(shares))
	copy(order, shares)
	sort.SliceStable(order, func(a, b int) bool { return order[a].rem.GreaterThan(order[b].rem) })
	for i := int64(0); i < left; i++ {
		shares[order[i%int64(len(order))].idx].base++
	}

	out := make([]Money, len(shares))
	for i, s := range shares {
		out[i] = FromCents(sign * s.base)
	}
	return out, nil
}
