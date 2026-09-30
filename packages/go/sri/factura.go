package sri

import (
	"embed"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// XSD oficiales, sin modificar (docs/fuentes/sri/): factura y nota de crédito 1.1.0 y el
// esquema de firma XML del W3C que ambos importan. El worker fiscal valida contra ellos.
//
//go:embed xsd/*.xsd
var XSD embed.FS

// VersionFactura es la versión de esquema elegida en F0-14 (6 decimales en precio y cantidad).
const VersionFactura = "1.1.0"

// Leyenda RIMPE que admite el XSD 1.1.0 en <contribuyenteRimpe> (Anexo 22 de la ficha).
const LeyendaRIMPE = "CONTRIBUYENTE RÉGIMEN RIMPE"

// Formas de pago de la Tabla 24 de la ficha técnica offline v2.34.
var formasPago = map[string]bool{"01": true, "15": true, "16": true, "17": true, "18": true, "19": true, "20": true, "21": true}

// Emisor son los datos tributarios del restaurante y del punto de emisión.
type Emisor struct {
	RUC                   string
	RazonSocial           string
	NombreComercial       string
	DirMatriz             string
	DirEstablecimiento    string
	ObligadoContabilidad  bool
	ContribuyenteEspecial string // número de resolución, vacío si no es
	AgenteRetencion       string // número de resolución, vacío si no es
	RIMPE                 bool
	Establecimiento       string // "001"
	PuntoEmision          string // "002"
}

// Comprador de la factura. Consumidor final: tipo 07 e identificación 9999999999999.
type Comprador struct {
	TipoIdentificacion string // Tabla 6: 04 RUC, 05 cédula, 06 pasaporte, 07 consumidor final, 08 exterior
	Identificacion     string
	RazonSocial        string
	Direccion          string
}

// Pago con su código de la Tabla 24.
type Pago struct {
	FormaPago string
	Total     money.Money
}

// CampoAdicional va en <infoAdicional> (hasta 15): correo del comprador, mesa, etc.
type CampoAdicional struct {
	Nombre string
	Valor  string
}

// DatosFactura es todo lo necesario para el XML, además del desglose.
type DatosFactura struct {
	Ambiente    Ambiente
	ClaveAcceso ClaveAcceso
	Secuencial  int64
	Fecha       time.Time // en la zona del local
	Emisor      Emisor
	Comprador   Comprador
	Pagos       []Pago
	Adicionales []CampoAdicional
}

var ErrFactura = errors.New("sri: factura inválida")

// FacturaXML arma el XML de la factura 1.1.0 (sin firmar) en el orden exacto del XSD.
func FacturaXML(d DatosFactura, f Factura) ([]byte, error) {
	if err := validarDatos(d, f); err != nil {
		return nil, err
	}
	x := xmlFactura{ID: "comprobante", Version: VersionFactura}
	e := d.Emisor
	x.InfoTributaria = xmlInfoTributaria{
		Ambiente: fmt.Sprint(int(d.Ambiente)), TipoEmision: TipoEmisionNormal,
		RazonSocial: texto(e.RazonSocial, 300), NombreComercial: texto(e.NombreComercial, 300),
		RUC: e.RUC, ClaveAcceso: d.ClaveAcceso.String(), CodDoc: TipoFactura,
		Estab: e.Establecimiento, PtoEmi: e.PuntoEmision, Secuencial: fmt.Sprintf("%09d", d.Secuencial),
		DirMatriz: texto(e.DirMatriz, 300), AgenteRetencion: e.AgenteRetencion,
	}
	if e.RIMPE {
		x.InfoTributaria.ContribuyenteRimpe = LeyendaRIMPE
	}
	obligado := "NO"
	if e.ObligadoContabilidad {
		obligado = "SI"
	}
	inf := xmlInfoFactura{
		FechaEmision:                d.Fecha.Format("02/01/2006"),
		DirEstablecimiento:          texto(e.DirEstablecimiento, 300),
		ContribuyenteEspecial:       e.ContribuyenteEspecial,
		ObligadoContabilidad:        obligado,
		TipoIdentificacionComprador: d.Comprador.TipoIdentificacion,
		RazonSocialComprador:        texto(d.Comprador.RazonSocial, 300),
		IdentificacionComprador:     d.Comprador.Identificacion,
		DireccionComprador:          texto(d.Comprador.Direccion, 300),
		TotalSinImpuestos:           f.TotalSinImpuestos.String(),
		TotalDescuento:              f.TotalDescuento.String(),
		ImporteTotal:                f.ImporteTotal.String(),
		Moneda:                      "DOLAR",
	}
	for _, imp := range f.Impuestos {
		inf.TotalConImpuestos = append(inf.TotalConImpuestos, xmlTotalImpuesto{
			Codigo: CodigoImpuestoIVA, CodigoPorcentaje: imp.Tarifa.Codigo, BaseImponible: imp.BaseImponible.String(),
			Tarifa: tarifaXML(imp.Tarifa), Valor: imp.Valor.String(),
		})
	}
	if !f.Propina.IsZero() {
		inf.Propina = f.Propina.String()
	} else {
		inf.Propina = "0.00"
	}
	if len(d.Pagos) > 0 {
		inf.Pagos = &xmlPagos{}
		for _, p := range d.Pagos {
			inf.Pagos.Pagos = append(inf.Pagos.Pagos, xmlPago{FormaPago: p.FormaPago, Total: p.Total.String()})
		}
	}
	x.InfoFactura = inf
	for _, l := range f.Lineas {
		x.Detalles = append(x.Detalles, xmlDetalle{
			CodigoPrincipal: texto(l.Codigo, 25), Descripcion: texto(l.Descripcion, 300),
			Cantidad: l.Cantidad.StringFixed(6), PrecioUnitario: l.PrecioUnitario.StringFixed(6),
			Descuento: l.Descuento.String(), PrecioTotalSinImpuesto: l.PrecioTotalSinImpuesto.String(),
			Impuestos: []xmlImpuesto{{Codigo: CodigoImpuestoIVA, CodigoPorcentaje: l.Tarifa.Codigo, Tarifa: tarifaXML(l.Tarifa),
				BaseImponible: l.BaseImponible.String(), Valor: l.IVA.String()}},
		})
	}
	for _, a := range d.Adicionales {
		if v := texto(a.Valor, 300); v != "" {
			if x.InfoAdicional == nil {
				x.InfoAdicional = &xmlInfoAdicional{}
			}
			x.InfoAdicional.Campos = append(x.InfoAdicional.Campos, xmlCampoAdicional{Nombre: texto(a.Nombre, 300), Valor: v})
		}
	}
	out, err := xml.Marshal(x)
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

func validarDatos(d DatosFactura, f Factura) error {
	mal := func(format string, a ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrFactura}, a...)...)
	}
	if d.Ambiente != AmbientePruebas && d.Ambiente != AmbienteProduccion {
		return mal("ambiente %d", d.Ambiente)
	}
	if d.ClaveAcceso.TipoComprobante() != TipoFactura || d.ClaveAcceso.RUC() != d.Emisor.RUC || d.ClaveAcceso.Ambiente() != d.Ambiente ||
		d.ClaveAcceso.Serie() != d.Emisor.Establecimiento+d.Emisor.PuntoEmision || d.ClaveAcceso.Secuencial() != fmt.Sprintf("%09d", d.Secuencial) ||
		d.ClaveAcceso.FechaEmision() != d.Fecha.Format("02012006") {
		return mal("la clave de acceso no corresponde a esta factura")
	}
	if r := ValidarRUCEmisor(d.Emisor.RUC); !r.Valida {
		return mal("RUC del emisor: %s", r.Motivo)
	}
	switch c := d.Comprador; c.TipoIdentificacion {
	case "07":
		if c.Identificacion != "9999999999999" {
			return mal("consumidor final va con 13 nueves")
		}
	case "04", "05", "06", "08":
		if c.Identificacion == "" || utf8.RuneCountInString(c.Identificacion) > 20 {
			return mal("identificación del comprador")
		}
	default:
		return mal("tipo de identificación %q del comprador", c.TipoIdentificacion)
	}
	if strings.TrimSpace(d.Comprador.RazonSocial) == "" || strings.TrimSpace(d.Emisor.RazonSocial) == "" || strings.TrimSpace(d.Emisor.DirMatriz) == "" {
		return mal("faltan la razón social o la dirección")
	}
	if len(f.Lineas) == 0 {
		return mal("sin detalles")
	}
	suma := money.Money{}
	for _, p := range d.Pagos {
		if !formasPago[p.FormaPago] {
			return mal("forma de pago %q (Tabla 24)", p.FormaPago)
		}
		suma = suma.Add(p.Total)
	}
	if len(d.Pagos) > 0 && !suma.Equal(f.ImporteTotal) {
		return mal("los pagos suman %s y el importe es %s", suma, f.ImporteTotal)
	}
	if len(d.Adicionales) > 15 {
		return mal("más de 15 campos adicionales")
	}
	return nil
}

