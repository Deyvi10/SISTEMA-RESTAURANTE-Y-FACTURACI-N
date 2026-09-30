package sri

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// validadorXSD valida contra el XSD oficial con xmllint (en CI) o con lxml de Python.
// Sin ninguno de los dos la prueba se salta: en CI se instala libxml2-utils.
type validadorXSD struct {
	dir string
	cmd func(esquema string, archivos ...string) *exec.Cmd
}

func nuevoValidador(t *testing.T) *validadorXSD {
	t.Helper()
	dir := t.TempDir()
	if err := fs.WalkDir(XSD, "xsd", func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		b, err := XSD.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, filepath.Base(p)), b, 0o600)
	}); err != nil {
		t.Fatal(err)
	}
	v := &validadorXSD{dir: dir}
	if ruta, err := exec.LookPath("xmllint"); err == nil {
		v.cmd = func(esquema string, archivos ...string) *exec.Cmd {
			return exec.Command(ruta, append([]string{"--noout", "--nonet", "--schema", esquema}, archivos...)...)
		}
		return v
	}
	if py, err := exec.LookPath("python3"); err == nil && exec.Command(py, "-c", "import lxml").Run() == nil {
		const script = `import sys
from lxml import etree
p = etree.XMLParser(no_network=True, load_dtd=False, resolve_entities=False)
esquema = etree.XMLSchema(etree.parse(sys.argv[1], p))
malos = 0
for archivo in sys.argv[2:]:
    if not esquema.validate(etree.parse(archivo, p)):
        print(archivo, esquema.error_log); malos += 1
sys.exit(1 if malos else 0)`
		v.cmd = func(esquema string, archivos ...string) *exec.Cmd {
			return exec.Command(py, append([]string{"-c", script, esquema}, archivos...)...)
		}
		return v
	}
	t.Skip("sin xmllint ni lxml para validar contra el XSD")
	return nil
}

