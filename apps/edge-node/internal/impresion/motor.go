package impresion

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Estados de una impresora (los mismos del evento printer.status).
const (
	EstadoOK          = "OK"
	EstadoPocoPapel   = "POCO_PAPEL"
	EstadoSinPapel    = "SIN_PAPEL"
	EstadoTapaAbierta = "TAPA_ABIERTA"
	EstadoSinConexion = "SIN_CONEXION"
	EstadoError       = "ERROR"
	EstadoDesconocido = "DESCONOCIDO"
)

// Impresora es la configuración que usa el motor (viene de la réplica de la nube).
type Impresora struct {
	ID     ids.ID
	Nombre string
	Host   string
	Puerto int
	Ancho  escpos.Paper
	// ColaWindows: si no está vacía, se imprime por el spooler de Windows con esa cola.
	ColaWindows string
}

// Addr es «host:puerto» para red o el nombre de la cola de Windows.
func (i Impresora) Addr() string {
	if i.ColaWindows != "" {
		return i.ColaWindows
	}
	return net.JoinHostPort(i.Host, strconv.Itoa(i.Puerto))
}

// Estado es la salud de una impresora para la caja, la app y el heartbeat.
type Estado struct {
	ID     ids.ID    `json:"id"`
	Nombre string    `json:"nombre"`
	Estado string    `json:"estado"`
	Motivo string    `json:"motivo"`
	Cola   int       `json:"cola"`
	Desde  time.Time `json:"desde"`
}

// Imprimible: con poco papel todavía imprime.
func (e Estado) Imprimible() bool { return e.Estado == EstadoOK || e.Estado == EstadoPocoPapel }

// Trabajo terminado (para avisar a quien lo pidió, p. ej. una prueba desde la nube).
type Trabajo struct {
	ID          ids.ID
	ImpresoraID ids.ID
	Tipo        string
	ComandaID   *string
	ComandoID   *string
}

// Motor mantiene un proceso por impresora. Cada uno toma su cola en orden de llegada, así
// una impresora caída nunca frena a las demás (RF-02-04.3).
type Motor struct {
	DB        *store.Store
	T         Transporte // impresoras de red (RAW 9100)
	Spooler   Transporte // impresoras instaladas en Windows (nil = no disponible)
	Log       *slog.Logger
	Now       func() time.Time
	Sondeo    time.Duration // cada cuánto revisa el estado aunque no haya trabajos
	Reintento time.Duration // espera tras un fallo de envío

	AlCambiarEstado func(Estado)                        // difundir printer.status
	AlImprimir      func(t Trabajo)                     // trabajo impreso
	AlFallar        func(t Trabajo, motivo string) bool // true = cancelar el trabajo (p. ej. prueba vencida)

	mu      sync.Mutex
	workers map[ids.ID]*worker
	ctx     context.Context
	wg      sync.WaitGroup
}

type worker struct {
	cfg    Impresora
	wake   chan struct{}
	cancel context.CancelFunc
	mu     sync.Mutex
	estado Estado
}

// Iniciar fija el contexto de vida de los procesos (se detienen cuando termina).
func (m *Motor) Iniciar(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ctx = ctx
	if m.workers == nil {
		m.workers = map[ids.ID]*worker{}
	}
	if m.Sondeo == 0 {
		m.Sondeo = 2 * time.Second // alerta de fallo en < 3 s (RF-02-05.2)
	}
	if m.Reintento == 0 {
		m.Reintento = 3 * time.Second
	}
}

// Esperar aguarda a que todos los procesos terminen (tras cancelar el contexto).
func (m *Motor) Esperar() { m.wg.Wait() }

// Sincronizar arranca procesos para impresoras nuevas, detiene los de las que ya no están
// y actualiza la configuración de las existentes.
func (m *Motor) Sincronizar(lista []Impresora) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx == nil {
		return
	}
	vigentes := map[ids.ID]bool{}
	for _, cfg := range lista {
		vigentes[cfg.ID] = true
		if w, ok := m.workers[cfg.ID]; ok {
			w.mu.Lock()
			cambio := w.cfg != cfg
			w.cfg, w.estado.Nombre = cfg, cfg.Nombre
			w.mu.Unlock()
			if cambio {
				w.despertar()
			}
			continue
		}
		ctx, cancel := context.WithCancel(m.ctx)
		w := &worker{cfg: cfg, wake: make(chan struct{}, 1), cancel: cancel,
			estado: Estado{ID: cfg.ID, Nombre: cfg.Nombre, Estado: EstadoDesconocido, Desde: m.Now()}}
		m.workers[cfg.ID] = w
		m.wg.Go(func() { m.correr(ctx, w) })
	}
	for id, w := range m.workers {
		if !vigentes[id] {
			w.cancel()
			delete(m.workers, id)
		}
	}
}

