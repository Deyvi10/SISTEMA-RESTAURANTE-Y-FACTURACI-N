// Package ride arma el RIDE A4 de una factura electrónica (F5-11) según el Anexo 2 de la ficha
// técnica offline v2.34: se genera al vuelo y de forma determinista desde el XML del
// comprobante y su autorización, así que el PDF no se guarda (el mismo XML da los mismos bytes).
//
// Anexo 2: el número de autorización es la clave de acceso; la fecha y hora de autorización
// no son obligatorias en el RIDE del emisor (se imprimen si ya se conocen); el código de barras
// es opcional (§9.20) y aquí va en Code 128; solo se muestran los subtotales con valor.
package ride

import (
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/pdf"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// Autorizacion es lo que respondió el SRI (vacía si todavía no autoriza).
type Autorizacion struct {
	Fecha *time.Time
}

const (
	margen  pdf.Pt = 30
	ancho   pdf.Pt = pdf.AnchoPagina - 2*margen
	limiteY pdf.Pt = pdf.AltoPagina - 40
	sans           = pdf.Sans
	bold           = pdf.SansBold
)

// columnas del detalle: borde izquierdo de cada una y su encabezado.
var columnas = []struct {
	x       pdf.Pt
	titulo  string
	derecha bool
}{
	{margen, "Cód. Principal", false},
	{margen + 70, "Cantidad", true},
	{margen + 120, "Descripción", false},
	{margen + 325, "Precio Unitario", true},
	{margen + 400, "Descuento", true},
	{margen + 465, "Precio Total", true},
}

// PDF devuelve el RIDE de la factura.
func PDF(doc []byte, a Autorizacion) ([]byte, error) {
	f, err := sri.LeerComprobante(doc)
	if err != nil {
		return nil, err
	}
	var l pdf.Lienzo
	y := cabecera(&l, f, a)
	y = comprador(&l, f, y+10)
	y = detalle(&l, f, y+10)
	pie(&l, f, y+10)
	return l.Bytes(), nil
}

func cabecera(l *pdf.Lienzo, f sri.FacturaLeida, a Autorizacion) pdf.Pt {
	// Izquierda: el emisor (en vez del logo, el nombre comercial en grande).
	nombre := f.NombreComercial
	if nombre == "" {
		nombre = f.RazonSocial
	}
	y := l.Parrafo(margen, 60, 255, bold, 18, nombre)
	izq := max(y+4, 100)
	alto := 260 - izq
	l.Rect(margen, izq, 260, alto)
	yy := l.Parrafo(margen+8, izq+16, 244, bold, 9, f.RazonSocial)
	if f.NombreComercial != "" {
		yy = l.Parrafo(margen+8, yy+2, 244, sans, 9, f.NombreComercial)
	}
	etiqueta := func(et, valor string) {
		if valor == "" {
			return
		}
		l.Texto(margen+8, yy+6, bold, 8, et)
		yy = l.Parrafo(margen+88, yy+6, 164, sans, 8, valor) - 2
	}
	etiqueta("Dirección Matriz:", f.DirMatriz)
	if f.DirEstablecimiento != "" && f.DirEstablecimiento != f.DirMatriz {
		etiqueta("Dirección Sucursal:", f.DirEstablecimiento)
	}
	yy += 6
	if f.ContribuyenteEspecial != "" {
		l.Texto(margen+8, yy+6, sans, 8, "Contribuyente Especial Nro: "+f.ContribuyenteEspecial)
		yy += 12
	}
	l.Texto(margen+8, yy+6, bold, 8, "OBLIGADO A LLEVAR CONTABILIDAD")
	l.TextoDerecha(margen+252, yy+6, sans, 8, strings.ToUpper(f.ObligadoContabilidad))
	yy += 12
	if f.AgenteRetencion != "" {
		l.Texto(margen+8, yy+6, sans, 8, "Agente de Retención Resolución No. "+f.AgenteRetencion)
		yy += 12
	}
	if f.ContribuyenteRimpe != "" {
		l.Texto(margen+8, yy+6, bold, 8, f.ContribuyenteRimpe)
	}

	// Derecha: RUC, número, autorización y clave de acceso.
	x, w := margen+270, ancho-270
	l.Rect(x, 30, w, 230)
	l.Texto(x+10, 50, bold, 11, "R.U.C.:")
	l.Texto(x+60, 50, sans, 11, f.RUC)
	l.Texto(x+10, 70, bold, 13, titulo(f))
	l.Texto(x+10, 86, sans, 9, "No.")
	l.Texto(x+40, 86, bold, 9, f.Numero())
	l.Texto(x+10, 104, bold, 8, "NÚMERO DE AUTORIZACIÓN")
	l.Texto(x+10, 116, sans, 8, f.ClaveAcceso)
	l.Texto(x+10, 134, bold, 8, "FECHA Y HORA DE")
	l.Texto(x+10, 143, bold, 8, "AUTORIZACIÓN:")
	fecha := "Pendiente de autorización del SRI"
	if a.Fecha != nil {
		fecha = a.Fecha.In(clock.Guayaquil).Format("02/01/2006 15:04:05")
	}
	l.Texto(x+90, 143, sans, 8, fecha)
	ambiente := "PRODUCCIÓN"
	if f.Ambiente == "1" {
		ambiente = "PRUEBAS"
	}
	l.Texto(x+10, 160, bold, 8, "AMBIENTE:")
	l.Texto(x+90, 160, sans, 8, ambiente)
	l.Texto(x+10, 174, bold, 8, "EMISIÓN:")
	l.Texto(x+90, 174, sans, 8, "NORMAL")
	l.Texto(x+10, 190, bold, 8, "CLAVE DE ACCESO")
	if barras, err := pdf.Code128Digitos(f.ClaveAcceso); err == nil {
		modulo := (w - 20) / pdf.Pt(suma(barras))
		l.Barras(x+10, 196, 38, modulo, barras)
	}
	l.TextoCentrado(x+w/2, 246, sans, 7, f.ClaveAcceso)
	if f.Ambiente == "1" {
		l.TextoCentrado(margen+ancho/2, 276, bold, 9, "AMBIENTE DE PRUEBAS - SIN VALIDEZ TRIBUTARIA")
		return 282
	}
	return 266
}

func titulo(f sri.FacturaLeida) string {
	if f.EsNotaCredito() {
		return "NOTA DE CRÉDITO"
	}
	return "FACTURA"
}

func suma(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x
	}
	return s
}

