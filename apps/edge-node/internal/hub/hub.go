// Package hub es el canal de tiempo real del Nodo Local (F2-06, RF-02-08): un WebSocket por
// dispositivo (caja, KDS, app de meseros) con mensajes tipados desde contracts/events.
//
// Difundir no bloquea nunca: cada cliente tiene su cola; si un cliente lento la llena, se
// le desconecta (se reconecta solo y pide el estado actual), así un teléfono con mala señal
// no retrasa a los demás.
package hub

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

const (
	// Un cliente es lento cuando su mensaje pendiente más antiguo lleva lentoTras sin salir
	// (no por el tamaño de la cola: una ráfaga legítima no debe tumbar a un teléfono sano).
	lentoTras = 3 * time.Second
	// Tope de memoria por cliente mientras se decide si está lento.
	maxBytesCliente = 16 << 20
	pingCada        = 20 * time.Second
	escrituraMax    = 10 * time.Second
	mensajeMax      = 64 << 10
)

// Cliente es un dispositivo conectado.
type Cliente struct {
	ID            ids.ID
	Tipo          string // POS, KDS, MOVIL, ESTADO…
	DispositivoID *ids.ID
	UsuarioID     *ids.ID

	mu         sync.Mutex
	pendientes []pendiente
	bytes      int
	senal      chan struct{} // hay mensajes para el escritor
	cerrar     func(websocket.StatusCode, string)
}

type pendiente struct {
	msg []byte
	at  time.Time
}

// encolar agrega un mensaje sin bloquear; false si el cliente superó su tope de memoria.
func (c *Cliente) encolar(msg []byte, now time.Time) bool {
	c.mu.Lock()
	if c.bytes+len(msg) > maxBytesCliente {
		c.mu.Unlock()
		return false
	}
	c.pendientes = append(c.pendientes, pendiente{msg: msg, at: now})
	c.bytes += len(msg)
	c.mu.Unlock()
	select {
	case c.senal <- struct{}{}:
	default:
	}
	return true
}

// siguiente saca el mensaje más antiguo (nil si no hay).
func (c *Cliente) siguiente() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pendientes) == 0 {
		return nil
	}
	p := c.pendientes[0]
	c.pendientes[0] = pendiente{}
	c.pendientes = c.pendientes[1:]
	c.bytes -= len(p.msg)
	return p.msg
}

// atascado: el pendiente más antiguo lleva demasiado sin salir.
func (c *Cliente) atascado(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pendientes) > 0 && now.Sub(c.pendientes[0].at) > lentoTras
}

// Autenticar decide si una conexión entra y quién es. Devuelve error para rechazarla.
type Autenticar func(r *http.Request) (Cliente, error)

// Recibir procesa un mensaje entrante válido (bloqueos de mesa, heartbeats… en F3).
type Recibir func(ctx context.Context, c *Cliente, s eventos.Sobre)

type Hub struct {
	Log     *slog.Logger
	Now     func() time.Time
	Recibir Recibir

	mu       sync.RWMutex
	clientes map[*Cliente]struct{}
}

func New(log *slog.Logger, now func() time.Time) *Hub {
	return &Hub{Log: log, Now: now, clientes: map[*Cliente]struct{}{}}
}

// Conectados devuelve cuántos dispositivos hay en línea.
func (h *Hub) Conectados() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clientes)
}

// Difundir envía un evento a todos los conectados.
func (h *Hub) Difundir(e eventos.Evento) error {
	return h.DifundirA(e, nil)
}

