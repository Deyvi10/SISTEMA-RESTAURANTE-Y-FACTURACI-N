// Package sriws es el cliente de los web services del esquema offline del SRI (F5-10):
// recepción (validarComprobante) y autorización (autorizacionComprobante).
//
// Fuentes (documentacion-proyecto/docs/fuentes/sri/): los WSDL oficiales del ambiente de
// pruebas y la ficha técnica offline v2.34 (§5.10–5.12, §7). Peticiones: el elemento de la
// operación en su namespace y sus hijos sin namespace (elementFormDefault="unqualified"),
// SOAPAction vacío; el comprobante firmado viaja en base64 en <xml>.
package sriws

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// URLs de la ficha técnica §7.2.
const (
	HostPruebas    = "https://celcer.sri.gob.ec"
	HostProduccion = "https://cel.sri.gob.ec"
	rutaRecepcion  = "/comprobantes-electronicos-ws/RecepcionComprobantesOffline"
	rutaAutoriza   = "/comprobantes-electronicos-ws/AutorizacionComprobantesOffline"
	nsRecepcion    = "http://ec.gob.sri.ws.recepcion"
	nsAutorizacion = "http://ec.gob.sri.ws.autorizacion"
	// Tamaño máximo de un comprobante individual (ficha §7).
	MaxComprobante = 320 << 10
)

// Estados de la respuesta del SRI.
const (
	Recibida     = "RECIBIDA"
	Devuelta     = "DEVUELTA"
	Autorizado   = "AUTORIZADO"
	NoAutorizado = "NO AUTORIZADO"
	// Rechazado: el ejemplo oficial de «comprobante no autorizado» de la ficha (§7.2.3)
	// devuelve RECHAZADO en vez de NO AUTORIZADO; se tratan igual.
	Rechazado = "RECHAZADO"
	EnProceso = "EN PROCESO"
)

// Mensaje de error o advertencia del SRI (Tabla de la ficha §11).
type Mensaje struct {
	Identificador        string `xml:"identificador"`
	Mensaje              string `xml:"mensaje"`
	InformacionAdicional string `xml:"informacionAdicional"`
	Tipo                 string `xml:"tipo"`
}

func (m Mensaje) String() string {
	s := m.Identificador + " " + m.Mensaje
	if m.InformacionAdicional != "" {
		s += ": " + m.InformacionAdicional
	}
	return s
}

// Recepcion es la respuesta de validarComprobante.
type Recepcion struct {
	Estado   string
	Mensajes []Mensaje
}

// Tiene indica si trae el mensaje con ese identificador (p. ej. 43, «clave registrada»).
func (r Recepcion) Tiene(id string) bool {
	for _, m := range r.Mensajes {
		if m.Identificador == id {
			return true
		}
	}
	return false
}

// Autorizacion es la respuesta de autorizacionComprobante. Hay = false si el SRI todavía no
// tiene nada para esa clave (numeroComprobantes 0).
type Autorizacion struct {
	Hay                bool
	Estado             string
	NumeroAutorizacion string
	FechaAutorizacion  time.Time
	Ambiente           string
	Comprobante        string // XML autorizado (firmado) tal como lo devuelve el SRI
	Mensajes           []Mensaje
}

// ErrTransitorio: la red, un timeout o un 5xx; se reintenta con la misma clave.
var ErrTransitorio = errors.New("sriws: error transitorio del SRI")

// Cliente habla con un ambiente del SRI (o con el stub, cambiando Host).
type Cliente struct {
	Host string
	HTTP *http.Client
}

// NuevoCliente apunta al ambiente de la clave (1 pruebas, 2 producción).
func NuevoCliente(ambiente sri.Ambiente, h *http.Client) *Cliente {
	host := HostPruebas
	if ambiente == sri.AmbienteProduccion {
		host = HostProduccion
	}
	if h == nil {
		h = &http.Client{Timeout: 30 * time.Second}
	}
	return &Cliente{Host: host, HTTP: h}
}

// Enviar entrega el comprobante firmado al servicio de recepción.
func (c *Cliente) Enviar(ctx context.Context, firmado []byte) (Recepcion, error) {
	if len(firmado) > MaxComprobante {
		return Recepcion{}, fmt.Errorf("sriws: el comprobante pesa %d bytes y el máximo es %d", len(firmado), MaxComprobante)
	}
	cuerpo := `<ec:validarComprobante xmlns:ec="` + nsRecepcion + `"><xml>` + base64.StdEncoding.EncodeToString(firmado) + `</xml></ec:validarComprobante>`
	var out struct {
		Estado       string `xml:"Body>validarComprobanteResponse>RespuestaRecepcionComprobante>estado"`
		Comprobantes []struct {
			Mensajes []Mensaje `xml:"mensajes>mensaje"`
		} `xml:"Body>validarComprobanteResponse>RespuestaRecepcionComprobante>comprobantes>comprobante"`
	}
	if err := c.llamar(ctx, rutaRecepcion, cuerpo, &out); err != nil {
		return Recepcion{}, err
	}
	r := Recepcion{Estado: limpio(out.Estado)}
	for _, cp := range out.Comprobantes {
		for _, m := range cp.Mensajes {
			r.Mensajes = append(r.Mensajes, limpiarMensaje(m))
		}
	}
	if r.Estado != Recibida && r.Estado != Devuelta {
		return r, fmt.Errorf("sriws: estado de recepción desconocido %q", r.Estado)
	}
	return r, nil
}

