// Package cierrez es el Cierre Z de un turno de caja (RF-04-10): conteo por denominación,
// esperado contra declarado por método de pago y un hash encadenado por caja que lo hace
// verificable. Lo calcula el Nodo Local; la nube lo guarda, lo verifica y lo envía al dueño.
package cierrez

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// Resultado de un método o del cierre completo.
const (
	Cuadrado = "CUADRADO"
	Sobrante = "SOBRANTE"
	Faltante = "FALTANTE"
)

// Denominacion es un billete o moneda de dólar que circula en Ecuador.
type Denominacion struct {
	Clave    string      `json:"clave"` // B20, M0.25…
	Valor    money.Money `json:"valor"`
	Moneda   bool        `json:"moneda"`
	Etiqueta string      `json:"etiqueta"`
}

// Denominaciones en el orden del asistente: billetes de mayor a menor y luego monedas.
var Denominaciones = []Denominacion{
	{"B100", money.MustParse("100"), false, "$100"},
	{"B50", money.MustParse("50"), false, "$50"},
	{"B20", money.MustParse("20"), false, "$20"},
	{"B10", money.MustParse("10"), false, "$10"},
	{"B5", money.MustParse("5"), false, "$5"},
	{"B1", money.MustParse("1"), false, "$1"},
	{"M1", money.MustParse("1"), true, "$1"},
	{"M0.50", money.MustParse("0.50"), true, "50¢"},
	{"M0.25", money.MustParse("0.25"), true, "25¢"},
	{"M0.10", money.MustParse("0.10"), true, "10¢"},
	{"M0.05", money.MustParse("0.05"), true, "5¢"},
	{"M0.01", money.MustParse("0.01"), true, "1¢"},
}

var porClave = func() map[string]Denominacion {
	out := map[string]Denominacion{}
	for _, d := range Denominaciones {
		out[d.Clave] = d
	}
	return out
}()

// Conteo es cuántas piezas de una denominación declaró el cajero.
type Conteo struct {
	Clave    string `json:"clave"`
	Cantidad int    `json:"cantidad"`
}

// TotalConteo suma el efectivo contado. Rechaza denominaciones que no existen, repetidas o
// cantidades fuera de rango.
func TotalConteo(cs []Conteo) (money.Money, error) {
	var total money.Money
	vistas := map[string]bool{}
	for _, c := range cs {
		d, ok := porClave[c.Clave]
		if !ok {
			return money.Money{}, fmt.Errorf("denominación desconocida %q", c.Clave)
		}
		if vistas[c.Clave] {
			return money.Money{}, fmt.Errorf("denominación repetida %q", c.Clave)
		}
		vistas[c.Clave] = true
		if c.Cantidad < 0 || c.Cantidad > 100_000 {
			return money.Money{}, fmt.Errorf("cantidad fuera de rango para %s", d.Etiqueta)
		}
		total = total.Add(d.Valor.Mul(decimal.NewFromInt(int64(c.Cantidad))))
	}
	return total.Round2(), nil
}

// Metodo de pago que participa en el cierre.
type Metodo struct {
	ID     ids.ID
	Nombre string
	Tipo   string // EFECTIVO, TARJETA_CREDITO…
}

// Movimientos del turno que mueven el efectivo esperado.
type Movimientos struct {
	FondoInicial money.Money `json:"fondoInicial"`
	Ingresos     money.Money `json:"ingresos"`
	Retiros      money.Money `json:"retiros"`
	Gastos       money.Money `json:"gastos"`
}

// Linea es el resultado de un método de pago.
type Linea struct {
	MetodoID   ids.ID      `json:"metodoId"`
	Metodo     string      `json:"metodo"`
	Tipo       string      `json:"tipo"`
	Cobrado    money.Money `json:"cobrado"`
	Esperado   money.Money `json:"esperado"`
	Declarado  money.Money `json:"declarado"`
	Diferencia money.Money `json:"diferencia"`
	Resultado  string      `json:"resultado"`
}

func resultadoDe(dif money.Money) string {
	switch {
	case dif.IsZero():
		return Cuadrado
	case dif.IsNegative():
		return Faltante
	}
	return Sobrante
}

// Calcular compara lo esperado con lo declarado por método (RF-04-10.3):
// efectivo esperado = fondo + cobrado en efectivo + ingresos − retiros − gastos; los demás
// métodos esperan lo cobrado. El resultado global es Faltante si falta en algún método,
// Sobrante si solo sobra y Cuadrado si todo coincide.
func Calcular(metodos []Metodo, mov Movimientos, cobrado map[ids.ID]money.Money, efectivoContado money.Money, declarado map[ids.ID]money.Money) ([]Linea, string) {
	lineas := make([]Linea, 0, len(metodos))
	global := Cuadrado
	for _, m := range metodos {
		l := Linea{MetodoID: m.ID, Metodo: m.Nombre, Tipo: m.Tipo, Cobrado: cobrado[m.ID].Round2()}
		if m.Tipo == "EFECTIVO" {
			l.Esperado = money.Sum(mov.FondoInicial, l.Cobrado, mov.Ingresos).Sub(mov.Retiros).Sub(mov.Gastos).Round2()
			l.Declarado = efectivoContado.Round2()
		} else {
			l.Esperado = l.Cobrado
			l.Declarado = declarado[m.ID].Round2()
		}
		l.Diferencia = l.Declarado.Sub(l.Esperado)
		l.Resultado = resultadoDe(l.Diferencia)
		if l.Resultado == Faltante || (l.Resultado == Sobrante && global == Cuadrado) {
			global = l.Resultado
		}
		lineas = append(lineas, l)
	}
	return lineas, global
}

// Cierre es el Cierre Z inmutable de un turno.
type Cierre struct {
	ID            ids.ID      `json:"id"`
	TurnoID       ids.ID      `json:"turnoId"`
	CajaID        ids.ID      `json:"cajaId"`
	JornadaID     ids.ID      `json:"jornadaId"`
	Caja          string      `json:"caja"`
	Local         string      `json:"local"`
	Numero        int         `json:"numero"` // secuencial por caja
	CajeroID      ids.ID      `json:"cajeroId"`
	Cajero        string      `json:"cajero"`
	CerradoPor    string      `json:"cerradoPor"`
	FechaNegocio  string      `json:"fechaNegocio"`
	AbiertoAt     time.Time   `json:"abiertoAt"`
	CerradoAt     time.Time   `json:"cerradoAt"`
	Movimientos               // fondo, ingresos, retiros y gastos
	Conteo        []Conteo    `json:"conteo"`
	EfectivoTotal money.Money `json:"efectivoContado"`
	Lineas        []Linea     `json:"lineas"`
	Resultado     string      `json:"resultado"`
	HashAnterior  string      `json:"hashAnterior"`
	Hash          string      `json:"hash"`
}

// CalcularHash: SHA-256 del cierre en JSON sin su propio hash (incluye el del anterior de la
// misma caja, así un cierre borrado o alterado rompe la cadena).
func (c Cierre) CalcularHash() string {
	c.Hash = ""
	// Horas en UTC: el JSON es el mismo aquí y en la nube tras el viaje.
	c.AbiertoAt, c.CerradoAt = c.AbiertoAt.UTC(), c.CerradoAt.UTC()
	b, err := json.Marshal(c)
	if err != nil {
		panic(err) // solo tipos serializables
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// Verificar confirma que el hash corresponde al contenido.
func (c Cierre) Verificar() bool { return c.Hash != "" && c.Hash == c.CalcularHash() }
