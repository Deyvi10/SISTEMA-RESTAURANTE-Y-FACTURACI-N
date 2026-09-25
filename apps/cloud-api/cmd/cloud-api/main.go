// Command cloud-api es la API de la nube (F1-01) y sus comandos de plataforma.
//
//	cloud-api serve              inicia la API (en APP_ENV=local migra y siembra la galería)
//	cloud-api migrate            aplica migraciones pendientes
//	cloud-api tenant-crear …     da de alta un restaurante (Super Admin, RF-01-01)
//	cloud-api galeria            sube la galería de fotos de platos al almacenamiento
//	cloud-api demo               crea el restaurante de demostración con fotos
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/catalogo"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/imagenes"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/impresoras"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/nodos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/personal"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/config"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/salon"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/server"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/tenants"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cmd, os.Args[min(2, len(os.Args)):]); err != nil {
		if ae, ok := apperr.As(err); ok {
			fmt.Fprintln(os.Stderr, "✗", ae.Message)
			for _, f := range ae.Fields {
				fmt.Fprintf(os.Stderr, "  · %s: %s\n", f.Campo, f.Mensaje)
			}
		} else {
			fmt.Fprintln(os.Stderr, "✗", err)
		}
		os.Exit(1)
	}
}

// App reúne las dependencias para los comandos.
type App struct {
	Cfg      config.Config
	DB       *db.DB
	Mail     mail.Sender
	Clock    clock.Clock
	Imagenes *imagenes.Service
	Tenants  *tenants.Service
	Catalogo *catalogo.Service
	Salon    *salon.Service
	Personal *personal.Service
	Signer   *auth.Signer
}

func build(ctx context.Context, migrate bool) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if migrate {
		n, err := db.Migrate(ctx, cfg.AdminDBURL)
		if err != nil {
			return nil, err
		}
		slog.Info("migraciones aplicadas", "nuevas", n)
	}
	d, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	var store imagenes.Store
	if cfg.StorageDriver == "azure" {
		store, err = imagenes.NewAzureBlob(ctx, cfg.AzureStorageConn, cfg.S3Bucket)
	} else {
		store, err = imagenes.NewS3(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3UseSSL)
	}
	if err != nil {
		return nil, err
	}
	clk := clock.Real{}
	m := mail.SMTP{Addr: cfg.SMTPAddr, From: cfg.MailFrom}
	img := &imagenes.Service{Store: store, PublicURL: cfg.PublicURL}
	return &App{
		Cfg: cfg, DB: d, Mail: m, Clock: clk, Imagenes: img,
		Tenants:  &tenants.Service{DB: d, Mail: m, BackofficeURL: cfg.BackofficeURL},
		Catalogo: &catalogo.Service{DB: d, ImagenURL: img.URL, Clock: clk},
		Salon:    &salon.Service{DB: d},
		Personal: &personal.Service{DB: d, Mail: m, Pepper: cfg.PINPepper, BackofficeURL: cfg.BackofficeURL, ImagenURL: img.URL},
		Signer:   auth.NewSigner(cfg.JWTKey, clk.Now),
	}, nil
}

func run(ctx context.Context, cmd string, args []string) error {
	switch cmd {
	case "serve":
		return serve(ctx)
	case "migrate":
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		n, err := db.Migrate(ctx, cfg.AdminDBURL)
		fmt.Printf("✓ %d migraciones aplicadas\n", n)
		return err
	case "galeria":
		app, err := build(ctx, false)
		if err != nil {
			return err
		}
		n, err := app.Imagenes.SembrarGaleria(ctx)
		fmt.Printf("✓ galería: %d fotos nuevas subidas\n", n)
		return err
	case "tenant-crear":
		return tenantCrear(ctx, args)
	case "demo":
		return demo(ctx, args)
	default:
		return fmt.Errorf("comando desconocido %q: usa serve, migrate, tenant-crear, galeria o demo", cmd)
	}
}

func serve(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	app, err := build(ctx, cfg.Env == "local")
	if err != nil {
		return err
	}
	defer app.DB.Close()
	if cfg.Env == "local" {
		if n, err := app.Imagenes.SembrarGaleria(ctx); err != nil {
			slog.Warn("no se pudo sembrar la galería (¿está arriba `make dev`?)", "err", err)
		} else if n > 0 {
			slog.Info("galería sembrada", "fotos", n)
		}
	}
	deps := server.Deps{
		DB: app.DB, Signer: app.Signer, Now: app.Clock.Now, BackofficeURL: cfg.BackofficeURL,
		Auth:  &auth.Handlers{Svc: &auth.Service{DB: app.DB, Signer: app.Signer, Mail: app.Mail, Clock: app.Clock, BackofficeURL: cfg.BackofficeURL}, CookieSecure: cfg.CookieSecure},
		Salon: app.Salon, Catalogo: app.Catalogo, Personal: app.Personal, Imagenes: app.Imagenes,
		Nodos: nodos.New(app.DB, app.Clock), Impresoras: &impresoras.Service{DB: app.DB, Clock: app.Clock},
	}
	go deps.Nodos.Avisos.Escuchar(ctx, app.DB.Pool, slog.Default())
	srv := &http.Server{
		Addr: cfg.HTTPAddr, Handler: server.Handler(deps, server.Routes(deps)),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	slog.Info("cloud-api escuchando", "addr", cfg.HTTPAddr, "entorno", cfg.Env)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func tenantCrear(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tenant-crear", flag.ContinueOnError)
	var a tenants.Alta
	fs.StringVar(&a.RUC, "ruc", "", "RUC del restaurante (13 dígitos, termina en 001)")
	fs.StringVar(&a.RazonSocial, "razon-social", "", "razón social")
	fs.StringVar(&a.NombreComercial, "nombre", "", "nombre comercial")
	fs.StringVar(&a.NombreDueno, "dueno", "", "nombre del dueño")
	fs.StringVar(&a.EmailDueno, "email", "", "correo del dueño (recibe la contraseña temporal)")
	fs.StringVar(&a.TelefonoDueno, "telefono", "", "teléfono del dueño")
	fs.StringVar(&a.Plan, "plan", "RESTAURANTE", "EMPRENDEDOR, RESTAURANTE o PRO")
	if err := fs.Parse(args); err != nil {
		return err
	}
	app, err := build(ctx, false)
	if err != nil {
		return err
	}
	defer app.DB.Close()
	r, err := app.Tenants.Crear(ctx, a)
	if err != nil {
		return err
	}
	fmt.Printf("✓ Restaurante creado\n  tenant:  %s\n  usuario: %s\n  La contraseña temporal se envió a %s\n", r.TenantID, a.EmailDueno, a.EmailDueno)
	return nil
}
