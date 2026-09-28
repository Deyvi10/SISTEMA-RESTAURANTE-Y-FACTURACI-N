// Package pdf arma documentos PDF de texto monoespaciado (Courier, WinAnsi) sin dependencias:
// basta para reportes con forma de ticket, como el Cierre Z que recibe el dueño.
package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	anchoA4, altoA4 = 595, 842 // puntos
	margen          = 50
)

type linea struct {
	texto   string
	tamano  int
	negrita bool
}

// Doc es un documento de líneas que se reparten solas en páginas A4.
type Doc struct {
	lineas []linea
}

// Linea agrega una línea de texto (tamaño en puntos).
func (d *Doc) Linea(texto string, tamano int, negrita bool) {
	d.lineas = append(d.lineas, linea{texto, tamano, negrita})
}

// winAnsi convierte a la codificación de las fuentes estándar: ASCII y Latin-1 pasan tal cual
// (tildes, ñ, ¢, ·); lo demás se vuelve «?».
func winAnsi(s string) []byte {
	var b []byte
	for _, r := range s {
		switch {
		case r == '(' || r == ')' || r == '\\':
			b = append(b, '\\', byte(r))
		case r >= 0x20 && r < 0x7f, r >= 0xa0 && r <= 0xff:
			b = append(b, byte(r))
		default:
			b = append(b, '?')
		}
	}
	return b
}

func (d *Doc) paginas() [][]byte {
	var pags [][]byte
	var cur bytes.Buffer
	y := altoA4 - margen
	for _, l := range d.lineas {
		alto := (l.tamano*13 + 9) / 10 // interlineado de 1,3
		if y-alto < margen && cur.Len() > 0 {
			pags = append(pags, bytes.Clone(cur.Bytes()))
			cur.Reset()
			y = altoA4 - margen
		}
		y -= alto
		fuente := "F1"
		if l.negrita {
			fuente = "F2"
		}
		fmt.Fprintf(&cur, "BT /%s %d Tf %d %d Td (", fuente, l.tamano, margen, y)
		cur.Write(winAnsi(l.texto))
		cur.WriteString(") Tj ET\n")
	}
	return append(pags, cur.Bytes())
}

// Bytes devuelve el PDF completo.
func (d *Doc) Bytes() []byte {
	pags := d.paginas()
	var out bytes.Buffer
	var offs []int
	obj := func(cuerpo string) {
		offs = append(offs, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", len(offs), cuerpo)
	}
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	// 1 catálogo, 2 páginas, 3 y 4 fuentes; luego cada página (hoja + contenido).
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	kids := make([]string, len(pags))
	for i := range pags {
		kids[i] = fmt.Sprintf("%d 0 R", 5+2*i)
	}
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pags)))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Courier-Bold /Encoding /WinAnsiEncoding >>")
	for i, p := range pags {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>", anchoA4, altoA4, 6+2*i))
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(p), p))
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, xref)
	return out.Bytes()
}
