package sri

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// FacturaLeida es lo que dice un XML de factura 1.1.0 (firmado o no), tal cual: el RIDE y la
// nota de crédito se arman desde el comprobante y no desde los datos de la venta.
type FacturaLeida struct {
	Ambiente              string
	RazonSocial           string
	NombreComercial       string
	RUC                   string
	ClaveAcceso           string
	CodDoc                string
	Estab, PtoEmi         string
	Secuencial            string
	DirMatriz             string
	AgenteRetencion       string
	ContribuyenteRimpe    string
	FechaEmision          string // dd/mm/aaaa
	DirEstablecimiento    string
	ContribuyenteEspecial string
	ObligadoContabilidad  string
	TipoIdComprador       string
	RazonSocialComprador  string
	IdComprador           string
	DireccionComprador    string
	TotalSinImpuestos     string
	TotalDescuento        string
	Impuestos             []ImpuestoLeido
	Propina               string
	ImporteTotal          string
	Pagos                 []PagoLeido
	Detalles              []DetalleLeido
	Adicionales           []CampoLeido
}

type ImpuestoLeido struct{ Codigo, CodigoPorcentaje, Tarifa, BaseImponible, Valor string }

type PagoLeido struct{ FormaPago, Total string }

type DetalleLeido struct {
	CodigoPrincipal, Descripcion, Cantidad, PrecioUnitario, Descuento, PrecioTotalSinImpuesto string
	Impuestos                                                                                 []ImpuestoLeido
}

type CampoLeido struct{ Nombre, Valor string }

// Numero es la serie y el secuencial como se imprimen: 001-002-000000067.
func (f FacturaLeida) Numero() string { return f.Estab + "-" + f.PtoEmi + "-" + f.Secuencial }

// LeerFactura interpreta un XML de factura (la firma, si la tiene, se ignora).
func LeerFactura(doc []byte) (FacturaLeida, error) {
	var x xmlFactura
	if err := xml.Unmarshal(doc, &x); err != nil {
		return FacturaLeida{}, fmt.Errorf("sri: el XML no es una factura: %w", err)
	}
	if x.InfoTributaria.CodDoc != "01" {
		return FacturaLeida{}, fmt.Errorf("sri: el comprobante %q no es una factura", x.InfoTributaria.CodDoc)
	}
	it, inf := x.InfoTributaria, x.InfoFactura
	f := FacturaLeida{
		Ambiente: it.Ambiente, RazonSocial: it.RazonSocial, NombreComercial: it.NombreComercial, RUC: it.RUC,
		ClaveAcceso: strings.TrimSpace(it.ClaveAcceso), CodDoc: it.CodDoc, Estab: it.Estab, PtoEmi: it.PtoEmi, Secuencial: it.Secuencial,
		DirMatriz: it.DirMatriz, AgenteRetencion: it.AgenteRetencion, ContribuyenteRimpe: it.ContribuyenteRimpe,
		FechaEmision: inf.FechaEmision, DirEstablecimiento: inf.DirEstablecimiento, ContribuyenteEspecial: inf.ContribuyenteEspecial,
		ObligadoContabilidad: inf.ObligadoContabilidad, TipoIdComprador: inf.TipoIdentificacionComprador,
		RazonSocialComprador: inf.RazonSocialComprador, IdComprador: inf.IdentificacionComprador, DireccionComprador: inf.DireccionComprador,
		TotalSinImpuestos: inf.TotalSinImpuestos, TotalDescuento: inf.TotalDescuento, Propina: inf.Propina, ImporteTotal: inf.ImporteTotal,
	}
	for _, t := range inf.TotalConImpuestos {
		f.Impuestos = append(f.Impuestos, ImpuestoLeido{Codigo: t.Codigo, CodigoPorcentaje: t.CodigoPorcentaje, Tarifa: t.Tarifa, BaseImponible: t.BaseImponible, Valor: t.Valor})
	}
	if inf.Pagos != nil {
		for _, p := range inf.Pagos.Pagos {
			f.Pagos = append(f.Pagos, PagoLeido(p))
		}
	}
	for _, d := range x.Detalles {
		dl := DetalleLeido{CodigoPrincipal: d.CodigoPrincipal, Descripcion: d.Descripcion, Cantidad: d.Cantidad,
			PrecioUnitario: d.PrecioUnitario, Descuento: d.Descuento, PrecioTotalSinImpuesto: d.PrecioTotalSinImpuesto}
		for _, i := range d.Impuestos {
			dl.Impuestos = append(dl.Impuestos, ImpuestoLeido{Codigo: i.Codigo, CodigoPorcentaje: i.CodigoPorcentaje, Tarifa: i.Tarifa,
				BaseImponible: i.BaseImponible, Valor: i.Valor})
		}
		f.Detalles = append(f.Detalles, dl)
	}
	if x.InfoAdicional != nil {
		for _, c := range x.InfoAdicional.Campos {
			f.Adicionales = append(f.Adicionales, CampoLeido{Nombre: c.Nombre, Valor: strings.TrimSpace(c.Valor)})
		}
	}
	return f, nil
}

// Adicional devuelve un campo adicional por nombre (sin distinguir mayúsculas).
func (f FacturaLeida) Adicional(nombre string) string {
	for _, c := range f.Adicionales {
		if strings.EqualFold(c.Nombre, nombre) {
			return c.Valor
		}
	}
	return ""
}

// FormasPago son las descripciones de la Tabla 24 de la ficha técnica offline v2.34.
var FormasPago = map[string]string{
	"01": "SIN UTILIZACION DEL SISTEMA FINANCIERO", "15": "COMPENSACION DE DEUDAS", "16": "TARJETA DE DEBITO",
	"17": "DINERO ELECTRONICO", "18": "TARJETA PREPAGO", "19": "TARJETA DE CREDITO",
	"20": "OTROS CON UTILIZACION DEL SISTEMA FINANCIERO", "21": "ENDOSO DE TITULOS",
}