func (v *validadorXSD) validar(t *testing.T, esquema string, docs ...[]byte) error {
	t.Helper()
	dir := t.TempDir()
	archivos := make([]string, len(docs))
	for i, doc := range docs {
		archivos[i] = filepath.Join(dir, fmt.Sprintf("comprobante-%04d.xml", i))
		if err := os.WriteFile(archivos[i], doc, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var salida bytes.Buffer
	c := v.cmd(filepath.Join(v.dir, esquema), archivos...)
	c.Stdout, c.Stderr = &salida, &salida
	if err := c.Run(); err != nil {
		return errors.New(strings.TrimSpace(salida.String()))
	}
	return nil
}

var emisorPrueba = Emisor{
	RUC: "1710034065001", RazonSocial: "PÉREZ ANDRADE JOSÉ LUIS", NombreComercial: "Cevichería Don Pepe",
	DirMatriz: "Malecón 2000 y Av. 9 de Octubre, Guayaquil", DirEstablecimiento: "Malecón 2000 y Av. 9 de Octubre",
	RIMPE: true, Establecimiento: "001", PuntoEmision: "002",
}

func datosPrueba(t *testing.T, f Factura, sec int64, comprador Comprador) DatosFactura {
	t.Helper()
	fecha := time.Date(2026, 9, 29, 13, 30, 0, 0, clock.Guayaquil)
	clave, err := NuevaClaveAcceso(ClaveAccesoInput{FechaEmision: fecha, TipoComprobante: TipoFactura, RUC: emisorPrueba.RUC,
		Ambiente: AmbientePruebas, Establecimiento: "001", PuntoEmision: "002", Secuencial: sec, CodigoNumerico: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	return DatosFactura{Ambiente: AmbientePruebas, ClaveAcceso: clave, Secuencial: sec, Fecha: fecha, Emisor: emisorPrueba,
		Comprador: comprador, Pagos: []Pago{{FormaPago: "01", Total: f.ImporteTotal}},
		Adicionales: []CampoAdicional{{Nombre: "Mesa", Valor: "Mesa 4"}}}
}

var consumidorFinal = Comprador{TipoIdentificacion: "07", Identificacion: "9999999999999", RazonSocial: "CONSUMIDOR FINAL"}

func TestFacturaXMLValidaContraElXSDOficial(t *testing.T) {
	v := nuevoValidador(t)
	casos := map[string]struct {
		venta     Venta
		comprador Comprador
	}{
		"plato de $15 a consumidor final": {totalesComoElNodo([]LineaVenta{linea("Ceviche de camarón", "1", "15.00", "15.00", iva15)}, true, false), consumidorFinal},
		"con RUC, descuento, dos tarifas y servicio": {totalesComoElNodo([]LineaVenta{
			linea("Parrillada\npara dos", "1", "24.00", "21.60", iva15), // salto de línea: el XSD no lo admite
			linea("Agua sin gas", "2", "2.00", "2.00", iva0),
			linea("Cerveza", "3", "9.00", "9.00", iva15),
		}, true, true), Comprador{TipoIdentificacion: "04", Identificacion: "1790011674001", RazonSocial: "Distribuidora del Pacífico S.A.", Direccion: "Av. Amazonas N34-120"}},
	}
	for nombre, c := range casos {
		f, err := Desglosar(c.venta)
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		doc, err := FacturaXML(datosPrueba(t, f, 67, c.comprador), f)
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		if err := v.validar(t, "factura_V1.1.0.xsd", doc); err != nil {
			t.Fatalf("%s: el XSD oficial la rechaza:\n%s\n%s", nombre, err, doc)
		}
	}
}

// El validador de verdad valida: una factura con la clave de 48 dígitos no pasa.
func TestElValidadorRechazaUnXMLMalo(t *testing.T) {
	v := nuevoValidador(t)
	f, _ := Desglosar(totalesComoElNodo([]LineaVenta{linea("Ceviche", "1", "15.00", "15.00", iva15)}, true, false))
	doc, err := FacturaXML(datosPrueba(t, f, 1, consumidorFinal), f)
	if err != nil {
		t.Fatal(err)
	}
	malo := bytes.Replace(doc, []byte("<secuencial>000000001</secuencial>"), []byte("<secuencial>00000001</secuencial>"), 1)
	if err := v.validar(t, "factura_V1.1.0.xsd", malo); err == nil {
		t.Fatal("un secuencial de 8 dígitos debía fallar contra el XSD")
	}
}

// QA-12: todos los casos del generador de QA-05 (una muestra de 300) validan contra el XSD.
func TestQA12MuestraAleatoriaValida(t *testing.T) {
	v := nuevoValidador(t)
	r := rand.New(rand.NewPCG(12, 2026))
	var docs [][]byte
	for i := range 300 {
		var lineas []LineaVenta
		for range 1 + r.IntN(10) {
			cant := decimal.NewFromInt(int64(1 + r.IntN(4)))
			if r.IntN(6) == 0 {
				cant = decimal.New(int64(50+r.IntN(1500)), -3)
			}
			bruto := money.FromDecimal(decimal.New(int64(25+r.IntN(6000)), -2).Mul(cant)).Round2()
			final := bruto
			if r.IntN(4) == 0 {
				final = money.FromDecimal(bruto.Decimal().Mul(decimal.New(int64(50+r.IntN(50)), -2))).Round2()
			}
			tarifa := iva15
			if r.IntN(5) == 0 {
				tarifa = iva0
			}
			lineas = append(lineas, LineaVenta{Codigo: "PLT-" + string(rune('A'+r.IntN(26))), Descripcion: "Plato con ñ y tildes: ají, jalapeño & «salsa»", Cantidad: cant, Bruto: bruto, Final: final, Tarifa: tarifa})
		}
		f, err := Desglosar(totalesComoElNodo(lineas, r.IntN(4) != 0, r.IntN(2) == 0))
		if err != nil {
			t.Fatalf("venta %d: %v", i, err)
		}
		doc, err := FacturaXML(datosPrueba(t, f, int64(i+1), consumidorFinal), f)
		if err != nil {
			t.Fatalf("venta %d: %v", i, err)
		}
		docs = append(docs, doc)
	}
	// Una sola llamada al validador para las 300 (lanzar uno por factura tardaba ~40 s).
	if err := v.validar(t, "factura_V1.1.0.xsd", docs...); err != nil {
		t.Fatal(err)
	}
}

func TestFacturaRechazaDatosIncoherentes(t *testing.T) {
	f, _ := Desglosar(totalesComoElNodo([]LineaVenta{linea("Ceviche", "1", "15.00", "15.00", iva15)}, true, false))
	for nombre, cambiar := range map[string]func(*DatosFactura){
		"clave de otro secuencial": func(d *DatosFactura) { d.Secuencial = 2 },
		"pagos que no suman":       func(d *DatosFactura) { d.Pagos[0].Total = m("14.99") },
		"forma de pago inexistente": func(d *DatosFactura) {
			d.Pagos[0].FormaPago = "99"
		},
		"consumidor final con cédula": func(d *DatosFactura) { d.Comprador.Identificacion = "1710034065" },
	} {
		d := datosPrueba(t, f, 1, consumidorFinal)
		cambiar(&d)
		if _, err := FacturaXML(d, f); !errors.Is(err, ErrFactura) {
			t.Errorf("%s: se esperaba ErrFactura, llegó %v", nombre, err)
		}
	}
}
