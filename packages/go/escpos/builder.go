// Package escpos genera y decodifica el subconjunto de ESC/POS que usa el sistema
// para imprimir comandas, pre-cuentas, comprobantes y abrir el cajón (RF-02-04, RF-02-07).
//
// Solo se usan comandos soportados por las térmicas de la lista de hardware certificado
// (Epson TM-T20, Xprinter XP-80, Rongta RP80). El texto se envía en la página de códigos
// PC850 para que las tildes y la ñ salgan bien.
package escpos

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// Bytes de control.
const (
	ESC = 0x1B
	GS  = 0x1D
	DLE = 0x10
	EOT = 0x04
	LF  = 0x0A
)

// Paper es el ancho del rollo.
type Paper int

const (
	Paper58 Paper = 58
	Paper80 Paper = 80
)

// Columns devuelve cuántos caracteres de la fuente A (12×24) caben por línea.
func (p Paper) Columns() int {
	if p == Paper58 {
		return 32
	}
	return 48
}

// Align es la alineación del párrafo.
type Align byte

const (
	Left   Align = 0
	Center Align = 1
	Right  Align = 2
)

// codePage850 es el número de tabla PC850 en ESC t (Epson y compatibles).
const codePage850 = 2

// Builder arma un documento ESC/POS. Sus métodos se encadenan y nunca fallan:
// los valores fuera de rango se ajustan al rango válido.
type Builder struct {
	buf   bytes.Buffer
	paper Paper
	w, h  int // multiplicador de tamaño actual (1..8)
}

// New crea un documento para el ancho de papel dado, ya inicializado.
func New(p Paper) *Builder {
	if p != Paper58 {
		p = Paper80
	}
	b := &Builder{paper: p, w: 1, h: 1}
	b.buf.Write([]byte{ESC, '@', ESC, 't', codePage850})
	return b
}

// Paper devuelve el ancho configurado.
func (b *Builder) Paper() Paper { return b.paper }

// Width devuelve las columnas disponibles con el tamaño de letra actual.
func (b *Builder) Width() int { return b.paper.Columns() / b.w }

// Text escribe texto sin salto de línea.
func (b *Builder) Text(s string) *Builder {
	b.buf.Write(encode(s))
	return b
}

// Line escribe texto y salta de línea.
func (b *Builder) Line(s string) *Builder { return b.Text(s).LF() }

// LF salta de línea.
func (b *Builder) LF() *Builder {
	b.buf.WriteByte(LF)
	return b
}

// Feed avanza n líneas en blanco (0..255).
func (b *Builder) Feed(n int) *Builder {
	b.buf.Write([]byte{ESC, 'd', byte(clamp(n, 0, 255))}) //nolint:gosec // G115: acotado a 0..255
	return b
}

// Bold activa o desactiva la negrita.
func (b *Builder) Bold(on bool) *Builder {
	b.buf.Write([]byte{ESC, 'E', boolByte(on)})
	return b
}

// Underline activa o desactiva el subrayado.
func (b *Builder) Underline(on bool) *Builder {
	b.buf.Write([]byte{ESC, '-', boolByte(on)})
	return b
}

// Invert imprime blanco sobre negro (útil para encabezados de estación).
func (b *Builder) Invert(on bool) *Builder {
	b.buf.Write([]byte{GS, 'B', boolByte(on)})
	return b
}

// Size fija el multiplicador de ancho y alto (1..8).
func (b *Builder) Size(w, h int) *Builder {
	b.w, b.h = clamp(w, 1, 8), clamp(h, 1, 8)
	b.buf.Write([]byte{GS, '!', byte((b.w-1)<<4 | (b.h - 1))}) //nolint:gosec // G115: w y h acotados a 1..8
	return b
}

// Align fija la alineación.
func (b *Builder) Align(a Align) *Builder {
	b.buf.Write([]byte{ESC, 'a', byte(a)})
	return b
}

// Separator dibuja una línea completa con el carácter dado.
func (b *Builder) Separator(ch rune) *Builder {
	return b.Line(strings.Repeat(string(ch), b.Width()))
}

// Columns escribe texto a la izquierda y a la derecha en la misma línea
// (p. ej. «2 Cerveza» … «$5.00»). Si no cabe, la izquierda se parte en varias líneas
// y la derecha queda en la última.
func (b *Builder) Columns(left, right string) *Builder {
	width := b.Width()
	rw := utf8.RuneCountInString(right)
	lines := Wrap(left, max(width-rw-1, 1))
	for i, l := range lines {
		if i < len(lines)-1 {
			b.Line(l)
			continue
		}
		pad := width - utf8.RuneCountInString(l) - rw
		if pad < 1 {
			b.Line(l)
			pad = width - rw
			l = ""
		}
		b.Line(l + strings.Repeat(" ", pad) + right)
	}
	return b
}

