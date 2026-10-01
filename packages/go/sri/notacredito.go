package sri

import (
	"encoding/xml"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// Nota de crédito 1.1.0 (F5-13, RF-05-08): revierte total o parcialmente una factura,
// línea por línea, sin pasar nunca de lo que queda por revertir de cada línea. La propina no
// forma parte de la nota de crédito (el XSD de la NC no tiene ese campo) 🔎 DP-07.

var ErrNotaCredito = errors.New("sri: nota de crédito inválida")

// Devolucion es una línea de la factura (por su posición, desde 0) y cuánto se devuelve.
type Devolucion struct {
	Indice   int
	Cantidad decimal.Decimal
}

// Revertido es lo que notas de crédito anteriores ya devolvieron de una línea.
type Revertido struct {
	Indice    int
	Cantidad  decimal.Decimal
	Total     money.Money // precio total sin impuestos
	Descuento money.Money
	IVA       money.Money
}

// LineaNC es un detalle de la nota de crédito.
type LineaNC struct {
	Indice                 int
	Codigo, Descripcion    string
	Cantidad               decimal.Decimal
	PrecioUnitario         decimal.Decimal
	Descuento              money.Money
	PrecioTotalSinImpuesto money.Money
	CodigoPorcentaje       string
	Tarifa                 string // como en la factura: "15.00"
	IVA                    money.Money
}

// TotalNC es la base y el IVA de una tarifa.
type TotalNC struct {
	CodigoPorcentaje string
	Base, Valor      money.Money
}

// NotaCredito es el cálculo de una nota de crédito sobre una factura.
type NotaCredito struct {
	Lineas            []LineaNC
	TotalSinImpuestos money.Money
	Impuestos         []TotalNC
	ValorModificacion money.Money
	// Total indica que con esta nota la factura queda revertida por completo.
	Total bool
}

// Saldo es lo que queda por revertir de una factura (sin la propina).
func Saldo(f FacturaLeida, previas []Revertido) (money.Money, error) {
	saldo := money.Zero
	ya := acumular(previas)
	for i, d := range f.Detalles {
		total, iva, err := totalesDetalle(d)
		if err != nil {
			return money.Zero, err
		}
		saldo = saldo.Add(total).Add(iva).Sub(ya[i].Total).Sub(ya[i].IVA)
	}
	return saldo, nil
}

func acumular(previas []Revertido) map[int]Revertido {
	ya := map[int]Revertido{}
	for _, p := range previas {
		r := ya[p.Indice]
		r.Indice = p.Indice
		r.Cantidad = r.Cantidad.Add(p.Cantidad)
		r.Total, r.Descuento, r.IVA = r.Total.Add(p.Total), r.Descuento.Add(p.Descuento), r.IVA.Add(p.IVA)
		ya[p.Indice] = r
	}
	return ya
}

func totalesDetalle(d DetalleLeido) (money.Money, money.Money, error) {
	total, err := money.Parse(d.PrecioTotalSinImpuesto)
	if err != nil {
		return money.Zero, money.Zero, err
	}
	iva := money.Zero
	for _, i := range d.Impuestos {
		v, err := money.Parse(i.Valor)
		if err != nil {
			return money.Zero, money.Zero, err
		}
		iva = iva.Add(v)
	}
	return total, iva, nil
}

// CalcularNC calcula la nota de crédito que devuelve esas cantidades. Una línea que se
// devuelve completa (lo que le queda) toma exactamente su resto, así la suma de todas las
// notas de una línea es igual a la línea de la factura; una parcial se calcula en proporción.
// todo = true devuelve todo lo que queda.
func CalcularNC(f FacturaLeida, devolver []Devolucion, previas []Revertido, todo bool) (NotaCredito, error) {
	mal := func(format string, a ...any) (NotaCredito, error) {
		return NotaCredito{}, fmt.Errorf("%w: "+format, append([]any{ErrNotaCredito}, a...)...)
	}
	ya := acumular(previas)
	if todo {
		devolver = nil
		for i, d := range f.Detalles {
			q, err := decimal.NewFromString(d.Cantidad)
			if err != nil {
				return mal("cantidad de la línea %d", i+1)
			}
			if resto := q.Sub(ya[i].Cantidad); resto.IsPositive() {
				devolver = append(devolver, Devolucion{Indice: i, Cantidad: resto})
			}
		}
	}
	if len(devolver) == 0 {
		return mal("no hay nada que devolver")
	}
	vistos := map[int]bool{}
	var nc NotaCredito
	porTarifa := map[string]*TotalNC{}
	for _, dv := range devolver {
		if dv.Indice < 0 || dv.Indice >= len(f.Detalles) || vistos[dv.Indice] {
			return mal("línea %d", dv.Indice+1)
		}
		vistos[dv.Indice] = true
		d := f.Detalles[dv.Indice]
		q, err := decimal.NewFromString(d.Cantidad)
		if err != nil {
			return mal("cantidad de la línea %d", dv.Indice+1)
		}
		total, iva, err := totalesDetalle(d)
		if err != nil {
			return mal("valores de la línea %d", dv.Indice+1)
		}
		desc, err := money.Parse(d.Descuento)
		if err != nil {
			return mal("descuento de la línea %d", dv.Indice+1)
		}
		previo := ya[dv.Indice]
		resto := q.Sub(previo.Cantidad)
		if !dv.Cantidad.IsPositive() || dv.Cantidad.GreaterThan(resto) {
			return mal("de «%s» quedan %s por devolver", d.Descripcion, resto.String())
		}
		if len(d.Impuestos) != 1 {
			return mal("la línea %d no tiene un solo impuesto", dv.Indice+1)
		}
		imp := d.Impuestos[0]
		l := LineaNC{Indice: dv.Indice, Codigo: d.CodigoPrincipal, Descripcion: d.Descripcion, Cantidad: dv.Cantidad,
			CodigoPorcentaje: imp.CodigoPorcentaje, Tarifa: imp.Tarifa}
		if dv.Cantidad.Equal(resto) {
			l.PrecioTotalSinImpuesto = total.Sub(previo.Total)
			l.Descuento = desc.Sub(previo.Descuento)
			l.IVA = iva.Sub(previo.IVA)
		} else {
			prop := dv.Cantidad.Div(q)
			l.PrecioTotalSinImpuesto = total.Mul(prop).Round2()
			l.Descuento = desc.Mul(prop).Round2()
			tarifa, err := decimal.NewFromString(imp.Tarifa)
			if err != nil {
				return mal("tarifa de la línea %d", dv.Indice+1)
			}
			l.IVA = l.PrecioTotalSinImpuesto.Mul(tarifa.Div(decimal.NewFromInt(100))).Round2()
		}
		if l.PrecioTotalSinImpuesto.IsNegative() || l.IVA.IsNegative() || l.Descuento.IsNegative() {
			return mal("la línea %d ya está revertida", dv.Indice+1)
		}
		// Precio unitario a 6 decimales tal que cantidad × precio − descuento = total.
		l.PrecioUnitario = l.PrecioTotalSinImpuesto.Decimal().Add(l.Descuento.Decimal()).Div(l.Cantidad).Round(6)
		nc.Lineas = append(nc.Lineas, l)
		t := porTarifa[l.CodigoPorcentaje]
		if t == nil {
			t = &TotalNC{CodigoPorcentaje: l.CodigoPorcentaje}
			porTarifa[l.CodigoPorcentaje] = t
		}
		t.Base, t.Valor = t.Base.Add(l.PrecioTotalSinImpuesto), t.Valor.Add(l.IVA)
		nc.TotalSinImpuestos = nc.TotalSinImpuestos.Add(l.PrecioTotalSinImpuesto)
		nc.ValorModificacion = nc.ValorModificacion.Add(l.PrecioTotalSinImpuesto).Add(l.IVA)
	}
	sort.Slice(nc.Lineas, func(i, j int) bool { return nc.Lineas[i].Indice < nc.Lineas[j].Indice })
	codigos := make([]string, 0, len(porTarifa))
	for c := range porTarifa {
		codigos = append(codigos, c)
	}
	sort.Strings(codigos)
	for _, c := range codigos {
		nc.Impuestos = append(nc.Impuestos, *porTarifa[c])
	}
	saldo, err := Saldo(f, previas)
	if err != nil {
		return mal("%v", err)
	}
	if nc.ValorModificacion.GreaterThan(saldo) {
		return mal("la nota de %s supera el saldo de %s", nc.ValorModificacion, saldo)
	}
	nc.Total = nc.ValorModificacion.Equal(saldo)
	return nc, nil
}

// DatosNC es lo necesario para el XML de la nota de crédito, además del cálculo.
type DatosNC struct {
	Ambiente    Ambiente
	ClaveAcceso ClaveAcceso
	Secuencial  int64
	Fecha       time.Time
	Emisor      Emisor
	Comprador   Comprador // nunca consumidor final (ficha, nota de la Tabla 6)
	Sustento    FacturaLeida
	Motivo      string
	Adicionales []CampoAdicional
}

// NotaCreditoXML arma el XML de la nota de crédito 1.1.0 (sin firmar) en el orden del XSD.
func NotaCreditoXML(d DatosNC, nc NotaCredito) ([]byte, error) {
	mal := func(format string, a ...any) ([]byte, error) {
		return nil, fmt.Errorf("%w: "+format, append([]any{ErrNotaCredito}, a...)...)
	}
	e := d.Emisor
	if d.ClaveAcceso.TipoComprobante() != TipoNotaCredito || d.ClaveAcceso.RUC() != e.RUC || d.ClaveAcceso.Ambiente() != d.Ambiente ||
		d.ClaveAcceso.Serie() != e.Establecimiento+e.PuntoEmision || d.ClaveAcceso.Secuencial() != fmt.Sprintf("%09d", d.Secuencial) ||
		d.ClaveAcceso.FechaEmision() != d.Fecha.Format("02012006") {
		return mal("la clave de acceso no corresponde a esta nota de crédito")
	}
	switch d.Comprador.TipoIdentificacion {
	case "04", "05", "06", "08":
		if strings.TrimSpace(d.Comprador.Identificacion) == "" || strings.TrimSpace(d.Comprador.RazonSocial) == "" {
			return mal("faltan los datos del cliente")
		}
	default:
		return mal("la nota de crédito exige identificar al cliente (no consumidor final)")
	}
	if d.Sustento.CodDoc != TipoFactura || d.Sustento.RUC != e.RUC {
		return mal("el comprobante que se modifica no es una factura de este emisor")
	}
	motivo := texto(d.Motivo, 300)
	if motivo == "" {
		return mal("falta el motivo")
	}
	if len(nc.Lineas) == 0 {
		return mal("sin detalles")
	}
	x := xmlNotaCredito{ID: "comprobante", Version: VersionFactura}
	x.InfoTributaria = xmlInfoTributaria{
		Ambiente: strconv.Itoa(int(d.Ambiente)), TipoEmision: TipoEmisionNormal,
		RazonSocial: texto(e.RazonSocial, 300), NombreComercial: texto(e.NombreComercial, 300),
		RUC: e.RUC, ClaveAcceso: d.ClaveAcceso.String(), CodDoc: TipoNotaCredito,
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
	x.Info = xmlInfoNotaCredito{
		FechaEmision: d.Fecha.Format("02/01/2006"), DirEstablecimiento: texto(e.DirEstablecimiento, 300),
		TipoIdentificacionComprador: d.Comprador.TipoIdentificacion, RazonSocialComprador: texto(d.Comprador.RazonSocial, 300),
		IdentificacionComprador: d.Comprador.Identificacion, ContribuyenteEspecial: e.ContribuyenteEspecial, ObligadoContabilidad: obligado,
		CodDocModificado: TipoFactura, NumDocModificado: d.Sustento.Numero(), FechaEmisionDocSustento: d.Sustento.FechaEmision,
		TotalSinImpuestos: nc.TotalSinImpuestos.String(), ValorModificacion: nc.ValorModificacion.String(), Moneda: "DOLAR", Motivo: motivo,
	}
	for _, t := range nc.Impuestos {
		x.Info.TotalConImpuestos = append(x.Info.TotalConImpuestos, xmlTotalImpuestoNC{Codigo: CodigoImpuestoIVA, CodigoPorcentaje: t.CodigoPorcentaje,
			BaseImponible: t.Base.String(), Valor: t.Valor.String()})
	}
	for _, l := range nc.Lineas {
		x.Detalles = append(x.Detalles, xmlDetalleNC{
			CodigoInterno: texto(l.Codigo, 25), Descripcion: texto(l.Descripcion, 300),
			Cantidad: l.Cantidad.StringFixed(6), PrecioUnitario: l.PrecioUnitario.StringFixed(6),
			Descuento: l.Descuento.String(), PrecioTotalSinImpuesto: l.PrecioTotalSinImpuesto.String(),
			Impuestos: []xmlImpuesto{{Codigo: CodigoImpuestoIVA, CodigoPorcentaje: l.CodigoPorcentaje, Tarifa: l.Tarifa,
				BaseImponible: l.PrecioTotalSinImpuesto.String(), Valor: l.IVA.String()}},
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

type xmlNotaCredito struct {
	XMLName        xml.Name           `xml:"notaCredito"`
	ID             string             `xml:"id,attr"`
	Version        string             `xml:"version,attr"`
	InfoTributaria xmlInfoTributaria  `xml:"infoTributaria"`
	Info           xmlInfoNotaCredito `xml:"infoNotaCredito"`
	Detalles       []xmlDetalleNC     `xml:"detalles>detalle"`
	InfoAdicional  *xmlInfoAdicional  `xml:"infoAdicional,omitempty"`
}

type xmlInfoNotaCredito struct {
	FechaEmision                string               `xml:"fechaEmision"`
	DirEstablecimiento          string               `xml:"dirEstablecimiento,omitempty"`
	TipoIdentificacionComprador string               `xml:"tipoIdentificacionComprador"`
	RazonSocialComprador        string               `xml:"razonSocialComprador"`
	IdentificacionComprador     string               `xml:"identificacionComprador"`
	ContribuyenteEspecial       string               `xml:"contribuyenteEspecial,omitempty"`
	ObligadoContabilidad        string               `xml:"obligadoContabilidad"`
	CodDocModificado            string               `xml:"codDocModificado"`
	NumDocModificado            string               `xml:"numDocModificado"`
	FechaEmisionDocSustento     string               `xml:"fechaEmisionDocSustento"`
	TotalSinImpuestos           string               `xml:"totalSinImpuestos"`
	ValorModificacion           string               `xml:"valorModificacion"`
	Moneda                      string               `xml:"moneda"`
	TotalConImpuestos           []xmlTotalImpuestoNC `xml:"totalConImpuestos>totalImpuesto"`
	Motivo                      string               `xml:"motivo"`
}

// En la NC 1.1.0 el total por impuesto no lleva tarifa (a diferencia de la factura).
type xmlTotalImpuestoNC struct {
	Codigo           string `xml:"codigo"`
	CodigoPorcentaje string `xml:"codigoPorcentaje"`
	BaseImponible    string `xml:"baseImponible"`
	Valor            string `xml:"valor"`
}

type xmlDetalleNC struct {
	CodigoInterno          string        `xml:"codigoInterno,omitempty"`
	Descripcion            string        `xml:"descripcion"`
	Cantidad               string        `xml:"cantidad"`
	PrecioUnitario         string        `xml:"precioUnitario"`
	Descuento              string        `xml:"descuento"`
	PrecioTotalSinImpuesto string        `xml:"precioTotalSinImpuesto"`
	Impuestos              []xmlImpuesto `xml:"impuestos>impuesto"`
}
