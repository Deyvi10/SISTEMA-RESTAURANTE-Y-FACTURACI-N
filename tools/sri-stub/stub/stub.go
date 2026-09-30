// Package stub imita los web services SOAP del esquema offline del SRI (F0-07).
//
// La forma de peticiones y respuestas sigue los WSDL oficiales del ambiente de pruebas y los
// ejemplos de la ficha técnica offline v2.34 (documentacion-proyecto/docs/fuentes/sri/).
// El resultado de cada comprobante se elige por los 3 últimos dígitos del secuencial (o con
// la cabecera X-Escenario):
//
//	901 DEVUELTA por error de estructura     905 DEVUELTA «clave registrada» (43)
//	902 timeout (no responde a tiempo)        906 NO AUTORIZADO
//	903 HTTP 503                              907 DEVUELTA «en procesamiento» (70)
//	904 EN PROCESO 2 consultas y luego        908 RECHAZADO (como el ejemplo oficial)
//	    AUTORIZADO
//	cualquier otro: RECIBIDA → AUTORIZADO
package stub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

const (
	RutaRecepcion    = "/comprobantes-electronicos-ws/RecepcionComprobantesOffline"
	RutaAutorizacion = "/comprobantes-electronicos-ws/AutorizacionComprobantesOffline"
)

// Escenario forzado para un comprobante.
type Escenario string

const (
	Autorizado    Escenario = "AUTORIZADO"
	Devuelta      Escenario = "DEVUELTA"
	Timeout       Escenario = "TIMEOUT"
	Error503      Escenario = "ERROR_503"
	EnProceso     Escenario = "EN_PROCESO"
	Registrada    Escenario = "CLAVE_REGISTRADA"
	NoAutorizado  Escenario = "NO_AUTORIZADO"
	Procesamiento Escenario = "EN_PROCESAMIENTO"
	Rechazado     Escenario = "RECHAZADO"
)

var porSufijo = map[string]Escenario{
	"901": Devuelta, "902": Timeout, "903": Error503, "904": EnProceso,
	"905": Registrada, "906": NoAutorizado, "907": Procesamiento, "908": Rechazado,
}

type comprobante struct {
	Clave      string    `json:"claveAcceso"`
	Escenario  Escenario `json:"escenario"`
	XML        string    `json:"-"`
	Recibido   time.Time `json:"recibido"`
	Consultas  int       `json:"consultasAutorizacion"`
	Autorizado time.Time `json:"autorizado,omitzero"`
}

// Stub guarda en memoria lo recibido.
type Stub struct {
	mu     sync.Mutex
	comps  map[string]*comprobante
	demora time.Duration
	log    *slog.Logger
}

func NewStub(demora time.Duration, log *slog.Logger) *Stub {
	return &Stub{comps: map[string]*comprobante{}, demora: demora, log: log}
}

func (s *Stub) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+RutaRecepcion, s.recepcion)
	mux.HandleFunc("POST "+RutaAutorizacion, s.autorizacion)
	mux.HandleFunc("GET /api/comprobantes", s.listar)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "Stub del SRI (esquema offline)\n\nPOST %s\nPOST %s\nGET  /api/comprobantes\n", RutaRecepcion, RutaAutorizacion)
	})
	return mux
}

func escenarioPara(r *http.Request, clave sri.ClaveAcceso) Escenario {
	if h := Escenario(strings.ToUpper(r.Header.Get("X-Escenario"))); h != "" {
		return h
	}
	if e, ok := porSufijo[clave.Secuencial()[6:]]; ok {
		return e
	}
	return Autorizado
}

func (s *Stub) recepcion(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		soapFault(w, "Solicitud demasiado grande o ilegible")
		return
	}
	b64 := elemento(body, "xml")
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if b64 == "" || err != nil {
		writeSOAP(w, recepcionResp("DEVUELTA", "", mensaje("35", "ARCHIVO NO CUMPLE ESTRUCTURA XML", "El elemento <xml> debe traer el comprobante en base64", "ERROR")))
		return
	}
	clave, err := sri.ParseClaveAcceso(strings.TrimSpace(elemento(raw, "claveAcceso")))
	if err != nil {
		writeSOAP(w, recepcionResp("DEVUELTA", "", mensaje("35", "ARCHIVO NO CUMPLE ESTRUCTURA XML", "Clave de acceso ausente o inválida: "+err.Error(), "ERROR")))
		return
	}

	esc := escenarioPara(r, clave)
	switch esc {
	case Timeout:
		select {
		case <-time.After(s.demora):
		case <-r.Context().Done():
			return
		}
	case Error503:
		http.Error(w, "Servicio no disponible", http.StatusServiceUnavailable)
		return
	}

	s.mu.Lock()
	previo, existe := s.comps[clave.String()]
	if !existe {
		s.comps[clave.String()] = &comprobante{Clave: clave.String(), Escenario: esc, XML: string(raw), Recibido: time.Now()}
	}
	s.mu.Unlock()
	s.log.Info("recepción", "clave", clave.String(), "escenario", esc)

	switch {
	case existe && previo.Escenario != Devuelta, esc == Registrada:
		writeSOAP(w, recepcionResp("DEVUELTA", clave.String(), mensaje("43", "CLAVE ACCESO REGISTRADA", "", "ERROR")))
	case esc == Procesamiento:
		writeSOAP(w, recepcionResp("DEVUELTA", clave.String(), mensaje("70", "CLAVE DE ACCESO EN PROCESAMIENTO", "", "ERROR")))
	case esc == Devuelta:
		writeSOAP(w, recepcionResp("DEVUELTA", clave.String(), mensaje("35", "ARCHIVO NO CUMPLE ESTRUCTURA XML", "Escenario de prueba 901", "ERROR")))
	default:
		writeSOAP(w, recepcionResp("RECIBIDA", "", ""))
	}
}