// DifundirA envía un evento a los clientes que cumplan el filtro (nil = todos).
func (h *Hub) DifundirA(e eventos.Evento, filtro func(*Cliente) bool) error {
	s, err := eventos.Nuevo(e, h.Now())
	if err != nil {
		return err
	}
	msg, err := json.Marshal(s)
	if err != nil {
		return err
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clientes {
		if filtro != nil && !filtro(c) {
			continue
		}
		if !c.encolar(msg, time.Now()) {
			go c.cerrar(websocket.StatusPolicyViolation, "cliente lento: reconecta")
		}
	}
	return nil
}

// Desconectar cierra las conexiones que cumplan el filtro (p. ej. un dispositivo revocado).
func (h *Hub) Desconectar(filtro func(*Cliente) bool) {
	h.mu.RLock()
	var cerrar []*Cliente
	for c := range h.clientes {
		if filtro(c) {
			cerrar = append(cerrar, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range cerrar {
		c.cerrar(websocket.StatusPolicyViolation, "dispositivo revocado")
	}
}

func (h *Hub) agregar(c *Cliente) {
	h.mu.Lock()
	h.clientes[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) quitar(c *Cliente) {
	h.mu.Lock()
	delete(h.clientes, c)
	h.mu.Unlock()
}

// Handler acepta conexiones WebSocket en /v1/ws.
func (h *Hub) Handler(auth Autenticar) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cli, err := auth(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		// El chequeo de Origin por defecto de websocket.Accept rechaza páginas de otros
		// sitios; las apps nativas no envían Origin y entran.
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		conn.SetReadLimit(mensajeMax)
		c := &cli
		c.ID = ids.New()
		c.senal = make(chan struct{}, 1)
		var once sync.Once
		c.cerrar = func(code websocket.StatusCode, motivo string) {
			once.Do(func() { _ = conn.Close(code, motivo) })
		}
		h.agregar(c)
		defer h.quitar(c)
		h.Log.Info("dispositivo conectado", "cliente", c.ID, "tipo", c.Tipo, "conectados", h.Conectados())

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		go h.escribir(ctx, conn, c, cancel)
		go vigilar(ctx, c)
		h.leer(ctx, conn, c)
		c.cerrar(websocket.StatusNormalClosure, "")
		h.Log.Info("dispositivo desconectado", "cliente", c.ID)
	})
}

func (h *Hub) escribir(ctx context.Context, conn *websocket.Conn, c *Cliente, cancel func()) {
	defer cancel()
	ping := time.NewTicker(pingCada)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.senal:
			for msg := c.siguiente(); msg != nil; msg = c.siguiente() {
				wctx, wcancel := context.WithTimeout(ctx, escrituraMax)
				err := conn.Write(wctx, websocket.MessageText, msg)
				wcancel()
				if err != nil {
					return
				}
			}
		case <-ping.C:
			pctx, pcancel := context.WithTimeout(ctx, escrituraMax)
			err := conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
		}
	}
}

// vigilar desconecta al cliente cuyo pendiente más antiguo no sale a tiempo: así un teléfono
// que dejó de leer no acumula memoria ni frena la difusión a los demás.
func vigilar(ctx context.Context, c *Cliente) {
	t := time.NewTicker(lentoTras / 6)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			if c.atascado(now) {
				c.cerrar(websocket.StatusPolicyViolation, "cliente lento: reconecta")
				return
			}
		}
	}
}

type errorMsg struct {
	Error string  `json:"error"`
	Ref   *ids.ID `json:"ref,omitempty"`
}

func (h *Hub) leer(ctx context.Context, conn *websocket.Conn, c *Cliente) {
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var s eventos.Sobre
		if err := json.Unmarshal(raw, &s); err != nil || s.Type == "" || s.V < 1 {
			h.responder(c, errorMsg{Error: "mensaje inválido: se espera {v, id, type, ts, data}"})
			continue
		}
		vigente, ok := eventos.Versiones[s.Type]
		switch {
		case !ok:
			h.responder(c, errorMsg{Error: "tipo de evento desconocido: " + s.Type, Ref: &s.ID})
			continue
		case s.V > vigente:
			// Un cliente más nuevo que el nodo: se le avisa para que actualice el nodo.
			h.responder(c, errorMsg{Error: "versión de evento no soportada por este nodo; actualiza el Nodo Local", Ref: &s.ID})
			continue
		}
		if h.Recibir != nil {
			h.Recibir(ctx, c, s)
		}
	}
}

func (h *Hub) responder(c *Cliente, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.encolar(b, time.Now())
}

// ErrNoAutorizado es el error de Autenticar para conexiones rechazadas.
var ErrNoAutorizado = errors.New("dispositivo no autorizado: empareja este equipo con el nodo")
