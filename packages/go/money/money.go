// Package money representa importes en dólares con aritmética decimal exacta.
//
// Reglas del proyecto (docs/05 §8 y hallazgo X-16):
//   - Nunca se usa float para dinero ni cantidades.
//   - Los importes se redondean a 2 decimales con half-up (0,005 → 0,01).
//   - Los precios unitarios admiten hasta 6 decimales (ficha técnica del SRI).
//   - Un reparto nunca crea ni pierde centavos: la suma de las partes es el total.
package money

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// Escalas usadas en el sistema.
const (
	// CentScale es la escala de importes: NUMERIC(12,2).
	CentScale int32 = 2
	// UnitPriceScale es la escala de precios unitarios: NUMERIC(18,6).
	UnitPriceScale int32 = 6
)

// Errores de dominio.
var (
	ErrInvalidAmount  = errors.New("money: importe inválido")
	ErrInvalidParts   = errors.New("money: el número de partes debe ser mayor que cero")
	ErrInvalidWeights = errors.New("money: los pesos deben ser no negativos y sumar más que cero")
)

// Money es un importe decimal exacto. El valor cero es $0 y es válido.
type Money struct {
	d decimal.Decimal
}

// Zero es $0,00.
var Zero = Money{}

// Parse interpreta un importe escrito con punto decimal ("14.50").
// No acepta separadores de miles ni símbolos: eso es formato de presentación.
func Parse(s string) (Money, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Zero, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}
	return Money{d: d}, nil
}

// MustParse es Parse para constantes en código y pruebas; entra en pánico si falla.
func MustParse(s string) Money {
	m, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return m
}

// FromCents construye un importe a partir de centavos enteros.
func FromCents(cents int64) Money {
	return Money{d: decimal.New(cents, -CentScale)}
}

// FromDecimal envuelve un decimal existente (por ejemplo, leído de la base de datos).
func FromDecimal(d decimal.Decimal) Money { return Money{d: d} }

// Decimal devuelve el valor subyacente.
func (m Money) Decimal() decimal.Decimal { return m.d }

func (m Money) Add(o Money) Money { return Money{d: m.d.Add(o.d)} }
func (m Money) Sub(o Money) Money { return Money{d: m.d.Sub(o.d)} }
func (m Money) Neg() Money        { return Money{d: m.d.Neg()} }

// Mul multiplica por un factor decimal exacto (cantidad, porcentaje, fracción).
// El resultado NO se redondea; redondear es una decisión explícita del llamador.
func (m Money) Mul(f decimal.Decimal) Money { return Money{d: m.d.Mul(f)} }

// Round2 redondea a centavos con half-up (half away from zero).
func (m Money) Round2() Money { return Money{d: m.d.Round(CentScale)} }

// RoundUnit redondea a la escala de precio unitario (6 decimales).
func (m Money) RoundUnit() Money { return Money{d: m.d.Round(UnitPriceScale)} }

// Cents devuelve el importe en centavos. Solo es exacto si ya está redondeado a 2 decimales.
func (m Money) Cents() int64 { return m.d.Shift(CentScale).Round(0).IntPart() }

func (m Money) IsZero() bool          { return m.d.IsZero() }
func (m Money) IsNegative() bool      { return m.d.IsNegative() }
func (m Money) Cmp(o Money) int       { return m.d.Cmp(o.d) }
func (m Money) Equal(o Money) bool    { return m.d.Equal(o.d) }
func (m Money) LessThan(o Money) bool { return m.d.LessThan(o.d) }
func (m Money) GreaterThan(o Money) bool {
	return m.d.GreaterThan(o.d)
}

// String devuelve el importe con 2 decimales y punto ("14.50"), el formato del XML del SRI.
func (m Money) String() string { return m.d.StringFixed(CentScale) }

// StringUnit devuelve el precio unitario con 6 decimales ("13.043478").
func (m Money) StringUnit() string { return m.d.StringFixed(UnitPriceScale) }

// MarshalText permite serializar a JSON como cadena, sin pérdida de precisión.
func (m Money) MarshalText() ([]byte, error) { return []byte(m.d.String()), nil }

// UnmarshalText lee un importe serializado como cadena.
func (m *Money) UnmarshalText(b []byte) error {
	v, err := Parse(string(b))
	if err != nil {
		return err
	}
	*m = v
	return nil
}

// Sum suma una lista de importes.
func Sum(ms ...Money) Money {
	total := Zero
	for _, m := range ms {
		total = total.Add(m)
	}
	return total
}
