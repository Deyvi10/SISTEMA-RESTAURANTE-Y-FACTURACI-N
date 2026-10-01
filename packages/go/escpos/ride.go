package escpos

import (
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// Ride es la representación impresa de la factura electrónica (RIDE, F5-05) en formato
// ticket, con el contenido del Anexo 2 de la ficha técnica offline v2.34: emisor, número,
// número de autorización (= clave de acceso), ambiente y emisión, comprador, detalle,
// forma de pago, solo los subtotales con valor, IVA, propina y total. La fecha y hora de
// autorización no son obligatorias en el RIDE del emisor (Anexo 2); el código de barras es
// opcional (§9.20): la clave va en texto y en un QR.
type Ride struct {
	NombreComercial       string
	RazonSocial           string
	RUC                   string
	DirMatriz             string
	DirEstablecimiento    string
	ContribuyenteEspecial string
	ObligadoContabilidad  bool
	AgenteRetencion       string
	RIMPE                 bool
	Numero                string // 001-002-000000067
	ClaveAcceso           string
	Pruebas               bool // ambiente 1: «SIN VALIDEZ TRIBUTARIA»
	Emision               time.Time
	Mesa                  string
	Cajero                string
	Comprador             string
	CompradorID           string // vacío para consumidor final
	CompradorDireccion    string
	Lineas                []LineaRide
	Subtotales            []ValorRide // «SUBTOTAL 15%», «SUBTOTAL 0%»…: solo los que tienen valor
	SubtotalSinImpuestos  money.Money
	TotalDescuento        money.Money
	IVAs                  []ValorRide // «IVA 15%»
	Propina               money.Money
	Total                 money.Money
	Pagos                 []PagoRide
	Recibido              money.Money
	Vuelto                money.Money
	Adicionales           []ValorTexto // información adicional (correo, RUC del proveedor…)
	AbrirCajon            bool
	// Nota de crédito (F5-13): la factura que modifica, su fecha y la razón (Anexo 2).
	NotaCredito   bool
	Sustento      string // 001-002-000000067
	FechaSustento string // dd/mm/aaaa
	Motivo        string
}

// LineaRide es un detalle de la factura: valores sin IVA, como en el XML.
type LineaRide struct {
	Cantidad       string
	Descripcion    string
	PrecioUnitario string // hasta 6 decimales
	Descuento      money.Money
	Total          money.Money // precio total sin impuestos
}

type ValorRide struct {
	Etiqueta string
	Valor    money.Money
}

type ValorTexto struct{ Nombre, Valor string }

// PagoRide es una forma de pago con su código de la Tabla 24.
type PagoRide struct {
	Codigo   string // 01, 16, 19, 20…
	Metodo   string
	Monto    money.Money
	Ultimos4 string
}

func ImprimirRide(p Paper, r Ride) []byte {
	b := New(p)
	if r.AbrirCajon {
		b.OpenDrawer()
	}
	b.Align(Center)
	if r.NombreComercial != "" {
		b.Bold(true).Size(1, 2).Wrapped(r.NombreComercial, "").Size(1, 1).Bold(false)
	}
	b.Wrapped(r.RazonSocial, "")
	b.Line("R.U.C.: " + r.RUC)
	b.Align(Left)
	colgante(b, "Matriz: "+r.DirMatriz)
	if r.DirEstablecimiento != "" && r.DirEstablecimiento != r.DirMatriz {
		colgante(b, "Sucursal: "+r.DirEstablecimiento)
	}
	if r.ContribuyenteEspecial != "" {
		b.Line("Contribuyente Especial Nro: " + r.ContribuyenteEspecial)
	}
	b.Line("OBLIGADO A LLEVAR CONTABILIDAD: " + map[bool]string{true: "SI", false: "NO"}[r.ObligadoContabilidad])
	if r.AgenteRetencion != "" {
		b.Line("Agente de Retención Resolución No. " + r.AgenteRetencion)
	}
	if r.RIMPE {
		b.Line("CONTRIBUYENTE RÉGIMEN RIMPE")
	}
	b.Separator('-')
	titulo := "FACTURA"
	if r.NotaCredito {
		titulo = "NOTA DE CRÉDITO"
	}
	b.Align(Center).Bold(true).Size(1, 2).Wrapped(titulo, "").Size(1, 1).Line("No. " + r.Numero).Bold(false)
	b.Align(Left)
	b.Columns("Emitida", r.Emision.Format("02/01/2006 15:04"))
	b.Line("Ambiente: " + map[bool]string{true: "PRUEBAS", false: "PRODUCCIÓN"}[r.Pruebas])
	b.Line("Emisión: NORMAL")
	b.Wrapped("NÚMERO DE AUTORIZACIÓN / CLAVE DE ACCESO:", "")
	b.Align(Center).Wrapped(partirClave(r.ClaveAcceso, b.Width()), "")
	b.QR(r.ClaveAcceso, 5)
	b.Wrapped("Comprobante pendiente de autorización del SRI", "")
	if r.Pruebas {
		b.Bold(true).Invert(true).Wrapped(" AMBIENTE DE PRUEBAS - SIN VALIDEZ TRIBUTARIA ", "").Invert(false).Bold(false)
	}
	b.Align(Left).Separator('-')
	colgante(b, "Razón Social / Nombres: "+r.Comprador)
	if r.CompradorID != "" {
		b.Line("Identificación: " + r.CompradorID)
	}
	if r.CompradorDireccion != "" {
		colgante(b, "Dirección: "+r.CompradorDireccion)
	}
	if r.NotaCredito {
		b.Separator('-')
		colgante(b, "Comprobante que se modifica: FACTURA "+r.Sustento)
		b.Line("Fecha emisión del comprobante: " + r.FechaSustento)
		colgante(b, "Razón de modificación: "+r.Motivo)
	}
	b.Columns(r.Mesa, "Caja: "+r.Cajero)
	b.Separator('-')
	for _, l := range r.Lineas {
		colgante(b, l.Cantidad+" x "+l.Descripcion)
		det := "  P.U. " + l.PrecioUnitario
		if !l.Descuento.IsZero() {
			det += "  Desc. " + l.Descuento.String()
		}
		b.Columns(det, l.Total.String())
	}
	b.Separator('-')
	for _, s := range r.Subtotales {
		b.Columns(s.Etiqueta, s.Valor.String())
	}
	b.Columns("SUBTOTAL SIN IMPUESTOS", r.SubtotalSinImpuestos.String())
	if !r.TotalDescuento.IsZero() {
		b.Columns("TOTAL DESCUENTO", r.TotalDescuento.String())
	}
	for _, s := range r.IVAs {
		b.Columns(s.Etiqueta, s.Valor.String())
	}
	if !r.Propina.IsZero() {
		b.Columns("PROPINA", r.Propina.String())
	}
	b.Bold(true).Size(1, 2).Columns("VALOR TOTAL", "$"+r.Total.String()).Size(1, 1).Bold(false)
	if len(r.Pagos) > 0 {
		b.Separator('-').Line("Forma de pago")
	}
	for _, p := range r.Pagos {
		desc := p.Codigo + " - " + sri.FormasPago[p.Codigo]
		colgante(b, desc)
		nombre := "  " + p.Metodo
		if p.Ultimos4 != "" {
			nombre += " ****" + p.Ultimos4
		}
		b.Columns(nombre, "$"+p.Monto.String())
	}
	if !r.Recibido.IsZero() {
		b.Columns("Recibido", "$"+r.Recibido.String())
		b.Bold(true).Columns("Vuelto", "$"+r.Vuelto.String()).Bold(false)
	}
	if len(r.Adicionales) > 0 {
		b.Separator('-').Line("Información adicional")
		for _, a := range r.Adicionales {
			colgante(b, a.Nombre+": "+a.Valor)
		}
	}
	b.Separator('-').Align(Center)
	b.Wrapped("Consulte su comprobante en www.sri.gob.ec con la clave de acceso.", "")
	b.Line("¡Gracias por su visita!")
	return b.Feed(3).Cut(true).Bytes()
}

// partirClave corta los 49 dígitos en trozos parejos que caben en una línea (25 + 24 en
// papel de 80 mm) en vez de dejar un dígito suelto.
func partirClave(clave string, ancho int) string {
	if ancho <= 0 || len(clave) <= ancho {
		return clave
	}
	n := (len(clave) + ancho - 1) / ancho
	tam := (len(clave) + n - 1) / n
	var partes []string
	for len(clave) > tam {
		partes = append(partes, clave[:tam])
		clave = clave[tam:]
	}
	return strings.Join(append(partes, clave), " ")
}

// colgante escribe un párrafo con la primera línea al margen y las siguientes sangradas.
func colgante(b *Builder, s string) {
	for i, l := range Wrap(s, b.Width()-2) {
		if i > 0 {
			l = "  " + l
		}
		b.Line(l)
	}
}
