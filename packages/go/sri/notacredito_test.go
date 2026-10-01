package sri

import (
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// ventaAlAzar es el generador de QA-05: cantidades enteras y por peso, descuentos, cortesías,
// dos tarifas, con o sin IVA incluido y propina.
func ventaAlAzar(r *rand.Rand) Venta {
	var lineas []LineaVenta
	for range 1 + r.IntN(8) {
		cant := decimal.NewFromInt(int64(1 + r.IntN(4)))
		if r.IntN(6) == 0 {
			cant = decimal.New(int64(50+r.IntN(1500)), -3)
		}
		precio := decimal.New(int64(25+r.IntN(6000)), -2)
		bruto := money.FromDecimal(precio.Mul(cant)).Round2()
		final := bruto
		if r.IntN(4) == 0 {
			final = money.FromDecimal(bruto.Decimal().Mul(decimal.New(int64(50+r.IntN(50)), -2))).Round2()
		}
		tarifa := iva15
		if r.IntN(5) == 0 {
			tarifa = iva0
		}
		lineas = append(lineas, LineaVenta{Codigo: "P", Descripcion: "plato", Cantidad: cant, Bruto: bruto, Final: final, Tarifa: tarifa})
	}
	return totalesComoElNodo(lineas, r.IntN(4) != 0, r.IntN(2) == 0)
}

var clienteNC = Comprador{TipoIdentificacion: "05", Identificacion: "1710034065", RazonSocial: "María Pérez"}

func facturaLeida(t *testing.T, v Venta, sec int64) FacturaLeida {
	t.Helper()
	f, err := Desglosar(v)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := FacturaXML(datosPrueba(t, f, sec, consumidorFinal), f)
	if err != nil {
		t.Fatal(err)
	}
	l, err := LeerFactura(doc)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func datosNC(t *testing.T, sustento FacturaLeida, sec int64) DatosNC {
	t.Helper()
	fecha := time.Date(2026, 10, 1, 10, 0, 0, 0, clock.Guayaquil)
	clave, err := NuevaClaveAcceso(ClaveAccesoInput{FechaEmision: fecha, TipoComprobante: TipoNotaCredito, RUC: emisorPrueba.RUC,
		Ambiente: AmbientePruebas, Establecimiento: "001", PuntoEmision: "002", Secuencial: sec, CodigoNumerico: "87654321"})
	if err != nil {
		t.Fatal(err)
	}
	return DatosNC{Ambiente: AmbientePruebas, ClaveAcceso: clave, Secuencial: sec, Fecha: fecha, Emisor: emisorPrueba,
		Comprador: clienteNC, Sustento: sustento, Motivo: "Devolución del plato"}
}

// Propiedad: devoluciones parciales al azar hasta revertir todo. Nunca se pasa de lo que queda
// de cada línea; la suma de las notas de una línea es exactamente la línea de la factura; la
// suma de todas las notas es el saldo inicial (la factura sin la propina); cada nota valida
// contra el XSD oficial de la NC 1.1.0.
func TestNCDevolucionesAlAzarHastaRevertirTodo(t *testing.T) {
	v := nuevoValidador(t)
	r := rand.New(rand.NewPCG(13, 2026))
	var xmls [][]byte
	for caso := range 400 {
		f := facturaLeida(t, ventaAlAzar(r), int64(caso+1))
		inicial, err := Saldo(f, nil)
		if err != nil {
			t.Fatal(err)
		}
		var previas []Revertido
		suma := money.Zero
		for paso := 0; ; paso++ {
			saldo, _ := Saldo(f, previas)
			if saldo.IsZero() && paso > 0 {
				break
			}
			todo := paso >= 3 || r.IntN(4) == 0
			var dev []Devolucion
			if !todo {
				ya := acumular(previas)
				for i, d := range f.Detalles {
					q := decimal.RequireFromString(d.Cantidad)
					resto := q.Sub(ya[i].Cantidad)
					if !resto.IsPositive() || r.IntN(2) == 0 {
						continue
					}
					c := resto
					if r.IntN(2) == 0 {
						c = resto.Mul(decimal.New(int64(1+r.IntN(99)), -2)).Round(3)
						if !c.IsPositive() {
							c = resto
						}
					}
					dev = append(dev, Devolucion{Indice: i, Cantidad: c})
				}
				if len(dev) == 0 {
					todo = true
				}
			}
			nc, err := CalcularNC(f, dev, previas, todo)
			if err != nil {
				if saldo.IsZero() {
					break // la factura era una cortesía completa
				}
				t.Fatalf("caso %d paso %d: %v", caso, paso, err)
			}
			for _, l := range nc.Lineas {
				calculado := money.FromDecimal(l.Cantidad.Mul(l.PrecioUnitario)).Round2().Sub(l.Descuento)
				if !calculado.Equal(l.PrecioTotalSinImpuesto) {
					t.Fatalf("caso %d: cantidad × precio − descuento = %s, total %s", caso, calculado, l.PrecioTotalSinImpuesto)
				}
				previas = append(previas, Revertido{Indice: l.Indice, Cantidad: l.Cantidad, Total: l.PrecioTotalSinImpuesto, Descuento: l.Descuento, IVA: l.IVA})
			}
			suma = suma.Add(nc.ValorModificacion)
			if suma.GreaterThan(inicial) {
				t.Fatalf("caso %d: las notas (%s) superan la factura (%s)", caso, suma, inicial)
			}
			if len(xmls) < 300 {
				doc, err := NotaCreditoXML(datosNC(t, f, int64(len(xmls)+1)), nc)
				if err != nil {
					t.Fatalf("caso %d: %v", caso, err)
				}
				xmls = append(xmls, doc)
			}
			if todo {
				if !nc.Total {
					t.Fatalf("caso %d: la nota de todo lo que queda debía cerrar la factura", caso)
				}
				break
			}
		}
		if !suma.Equal(inicial) {
			t.Fatalf("caso %d: las notas suman %s y la factura (sin propina) %s", caso, suma, inicial)
		}
		for i, d := range f.Detalles {
			total, iva, _ := totalesDetalle(d)
			ya := acumular(previas)[i]
			if !ya.Total.Equal(total) || !ya.IVA.Equal(iva) || !ya.Cantidad.Equal(decimal.RequireFromString(d.Cantidad)) {
				t.Fatalf("caso %d línea %d: revertido %+v de %s + %s", caso, i, ya, total, iva)
			}
		}
	}
	if err := v.validar(t, "NotaCredito_V1.1.0.xsd", xmls...); err != nil {
		t.Fatalf("el XSD oficial rechaza una nota de crédito: %v", err)
	}
	t.Logf("%d notas de crédito validadas contra el XSD", len(xmls))
}

func TestNCReglas(t *testing.T) {
	f := facturaLeida(t, totalesComoElNodo([]LineaVenta{linea("Ceviche", "2", "30.00", "30.00", iva15), linea("Agua", "1", "1.00", "1.00", iva0)}, true, true), 7)
	nc, err := CalcularNC(f, []Devolucion{{Indice: 0, Cantidad: d("1")}}, nil, false)
	if err != nil || nc.Total || len(nc.Lineas) != 1 {
		t.Fatalf("parcial: %+v %v", nc, err)
	}
	datos := datosNC(t, f, 1)
	doc, err := NotaCreditoXML(datos, nc)
	if err != nil {
		t.Fatal(err)
	}
	for _, quiero := range []string{"<codDocModificado>01</codDocModificado>", "<numDocModificado>001-002-000000007</numDocModificado>",
		"<fechaEmisionDocSustento>29/09/2026</fechaEmisionDocSustento>", "<motivo>Devolución del plato</motivo>", "<valorModificacion>" + nc.ValorModificacion.String() + "</valorModificacion>"} {
		if !strings.Contains(string(doc), quiero) {
			t.Errorf("el XML no trae %s", quiero)
		}
	}
	leida, err := LeerComprobante(doc)
	if err != nil || !leida.EsNotaCredito() || leida.NumDocModificado != "001-002-000000007" || leida.Motivo != "Devolución del plato" ||
		leida.ImporteTotal != nc.ValorModificacion.String() || leida.Numero() != "001-002-000000001" || leida.Impuestos[0].Tarifa != "15.00" {
		t.Fatalf("leer la NC: %+v %v", leida, err)
	}
	if _, err := LeerFactura(doc); err == nil {
		t.Fatal("una nota de crédito no se lee como factura")
	}
	casos := map[string]func() error{
		"pasarse de la cantidad": func() error {
			_, err := CalcularNC(f, []Devolucion{{Indice: 0, Cantidad: d("3")}}, nil, false)
			return err
		},
		"línea inexistente": func() error {
			_, err := CalcularNC(f, []Devolucion{{Indice: 9, Cantidad: d("1")}}, nil, false)
			return err
		},
		"línea repetida": func() error {
			_, err := CalcularNC(f, []Devolucion{{Indice: 0, Cantidad: d("1")}, {Indice: 0, Cantidad: d("1")}}, nil, false)
			return err
		},
		"cantidad cero": func() error {
			_, err := CalcularNC(f, []Devolucion{{Indice: 1, Cantidad: d("0")}}, nil, false)
			return err
		},
		"consumidor final": func() error {
			x := datos
			x.Comprador = consumidorFinal
			_, err := NotaCreditoXML(x, nc)
			return err
		},
		"sin motivo": func() error {
			x := datos
			x.Motivo = "  "
			_, err := NotaCreditoXML(x, nc)
			return err
		},
	}
	for nombre, f := range casos {
		if err := f(); !errors.Is(err, ErrNotaCredito) {
			t.Errorf("%s: %v", nombre, err)
		}
	}
	// Después de revertir todo, no queda nada.
	todo, err := CalcularNC(f, nil, nil, true)
	if err != nil || !todo.Total {
		t.Fatal(err)
	}
	var previas []Revertido
	for _, l := range todo.Lineas {
		previas = append(previas, Revertido{Indice: l.Indice, Cantidad: l.Cantidad, Total: l.PrecioTotalSinImpuesto, Descuento: l.Descuento, IVA: l.IVA})
	}
	if _, err := CalcularNC(f, nil, previas, true); !errors.Is(err, ErrNotaCredito) {
		t.Fatal("una factura ya revertida no admite otra nota")
	}
	if s, _ := Saldo(f, previas); !s.IsZero() {
		t.Fatalf("saldo %s", s)
	}
}

func TestElValidadorRechazaUnaNCMala(t *testing.T) {
	v := nuevoValidador(t)
	f := facturaLeida(t, totalesComoElNodo([]LineaVenta{linea("Ceviche", "1", "15.00", "15.00", iva15)}, true, false), 3)
	nc, _ := CalcularNC(f, nil, nil, true)
	doc, err := NotaCreditoXML(datosNC(t, f, 1), nc)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.validar(t, "NotaCredito_V1.1.0.xsd", doc); err != nil {
		t.Fatalf("la buena: %v", err)
	}
	malo := strings.Replace(string(doc), "<numDocModificado>001-002-000000003</numDocModificado>", "<numDocModificado>001002000000003</numDocModificado>", 1)
	if err := v.validar(t, "NotaCredito_V1.1.0.xsd", []byte(malo)); err == nil {
		t.Fatal("un número de documento modificado sin guiones debía fallar")
	}
}