// texto deja una línea sin saltos (el XSD exige [^\n]*) y la corta al largo máximo.
func texto(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max])
	}
	return s
}

func tarifaXML(t Tarifa) string {
	p, err := decimal.NewFromString(t.Porcentaje)
	if err != nil {
		return t.Porcentaje
	}
	return p.StringFixed(2)
}

// Estructura XML en el orden del XSD factura 1.1.0 (los opcionales vacíos se omiten).
type xmlFactura struct {
	XMLName        xml.Name          `xml:"factura"`
	ID             string            `xml:"id,attr"`
	Version        string            `xml:"version,attr"`
	InfoTributaria xmlInfoTributaria `xml:"infoTributaria"`
	InfoFactura    xmlInfoFactura    `xml:"infoFactura"`
	Detalles       []xmlDetalle      `xml:"detalles>detalle"`
	// Puntero: sin campos no debe salir <infoAdicional/> vacío (el XSD exige al menos uno).
	InfoAdicional *xmlInfoAdicional `xml:"infoAdicional,omitempty"`
}

type xmlInfoAdicional struct {
	Campos []xmlCampoAdicional `xml:"campoAdicional"`
}

type xmlPagos struct {
	Pagos []xmlPago `xml:"pago"`
}

type xmlInfoTributaria struct {
	Ambiente           string `xml:"ambiente"`
	TipoEmision        string `xml:"tipoEmision"`
	RazonSocial        string `xml:"razonSocial"`
	NombreComercial    string `xml:"nombreComercial,omitempty"`
	RUC                string `xml:"ruc"`
	ClaveAcceso        string `xml:"claveAcceso"`
	CodDoc             string `xml:"codDoc"`
	Estab              string `xml:"estab"`
	PtoEmi             string `xml:"ptoEmi"`
	Secuencial         string `xml:"secuencial"`
	DirMatriz          string `xml:"dirMatriz"`
	AgenteRetencion    string `xml:"agenteRetencion,omitempty"`
	ContribuyenteRimpe string `xml:"contribuyenteRimpe,omitempty"`
}

