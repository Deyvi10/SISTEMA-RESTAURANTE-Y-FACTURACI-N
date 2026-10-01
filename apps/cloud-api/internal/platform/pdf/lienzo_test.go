package pdf

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/pdf/pdfprueba"
)

func TestPatronesCode128(t *testing.T) {
	for i, p := range patrones128 {
		suma := 0
		for _, c := range p {
			suma += int(c - '0')
		}
		if (i < parada128 && (len(p) != 6 || suma != 11)) || (i == parada128 && suma != 13) {
			t.Fatalf("símbolo %d mal formado: %s", i, p)
		}
	}
	if _, err := Code128Digitos("12a"); err == nil {
		t.Fatal("solo dígitos")
	}
}

func TestCode128SeLeeConOtraImplementacion(t *testing.T) {
	var l Lienzo
	// 49 dígitos (impar: termina en el juego B) y uno par.
	claves := []string{"2909202601179001167400110010010000000071234567813", "1234567890"}
	for i, c := range claves {
		anchos, err := Code128Digitos(c)
		if err != nil {
			t.Fatal(err)
		}
		l.Barras(60, 80+Pt(i)*100, 40, 0.9, anchos)
	}
	leidos := pdfprueba.Barras(t, l.Bytes())
	if leidos == nil {
		return
	}
	for _, c := range claves {
		found := false
		for _, r := range leidos {
			if strings.HasSuffix(r, "|"+c) && strings.Contains(strings.ReplaceAll(r, " ", ""), "Code128") {
				found = true
			}
		}
		if !found {
			t.Fatalf("no se leyó %s: %v", c, leidos)
		}
	}
}

func TestLienzoDeterministaYMedidas(t *testing.T) {
	dibujar := func() []byte {
		var l Lienzo
		l.Rect(40, 40, 200, 100)
		l.Texto(50, 60, SansBold, 12, "FACTURA")
		l.TextoDerecha(230, 80, Sans, 9, "$1,234.56")
		l.NuevaPagina()
		l.Parrafo(50, 60, 100, Sans, 9, "Una descripción muy larga que no cabe en una sola línea del detalle")
		return l.Bytes()
	}
	a, b := dibujar(), dibujar()
	if !bytes.Equal(a, b) || !bytes.Contains(a, []byte("/Count 2")) || !bytes.Contains(a, []byte("/Helvetica-Bold")) {
		t.Fatal("el mismo dibujo debe dar los mismos bytes, en dos páginas")
	}
	if w := Ancho(Sans, 10, "0123456789"); w != 55.6 {
		t.Fatalf("ancho de los dígitos: %v", w)
	}
	if Ancho(Sans, 10, "Ñandú") != Ancho(Sans, 10, "Nandu") {
		t.Fatal("las tildes miden como su letra base")
	}
	for _, ln := range Partir(Sans, 9, "Una descripción muy larga que no cabe en una sola línea del detalle", 100) {
		if Ancho(Sans, 9, ln) > 100 {
			t.Fatalf("línea demasiado ancha: %q", ln)
		}
	}
	if got := Partir(Mono, 10, "AAAAAAAAAAAAAAAAAAAA", 60); len(got) != 2 || got[0] != "AAAAAAAAAA" {
		t.Fatalf("palabra larga: %q", got)
	}
}
