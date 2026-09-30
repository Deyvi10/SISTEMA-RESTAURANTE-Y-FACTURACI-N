// Command sri-stub imita los web services SOAP del esquema offline del SRI (F0-07).
// Los escenarios y la forma de las respuestas están en el paquete stub.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/tools/sri-stub/stub"
)

func main() {
	addr := flag.String("addr", ":8091", "dirección HTTP")
	demora := flag.Duration("timeout-demora", 40*time.Second, "cuánto tarda en responder el escenario 902")
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	srv := &http.Server{Addr: *addr, Handler: stub.NewStub(*demora, log).Routes(), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Info("stub del SRI escuchando", "addr", *addr, "recepcion", stub.RutaRecepcion, "autorizacion", stub.RutaAutorizacion)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Error("stub del SRI terminó con error", "err", err)
		os.Exit(1)
	}
}
