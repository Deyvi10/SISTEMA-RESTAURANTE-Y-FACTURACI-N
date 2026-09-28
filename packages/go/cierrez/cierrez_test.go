package cierrez

import (
	"encoding/json"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

var m = money.MustParse

func TestTotalDelConteo(t *testing.T) {
	total, err := TotalConteo([]Conteo{{Clave: "B20", Cantidad: 3}, {Clave: "B1", Cantidad: 2}, {Clave: "M1", Cantidad: 1}, {Clave: "M0.25", Cantidad: 3}, {Clave: "M0.01", Cantidad: 4}})
	if err != nil || !total.Equal(m("63.79")) {
		t.Fatalf("total = %s, %v", total, err)
	}
	if _, err := TotalConteo([]Conteo{{Clave: "B3", Cantidad: 1}}); err == nil {
		t.Fatal("aceptó un billete de $3")
	}
	if _, err := TotalConteo([]Conteo{{Clave: "B5", Cantidad: -1}}); err == nil {
		t.Fatal("aceptó una cantidad negativa")
	}
	if _, err := TotalConteo([]Conteo{{Clave: "B5", Cantidad: 1}, {Clave: "B5", Cantidad: 2}}); err == nil {
		t.Fatal("aceptó la misma denominación dos veces")
	}
}

func TestCalcularPorMetodo(t *testing.T) {
	efectivo, tarjeta, transf := ids.New(), ids.New(), ids.New()
	metodos := []Metodo{{ID: efectivo, Nombre: "Efectivo", Tipo: "EFECTIVO"}, {ID: tarjeta, Nombre: "Tarjeta crédito", Tipo: "TARJETA_CREDITO"}, {ID: transf, Nombre: "Transferencia", Tipo: "TRANSFERENCIA"}}
	mov := Movimientos{FondoInicial: m("50"), Ingresos: m("10"), Retiros: m("100"), Gastos: m("3.50")}
	cobrado := map[ids.ID]money.Money{efectivo: m("120.40"), tarjeta: m("35.40")}
	// Esperado en efectivo: 50 + 120.40 + 10 − 100 − 3.50 = 76.90.
	lineas, res := Calcular(metodos, mov, cobrado, m("76.90"), map[ids.ID]money.Money{tarjeta: m("30.40"), transf: m("2")})
	if res != Faltante {
		t.Fatalf("resultado global %s", res)
	}
	want := map[string][4]string{
		"Efectivo":        {"76.90", "76.90", "0.00", Cuadrado},
		"Tarjeta crédito": {"35.40", "30.40", "-5.00", Faltante},
		"Transferencia":   {"0.00", "2.00", "2.00", Sobrante},
	}
	if len(lineas) != 3 {
		t.Fatalf("lineas: %+v", lineas)
	}
	for _, l := range lineas {
		w := want[l.Metodo]
		if l.Esperado.String() != w[0] || l.Declarado.String() != w[1] || l.Diferencia.String() != w[2] || l.Resultado != w[3] {
			t.Fatalf("%s: %+v, quería %v", l.Metodo, l, w)
		}
	}
	// Todo cuadra: verde. Solo sobrantes: sobrante.
	if _, r := Calcular(metodos[:1], mov, cobrado, m("76.90"), nil); r != Cuadrado {
		t.Fatalf("cuadrado: %s", r)
	}
	if _, r := Calcular(metodos[:1], mov, cobrado, m("80"), nil); r != Sobrante {
		t.Fatalf("sobrante: %s", r)
	}
}

// Propiedad: la suma de diferencias es siempre declarado − esperado, al centavo.
func TestCuadreAlCentavo(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	cent := func() money.Money { return money.FromCents(r.Int64N(100_000)) }
	for range 1000 {
		ms := []Metodo{{ID: ids.New(), Nombre: "Efectivo", Tipo: "EFECTIVO"}, {ID: ids.New(), Nombre: "T", Tipo: "TARJETA_DEBITO"}}
		mov := Movimientos{FondoInicial: cent(), Ingresos: cent(), Retiros: cent(), Gastos: cent()}
		cob := map[ids.ID]money.Money{ms[0].ID: cent(), ms[1].ID: cent()}
		dec := map[ids.ID]money.Money{ms[1].ID: cent()}
		ef := cent()
		lineas, _ := Calcular(ms, mov, cob, ef, dec)
		var esp, decl, dif money.Money
		for _, l := range lineas {
			esp, decl, dif = esp.Add(l.Esperado), decl.Add(l.Declarado), dif.Add(l.Diferencia)
		}
		if !dif.Equal(decl.Sub(esp)) {
			t.Fatalf("no cuadra: %s ≠ %s − %s", dif, decl, esp)
		}
	}
}

func TestHashEncadenadoYVerificable(t *testing.T) {
	c := Cierre{ID: ids.New(), TurnoID: ids.New(), CajaID: ids.New(), Caja: "Caja 1", Numero: 1, Cajero: "Luis P.",
		FondoInicial: m("50"), CerradoAt: time.Date(2026, 9, 26, 3, 0, 0, 0, time.UTC), Resultado: Cuadrado}
	c.Hash = c.CalcularHash()
	if len(c.Hash) != 64 || !c.Verificar() {
		t.Fatalf("hash %q", c.Hash)
	}
	// Viaja como JSON a la nube y se verifica igual.
	b, _ := json.Marshal(c)
	var d Cierre
	if err := json.Unmarshal(b, &d); err != nil || !d.Verificar() {
		t.Fatalf("no se verifica tras JSON: %v", err)
	}
	// Cualquier cambio rompe el hash.
	d.FondoInicial = m("40")
	if d.Verificar() {
		t.Fatal("un fondo alterado pasó la verificación")
	}
	// El siguiente se encadena con el anterior.
	s := Cierre{ID: ids.New(), CajaID: c.CajaID, Numero: 2, HashAnterior: c.Hash}
	s.Hash = s.CalcularHash()
	s2 := s
	s2.HashAnterior = ""
	if s2.CalcularHash() == s.Hash {
		t.Fatal("el hash no depende del anterior")
	}
}