type xmlInfoFactura struct {
	FechaEmision                string             `xml:"fechaEmision"`
	DirEstablecimiento          string             `xml:"dirEstablecimiento,omitempty"`
	ContribuyenteEspecial       string             `xml:"contribuyenteEspecial,omitempty"`
	ObligadoContabilidad        string             `xml:"obligadoContabilidad"`
	TipoIdentificacionComprador string             `xml:"tipoIdentificacionComprador"`
	RazonSocialComprador        string             `xml:"razonSocialComprador"`
	IdentificacionComprador     string             `xml:"identificacionComprador"`
	DireccionComprador          string             `xml:"direccionComprador,omitempty"`
	TotalSinImpuestos           string             `xml:"totalSinImpuestos"`
	TotalDescuento              string             `xml:"totalDescuento"`
	TotalConImpuestos           []xmlTotalImpuesto `xml:"totalConImpuestos>totalImpuesto"`
	Propina                     string             `xml:"propina"`
	ImporteTotal                string             `xml:"importeTotal"`
	Moneda                      string             `xml:"moneda"`
	Pagos                       *xmlPagos          `xml:"pagos,omitempty"` // sin pagos no sale <pagos/> vacío
}

type xmlTotalImpuesto struct {
	Codigo           string `xml:"codigo"`
	CodigoPorcentaje string `xml:"codigoPorcentaje"`
	BaseImponible    string `xml:"baseImponible"`
	Tarifa           string `xml:"tarifa"`
	Valor            string `xml:"valor"`
}

type xmlPago struct {
	FormaPago string `xml:"formaPago"`
	Total     string `xml:"total"`
}

type xmlDetalle struct {
	CodigoPrincipal        string        `xml:"codigoPrincipal,omitempty"`
	Descripcion            string        `xml:"descripcion"`
	Cantidad               string        `xml:"cantidad"`
	PrecioUnitario         string        `xml:"precioUnitario"`
	Descuento              string        `xml:"descuento"`
	PrecioTotalSinImpuesto string        `xml:"precioTotalSinImpuesto"`
	Impuestos              []xmlImpuesto `xml:"impuestos>impuesto"`
}

type xmlImpuesto struct {
	Codigo           string `xml:"codigo"`
	CodigoPorcentaje string `xml:"codigoPorcentaje"`
	Tarifa           string `xml:"tarifa"`
	BaseImponible    string `xml:"baseImponible"`
	Valor            string `xml:"valor"`
}

type xmlCampoAdicional struct {
	Nombre string `xml:"nombre,attr"`
	Valor  string `xml:",chardata"`
}
