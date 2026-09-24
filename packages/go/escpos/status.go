package escpos

// Consultas de estado en tiempo real DLE EOT n (RF-02-05). La impresora responde un
// byte por consulta. Los bits 1 y 4 siempre valen 1 y el bit 7 siempre 0 (Epson y
// compatibles); si no se cumple, la respuesta no es un estado válido.
const (
	StatusPrinter = 1 // bit 3: fuera de línea
	StatusOffline = 2 // bit 2: tapa abierta · bit 5: detenida por falta de papel · bit 6: error
	StatusError   = 3 // bit 3: error de cuchilla · bit 5: error irrecuperable
	StatusPaper   = 4 // bits 2-3: papel por acabarse · bits 5-6: sin papel
)

// StatusRequest devuelve los bytes de la consulta n.
func StatusRequest(n byte) []byte { return []byte{DLE, EOT, n} }

// Status es el estado combinado de la impresora.
type Status struct {
	Offline      bool
	CoverOpen    bool
	PaperOut     bool
	PaperNearEnd bool
	Error        bool
}

// OK indica si la impresora puede imprimir.
func (s Status) OK() bool { return !s.Offline && !s.CoverOpen && !s.PaperOut && !s.Error }

// Motivo describe el problema en lenguaje claro para la caja y el mesero (RNF-33).
func (s Status) Motivo() string {
	switch {
	case s.CoverOpen:
		return "La tapa de la impresora está abierta. Ciérrala para seguir imprimiendo."
	case s.PaperOut:
		return "La impresora se quedó sin papel. Cambia el rollo; lo pendiente se imprime solo."
	case s.Error:
		return "La impresora reporta un error. Apágala y vuelve a encenderla."
	case s.Offline:
		return "La impresora está fuera de línea. Revisa que esté encendida."
	case s.PaperNearEnd:
		return "Queda poco papel en la impresora."
	default:
		return ""
	}
}

// ValidStatusByte comprueba el patrón fijo de un byte de estado.
func ValidStatusByte(b byte) bool { return b&0x93 == 0x12 }

// ApplyStatus incorpora la respuesta a la consulta n. Devuelve false si el byte no es válido.
func (s *Status) ApplyStatus(n, b byte) bool {
	if !ValidStatusByte(b) {
		return false
	}
	switch n {
	case StatusPrinter:
		s.Offline = b&0x08 != 0
	case StatusOffline:
		s.CoverOpen = b&0x04 != 0
		s.PaperOut = s.PaperOut || b&0x20 != 0
		s.Error = s.Error || b&0x40 != 0
	case StatusError:
		s.Error = s.Error || b&0x28 != 0
	case StatusPaper:
		s.PaperNearEnd = b&0x0C != 0
		s.PaperOut = s.PaperOut || b&0x60 != 0
	}
	return true
}

// StatusByte construye la respuesta que daría una impresora con el estado s.
// La usa el simulador (tools/printer-sim) y las pruebas.
func StatusByte(n byte, s Status) byte {
	b := byte(0x12)
	switch n {
	case StatusPrinter:
		if s.Offline || s.CoverOpen || s.PaperOut || s.Error {
			b |= 0x08
		}
	case StatusOffline:
		if s.CoverOpen {
			b |= 0x04
		}
		if s.PaperOut {
			b |= 0x20
		}
		if s.Error {
			b |= 0x40
		}
	case StatusError:
		if s.Error {
			b |= 0x20
		}
	case StatusPaper:
		if s.PaperNearEnd {
			b |= 0x0C
		}
		if s.PaperOut {
			b |= 0x60
		}
	}
	return b
}