// Wrapped escribe un párrafo partido por palabras al ancho actual, con sangría opcional.
func (b *Builder) Wrapped(s, indent string) *Builder {
	for _, l := range Wrap(s, b.Width()-utf8.RuneCountInString(indent)) {
		b.Line(indent + l)
	}
	return b
}

// MaxBarcodeLen es el máximo de caracteres de un CODE128: el comando GS k lleva la
// longitud en un solo byte (incluidos los 2 bytes de selección de juego «{B»).
const MaxBarcodeLen = 253

// Barcode128 imprime un código de barras CODE128 (juego B) con el texto debajo.
// Un texto más largo que MaxBarcodeLen se recorta: nunca se envía un comando corrupto.
func (b *Builder) Barcode128(data string) *Builder {
	enc := encode(data)
	if len(enc) > MaxBarcodeLen {
		enc = enc[:MaxBarcodeLen]
	}
	payload := append([]byte("{B"), enc...)
	b.buf.Write([]byte{GS, 'H', 2})                      // texto HRI debajo
	b.buf.Write([]byte{GS, 'h', 80})                     // alto en puntos
	b.buf.Write([]byte{GS, 'w', 2})                      // ancho de módulo
	b.buf.Write([]byte{GS, 'k', 73, byte(len(payload))}) //nolint:gosec // G115: len ≤ 255 por MaxBarcodeLen
	b.buf.Write(payload)
	return b // la impresora queda al inicio de línea: no hace falta LF
}

// MaxQRLen es la capacidad en bytes de un QR versión 40 con corrección M.
const MaxQRLen = 2331

// QR imprime un código QR (modelo 2, corrección M) con módulo de 1..16 puntos.
// Si los datos no caben en un QR se imprimen como texto: recortarlos cambiaría su significado.
func (b *Builder) QR(data string, module int) *Builder {
	d := []byte(data) // el QR se codifica en UTF-8, no en PC850
	if len(d) > MaxQRLen {
		return b.Wrapped(data, "")
	}
	// GS ( k pL pH cn fn … : pL/pH cuentan desde cn (= '1', símbolo QR) hasta el final.
	fn := func(args ...byte) {
		n := len(args) + 1
		b.buf.Write([]byte{GS, '(', 'k', byte(n), byte(n >> 8), '1'}) //nolint:gosec // G115: pL/pH son los bytes bajo y alto de n ≤ MaxQRLen+3
		b.buf.Write(args)
	}
	fn('A', '2', 0)                     // modelo 2
	fn('C', byte(clamp(module, 1, 16))) //nolint:gosec // G115: acotado a 1..16
	fn('E', '1')                        // corrección M
	store := append([]byte{'P', '0'}, d...)
	fn(store...)
	fn('Q', '0') // imprimir
	return b
}

// Cut corta el papel dejando margen para que el texto no quede bajo la cuchilla.
func (b *Builder) Cut(partial bool) *Builder {
	m := byte('A')
	if partial {
		m = 'B'
	}
	b.buf.Write([]byte{GS, 'V', m, 3})
	return b
}

// OpenDrawer envía el pulso al cajón conectado a la impresora (pin 2, 50 ms / 250 ms).
func (b *Builder) OpenDrawer() *Builder {
	b.buf.Write([]byte{ESC, 'p', 0, 25, 125})
	return b
}

// Bytes devuelve el documento listo para enviar.
func (b *Builder) Bytes() []byte { return bytes.Clone(b.buf.Bytes()) }

// Wrap parte un texto por palabras sin superar width runas por línea.
func Wrap(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		cur := ""
		for _, w := range strings.Fields(para) {
			for utf8.RuneCountInString(w) > width { // palabra más larga que la línea
				if cur != "" {
					lines, cur = append(lines, cur), ""
				}
				r := []rune(w)
				lines, w = append(lines, string(r[:width])), string(r[width:])
			}
			switch {
			case cur == "":
				cur = w
			case utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w) <= width:
				cur += " " + w
			default:
				lines, cur = append(lines, cur), w
			}
		}
		lines = append(lines, cur)
	}
	return lines
}

// encode convierte UTF-8 a PC850; lo que no existe en PC850 se reemplaza por «?».
func encode(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 0x80 {
			out = append(out, byte(r)) //nolint:gosec // G115: r < 0x80
			continue
		}
		if c, ok := charmap.CodePage850.EncodeRune(r); ok {
			out = append(out, c)
		} else {
			out = append(out, '?')
		}
	}
	return out
}

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }

// String describe el documento (para logs de depuración, sin contenido).
func (b *Builder) String() string {
	return fmt.Sprintf("escpos.Builder{papel:%dmm, bytes:%d}", b.paper, b.buf.Len())
}