// Consultar pide el resultado de la autorización de una clave.
func (c *Cliente) Consultar(ctx context.Context, clave sri.ClaveAcceso) (Autorizacion, error) {
	cuerpo := `<ec:autorizacionComprobante xmlns:ec="` + nsAutorizacion + `"><claveAccesoComprobante>` + clave.String() + `</claveAccesoComprobante></ec:autorizacionComprobante>`
	var out struct {
		Numero         string `xml:"Body>autorizacionComprobanteResponse>RespuestaAutorizacionComprobante>numeroComprobantes"`
		Autorizaciones []struct {
			Estado      string    `xml:"estado"`
			Numero      string    `xml:"numeroAutorizacion"`
			Fecha       string    `xml:"fechaAutorizacion"`
			Ambiente    string    `xml:"ambiente"`
			Comprobante string    `xml:"comprobante"`
			Mensajes    []Mensaje `xml:"mensajes>mensaje"`
		} `xml:"Body>autorizacionComprobanteResponse>RespuestaAutorizacionComprobante>autorizaciones>autorizacion"`
	}
	if err := c.llamar(ctx, rutaAutoriza, cuerpo, &out); err != nil {
		return Autorizacion{}, err
	}
	if len(out.Autorizaciones) == 0 {
		return Autorizacion{}, nil
	}
	// Si el SRI guarda varios intentos, el primero autorizado manda; si no, el último (§5.11).
	elegida := out.Autorizaciones[len(out.Autorizaciones)-1]
	for _, a := range out.Autorizaciones {
		if limpio(a.Estado) == Autorizado {
			elegida = a
			break
		}
	}
	a := Autorizacion{Hay: true, Estado: limpio(elegida.Estado), NumeroAutorizacion: limpio(elegida.Numero),
		Ambiente: limpio(elegida.Ambiente), Comprobante: strings.TrimSpace(elegida.Comprobante)}
	if f := limpio(elegida.Fecha); f != "" {
		t, err := time.Parse(time.RFC3339Nano, f)
		if err != nil {
			return a, fmt.Errorf("sriws: fecha de autorización %q: %w", f, err)
		}
		a.FechaAutorizacion = t
	}
	for _, m := range elegida.Mensajes {
		a.Mensajes = append(a.Mensajes, limpiarMensaje(m))
	}
	switch a.Estado {
	case Autorizado, NoAutorizado, Rechazado, EnProceso:
	default:
		return a, fmt.Errorf("sriws: estado de autorización desconocido %q", a.Estado)
	}
	if a.Estado == Autorizado && a.NumeroAutorizacion == "" {
		return a, errors.New("sriws: autorizado sin número de autorización")
	}
	return a, nil
}

func (c *Cliente) llamar(ctx context.Context, ruta, cuerpo string, out any) error {
	env := `<?xml version="1.0" encoding="UTF-8"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Header/><soapenv:Body>` +
		cuerpo + `</soapenv:Body></soapenv:Envelope>`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Host+ruta, strings.NewReader(env))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", `""`)
	res, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", ErrTransitorio, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransitorio, err)
	}
	if res.StatusCode >= 500 && !bytes.Contains(body, []byte("Fault")) {
		return fmt.Errorf("%w: HTTP %d", ErrTransitorio, res.StatusCode)
	}
	var falla struct {
		Texto string `xml:"Body>Fault>faultstring"`
	}
	if xml.Unmarshal(body, &falla) == nil && strings.TrimSpace(falla.Texto) != "" {
		return fmt.Errorf("sriws: el SRI rechazó la petición: %s", strings.TrimSpace(falla.Texto))
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("sriws: HTTP %d", res.StatusCode)
	}
	if err := xml.Unmarshal(body, out); err != nil {
		return fmt.Errorf("sriws: respuesta ilegible: %w", err)
	}
	return nil
}

func limpio(s string) string { return strings.Join(strings.Fields(s), " ") }

func limpiarMensaje(m Mensaje) Mensaje {
	return Mensaje{Identificador: limpio(m.Identificador), Mensaje: limpio(m.Mensaje),
		InformacionAdicional: limpio(m.InformacionAdicional), Tipo: limpio(m.Tipo)}
}
