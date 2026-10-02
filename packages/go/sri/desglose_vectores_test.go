package sri

import (
	"bytes"
	"encoding/json"
	"flag"
	"math/rand/v2"
	"os"
	"testing"

	"github.com/shopspring/decimal"
)

var actualizarVectores = flag.Bool("actualizar", false, "reescribe packages/testdata/desglose-sri.json")

const archivoVectores = "../../testdata/desglose-sri.json"

type vLineaVenta struct {
	Cantidad   string `json:"cantidad"`
	Bruto      string `json:"bruto"`
	Final      string `json:"final"`
	Porcentaje string `json:"porcentaje"`
	Codigo     string `json:"codigoTarifa"`
}

type vVenta struct {
	Lineas     []vLineaVenta                `json:"lineas"`
	PorTarifa  map[string]map[string]string `json:"porTarifa"`
	Propina    string                       `json:"propina"`
	IncluyeIVA bool                         `json:"incluyeIva"`
	Total      string                       `json:"total"`
}

type vLineaFactura struct {
	Cantidad       string `json:"cantidad"`
	PrecioUnitario string `json:"precioUnitario"`
	Descuento      string `json:"descuento"`
	Total          string `json:"total"`
	IVA            string `json:"iva"`
}

type vFactura struct {
	Lineas            []vLineaFactura     `json:"lineas"`
	TotalSinImpuestos string              `json:"totalSinImpuestos"`
	TotalDescuento    string              `json:"totalDescuento"`
	Impuestos         []map[string]string `json:"impuestos"`
	Propina           string              `json:"propina"`
	ImporteTotal      string              `json:"importeTotal"`
	Ajustes           int                 `json:"ajustes"`
}

type vector struct {
	Nombre  string    `json:"nombre"`
	Venta   vVenta    `json:"venta"`
	Factura *vFactura `json:"factura,omitempty"`
	Error   bool      `json:"error,omitempty"`
}

func aVector(nombre string, v Venta) vector {
	out := vector{Nombre: nombre, Venta: vVenta{PorTarifa: map[string]map[string]string{}, Propina: v.Propina.String(), IncluyeIVA: v.IncluyeIVA, Total: v.Total.String()}}
	for _, l := range v.Lineas {
		out.Venta.Lineas = append(out.Venta.Lineas, vLineaVenta{Cantidad: l.Cantidad.String(), Bruto: l.Bruto.String(), Final: l.Final.String(),
			Porcentaje: l.Tarifa.Porcentaje, Codigo: l.Tarifa.Codigo})
	}
	for p, t := range v.PorTarifa {
		out.Venta.PorTarifa[p] = map[string]string{"base": t.Base.String(), "iva": t.IVA.String()}
	}
	f, err := Desglosar(v)
	if err != nil {
		out.Error = true
		return out
	}
	vf := &vFactura{TotalSinImpuestos: f.TotalSinImpuestos.String(), TotalDescuento: f.TotalDescuento.String(), Propina: f.Propina.String(),
		ImporteTotal: f.ImporteTotal.String(), Ajustes: f.Ajustes}
	for _, l := range f.Lineas {
		vf.Lineas = append(vf.Lineas, vLineaFactura{Cantidad: l.Cantidad.StringFixed(6), PrecioUnitario: l.PrecioUnitario.StringFixed(6),
			Descuento: l.Descuento.String(), Total: l.PrecioTotalSinImpuesto.String(), IVA: l.IVA.String()})
	}
	for _, i := range f.Impuestos {
		vf.Impuestos = append(vf.Impuestos, map[string]string{"porcentaje": i.Tarifa.Porcentaje, "codigo": i.Tarifa.Codigo, "base": i.BaseImponible.String(), "valor": i.Valor.String()})
	}
	out.Factura = vf
	return out
}

func generarVectores() []vector {
	var vs []vector
	vs = append(vs,
		aVector("plato de $15 con IVA incluido", totalesComoElNodo([]LineaVenta{linea("Ceviche", "1", "15.00", "15.00", iva15)}, true, false)),
		aVector("descuento, dos tarifas y servicio", totalesComoElNodo([]LineaVenta{
			linea("Parrillada", "1", "24.00", "21.60", iva15), linea("Agua", "2", "2.00", "2.00", iva0), linea("Cerveza", "3", "9.00", "9.00", iva15),
		}, true, true)),
		aVector("precios sin IVA", totalesComoElNodo([]LineaVenta{linea("Menú", "3", "10.00", "10.00", iva15), linea("Postre", "1", "3.33", "3.00", iva15)}, false, true)),
		aVector("cortesía completa", totalesComoElNodo([]LineaVenta{linea("Café", "1", "2.00", "0.00", iva15)}, true, false)),
		aVector("por peso", totalesComoElNodo([]LineaVenta{linea("Camarón", "0.375", "7.50", "7.50", iva15)}, true, false)),
	)
	// Inválidos: el TS debe rechazarlos igual.
	malo := totalesComoElNodo([]LineaVenta{linea("Ceviche", "1", "15.00", "15.00", iva15)}, true, false)
	malo.Total = m("15.01")
	vs = append(vs, aVector("el total no cuadra", malo))
	propina := totalesComoElNodo([]LineaVenta{linea("Ceviche", "1", "15.00", "15.00", iva15)}, true, false)
	propina.Propina, propina.Total = m("1.40"), m("16.40")
	vs = append(vs, aVector("propina sobre el 10 %", propina))
	r := rand.New(rand.NewPCG(4, 2026))
	for i := range 300 {
		vs = append(vs, aVector("al azar "+decimal.NewFromInt(int64(i+1)).String(), ventaAlAzar(r)))
	}
	return vs
}

// Los vectores compartidos con @restpos/sri (packages/testdata) son exactamente lo que
// produce este motor; con -actualizar se reescriben.
func TestVectoresDesgloseCompartidos(t *testing.T) {
	vs := generarVectores()
	b, err := json.MarshalIndent(vs, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	if *actualizarVectores {
		if err := os.WriteFile(archivoVectores, b, 0o644); err != nil { //nolint:gosec // vectores de prueba versionados
			t.Fatal(err)
		}
		return
	}
	actual, err := os.ReadFile(archivoVectores)
	if err != nil || !bytes.Equal(actual, b) {
		t.Fatalf("%s está desactualizado: go test ./packages/go/sri -run VectoresDesglose -actualizar", archivoVectores)
	}
	var errores int
	for _, v := range vs {
		if v.Error {
			errores++
		}
	}
	if errores != 2 {
		t.Fatalf("los dos casos inválidos deben fallar: %d", errores)
	}
}