func comprador(l *pdf.Lienzo, f sri.FacturaLeida, y pdf.Pt) pdf.Pt {
	lineas := []struct{ et, valor string }{
		{"Razón Social / Nombres y Apellidos:", f.RazonSocialComprador},
		{"Identificación:", f.IdComprador},
		{"Fecha Emisión:", f.FechaEmision},
	}
	if f.DireccionComprador != "" {
		lineas = append(lineas, struct{ et, valor string }{"Dirección:", f.DireccionComprador})
	}
	if f.EsNotaCredito() {
		// Anexo 2, nota de crédito: el comprobante que se modifica, su fecha y la razón.
		lineas = append(lineas,
			struct{ et, valor string }{"Comprobante que se modifica:", "FACTURA " + f.NumDocModificado},
			struct{ et, valor string }{"Fecha Emisión (Comprobante a modificar):", f.FechaEmisionDocSustento},
			struct{ et, valor string }{"Razón de Modificación:", f.Motivo})
	}
	alto := pdf.Pt(len(lineas))*13 + 10
	l.Rect(margen, y, ancho, alto)
	for i, ln := range lineas {
		yy := y + 15 + pdf.Pt(i)*13
		l.Texto(margen+8, yy, bold, 8, ln.et)
		l.Texto(margen+200, yy, sans, 8, ln.valor)
	}
	return y + alto
}

// continuacion abre otra hoja que dice de qué factura es y devuelve dónde seguir.
func continuacion(l *pdf.Lienzo, f sri.FacturaLeida) pdf.Pt {
	l.NuevaPagina()
	l.Texto(margen, 40, bold, 9, titulo(f)+" No. "+f.Numero())
	l.TextoDerecha(margen+ancho, 40, sans, 8, "R.U.C. "+f.RUC+" · continuación")
	return 52
}

func encabezadoDetalle(l *pdf.Lienzo, y pdf.Pt) pdf.Pt {
	l.Relleno(margen, y, ancho, 16, 0.92)
	l.Rect(margen, y, ancho, 16)
	for i, c := range columnas {
		if c.derecha {
			l.TextoDerecha(fin(i)-4, y+11, bold, 7, c.titulo)
		} else {
			l.Texto(c.x+4, y+11, bold, 7, c.titulo)
		}
	}
	return y + 16
}

// fin es el borde derecho de la columna i.
func fin(i int) pdf.Pt {
	if i+1 < len(columnas) {
		return columnas[i+1].x
	}
	return margen + ancho
}

func detalle(l *pdf.Lienzo, f sri.FacturaLeida, y pdf.Pt) pdf.Pt {
	y = encabezadoDetalle(l, y)
	for _, d := range f.Detalles {
		desc := pdf.Partir(sans, 7.5, d.Descripcion, fin(2)-columnas[2].x-8)
		alto := pdf.Pt(len(desc))*9.5 + 6
		if y+alto > limiteY {
			y = encabezadoDetalle(l, continuacion(l, f))
		}
		l.Rect(margen, y, ancho, alto)
		base := y + 10
		l.Texto(columnas[0].x+4, base, sans, 7.5, d.CodigoPrincipal)
		l.TextoDerecha(fin(1)-4, base, sans, 7.5, cantidad(d.Cantidad))
		for k, ln := range desc {
			l.Texto(columnas[2].x+4, base+pdf.Pt(k)*9.5, sans, 7.5, ln)
		}
		l.TextoDerecha(fin(3)-4, base, sans, 7.5, precio(d.PrecioUnitario))
		l.TextoDerecha(fin(4)-4, base, sans, 7.5, d.Descuento)
		l.TextoDerecha(fin(5)-4, base, sans, 7.5, d.PrecioTotalSinImpuesto)
		y += alto
	}
	return y
}

