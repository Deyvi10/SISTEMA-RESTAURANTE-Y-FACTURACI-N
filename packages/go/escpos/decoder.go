package escpos

import (
	"fmt"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// Style es el formato vigente de un tramo de texto.
type Style struct {
	Bold, Underline, Invert bool
	W, H                    int
}

// Run es un tramo de texto con un mismo estilo.
type Run struct {
	Text  string
	Style Style
}

// ElementKind distingue los bloques de un documento decodificado.
type ElementKind string

const (
	KindText    ElementKind = "text"
	KindBarcode ElementKind = "barcode"
	KindQR      ElementKind = "qr"
	KindCut     ElementKind = "cut"
	KindDrawer  ElementKind = "drawer"
)

// Element es una línea de texto, un código o un evento físico (corte, cajón).
type Element struct {
	Kind  ElementKind
	Align Align
	Runs  []Run  // KindText
	Data  string // KindBarcode, KindQR
}

// PlainText devuelve el texto de una línea sin formato.
func (e Element) PlainText() string {
	var sb strings.Builder
	for _, r := range e.Runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

// Document es el resultado de decodificar un trabajo de impresión.
type Document struct {
	Elements []Element
	// StatusQueries son las consultas DLE EOT n encontradas, en orden.
	StatusQueries []byte
	// Warnings lista comandos desconocidos o mal formados.
	Warnings []string
	// Consumed es cuántos bytes se procesaron; el resto es un comando incompleto
	// que puede completarse con más datos de la conexión.
	Consumed int
}

// Text devuelve el documento como texto plano, una línea por elemento.
func (d Document) Text() string {
	var sb strings.Builder
	for _, e := range d.Elements {
		switch e.Kind {
		case KindText:
			sb.WriteString(e.PlainText())
		case KindBarcode:
			sb.WriteString("[CODE128 " + e.Data + "]")
		case KindQR:
			sb.WriteString("[QR " + e.Data + "]")
		case KindCut:
			sb.WriteString("-------- corte --------")
		case KindDrawer:
			sb.WriteString("[abrir cajón]")
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

var decoder850 = charmap.CodePage850

// Decode interpreta un flujo ESC/POS. Nunca falla: lo desconocido se reporta en Warnings
// y un comando cortado al final queda sin consumir (ver Document.Consumed).
func Decode(data []byte) Document {
	d := &decodeState{doc: Document{}, style: Style{W: 1, H: 1}}
	i := 0
	for i < len(data) {
		n, ok := d.step(data[i:])
		if !ok {
			break // comando incompleto: esperar más bytes
		}
		i += n
	}
	d.flushLine(false)
	d.doc.Consumed = i
	return d.doc
}

type decodeState struct {
	doc    Document
	style  Style
	align  Align
	runs   []Run
	cur    strings.Builder
	qrData string
}

func (d *decodeState) flushRun() {
	if d.cur.Len() == 0 {
		return
	}
	d.runs = append(d.runs, Run{Text: d.cur.String(), Style: d.style})
	d.cur.Reset()
}

// flushLine cierra la línea en curso. Con force=true emite también líneas vacías (LF).
func (d *decodeState) flushLine(force bool) {
	d.flushRun()
	if len(d.runs) == 0 && !force {
		return
	}
	d.doc.Elements = append(d.doc.Elements, Element{Kind: KindText, Align: d.align, Runs: d.runs})
	d.runs = nil
}

func (d *decodeState) setStyle(s Style) {
	d.flushRun()
	d.style = s
}

func (d *decodeState) warn(format string, a ...any) {
	d.doc.Warnings = append(d.doc.Warnings, fmt.Sprintf(format, a...))
}

// step procesa un comando o carácter al inicio de b y devuelve cuántos bytes usó.
func (d *decodeState) step(b []byte) (int, bool) {
	need := func(n int) bool { return len(b) >= n }
	switch c := b[0]; c {
	case LF:
		d.flushLine(true)
		return 1, true
	case '\r':
		return 1, true
	case DLE:
		if !need(3) {
			return 0, false
		}
		if b[1] == EOT {
			d.doc.StatusQueries = append(d.doc.StatusQueries, b[2])
			return 3, true
		}
		d.warn("DLE 0x%02x desconocido", b[1])
		return 2, true
	case ESC:
		if !need(2) {
			return 0, false
		}
		switch b[1] {
		case '@':
			d.flushLine(false)
			d.setStyle(Style{W: 1, H: 1})
			d.align = Left
			return 2, true
		case '2':
			return 2, true
		}
		if !need(3) {
			return 0, false
		}
		arg := b[2]
		s := d.style
		switch b[1] {
		case 'E':
			s.Bold = arg&1 == 1
			d.setStyle(s)
		case '-':
			s.Underline = arg&3 != 0
			d.setStyle(s)
		case 'a':
			d.flushLine(false)
			d.align = Align(arg % 3)
		case 'd':
			d.flushLine(false)
			for range int(arg) {
				d.flushLine(true)
			}
		case 't', '3', 'M', 'R':
			// página de códigos, interlineado, fuente, juego internacional: sin efecto visual aquí
		case 'p':
			if !need(5) {
				return 0, false
			}
			d.flushLine(false)
			d.doc.Elements = append(d.doc.Elements, Element{Kind: KindDrawer})
			return 5, true
		default:
			d.warn("ESC %q desconocido", b[1])
			return 2, true
		}
		return 3, true
	case GS:
		if !need(3) {
			return 0, false
		}
		arg := b[2]
		s := d.style
		switch b[1] {
		case '!':
			s.W, s.H = int(arg>>4)+1, int(arg&0x0F)+1
			d.setStyle(s)
		case 'B':
			s.Invert = arg&1 == 1
			d.setStyle(s)
		case 'H', 'h', 'w', 'f':
			// formato del código de barras
		case 'V':
			size := 3
			if arg == 'A' || arg == 'B' { // modo con avance: GS V m n
				size = 4
			}
			if !need(size) {
				return 0, false
			}
			d.flushLine(false)
			d.doc.Elements = append(d.doc.Elements, Element{Kind: KindCut})
			return size, true
		case 'k':
			return d.barcode(b)
		case '(':
			return d.function(b)
		default:
			d.warn("GS %q desconocido", b[1])
			return 2, true
		}
		return 3, true
	default:
		if c < 0x20 {
			d.warn("control 0x%02x ignorado", c)
			return 1, true
		}
		d.cur.WriteRune(decoder850.DecodeByte(c))
		return 1, true
	}
}

// barcode: GS k m n d1…dn (formato B, m ≥ 65).
func (d *decodeState) barcode(b []byte) (int, bool) {
	if len(b) < 4 {
		return 0, false
	}
	m, n := b[2], int(b[3])
	if m < 65 {
		d.warn("código de barras formato A (m=%d) no soportado", m)
		return 3, true
	}
	if len(b) < 4+n {
		return 0, false
	}
	data := decodeString(b[4 : 4+n])
	if m == 73 {
		data = strings.TrimPrefix(strings.TrimPrefix(data, "{B"), "{A")
	}
	d.flushLine(false)
	d.doc.Elements = append(d.doc.Elements, Element{Kind: KindBarcode, Align: d.align, Data: data})
	return 4 + n, true
}

// function: GS ( k pL pH cn fn … (solo QR, cn = '1').
func (d *decodeState) function(b []byte) (int, bool) {
	if len(b) < 5 {
		return 0, false
	}
	if b[2] != 'k' {
		d.warn("GS ( %q no soportado", b[2])
		n := int(b[3]) | int(b[4])<<8
		if len(b) < 5+n {
			return 0, false
		}
		return 5 + n, true
	}
	n := int(b[3]) | int(b[4])<<8
	if len(b) < 5+n {
		return 0, false
	}
	params := b[5 : 5+n]
	if len(params) >= 2 && params[0] == '1' {
		switch params[1] {
		case 'P': // almacenar datos: '1' 'P' '0' datos…
			if len(params) >= 3 {
				d.qrData = string(params[3:])
			}
		case 'Q': // imprimir
			d.flushLine(false)
			d.doc.Elements = append(d.doc.Elements, Element{Kind: KindQR, Align: d.align, Data: d.qrData})
		}
	}
	return 5 + n, true
}

func decodeString(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		sb.WriteRune(decoder850.DecodeByte(c))
	}
	return sb.String()
}