// Despertar avisa a las impresoras que tienen trabajo nuevo (tras el commit).
func (m *Motor) Despertar(impresoras ...ids.ID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range impresoras {
		if w, ok := m.workers[id]; ok {
			w.despertar()
		}
	}
}

func (w *worker) despertar() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Estados devuelve la salud de todas las impresoras, con su cola pendiente.
func (m *Motor) Estados(ctx context.Context) []Estado {
	m.mu.Lock()
	out := make([]Estado, 0, len(m.workers))
	for _, w := range m.workers {
		w.mu.Lock()
		out = append(out, w.estado)
		w.mu.Unlock()
	}
	m.mu.Unlock()
	colas := map[string]int{}
	if rows, err := m.DB.Read().QueryContext(ctx, `SELECT impresora_id, count(*) FROM trabajos_impresion WHERE estado = 'PENDIENTE' GROUP BY impresora_id`); err == nil {
		for rows.Next() {
			var id string
			var n int
			if rows.Scan(&id, &n) == nil {
				colas[id] = n
			}
		}
		_ = rows.Close()
	}
	for i := range out {
		out[i].Cola = colas[out[i].ID.String()]
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nombre < out[j].Nombre })
	return out
}

// Estado de una impresora (EstadoDesconocido si el motor no la conoce).
func (m *Motor) Estado(id ids.ID) Estado {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.workers[id]; ok {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.estado
	}
	return Estado{ID: id, Estado: EstadoDesconocido}
}

func estadoDe(st escpos.Status, soporta bool) (string, string) {
	switch {
	case !soporta:
		return EstadoOK, ""
	case st.CoverOpen:
		return EstadoTapaAbierta, st.Motivo()
	case st.PaperOut:
		return EstadoSinPapel, st.Motivo()
	case st.Error:
		return EstadoError, st.Motivo()
	case st.Offline:
		return EstadoSinConexion, st.Motivo()
	case st.PaperNearEnd:
		return EstadoPocoPapel, st.Motivo()
	}
	return EstadoOK, ""
}

func (m *Motor) fijarEstado(w *worker, estado, motivo string) {
	w.mu.Lock()
	cambio := w.estado.Estado != estado
	if cambio {
		w.estado.Estado, w.estado.Motivo, w.estado.Desde = estado, motivo, m.Now()
	}
	e := w.estado
	w.mu.Unlock()
	if cambio {
		m.Log.Info("estado de impresora", "impresora", e.Nombre, "estado", estado)
		if m.AlCambiarEstado != nil {
			m.AlCambiarEstado(e)
		}
	}
}

func (m *Motor) correr(ctx context.Context, w *worker) {
	t := time.NewTicker(m.Sondeo)
	defer t.Stop()
	for ctx.Err() == nil {
		espera := m.ronda(ctx, w)
		timer := time.NewTimer(espera)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-w.wake:
		case <-t.C:
		case <-timer.C:
		}
		timer.Stop()
	}
}

// ronda revisa el estado e imprime lo pendiente. Devuelve cuánto esperar antes de la
// próxima ronda si no llega un aviso.
func (m *Motor) ronda(ctx context.Context, w *worker) time.Duration {
	w.mu.Lock()
	cfg := w.cfg
	w.mu.Unlock()
	t := m.transporte(cfg)
	st, soporta, err := t.Consultar(ctx, cfg.Addr())
	if err != nil {
		m.fijarEstado(w, EstadoSinConexion, "No se puede conectar con la impresora. Revisa que esté encendida y conectada a la red.")
		m.fallarPendientes(ctx, w, "sin conexión")
		return m.Sondeo
	}
	estado, motivo := estadoDe(st, soporta)
	m.fijarEstado(w, estado, motivo)
	if estado != EstadoOK && estado != EstadoPocoPapel {
		m.fallarPendientes(ctx, w, motivo)
		return m.Sondeo
	}
	for ctx.Err() == nil {
		trabajo, payload, err := m.siguiente(ctx, cfg.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return m.Sondeo
		}
		if err != nil {
			m.Log.Error("cola de impresión ilegible", "impresora", cfg.Nombre, "err", err)
			return m.Reintento
		}
		if err := t.Enviar(ctx, cfg.Addr(), payload); err != nil {
			m.registrarFallo(ctx, trabajo, err.Error())
			m.fijarEstado(w, EstadoSinConexion, "La impresora dejó de responder al imprimir. Se reintentará sola.")
			return m.Reintento
		}
		if err := m.DB.Write(ctx, func(tx *store.Tx) error {
			_, err := tx.ExecContext(ctx, `UPDATE trabajos_impresion SET estado = 'IMPRESO', impreso_at = ?, intentos = intentos + 1 WHERE id = ?`, m.ahora(), trabajo.ID.String())
			return err
		}); err != nil {
			m.Log.Error("no se pudo marcar el trabajo como impreso", "err", err)
			return m.Reintento
		}
		if m.AlImprimir != nil {
			m.AlImprimir(trabajo)
		}
	}
	return m.Sondeo
}