// cantidad y precio sin los ceros de relleno de los 6 decimales (1.000000 → 1.00).
func cantidad(s string) string { return recortar(s, 2) }
func precio(s string) string   { return recortar(s, 2) }

func recortar(s string, minimo int) string {
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return s
	}
	ent, dec := s[:i], strings.TrimRight(s[i+1:], "0")
	for len(dec) < minimo {
		dec += "0"
	}
	return ent + "." + dec
}

type fila struct{ et, valor string }

func pie(l *pdf.Lienzo, f sri.FacturaLeida, y pdf.Pt) {
	totales := totalesDe(f)
	adicionales := f.Adicionales
	pagos := f.Pagos
	altoIzq := 16 + pdf.Pt(len(adicionales))*11 + 14 + 16 + pdf.Pt(len(pagos))*11 + 10
	altoDer := pdf.Pt(len(totales)) * 13
	if y+max(altoIzq, altoDer) > limiteY {
		y = continuacion(l, f)
	}
	// Totales a la derecha.
	xt, wt := margen+325, ancho-325
	for i, t := range totales {
		yy := y + pdf.Pt(i)*13
		l.Rect(xt, yy, wt, 13)
		fuente := sans
		if i == len(totales)-1 {
			fuente = bold
		}
		l.Texto(xt+4, yy+9.5, fuente, 7.5, t.et)
		l.TextoDerecha(xt+wt-4, yy+9.5, fuente, 7.5, t.valor)
	}
	// Información adicional y formas de pago a la izquierda.
	wi := pdf.Pt(315)
	yy := y
	if len(adicionales) > 0 {
		alto := 16 + pdf.Pt(len(adicionales))*11
		l.Rect(margen, yy, wi, alto)
		l.TextoCentrado(margen+wi/2, yy+11, bold, 8, "Información Adicional")
		for i, c := range adicionales {
			l.Texto(margen+8, yy+25+pdf.Pt(i)*11, bold, 7.5, c.Nombre)
			l.Texto(margen+110, yy+25+pdf.Pt(i)*11, sans, 7.5, c.Valor)
		}
		yy += alto + 8
	}
	if len(pagos) > 0 {
		l.Relleno(margen, yy, wi, 14, 0.92)
		l.Rect(margen, yy, wi, 14+pdf.Pt(len(pagos))*11+4)
		l.Texto(margen+8, yy+10, bold, 7.5, "Forma de pago")
		l.TextoDerecha(margen+wi-8, yy+10, bold, 7.5, "Valor")
		for i, p := range pagos {
			fy := yy + 24 + pdf.Pt(i)*11
			l.Texto(margen+8, fy, sans, 7, p.FormaPago+" - "+sri.FormasPago[p.FormaPago])
			l.TextoDerecha(margen+wi-8, fy, sans, 7.5, p.Total)
		}
	}
}

// totalesDe arma el cuadro de totales con solo los subtotales llenos (Anexo 2).
func totalesDe(f sri.FacturaLeida) []fila {
	var subtotales, ivas []fila
	for _, t := range f.Impuestos {
		if t.Codigo != "2" { // ICE u otros: no aplica en restaurantes
			continue
		}
		etq := "SUBTOTAL " + porcentaje(t.Tarifa)
		switch t.CodigoPorcentaje {
		case "6":
			etq = "SUBTOTAL NO OBJETO DE IVA"
		case "7":
			etq = "SUBTOTAL EXENTO DE IVA"
		}
		subtotales = append(subtotales, fila{etq, t.BaseImponible})
		if !cero(t.Valor) {
			ivas = append(ivas, fila{"IVA " + porcentaje(t.Tarifa), t.Valor})
		}
	}
	out := append(subtotales, fila{"SUBTOTAL SIN IMPUESTOS", f.TotalSinImpuestos}, fila{"TOTAL DESCUENTO", f.TotalDescuento})
	out = append(out, ivas...)
	if !cero(f.Propina) {
		out = append(out, fila{"PROPINA", f.Propina})
	}
	return append(out, fila{"VALOR TOTAL", f.ImporteTotal})
}

// porcentaje: «15.00» → «15%», «12.50» → «12.5%».
func porcentaje(tarifa string) string { return strings.TrimSuffix(recortar(tarifa, 0), ".") + "%" }

func cero(s string) bool {
	v, err := money.Parse(s)
	return err != nil || v.IsZero()
}
