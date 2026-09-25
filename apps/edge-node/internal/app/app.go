// Package app arma el Nodo Local: base, servidor HTTP de la LAN y procesos de fondo.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/nube"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/web"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
)

// App es el nodo en ejecución.
type App struct {
	Cfg    Config
	Store  *store.Store
	Clock  clock.Clock
	Log    *slog.Logger
	Inicio time.Time

	mux *http.ServeMux

	// Procesos de fondo: bg vive mientras corre Run; la sincronización tiene su propio
	// contexto para poder detenerla si la nube revoca al nodo.
	bg         context.Context
	wg         sync.WaitGroup
	syncMu     sync.Mutex
	syncCancel context.CancelFunc
	salud      Salud
	httpNube   *http.Client // nil = cliente por defecto (las pruebas lo reemplazan)
	nube       *nube.Client
	outbox     *edgesync.Outbox
	pusher     *edgesync.Pusher
}

// New abre la base (migrando) y prepara las rutas. No escucha todavía.
func New(ctx context.Context, cfg Config, log *slog.Logger) (*App, error) {
	inicio := time.Now()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("no se pudo crear %s: %w", cfg.DataDir, err)
	}
	st, err := store.Open(ctx, cfg.DBPath())
	if err != nil {
		return nil, err
	}
	if err := st.Check(ctx); err != nil {
		_ = st.Close()
		return nil, err
	}
	clk := clock.Real{}
	a := &App{Cfg: cfg, Store: st, Clock: clk, Log: log, Inicio: inicio, mux: http.NewServeMux()}
	a.routes()
	return a, nil
}

func (a *App) routes() {
	a.mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"estado": "ok", "version": Version})
	})
	a.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(web.Static())))
	a.mux.HandleFunc("GET /{$}", a.inicio)
	a.mux.HandleFunc("GET /activar", pagina("activar.html"))
	a.mux.HandleFunc("POST /v1/activacion", a.handleActivar)
}

// Handler es el router completo (lo usan las pruebas con httptest).
func (a *App) Handler() http.Handler { return withRecover(a.Log, a.mux) }

// Run escucha en la LAN y corre los procesos de fondo hasta que ctx se cancele.
// Al cancelar, deja de aceptar peticiones, espera las que están en curso (máx. 10 s)
// y cierra la base: un apagado ordenado nunca deja la base a medias.
func (a *App) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", a.Cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("no se pudo escuchar en %s (¿otro programa usa el puerto?): %w", a.Cfg.HTTPAddr, err)
	}
	srv := &http.Server{Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second}
	bg, cancel := context.WithCancel(ctx)
	defer cancel()
	a.syncMu.Lock()
	a.bg = bg
	a.syncMu.Unlock()
	a.wg.Go(func() { a.watchdog(bg) })
	if id, err := a.Identidad(ctx); err != nil {
		a.Log.Error("no se pudo leer la identidad del nodo", "err", err)
	} else if id == nil {
		a.Log.Info("nodo sin activar: abre esta PC en el navegador para ingresar el código", "url", urlLocal(ln.Addr().String())+"/activar")
	} else {
		a.iniciarSync(id)
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	a.Log.Info("nodo en marcha", "version", Version, "http", ln.Addr().String(), "datos", a.Cfg.DataDir, "arranque", time.Since(a.Inicio).Round(time.Millisecond).String())

	select {
	case <-ctx.Done():
	case err = <-errc:
	}
	sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer scancel()
	_ = srv.Shutdown(sctx)
	cancel()
	a.wg.Wait()
	if cerr := a.Store.Close(); cerr != nil {
		err = errors.Join(err, cerr)
	}
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	a.Log.Info("nodo detenido")
	return err
}

// watchdog comprueba cada 30 s que el escritor responde. Si queda colgado más de 60 s,
// el proceso sale con error y el servicio del SO lo reinicia (docs/03 §7.1).
func (a *App) watchdog(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		err := a.Store.Write(cctx, func(tx *store.Tx) error { _, err := tx.ExecContext(cctx, "SELECT 1"); return err })
		cancel()
		if err != nil && ctx.Err() == nil {
			a.Log.Error("watchdog: la base no responde; se reinicia el servicio", "err", err)
			os.Exit(3)
		}
	}
}

// urlLocal convierte la dirección de escucha en una URL que el dueño puede abrir en esta PC.
func urlLocal(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://localhost"
	}
	return "http://localhost:" + port
}
