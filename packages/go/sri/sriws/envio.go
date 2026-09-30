package sriws

import (
	"context"
	"errors"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// Fase del envío de un comprobante al SRI (docs/05 §10, ficha §5.10–5.12 y §7.4).
type Fase string

const (
	PorEnviar  Fase = "POR_ENVIAR"    // firmado, falta entregarlo a recepción
	Recibido   Fase = "RECIBIDO"      // RECIBIDA: esperar y consultar la autorización
	Autorizada Fase = "AUTORIZADO"    // final
	Rechazada  Fase = "NO_AUTORIZADO" // final hasta corregir: se reenvía con la misma clave y secuencial (§5.10)
	Devuelto   Fase = "DEVUELTO"      // la recepción lo devolvió por un error del comprobante
)

// Final indica que no queda nada automático por hacer.
func (f Fase) Final() bool { return f == Autorizada || f == Rechazada || f == Devuelto }

// Envio es el estado persistible de un comprobante en camino al SRI.
type Envio struct {
	Clave              sri.ClaveAcceso
	Fase               Fase
	Intentos           int       // errores transitorios seguidos
	Proximo            time.Time // cuándo volver a intentar
	Mensajes           []Mensaje
	NumeroAutorizacion string
	FechaAutorizacion  time.Time
	XMLAutorizado      string
	UltimoError        string
	RecibidoAt         time.Time
}

// Politica son los tiempos; la ficha recomienda que la espera tras RECIBIDA sea parametrizable.
type Politica struct {
	EsperaTrasRecepcion time.Duration   // antes de la primera consulta
	EsperaEnProceso     time.Duration   // entre consultas mientras el SRI procesa
	Reintentos          []time.Duration // tras errores transitorios (el último se repite)
	// Alerta: tiempo tras RECIBIDA en que el SRI debió responder (ficha: hasta 24 h).
	Alerta time.Duration
}

// PoliticaPorDefecto: 3 s para la primera consulta, 10 s entre consultas, reintentos de
// 5 s a 15 min, alerta a las 24 h.
var PoliticaPorDefecto = Politica{
	EsperaTrasRecepcion: 3 * time.Second,
	EsperaEnProceso:     10 * time.Second,
	Reintentos:          []time.Duration{5 * time.Second, 15 * time.Second, time.Minute, 5 * time.Minute, 15 * time.Minute},
	Alerta:              24 * time.Hour,
}

// Retrasado indica que el SRI lleva más que la política sin resolver (alerta fiscal).
func (e Envio) Retrasado(ahora time.Time, p Politica) bool {
	return e.Fase == Recibido && !e.RecibidoAt.IsZero() && ahora.Sub(e.RecibidoAt) > p.Alerta
}

// Avanzar hace el siguiente paso de un envío y devuelve el estado nuevo. No guarda nada:
// quien llama persiste el resultado. firmado es el XML firmado (solo se usa si hay que enviar).
//
//   - PorEnviar → recepción: RECIBIDA pasa a Recibido. DEVUELTA con 43 («clave registrada»)
//     o 70 («en procesamiento») significa que el SRI ya lo tiene: también Recibido y se
//     consulta. Cualquier otra DEVUELTA es un error del comprobante: Devuelto.
//   - Recibido → autorización: AUTORIZADO (final), NO AUTORIZADO / RECHAZADO (final hasta
//     corregir), EN PROCESO o sin respuesta: se vuelve a consultar más tarde.
//   - Red, timeout o 5xx: la fase no cambia y se reintenta con la misma clave.
func (c *Cliente) Avanzar(ctx context.Context, e Envio, firmado []byte, ahora time.Time, p Politica) Envio {
	if e.Fase == "" {
		e.Fase = PorEnviar
	}
	if e.Fase.Final() {
		return e
	}
	switch e.Fase {
	case PorEnviar:
		r, err := c.Enviar(ctx, firmado)
		if err != nil {
			return e.fallo(err, ahora, p)
		}
		e.Intentos, e.UltimoError, e.Mensajes = 0, "", r.Mensajes
		switch {
		case r.Estado == Recibida, r.Tiene("43"), r.Tiene("70"):
			e.Fase, e.RecibidoAt, e.Proximo = Recibido, ahora, ahora.Add(p.EsperaTrasRecepcion)
		default:
			e.Fase, e.Proximo = Devuelto, time.Time{}
		}
	case Recibido:
		a, err := c.Consultar(ctx, e.Clave)
		if err != nil {
			return e.fallo(err, ahora, p)
		}
		e.Intentos, e.UltimoError = 0, ""
		if a.Hay {
			e.Mensajes = a.Mensajes
		}
		switch {
		case a.Hay && a.Estado == Autorizado:
			e.Fase, e.Proximo = Autorizada, time.Time{}
			e.NumeroAutorizacion, e.FechaAutorizacion, e.XMLAutorizado = a.NumeroAutorizacion, a.FechaAutorizacion, a.Comprobante
		case a.Hay && (a.Estado == NoAutorizado || a.Estado == Rechazado):
			e.Fase, e.Proximo = Rechazada, time.Time{}
		default: // EN PROCESO o todavía nada
			e.Proximo = ahora.Add(p.EsperaEnProceso)
		}
	}
	return e
}

func (e Envio) fallo(err error, ahora time.Time, p Politica) Envio {
	e.UltimoError = err.Error()
	if !errors.Is(err, ErrTransitorio) {
		// Un error que no es de red (SOAP fault, respuesta rara) también se reintenta, pero
		// queda registrado para revisarlo.
		e.UltimoError = "no transitorio: " + e.UltimoError
	}
	espera := p.Reintentos[min(e.Intentos, len(p.Reintentos)-1)]
	e.Intentos++
	e.Proximo = ahora.Add(espera)
	return e
}
