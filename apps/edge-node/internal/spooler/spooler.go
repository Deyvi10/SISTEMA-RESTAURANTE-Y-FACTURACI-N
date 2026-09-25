// Package spooler usa las impresoras que ya están instaladas en Windows en la PC del nodo
// (USB, red o cualquier puerto que Windows sepa usar). Lista las colas del spooler y envía
// los tickets en modo RAW (ESC/POS tal cual, sin el diálogo de impresión ni el driver
// dibujando la página).
package spooler

import (
	"errors"
	"net"
	"regexp"
	"strings"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
)

// ErrNoSoportado: fuera de Windows no hay spooler que listar.
var ErrNoSoportado = errors.New("spooler: las impresoras instaladas solo se leen en Windows")

// Instalada es una impresora de la lista de Windows («Configuración › Impresoras»).
type Instalada struct {
	Nombre         string `json:"nombre"` // nombre de la cola, p. ej. «EPSON TM-T20III Receipt»
	Puerto         string `json:"puerto"` // USB001, IP_192.168.1.50, WSD-…
	Driver         string `json:"driver"`
	Host           string `json:"host,omitempty"` // IP si es un puerto TCP/IP estándar en modo RAW
	PuertoTCP      int    `json:"puertoTcp,omitempty"`
	Estado         string `json:"estado"` // OK, SIN_PAPEL, TAPA_ABIERTA, SIN_CONEXION, ERROR
	AnchoSugerido  int    `json:"anchoSugerido"`
	Predeterminada bool   `json:"predeterminada,omitempty"`
}

// EsRed indica que Windows la usa por red con IP conocida: el nodo le imprime directo.
func (i Instalada) EsRed() bool { return i.Host != "" && i.PuertoTCP > 0 }

// Bits de PRINTER_INFO_2.Status y .Attributes (winspool.h).
const (
	statusError        = 0x00000002
	statusPaperJam     = 0x00000008
	statusPaperOut     = 0x00000010
	statusPaperProblem = 0x00000040
	statusOffline      = 0x00000080
	statusNotAvailable = 0x00001000
	statusDoorOpen     = 0x00400000
	attrWorkOffline    = 0x00000400
)

// EstadoDe traduce el estado del spooler. Windows marca «sin conexión» a muchas
// térmicas USB apenas entran en reposo, así que eso NO frena el envío (el spooler guarda el
// trabajo y lo imprime al volver); solo frenan el papel, la tapa y los errores.
func EstadoDe(status, _ uint32) (escpos.Status, bool) {
	st := escpos.Status{
		PaperOut:  status&(statusPaperOut|statusPaperProblem) != 0,
		CoverOpen: status&statusDoorOpen != 0,
		Error:     status&(statusError|statusPaperJam) != 0,
	}
	return st, true
}

// EtiquetaEstado es el estado legible para la lista del backoffice.
func EtiquetaEstado(status, attributes uint32) string {
	st, _ := EstadoDe(status, attributes)
	switch {
	case st.CoverOpen:
		return "TAPA_ABIERTA"
	case st.PaperOut:
		return "SIN_PAPEL"
	case st.Error:
		return "ERROR"
	case status&(statusOffline|statusNotAvailable) != 0 || attributes&attrWorkOffline != 0:
		return "SIN_CONEXION"
	}
	return "OK"
}

var virtuales = []string{"pdf", "xps", "onenote", "fax", "print to", "document writer", "anydesk", "teamviewer", "snagit", "adobe"}
var puertosVirtuales = []string{"portprompt:", "nul:", "file:", "xpsport:", "shrfax:", "onenote", "microsoft.office"}

// Virtual indica impresoras que no son papel: PDF, XPS, Fax, OneNote…
func Virtual(nombre, puerto, driver string) bool {
	n, p, d := strings.ToLower(nombre), strings.ToLower(puerto), strings.ToLower(driver)
	for _, v := range virtuales {
		if strings.Contains(n, v) || strings.Contains(d, v) {
			return true
		}
	}
	for _, v := range puertosVirtuales {
		if strings.HasPrefix(p, v) {
			return true
		}
	}
	return false
}

var re58 = regexp.MustCompile(`(^|[^0-9])58(mm|[^0-9]|$)`)

// AnchoSugerido adivina el ancho del papel por el nombre (el dueño lo confirma).
func AnchoSugerido(nombre, driver string) int {
	if re58.MatchString(strings.ToLower(nombre + " " + driver)) {
		return 58
	}
	return 80
}

// IPDePuerto saca la IP de nombres de puerto como «IP_192.168.1.50» o «192.168.1.50_1»,
// para cuando el registro no tiene los datos del puerto.
func IPDePuerto(puerto string) string {
	p := strings.TrimPrefix(strings.ToUpper(puerto), "IP_")
	if i := strings.IndexAny(p, "_:"); i > 0 {
		p = p[:i]
	}
	if ip := net.ParseIP(p); ip != nil && ip.To4() != nil {
		return ip.String()
	}
	return ""
}
