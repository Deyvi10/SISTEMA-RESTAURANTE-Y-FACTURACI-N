package ride

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/pdf/pdfprueba"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

var (
	iva15 = sri.Tarifa{Porcentaje: "15", Codigo: "4"}
	iva0  = sri.Tarifa{Porcentaje: "0", Codigo: "0"}
)

// factura arma un XML como el que emite el nodo: n platos de $5 con IVA 15 % incluido, una
// botella de agua con tarifa 0 % y el servicio del 10 %.
func factura(t *testing.T, platos int, ambiente sri.Ambiente) []byte {
	t.Helper()
	var lineas []sri.LineaVenta
	total15 := money.Zero
	for i := range platos {
		desc := fmt.Sprintf("Plato %d", i+1)
		if i == 0 {
			desc = "Parrillada mixta para compartir con papas fritas, ensalada de la casa y chimichurri"
		}
		lineas = append(lineas, sri.LineaVenta{Codigo: fmt.Sprintf("P%03d", i+1), Descripcion: desc, Cantidad: decimal.NewFromInt(1),
			Bruto: money.MustParse("5.75"), Final: money.MustParse("5.75"), Tarifa: iva15})
		total15 = total15.Add(money.MustParse("5.75"))
	}
	lineas = append(lineas, sri.LineaVenta{Codigo: "AGUA", Descripcion: "Agua sin gas", Cantidad: decimal.NewFromInt(2),
		Bruto: money.MustParse("2.00"), Final: money.MustParse("2.00"), Tarifa: iva0})
	base15 := money.MustParse(decimal.RequireFromString(total15.String()).Div(decimal.RequireFromString("1.15")).StringFixedBank(2))
	iva := total15.Sub(base15)
	propina := money.MustParse(decimal.RequireFromString(base15.Add(money.MustParse("2.00")).String()).Mul(decimal.RequireFromString("0.10")).RoundDown(2).StringFixed(2))
	total := total15.Add(money.MustParse("2.00")).Add(propina)
	f, err := sri.Desglosar(sri.Venta{Lineas: lineas, IncluyeIVA: true, Propina: propina, Total: total,
		PorTarifa: map[string]sri.TotalTarifa{"15": {Base: base15, IVA: iva}, "0": {Base: money.MustParse("2.00"), IVA: money.Zero}}})
	if err != nil {
		t.Fatal(err)
	}
	fecha := time.Date(2026, 9, 29, 13, 30, 0, 0, clock.Guayaquil)
	clave, err := sri.NuevaClaveAcceso(sri.ClaveAccesoInput{FechaEmision: fecha, TipoComprobante: "01", RUC: "1790011674001", Ambiente: ambiente,
		Establecimiento: "001", PuntoEmision: "002", Secuencial: 67, CodigoNumerico: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := sri.FacturaXML(sri.DatosFactura{Ambiente: ambiente, ClaveAcceso: clave, Secuencial: 67, Fecha: fecha,
		Emisor: sri.Emisor{RUC: "1790011674001", RazonSocial: "DISTRIBUIDORA DEL PACIFICO S.A.", NombreComercial: "Cevichería Don Pepe",
			DirMatriz: "Malecón 2000 y Av. 9 de Octubre, Guayaquil", Establecimiento: "001", PuntoEmision: "002", ObligadoContabilidad: true},
		Comprador: sri.Comprador{TipoIdentificacion: "05", Identificacion: "1710034065", RazonSocial: "María Pérez", Direccion: "Quito"},
		Pagos:     []sri.Pago{{FormaPago: "19", Total: f.ImporteTotal}},
		Adicionales: []sri.CampoAdicional{{Nombre: "Email", Valor: "maria@example.com"}, {Nombre: "Mesa", Valor: "Mesa 4"},
			{Nombre: "RUC Proveedor", Valor: "1790011674001"}}}, f)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestRIDEAnexo2(t *testing.T) {
	doc := factura(t, 3, sri.AmbienteProduccion)
	fecha := time.Date(2026, 9, 29, 18, 31, 2, 0, time.UTC)
	b, err := PDF(doc, Autorizacion{Fecha: &fecha})
	if err != nil {
		t.Fatal(err)
	}
	if otro, _ := PDF(doc, Autorizacion{Fecha: &fecha}); !bytes.Equal(b, otro) {
		t.Fatal("el RIDE debe ser determinista: el mismo XML da el mismo PDF")
	}
	if os.Getenv("RIDE_PDF") != "" {
		_ = os.WriteFile(os.Getenv("RIDE_PDF"), b, 0o600)
	}
	l, _ := sri.LeerFactura(doc)
	if txt := pdfprueba.Texto(t, b); txt != "" {
		for _, quiero := range []string{"R.U.C.:", "1790011674001", "FACTURA", "001-002-000000067", "NÚMERO DE AUTORIZACIÓN", l.ClaveAcceso,
			"29/09/2026 13:31:02", "PRODUCCIÓN", "NORMAL", "OBLIGADO A LLEVAR CONTABILIDAD", "María Pérez", "1710034065", "29/09/2026",
			"Parrillada mixta", "SUBTOTAL 15%", "SUBTOTAL 0%", "SUBTOTAL SIN IMPUESTOS", "IVA 15%", "PROPINA", "VALOR TOTAL", l.ImporteTotal,
			"19 - TARJETA DE CREDITO", "RUC Proveedor", "maria@example.com"} {
			if !strings.Contains(txt, quiero) {
				t.Errorf("el RIDE no trae %q", quiero)
			}
		}
		if strings.Contains(txt, "SIN VALIDEZ") || strings.Contains(txt, "NO OBJETO") || strings.Contains(txt, "EXENTO") {
			t.Error("en producción y sin esos subtotales no se muestran")
		}
	}
	if leidos := pdfprueba.Barras(t, b); leidos != nil && (len(leidos) != 1 || !strings.HasSuffix(leidos[0], "|"+l.ClaveAcceso)) {
		t.Fatalf("código de barras: %v", leidos)
	}
}

func TestRIDEPendienteEnPruebasYLargo(t *testing.T) {
	doc := factura(t, 70, sri.AmbientePruebas)
	b, err := PDF(doc, Autorizacion{})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("RIDE_LARGO") != "" {
		_ = os.WriteFile(os.Getenv("RIDE_LARGO"), b, 0o600)
	}
	paginas := 0
	_, _ = fmt.Sscanf(string(b[bytes.Index(b, []byte("/Count ")):]), "/Count %d", &paginas)
	if paginas < 2 {
		t.Fatalf("70 platos no caben en una página: %d", paginas)
	}
	if txt := pdfprueba.Texto(t, b); txt != "" {
		for _, quiero := range []string{"PRUEBAS", "AMBIENTE DE PRUEBAS - SIN VALIDEZ TRIBUTARIA", "Pendiente de autorización del SRI", "Plato 70", "VALOR TOTAL"} {
			if !strings.Contains(txt, quiero) {
				t.Errorf("no trae %q", quiero)
			}
		}
		if strings.Count(txt, "FACTURA No. 001-002-000000067") != paginas-1 {
			t.Error("cada hoja de continuación dice de qué factura es")
		}
		// Cada página con platos repite el encabezado del detalle (el pie puede quedar solo).
		if n := strings.Count(txt, "Precio Unitario"); n < paginas-1 || n > paginas {
			t.Errorf("encabezados del detalle: %d", strings.Count(txt, "Precio Unitario"))
		}
	}
	if _, err := PDF([]byte("<notaCredito/>"), Autorizacion{}); err == nil {
		t.Fatal("solo facturas")
	}
}

// F5-13: el RIDE de la nota de crédito sigue el Anexo 2 (pág. 61).
func TestRIDENotaDeCredito(t *testing.T) {
	factura, err := sri.LeerFactura(factura(t, 2, sri.AmbientePruebas))
	if err != nil {
		t.Fatal(err)
	}
	nc, err := sri.CalcularNC(factura, []sri.Devolucion{{Indice: 0, Cantidad: decimal.NewFromInt(1)}}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	fecha := time.Date(2026, 10, 1, 9, 0, 0, 0, clock.Guayaquil)
	clave, _ := sri.NuevaClaveAcceso(sri.ClaveAccesoInput{FechaEmision: fecha, TipoComprobante: "04", RUC: "1790011674001", Ambiente: sri.AmbientePruebas,
		Establecimiento: "001", PuntoEmision: "002", Secuencial: 3, CodigoNumerico: "12345678"})
	doc, err := sri.NotaCreditoXML(sri.DatosNC{Ambiente: sri.AmbientePruebas, ClaveAcceso: clave, Secuencial: 3, Fecha: fecha,
		Emisor:    sri.Emisor{RUC: "1790011674001", RazonSocial: "DISTRIBUIDORA DEL PACIFICO S.A.", DirMatriz: "Guayaquil", Establecimiento: "001", PuntoEmision: "002"},
		Comprador: sri.Comprador{TipoIdentificacion: "05", Identificacion: "1710034065", RazonSocial: "María Pérez"},
		Sustento:  factura, Motivo: "El plato llegó frío"}, nc)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PDF(doc, Autorizacion{})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("RIDE_NC") != "" {
		_ = os.WriteFile(os.Getenv("RIDE_NC"), b, 0o600)
	}
	if txt := pdfprueba.Texto(t, b); txt != "" {
		for _, quiero := range []string{"NOTA DE CRÉDITO", "001-002-000000003", "Comprobante que se modifica:", "FACTURA 001-002-000000067",
			"Fecha Emisión (Comprobante a modificar):", "29/09/2026", "Razón de Modificación:", "El plato llegó frío", "VALOR TOTAL", nc.ValorModificacion.String()} {
			if !strings.Contains(txt, quiero) {
				t.Errorf("el RIDE de la NC no trae %q", quiero)
			}
		}
		if strings.Contains(txt, "Forma de pago") {
			t.Error("la NC no lleva forma de pago")
		}
	}
}
