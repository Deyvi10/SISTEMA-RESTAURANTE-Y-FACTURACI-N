// Package xlsx escribe un libro de Excel (Office Open XML) de una hoja, sin dependencias: textos
// en línea, números como números (sin float: se escriben tal cual vienen, p. ej. «18.47») y la
// primera fila en negrita y fija. Basta para exportar ventas al contador.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Celda es texto o número. Los números van como texto decimal ("18.47") y Excel los suma.
type Celda struct {
	Texto  string
	Numero string
}

func T(s string) Celda { return Celda{Texto: s} }
func N(s string) Celda { return Celda{Numero: s} }

var numero = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// Libro arma el .xlsx con una hoja: encabezados y filas.
func Libro(hoja string, encabezados []string, filas [][]Celda) ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	// Fecha fija: el mismo contenido da los mismos bytes.
	fijo := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	escribir := func(nombre, contenido string) error {
		w, err := z.CreateHeader(&zip.FileHeader{Name: nombre, Method: zip.Deflate, Modified: fijo})
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(contenido))
		return err
	}
	var hojaXML strings.Builder
	hojaXML.WriteString(xml.Header + `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><sheetData>`)
	fila := func(n int, celdas []Celda, estilo string) error {
		fmt.Fprintf(&hojaXML, `<row r="%d">`, n)
		for i, c := range celdas {
			ref := columna(i) + fmt.Sprint(n)
			if c.Numero != "" {
				if !numero.MatchString(c.Numero) {
					return fmt.Errorf("xlsx: %q no es un número", c.Numero)
				}
				fmt.Fprintf(&hojaXML, `<c r="%s"%s><v>%s</v></c>`, ref, estilo, c.Numero)
				continue
			}
			var esc bytes.Buffer
			_ = xml.EscapeText(&esc, []byte(c.Texto))
			fmt.Fprintf(&hojaXML, `<c r="%s" t="inlineStr"%s><is><t xml:space="preserve">%s</t></is></c>`, ref, estilo, esc.String())
		}
		hojaXML.WriteString(`</row>`)
		return nil
	}
	cab := make([]Celda, len(encabezados))
	for i, e := range encabezados {
		cab[i] = T(e)
	}
	if err := fila(1, cab, ` s="1"`); err != nil {
		return nil, err
	}
	for i, f := range filas {
		if err := fila(i+2, f, ""); err != nil {
			return nil, err
		}
	}
	hojaXML.WriteString(`</sheetData></worksheet>`)
	var nombre bytes.Buffer
	_ = xml.EscapeText(&nombre, []byte(hoja))
	partes := []struct{ n, c string }{
		{"[Content_Types].xml", xml.Header + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`},
		{"_rels/.rels", xml.Header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", xml.Header + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="` + nombre.String() + `" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", xml.Header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`},
		{"xl/styles.xml", xml.Header + `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
			`<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts>` +
			`<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>` +
			`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
			`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
			`<cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`},
		{"xl/worksheets/sheet1.xml", hojaXML.String()},
	}
	for _, p := range partes {
		if err := escribir(p.n, p.c); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// columna: 0 → A, 25 → Z, 26 → AA.
func columna(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}
