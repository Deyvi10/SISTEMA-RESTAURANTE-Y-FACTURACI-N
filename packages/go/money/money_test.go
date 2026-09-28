package money

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

func TestRound2HalfUp(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0.005", "0.01"},
		{"0.004999", "0.00"},
		{"1.125", "1.13"},
		{"1.135", "1.14"},
		{"2.675", "2.68"}, // con float64 daría 2.67
		{"-0.005", "-0.01"},
		{"13.043478", "13.04"},
	}
	for _, c := range cases {
		if got := MustParse(c.in).Round2().String(); got != c.want {
			t.Errorf("Round2(%s) = %s, se esperaba %s", c.in, got, c.want)
		}
	}
}

func TestParseRejectsFormatting(t *testing.T) {
	for _, s := range []string{"", "$14.50", "1,250.50", "abc", "14,50"} {
		if _, err := Parse(s); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("Parse(%q) debería fallar con ErrInvalidAmount, obtuvo %v", s, err)
		}
	}
}

func TestClassicFloatTrap(t *testing.T) {
	// 0.1 + 0.2 = 0.30000000000000004 en float64
	if got := MustParse("0.1").Add(MustParse("0.2")); !got.Equal(MustParse("0.3")) {
		t.Fatalf("0.1 + 0.2 = %s", got.Decimal())
	}
}

func TestJSONRoundTrip(t *testing.T) {
	in := struct{ Total Money }{MustParse("16.30")}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"Total":"16.3"}` {
		t.Fatalf("JSON inesperado: %s", b)
	}
	var out struct{ Total Money }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Total.Equal(in.Total) {
		t.Fatalf("ida y vuelta: %s != %s", out.Total, in.Total)
	}
}

func TestSplitEqual(t *testing.T) {
	cases := []struct {
		total string
		n     int
		want  []string
	}{
		{"10.00", 3, []string{"3.33", "3.33", "3.34"}},
		{"14.50", 4, []string{"3.62", "3.62", "3.62", "3.64"}},
		{"0.01", 3, []string{"0.00", "0.00", "0.01"}},
		{"-10.00", 3, []string{"-3.33", "-3.33", "-3.34"}},
		{"7.00", 1, []string{"7.00"}},
	}
	for _, c := range cases {
		got, err := SplitEqual(MustParse(c.total), c.n)
		if err != nil {
			t.Fatal(err)
		}
		for i := range got {
			if got[i].String() != c.want[i] {
				t.Errorf("SplitEqual(%s,%d)[%d] = %s, se esperaba %s", c.total, c.n, i, got[i], c.want[i])
			}
		}
	}
	if _, err := SplitEqual(MustParse("1"), 0); !errors.Is(err, ErrInvalidParts) {
		t.Errorf("n=0 debería fallar")
	}
}

func TestAllocateLargestRemainder(t *testing.T) {
	w := []decimal.Decimal{decimal.NewFromInt(1), decimal.NewFromInt(1), decimal.NewFromInt(1)}
	got, err := Allocate(MustParse("100.00"), w)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"33.34", "33.33", "33.33"} // el centavo extra va al índice menor en empate
	for i := range got {
		if got[i].String() != want[i] {
			t.Errorf("Allocate[%d] = %s, se esperaba %s", i, got[i], want[i])
		}
	}

	// Pesos proporcionales a precios: 13.04 y 2.61 reparten 1.30 de descuento
	got, _ = Allocate(MustParse("1.30"), []decimal.Decimal{decimal.RequireFromString("13.04"), decimal.RequireFromString("2.61")})
	if got[0].String() != "1.08" || got[1].String() != "0.22" {
		t.Errorf("Allocate proporcional = %v", got)
	}

	for _, bad := range [][]decimal.Decimal{nil, {decimal.Zero}, {decimal.NewFromInt(-1), decimal.NewFromInt(2)}} {
		if _, err := Allocate(MustParse("1"), bad); !errors.Is(err, ErrInvalidWeights) {
			t.Errorf("pesos %v deberían fallar", bad)
		}
	}
}

// Propiedad central del dinero: repartir nunca crea ni pierde un centavo.
func TestPropertySplitAndAllocatePreserveTotal(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cents := rapid.Int64Range(-10_000_000, 10_000_000).Draw(t, "cents")
		total := FromCents(cents)

		n := rapid.IntRange(1, 40).Draw(t, "n")
		parts, err := SplitEqual(total, n)
		if err != nil {
			t.Fatal(err)
		}
		if got := Sum(parts...); !got.Equal(total) {
			t.Fatalf("SplitEqual: suma %s != total %s", got, total)
		}
		// Ninguna parte difiere de otra en más de n-1 centavos (el residuo va a la última)
		for _, p := range parts[:n-1] {
			if p.Cents() != parts[0].Cents() {
				t.Fatalf("partes no iguales: %v", parts)
			}
		}

		weights := rapid.SliceOfN(rapid.Int64Range(0, 1_000_000), 1, 30).Draw(t, "weights")
		ws := make([]decimal.Decimal, len(weights))
		var sumW int64
		for i, w := range weights {
			ws[i] = decimal.New(w, -2)
			sumW += w
		}
		if sumW == 0 {
			return
		}
		alloc, err := Allocate(total, ws)
		if err != nil {
			t.Fatal(err)
		}
		if got := Sum(alloc...); !got.Equal(total) {
			t.Fatalf("Allocate: suma %s != total %s (pesos %v)", got, total, weights)
		}
		for i, w := range weights {
			if w == 0 && !alloc[i].IsZero() {
				t.Fatalf("peso cero recibió %s", alloc[i])
			}
		}
	})
}
