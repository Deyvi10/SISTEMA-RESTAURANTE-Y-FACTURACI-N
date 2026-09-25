package app

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/nube"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/sistema"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
)

// HeartbeatCada: telemetría de salud hacia la nube (F2-04).
const HeartbeatCada = 60 * time.Second

// EstadoNube es lo que el nodo sabe de su conexión con la nube (para /estado y la caja).
type EstadoNube struct {
	UltimaNube  time.Time // último intercambio exitoso con la nube
	UltimoError string
	DerivaSeg   int64
	AlertaReloj bool
	Revocado    bool
	SyncActivo  bool
}

// Salud protege EstadoNube para leerlo y escribirlo desde varios procesos.
type Salud struct {
	mu sync.Mutex
	e  EstadoNube
}

func (s *Salud) cambiar(fn func(*EstadoNube)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.e)
}

func (s *Salud) ok(res edgesync.HeartbeatResponse, now time.Time) {
	s.cambiar(func(e *EstadoNube) {
		e.UltimaNube, e.UltimoError, e.DerivaSeg, e.AlertaReloj = now, "", res.DerivaSegundos, res.AlertaReloj
	})
}

func (s *Salud) fallo(msg string) { s.cambiar(func(e *EstadoNube) { e.UltimoError = msg }) }

// Copia devuelve una instantánea para leer.
func (s *Salud) Copia() EstadoNube {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.e
}

// iniciarSync arranca los procesos de sincronización del nodo activado. Es idempotente:
// llamarlo dos veces no duplica procesos. Se detiene si la nube revoca al nodo.
func (a *App) iniciarSync(id *Identidad) {
	if !id.Activo() {
		return
	}
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.syncCancel != nil || a.bg == nil {
		return
	}
	ctx, cancel := context.WithCancel(a.bg)
	a.syncCancel = cancel
	a.salud.cambiar(func(e *EstadoNube) { e.SyncActivo, e.Revocado = true, false })

	cli := &nube.Client{BaseURL: id.NubeURL, NodoID: id.NodoID, Llave: id.Llave, Now: a.Clock.Now, HTTP: a.httpNube}
	a.nube = cli
	a.pusher = &edgesync.Pusher{Outbox: a.outbox, NodeID: id.NodoID, URL: cli.URL("/v1/sync/push"), Client: cli.Firmado(30 * time.Second), Clock: a.Clock, Log: a.Log}
	a.wg.Go(func() { a.pusher.Run(ctx) })
	a.wg.Go(func() { a.heartbeatLoop(ctx) })
	a.wg.Go(func() { a.pullLoop(ctx) })
	a.wg.Go(func() { a.busquedaLoop(ctx) })
	a.Log.Info("sincronización iniciada", "nodo", id.NodoID)
}

func (a *App) detenerSync() {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.syncCancel != nil {
		a.syncCancel()
		a.syncCancel = nil
	}
	a.salud.cambiar(func(e *EstadoNube) { e.SyncActivo = false })
}

func (a *App) heartbeatLoop(ctx context.Context) {
	t := time.NewTicker(HeartbeatCada)
	defer t.Stop()
	for {
		a.EnviarHeartbeat(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-a.heartbeatYa:
			// Un cambio importante (p. ej. impresora sin papel): se informa ya, sin
			// esperar el minuto, pero como mucho uno cada 2 s.
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}

// pedirHeartbeat adelanta el próximo heartbeat (no bloquea).
func (a *App) pedirHeartbeat() {
	select {
	case a.heartbeatYa <- struct{}{}:
	default:
	}
}

// EnviarHeartbeat manda la telemetría una vez (exportado para pruebas).
func (a *App) EnviarHeartbeat(ctx context.Context) {
	hb := a.telemetria(ctx)
	var res edgesync.HeartbeatResponse
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	err := a.nube.Do(cctx, http.MethodPost, "/v1/nodos/heartbeat", hb, &res, true)
	switch {
	case nube.EsRevocado(err):
		a.revocado(ctx)
	case err != nil:
		if ctx.Err() == nil {
			a.salud.fallo("Sin conexión con la nube")
			a.Log.Warn("heartbeat fallido", "err", err)
		}
	default:
		a.salud.ok(res, a.Clock.Now())
		if res.AlertaReloj {
			a.Log.Warn("el reloj de esta PC está desfasado respecto de la nube; corrígelo (afecta la fecha de las facturas)", "derivaSeg", res.DerivaSegundos)
		}
	}
}

// revocado detiene la sincronización cuando la nube ya no reconoce al nodo.
func (a *App) revocado(ctx context.Context) {
	a.Log.Error("la nube revocó este nodo: se detiene la sincronización; la operación local continúa")
	if err := a.marcarRevocado(context.WithoutCancel(ctx)); err != nil {
		a.Log.Error("no se pudo marcar el nodo como revocado", "err", err)
	}
	a.salud.cambiar(func(e *EstadoNube) { e.Revocado = true })
	go a.detenerSync()
}

func (a *App) telemetria(ctx context.Context) edgesync.Heartbeat {
	hb := edgesync.Heartbeat{Version: Version, HoraNodo: a.Clock.Now(), ArranqueAt: a.Inicio.UTC(), DiscoLibreMB: sistema.DiscoLibreMB(a.Cfg.DataDir), Impresoras: a.saludImpresoras(ctx)}
	for _, f := range []string{a.Cfg.DBPath(), a.Cfg.DBPath() + "-wal"} {
		if fi, err := os.Stat(f); err == nil {
			hb.BaseMB += fi.Size() >> 20
		}
	}
	if a.outbox != nil {
		if st, err := a.outbox.Stats(ctx); err == nil {
			hb.OutboxPendientes = st.Pending
			if !st.OldestUnsent.IsZero() {
				hb.OutboxAntiguedad = int64(a.Clock.Now().Sub(st.OldestUnsent) / time.Second)
			}
		}
	}
	return hb
}
