package pdf

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"testing"
)

func TestPDFValidoConTildesYVariasPaginas(t *testing.T) {
	var d Doc
	d.Linea("Cevichería Don Pepe (Guayaquil)", 14, true)
	for i := range 120 {
		d.Linea(fmt.Sprintf("Línea %d · 25¢ ñ \\", i), 9, false)
	}
	b := d.Bytes()
	if !bytes.HasPrefix(b, []byte("%PDF-1.4")) || !bytes.HasSuffix(b, []byte("%%EOF\n")) {
		t.Fatal("cabecera o final")
	}
	if !bytes.Contains(b, []byte("/Count 2")) {
		t.Fatal("120 líneas deberían ocupar dos páginas")
	}
	// Tildes en WinAnsi (un byte) y paréntesis escapados.
	if !bytes.Contains(b, []byte("Cevicher\xeda Don Pepe \\(Guayaquil\\)")) || !bytes.Contains(b, []byte("25\xa2 \xf1 \\\\")) {
		t.Fatal("codificación")
	}
	// Cada entrada de la tabla xref apunta al inicio de su objeto.
	m := regexp.MustCompile(`startxref\n(\d+)`).FindSubmatch(b)
	xref, _ := strconv.Atoi(string(m[1]))
	if !bytes.HasPrefix(b[xref:], []byte("xref")) {
		t.Fatal("startxref")
	}
	for i, off := range regexp.MustCompile(`(\d{10}) 00000 n`).FindAllSubmatch(b[xref:], -1) {
		o, _ := strconv.Atoi(string(off[1]))
		if !bytes.HasPrefix(b[o:], fmt.Appendf(nil, "%d 0 obj", i+1)) {
			t.Fatalf("objeto %d fuera de lugar", i+1)
		}
	}
}
