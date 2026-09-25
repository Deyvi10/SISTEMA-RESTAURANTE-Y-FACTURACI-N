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
	"sync/atomic"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/descubrir"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/fotos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/hub"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/nube"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/replica"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/spooler"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/web"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
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
	bg          context.Context
	wg          sync.WaitGroup
	syncMu      sync.Mutex
	syncCancel  context.CancelFunc
	salud       Salud
	httpNube    *http.Client // nil = cliente por defecto (las pruebas lo reemplazan)
	nube        *nube.Client
	outbox      *edgesync.Outbox
	pusher      *edgesync.Pusher
	replica     *replica.Replica
	alAplicar   func(CambiosAplicados) // por defecto difunde por el hub (F2-06)
	hub         *hub.Hub
	motor       *impresion.Motor
	heartbeatYa chan struct{}
	buscarYa    chan struct{}
	fotosYa     chan struct{}
	fotos       *fotos.Cache
	// listarInstaladas lee las impresoras de Windows (las pruebas lo reemplazan).
	listarInstaladas func() ([]spooler.Instalada, error)
	busqueda         Busqueda
	buscar           func(context.Context) []descubrir.Encontrada // las pruebas lo reemplazan
	ultimaCaida      atomic.Int64                                 // unix nano de la última búsqueda por caída
	sinMDNS          bool                                         // pruebas: no anunciar en la red
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
	rep, err := replica.Cargar(ctx, st.Read())
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	clk := clock.Real{}
	a := &App{Cfg: cfg, Store: st, Clock: clk, Log: log, Inicio: inicio, mux: http.NewServeMux(), replica: rep}
	a.hub = hub.New(log, clk.Now)
	a.heartbeatYa = make(chan struct{}, 1)
	a.buscarYa = make(chan struct{}, 1)
	a.fotosYa = make(chan struct{}, 1)
	a.fotos = &fotos.Cache{Dir: dirFotos(cfg.DataDir), NubeURL: cfg.NubeURL, Log: log, Pausa: 100 * time.Millisecond}
	a.buscar = func(ctx context.Context) []descubrir.Encontrada { return descubrir.Buscar(ctx, nil) }
	if a.outbox, err = edgesync.NewOutbox(ctx, st.Writer()); err != nil {
		_ = st.Close()
		return nil, err
	}
	a.motor = a.nuevoMotor(impresion.TCP{})
	a.motor.Spooler = spooler.Transporte{}
	a.listarInstaladas = spooler.Listar
	a.alAplicar = func(c CambiosAplicados) {
		a.difundirCambios(c)
		if c.Completo || c.Tablas["productos"] > 0 {
			a.pedirFotos()
		}
		a.sincronizarImpresoras(context.Background())
		a.ejecutarComandos(context.Background())
	}
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
	a.mux.HandleFunc("GET /estado", pagina("estado.html"))
	a.mux.HandleFunc("GET /v1/estado", a.handleEstado)
	a.mux.Handle("GET /media/", a.fotos.Handler())
	a.mux.Handle("POST /v1/impresoras/buscar", soloLocal(http.HandlerFunc(a.handleBuscarAhora)))
	a.mux.Handle("POST /v1/activacion", soloLocal(http.HandlerFunc(a.handleActivar)))
	a.mux.HandleFunc("GET /v1/conectividad", a.handleConectividad)
	a.mux.Handle("GET /v1/ws", a.hub.Handler(autenticarLAN))

	// Operación: por ahora solo desde esta PC; los teléfonos entran con el emparejamiento (F3-02).
	a.mux.Handle("POST /v1/comandas", soloLocal(jsonHandler(a.EnviarComanda, http.StatusCreated)))
	a.mux.Handle("POST /v1/comandas/{id}/anular", soloLocal(conID(a.AnularLineas)))
	a.mux.Handle("POST /v1/comandas/{id}/reimprimir", soloLocal(conID(a.Reimprimir)))
	a.mux.Handle("POST /v1/estaciones/{id}/redirigir", soloLocal(conID(func(ctx context.Context, id ids.ID, in RedirigirIn) (map[string]int, error) {
		n, err := a.Redirigir(ctx, id, in)
		return map[string]int{"trabajosMovidos": n}, err
	})))
	a.mux.Handle("DELETE /v1/estaciones/{id}/redirigir", soloLocal(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := ids.Parse(r.PathValue("id"))
		if err == nil {
			err = a.QuitarRedireccion(r.Context(), id, nil)
		}
		if err != nil {
			responderError(w, a.Log, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	a.mux.HandleFunc("GET /v1/impresoras", func(w http.ResponseWriter, r *http.Request) {
		v, err := a.vistaImpresion(r.Context())
		if err != nil {
			responderError(w, a.Log, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	})
}

// Hub expone el canal de tiempo real (para otros módulos del nodo).
func (a *App) Hub() *hub.Hub { return a.hub }

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
	a.motor.Iniciar(bg)
	a.sincronizarImpresoras(ctx)
	if !a.sinMDNS {
		if tcp, ok := ln.Addr().(*net.TCPAddr); ok {
			a.wg.Go(func() { a.anunciarMDNS(bg, tcp.Port) })
		}
	}
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
	a.motor.Esperar()
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