func (s *Stub) autorizacion(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		soapFault(w, "Solicitud ilegible")
		return
	}
	clave := strings.TrimSpace(elemento(body, "claveAccesoComprobante"))

	s.mu.Lock()
	c := s.comps[clave]
	var estado string
	if c != nil {
		c.Consultas++
		switch {
		case c.Escenario == NoAutorizado:
			estado = "NO AUTORIZADO"
		case c.Escenario == Rechazado:
			estado = "RECHAZADO" // así responde el ejemplo oficial de la ficha (§7.2.3)
		case c.Escenario == EnProceso && c.Consultas <= 2:
			estado = "EN PROCESO"
		case c.Escenario == Devuelta:
			estado = "" // un comprobante devuelto nunca llega a autorización
		default:
			estado = "AUTORIZADO"
			if c.Autorizado.IsZero() {
				c.Autorizado = time.Now()
			}
		}
	}
	s.mu.Unlock()

	if c == nil || estado == "" {
		writeSOAP(w, autorizacionResp(clave, 0, ""))
		return
	}
	amb := "PRUEBAS"
	if clave[23] == '2' {
		amb = "PRODUCCIÓN"
	}
	var aut string
	switch estado {
	case "EN PROCESO":
		aut = "<autorizacion><estado>EN PROCESO</estado><mensajes/></autorizacion>"
	case "NO AUTORIZADO", "RECHAZADO":
		aut = fmt.Sprintf("<autorizacion><estado>%s</estado><fechaAutorizacion>%s</fechaAutorizacion><ambiente>%s</ambiente><comprobante>%s</comprobante><mensajes>%s</mensajes></autorizacion>",
			estado, time.Now().Format(time.RFC3339), amb, cdata(c.XML), mensaje("56", "ERROR ESTABLECIMIENTO CERRADO", "Escenario de prueba "+string(c.Escenario), "ERROR"))
	default:
		aut = fmt.Sprintf("<autorizacion><estado>AUTORIZADO</estado><numeroAutorizacion>%s</numeroAutorizacion><fechaAutorizacion>%s</fechaAutorizacion><ambiente>%s</ambiente><comprobante>%s</comprobante><mensajes/></autorizacion>",
			clave, c.Autorizado.Format(time.RFC3339), amb, cdata(c.XML))
	}
	writeSOAP(w, autorizacionResp(clave, 1, aut))
}

func (s *Stub) listar(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	out := make([]comprobante, 0, len(s.comps))
	for _, c := range s.comps {
		out = append(out, *c)
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Recibido.Before(out[j].Recibido) })
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(out)
}

// elemento devuelve el texto del primer elemento con ese nombre local (ignora prefijos).
func elemento(doc []byte, local string) string {
	d := xml.NewDecoder(bytes.NewReader(doc))
	for {
		tok, err := d.Token()
		if err != nil {
			return ""
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == local {
			var v string
			if d.DecodeElement(&v, &se) != nil {
				return ""
			}
			return v
		}
	}
}

func mensaje(id, msg, info, tipo string) string {
	var b strings.Builder
	b.WriteString("<mensaje><identificador>" + id + "</identificador><mensaje>")
	_ = xml.EscapeText(&b, []byte(msg))
	b.WriteString("</mensaje>")
	if info != "" {
		b.WriteString("<informacionAdicional>")
		_ = xml.EscapeText(&b, []byte(info))
		b.WriteString("</informacionAdicional>")
	}
	b.WriteString("<tipo>" + tipo + "</tipo></mensaje>")
	return b.String()
}

func recepcionResp(estado, clave, mensajes string) string {
	comps := "<comprobantes/>"
	if mensajes != "" {
		comps = "<comprobantes><comprobante><claveAcceso>" + clave + "</claveAcceso><mensajes>" + mensajes + "</mensajes></comprobante></comprobantes>"
	}
	return `<ns2:validarComprobanteResponse xmlns:ns2="http://ec.gob.sri.ws.recepcion"><RespuestaRecepcionComprobante><estado>` +
		estado + `</estado>` + comps + `</RespuestaRecepcionComprobante></ns2:validarComprobanteResponse>`
}

func autorizacionResp(clave string, n int, autorizaciones string) string {
	return fmt.Sprintf(`<ns2:autorizacionComprobanteResponse xmlns:ns2="http://ec.gob.sri.ws.autorizacion"><RespuestaAutorizacionComprobante><claveAccesoConsultada>%s</claveAccesoConsultada><numeroComprobantes>%d</numeroComprobantes><autorizaciones>%s</autorizaciones></RespuestaAutorizacionComprobante></ns2:autorizacionComprobanteResponse>`,
		clave, n, autorizaciones)
}

func cdata(s string) string {
	return "<![CDATA[" + strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>") + "]]>"
}

func writeSOAP(w http.ResponseWriter, body string) { writeSOAPStatus(w, http.StatusOK, body) }

func writeSOAPStatus(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body>`+body+`</soap:Body></soap:Envelope>`)
}

func soapFault(w http.ResponseWriter, msg string) {
	writeSOAPStatus(w, http.StatusInternalServerError, "<soap:Fault><faultcode>soap:Client</faultcode><faultstring>"+msg+"</faultstring></soap:Fault>")
}
