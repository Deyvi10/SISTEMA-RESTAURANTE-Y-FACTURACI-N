package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

// Pt es una medida en puntos tipográficos (1/72 de pulgada). Es geometría del dibujo, nunca
// dinero: por eso este paquete es la única excepción de `make check-float`.
type Pt float64

// Fuente estándar de PDF (no se incrustan: todo lector las tiene).
type Fuente string

const (
	Mono        Fuente = "F1" // Courier
	MonoBold    Fuente = "F2" // Courier-Bold
	Sans        Fuente = "F3" // Helvetica
	SansBold    Fuente = "F4" // Helvetica-Bold
	AnchoPagina        = anchoA4
	AltoPagina         = altoA4
)

// Lienzo dibuja páginas A4 con coordenadas desde la esquina superior izquierda, en puntos:
// texto, cajas, líneas y códigos de barras. Sirve para documentos con forma, como el RIDE.
type Lienzo struct {
	pags []*bytes.Buffer
}

func (l *Lienzo) actual() *bytes.Buffer {
	if len(l.pags) == 0 {
		l.NuevaPagina()
	}
	return l.pags[len(l.pags)-1]
}

// NuevaPagina empieza otra hoja.
func (l *Lienzo) NuevaPagina() { l.pags = append(l.pags, &bytes.Buffer{}) }

// Paginas cuenta las hojas dibujadas.
func (l *Lienzo) Paginas() int { return len(l.pags) }

func num(v Pt) string {
	s := fmt.Sprintf("%.2f", float64(v))
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}

// Texto escribe s con la línea base en (x, y).
func (l *Lienzo) Texto(x, y Pt, f Fuente, tam Pt, s string) {
	b := l.actual()
	fmt.Fprintf(b, "BT /%s %s Tf %s %s Td (", f, num(tam), num(x), num(altoA4-y))
	b.Write(winAnsi(s))
	b.WriteString(") Tj ET\n")
}

// TextoDerecha alinea el final del texto en x.
func (l *Lienzo) TextoDerecha(x, y Pt, f Fuente, tam Pt, s string) {
	l.Texto(x-Ancho(f, tam, s), y, f, tam, s)
}

// TextoCentrado centra el texto en x.
func (l *Lienzo) TextoCentrado(x, y Pt, f Fuente, tam Pt, s string) {
	l.Texto(x-Ancho(f, tam, s)/2, y, f, tam, s)
}

// Parrafo parte s en líneas de hasta ancho puntos y devuelve la y siguiente.
func (l *Lienzo) Parrafo(x, y, ancho Pt, f Fuente, tam Pt, s string) Pt {
	for _, ln := range Partir(f, tam, s, ancho) {
		l.Texto(x, y, f, tam, ln)
		y += tam * 1.25
	}
	return y
}

// Rect dibuja el borde de una caja (x, y es la esquina superior izquierda).
func (l *Lienzo) Rect(x, y, w, h Pt) {
	fmt.Fprintf(l.actual(), "0.6 w %s %s %s %s re S\n", num(x), num(altoA4-y-h), num(w), num(h))
}

// Relleno pinta una caja en gris (0 negro, 1 blanco).
func (l *Lienzo) Relleno(x, y, w, h, gris Pt) {
	fmt.Fprintf(l.actual(), "%s g %s %s %s %s re f 0 g\n", num(gris), num(x), num(altoA4-y-h), num(w), num(h))
}

// Linea traza un segmento.
func (l *Lienzo) Linea(x1, y1, x2, y2 Pt) {
	fmt.Fprintf(l.actual(), "0.6 w %s %s m %s %s l S\n", num(x1), num(altoA4-y1), num(x2), num(altoA4-y2))
}

// Barras dibuja un código de barras lineal: módulos alternan barra/espacio empezando por barra.
func (l *Lienzo) Barras(x, y, alto, modulo Pt, anchos []int) {
	b := l.actual()
	pos := x
	for i, w := range anchos {
		if i%2 == 0 {
			fmt.Fprintf(b, "%s %s %s %s re ", num(pos), num(altoA4-y-alto), num(Pt(w)*modulo), num(alto))
		}
		pos += Pt(w) * modulo
	}
	b.WriteString("f\n")
}

// Bytes devuelve el PDF.
func (l *Lienzo) Bytes() []byte {
	pags := make([][]byte, 0, len(l.pags))
	for _, p := range l.pags {
		pags = append(pags, p.Bytes())
	}
	if len(pags) == 0 {
		pags = append(pags, nil)
	}
	return escribir(pags)
}

// Partir corta s en líneas que caben en ancho puntos (por palabras; una palabra más larga
// que la línea se corta).
func Partir(f Fuente, tam Pt, s string, ancho Pt) []string {
	var out []string
	for _, parrafo := range strings.Split(s, "\n") {
		linea := ""
		for _, p := range strings.Fields(parrafo) {
			for Ancho(f, tam, p) > ancho {
				corte := len([]rune(p))
				for corte > 1 && Ancho(f, tam, string([]rune(p)[:corte])) > ancho {
					corte--
				}
				if linea != "" {
					out, linea = append(out, linea), ""
				}
				out = append(out, string([]rune(p)[:corte]))
				p = string([]rune(p)[corte:])
			}
			switch {
			case linea == "":
				linea = p
			case Ancho(f, tam, linea+" "+p) <= ancho:
				linea += " " + p
			default:
				out, linea = append(out, linea), p
			}
		}
		if linea != "" || len(out) == 0 {
			out = append(out, linea)
		}
	}
	return out
}

// Ancho mide el texto en puntos con las métricas estándar (AFM de Adobe) de Courier y
// Helvetica. Las letras con tilde miden como su letra base.
func Ancho(f Fuente, tam Pt, s string) Pt {
	if f == Mono || f == MonoBold {
		return Pt(len([]rune(s))) * 600 * tam / 1000
	}
	total := 0
	for _, r := range s {
		total += anchoHelvetica(r, f == SansBold)
	}
	return Pt(total) * tam / 1000
}

// helvetica tiene los anchos de ' ' (32) a '~' (126) de Helvetica y Helvetica-Bold.
var helvetica = [2][95]int{
	{
		278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
		556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
		1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
		667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
		333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
		556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
	},
	{
		278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
		556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611,
		975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778,
		667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556,
		333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611,
		611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584,
	},
}

var bases = map[rune]rune{
	'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a', 'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e', 'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i',
	'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o', 'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u', 'ñ': 'n', 'ç': 'c',
	'Á': 'A', 'À': 'A', 'Ä': 'A', 'É': 'E', 'È': 'E', 'Í': 'I', 'Ó': 'O', 'Ö': 'O', 'Ú': 'U', 'Ü': 'U', 'Ñ': 'N', 'Ç': 'C',
}

func anchoHelvetica(r rune, negrita bool) int {
	if b, ok := bases[r]; ok {
		r = b
	}
	i := 0
	if negrita {
		i = 1
	}
	switch {
	case r >= 32 && r <= 126:
		return helvetica[i][r-32]
	case r == '·':
		return 278
	case r == '«' || r == '»':
		return 556
	default:
		return 556
	}
}
