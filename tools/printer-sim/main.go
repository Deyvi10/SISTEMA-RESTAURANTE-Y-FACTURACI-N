// Command printer-sim simula impresoras térmicas ESC/POS por TCP (F0-06).
//
// Cada impresora escucha en su puerto (9100, 9101…), responde a DLE EOT como el
// firmware real y guarda lo que recibe. Un panel web muestra los tickets en vivo y
// permite simular fallos: sin papel, tapa abierta, desconectada y latencia.
//
//	printer-sim -impresoras "Cocina caliente=:9100,Bar=:9101,Caja=:9102" -http :8090
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
)

//go:embed web
var webFS embed.FS

func main() {
	impresoras := flag.String("impresoras", "Cocina caliente=:9100,Bar=:9101,Caja=:9102", "lista nombre=dirección separada por comas")
	httpAddr := flag.String("http", ":8090", "dirección del panel web")
	papel := flag.Int("papel", 80, "ancho de papel en mm (58 u 80)")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(*impresoras, *httpAddr, *papel, log); err != nil {
		log.Error("printer-sim terminó con error", "err", err)
		os.Exit(1)
	}
}

func run(spec, httpAddr string, papel int, log *slog.Logger) error {
	if papel != 58 && papel != 80 {
		return fmt.Errorf("papel %d mm no soportado: usa 58 u 80", papel)
	}
	store := NewStore()
	printers, err := parsePrinters(spec, papel, store, log)
	if err != nil {
		return err
	}
	for _, p := range printers {
		if err := p.Start(context.Background()); err != nil {
			return err
		}
	}

	srv := &http.Server{Addr: httpAddr, Handler: routes(printers, store), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("panel del simulador", "url", "http://localhost"+httpAddr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func parsePrinters(spec string, papel int, store *Store, log *slog.Logger) ([]*Printer, error) {
	var out []*Printer
	seen := map[string]bool{}
	for _, item := range strings.Split(spec, ",") {
		nombre, addr, ok := strings.Cut(strings.TrimSpace(item), "=")
		if !ok || strings.TrimSpace(nombre) == "" || addr == "" {
			return nil, fmt.Errorf("impresora %q: usa el formato nombre=:puerto", item)
		}
		id := slug(nombre)
		if seen[id] {
			return nil, fmt.Errorf("impresora repetida: %s", nombre)
		}
		seen[id] = true
		out = append(out, &Printer{ID: id, Nombre: strings.TrimSpace(nombre), Addr: addr, Papel: papel, estado: Normal, store: store, log: log})
	}
	return out, nil
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) && r < 128, unicode.IsDigit(r):
			b.WriteRune(r)
		case r == 'á':
			b.WriteRune('a')
		case r == 'é':
			b.WriteRune('e')
		case r == 'í':
			b.WriteRune('i')
		case r == 'ó':
			b.WriteRune('o')
		case r == 'ú':
			b.WriteRune('u')
		case r == 'ñ':
			b.WriteRune('n')
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteRune('-')
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func routes(printers []*Printer, store *Store) http.Handler {
	byID := map[string]*Printer{}
	for _, p := range printers {
		byID[p.ID] = p
	}
	mux := http.NewServeMux()
	web, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /", http.FileServerFS(web))

	mux.HandleFunc("GET /api/impresoras", func(w http.ResponseWriter, r *http.Request) {
		snaps := make([]Snapshot, 0, len(printers))
		for _, p := range printers {
			snaps = append(snaps, p.Snapshot())
		}
		writeJSON(w, http.StatusOK, snaps)
	})

	mux.HandleFunc("POST /api/impresoras/{id}/estado", func(w http.ResponseWriter, r *http.Request) {
		p := byID[r.PathValue("id")]
		if p == nil {
			problem(w, http.StatusNotFound, "No existe esa impresora.")
			return
		}
		var body struct {
			Estado     Estado `json:"estado"`
			LatenciaMs *int   `json:"latenciaMs"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
			problem(w, http.StatusBadRequest, "El cuerpo debe ser JSON: {\"estado\": \"sin_papel\", \"latenciaMs\": 0}.")
			return
		}
		snap := p.Snapshot()
		if body.Estado == "" {
			body.Estado = snap.Estado
		}
		lat := time.Duration(snap.LatenciaMs) * time.Millisecond
		if body.LatenciaMs != nil {
			lat = time.Duration(*body.LatenciaMs) * time.Millisecond
		}
		if err := p.SetEstado(r.Context(), body.Estado, lat); err != nil {
			problem(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, p.Snapshot())
	})

	// Envía por TCP un ticket de ejemplo a la impresora, igual que lo haría el nodo.
	mux.HandleFunc("POST /api/impresoras/{id}/ejemplo", func(w http.ResponseWriter, r *http.Request) {
		p := byID[r.PathValue("id")]
		if p == nil {
			problem(w, http.StatusNotFound, "No existe esa impresora.")
			return
		}
		papel := escpos.Paper(p.Papel)
		var data []byte
		switch r.URL.Query().Get("tipo") {
		case "comanda":
			data = escpos.ImprimirComanda(papel, comandaDemo(p.Nombre))
		default:
			data = escpos.ImprimirPrueba(papel, p.Nombre, "TCP "+p.Addr, time.Now())
		}
		if err := send(r.Context(), p.Addr, data); err != nil {
			problem(w, http.StatusBadGateway, "No se pudo enviar: "+err.Error())
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("GET /api/tickets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.List())
	})
	mux.HandleFunc("DELETE /api/tickets", func(w http.ResponseWriter, r *http.Request) {
		store.Clear()
		w.WriteHeader(http.StatusNoContent)
	})

	// Server-Sent Events: el panel se refresca con cada ticket o cambio de estado.
	mux.HandleFunc("GET /api/eventos", func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			problem(w, http.StatusInternalServerError, "streaming no soportado")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		ch, cancel := store.Subscribe()
		defer cancel()
		ping := time.NewTicker(20 * time.Second)
		defer ping.Stop()
		msg := "event: cambio\ndata: {}\n\n"
		for {
			if _, err := fmt.Fprint(w, msg); err != nil {
				return // el panel se desconectó
			}
			fl.Flush()
			select {
			case <-r.Context().Done():
				return
			case <-ch:
				msg = "event: cambio\ndata: {}\n\n"
			case <-ping.C:
				msg = ": ping\n\n"
			}
		}
	})
	return mux
}

func send(ctx context.Context, addr string, data []byte) error {
	host := addr
	if strings.HasPrefix(addr, ":") {
		host = "127.0.0.1" + addr
	}
	c, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", host)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	_ = c.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_, err = c.Write(data)
	return err
}

func comandaDemo(estacion string) escpos.Comanda {
	return escpos.Comanda{
		Estacion: estacion, Mesa: "Mesa 4", Mesero: "Carlos M.", Numero: 27, Hora: time.Now(),
		Lineas: []escpos.LineaComanda{
			{Cantidad: "1", Producto: "Ceviche mixto", Tiempo: "ENTRADA", Nota: "Sin cebolla"},
			{Cantidad: "2", Producto: "Hamburguesa doble", Tiempo: "FUERTE", Modificadores: []string{"Término medio", "Extra queso"}},
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// problem responde un error RFC 9457.
func problem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"title": http.StatusText(status), "status": status, "detail": detail})
}