// transporte elige cómo hablar con la impresora: red directa o spooler de Windows.
func (m *Motor) transporte(cfg Impresora) Transporte {
	if cfg.ColaWindows != "" {
		if m.Spooler == nil {
			return sinSpooler{}
		}
		return m.Spooler
	}
	return m.T
}

type sinSpooler struct{}

var errSinSpooler = fmt.Errorf("%w: esta PC no tiene el spooler de Windows", ErrSinConexion)

func (sinSpooler) Consultar(context.Context, string) (escpos.Status, bool, error) {
	return escpos.Status{}, false, errSinSpooler
}
func (sinSpooler) Enviar(context.Context, string, []byte) error { return errSinSpooler }

func (m *Motor) siguiente(ctx context.Context, impresora ids.ID) (Trabajo, []byte, error) {
	var t Trabajo
	var id string
	var payload []byte
	err := m.DB.Read().QueryRowContext(ctx, `SELECT id, tipo, comanda_id, comando_id, payload FROM trabajos_impresion
		WHERE impresora_id = ? AND estado = 'PENDIENTE' ORDER BY created_at, rowid LIMIT 1`, impresora.String()).Scan(&id, &t.Tipo, &t.ComandaID, &t.ComandoID, &payload)
	if err != nil {
		return t, nil, err
	}
	t.ID, err = ids.Parse(id)
	t.ImpresoraID = impresora
	return t, payload, err
}

func (m *Motor) registrarFallo(ctx context.Context, t Trabajo, motivo string) {
	cancelar := m.AlFallar != nil && m.AlFallar(t, motivo)
	if err := m.DB.Write(ctx, func(tx *store.Tx) error {
		estado := "PENDIENTE"
		if cancelar {
			estado = "CANCELADO"
		}
		_, err := tx.ExecContext(ctx, `UPDATE trabajos_impresion SET intentos = intentos + 1, ultimo_error = ?, estado = ? WHERE id = ? AND estado = 'PENDIENTE'`,
			truncar(motivo, 200), estado, t.ID.String())
		return err
	}); err != nil && ctx.Err() == nil {
		m.Log.Error("no se pudo registrar el fallo de impresión", "err", err)
	}
}

// fallarPendientes avisa a quienes esperan un resultado (pruebas desde la nube) sin tocar
// las comandas: esas esperan a que la impresora vuelva o a que alguien redirija la estación.
func (m *Motor) fallarPendientes(ctx context.Context, w *worker, motivo string) {
	if m.AlFallar == nil {
		return
	}
	w.mu.Lock()
	impresora := w.cfg.ID
	w.mu.Unlock()
	rows, err := m.DB.Read().QueryContext(ctx, `SELECT id, tipo, comanda_id, comando_id FROM trabajos_impresion
		WHERE impresora_id = ? AND estado = 'PENDIENTE' AND comando_id IS NOT NULL`, impresora.String())
	if err != nil {
		return
	}
	var lista []Trabajo
	for rows.Next() {
		var t Trabajo
		var id string
		if rows.Scan(&id, &t.Tipo, &t.ComandaID, &t.ComandoID) == nil {
			t.ID, _ = ids.Parse(id)
			t.ImpresoraID = impresora
			lista = append(lista, t)
		}
	}
	_ = rows.Close()
	for _, t := range lista {
		m.registrarFallo(ctx, t, motivo)
	}
}

func (m *Motor) ahora() string { return m.Now().UTC().Format(time.RFC3339Nano) }

func truncar(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// Encolar agrega un trabajo dentro de la transacción del llamador. Despertar la impresora
// después del commit.
func Encolar(ctx context.Context, tx *store.Tx, t Trabajo, estacion *ids.ID, payload []byte, documento []byte, now time.Time) error {
	var est, doc any
	if estacion != nil {
		est = estacion.String()
	}
	if documento != nil {
		doc = string(documento)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO trabajos_impresion (id, impresora_id, estacion_id, comanda_id, comando_id, tipo, payload, documento, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.ID.String(), t.ImpresoraID.String(), est, t.ComandaID, t.ComandoID, t.Tipo, payload, doc, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("encolar trabajo: %w", err)
	}
	return nil
}
