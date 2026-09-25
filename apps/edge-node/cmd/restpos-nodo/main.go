// Command restpos-nodo es el Nodo Local (F2-01): corre como servicio del sistema operativo
// en la PC de caja, arranca con el equipo y se reinicia solo si falla.
//
//	restpos-nodo run         en primer plano (desarrollo o diagnóstico)
//	restpos-nodo install     registra el servicio (Windows: SCM con reinicio automático; Linux: systemd)
//	restpos-nodo uninstall   quita el servicio (los datos se conservan)
//	restpos-nodo start|stop|restart|status
//	restpos-nodo version
//
// Configuración por variables de entorno: RESTPOS_DATA, RESTPOS_HTTP, RESTPOS_NUBE.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/kardianos/service"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/app"
)

type programa struct {
	cfg    app.Config
	cancel context.CancelFunc
	done   chan struct{}
	logs   io.Closer
}

// Start no debe bloquear: el SCM de Windows espera una respuesta rápida.
func (p *programa) Start(s service.Service) error {
	log, closer, err := app.OpenLog(p.cfg.DataDir, service.Interactive())
	if err != nil {
		return err
	}
	p.logs = closer
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done = cancel, make(chan struct{})
	a, err := app.New(ctx, p.cfg, log)
	if err != nil {
		cancel()
		log.Error("no se pudo iniciar el nodo", "err", err)
		return err
	}
	go func() {
		defer close(p.done)
		if err := a.Run(ctx); err != nil {
			log.Error("el nodo se detuvo con error", "err", err)
			os.Exit(1) // el servicio del SO lo reinicia
		}
	}()
	return nil
}

func (p *programa) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
		<-p.done
	}
	if p.logs != nil {
		_ = p.logs.Close()
	}
	return nil
}

func main() {
	cfg := app.LoadConfig()
	svcCfg := &service.Config{
		Name:        "RestPOSNodo",
		DisplayName: "RestPOS · Nodo Local",
		Description: "Servidor local del restaurante: pedidos, impresión y facturación sin internet.",
		Arguments:   []string{"run"},
		EnvVars:     map[string]string{"RESTPOS_DATA": cfg.DataDir, "RESTPOS_HTTP": cfg.HTTPAddr, "RESTPOS_NUBE": cfg.NubeURL},
		Option: service.KeyValue{
			// Windows: reinicio automático ante cualquier fallo (docs/03 §7.1).
			"OnFailure": "restart", "OnFailureDelayDuration": "5s", "OnFailureResetPeriod": 60,
			"DelayedAutoStart": false, "StartType": "automatic",
			// Linux (systemd).
			"Restart": "always", "SuccessExitStatus": "0",
		},
	}
	prg := &programa{cfg: cfg}
	s, err := service.New(prg, svcCfg)
	if err != nil {
		fatal(err)
	}
	cmd := "run"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "run":
		if service.Interactive() {
			runInteractive(cfg)
			return
		}
		if err := s.Run(); err != nil {
			fatal(err)
		}
	case "version":
		fmt.Println(app.Version)
	case "status":
		st, err := s.Status()
		if err != nil {
			fatal(err)
		}
		fmt.Println(map[service.Status]string{service.StatusRunning: "en marcha", service.StatusStopped: "detenido"}[st])
	case "install", "uninstall", "start", "stop", "restart":
		if err := service.Control(s, cmd); err != nil {
			fatal(fmt.Errorf("%s: %w (¿ejecutaste como administrador?)", cmd, err))
		}
		fmt.Println("✓", cmd)
	default:
		fatal(fmt.Errorf("comando desconocido %q; usa run, install, uninstall, start, stop, restart, status o version", cmd))
	}
}

// runInteractive corre en primer plano y se detiene con Ctrl+C.
func runInteractive(cfg app.Config) {
	log, closer, err := app.OpenLog(cfg.DataDir, true)
	if err != nil {
		fatal(err)
	}
	defer func() { _ = closer.Close() }()
	slog.SetDefault(log)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, err := app.New(ctx, cfg, log)
	if err != nil {
		fatal(err)
	}
	if err := a.Run(ctx); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "✗", err)
	os.Exit(1)
}
