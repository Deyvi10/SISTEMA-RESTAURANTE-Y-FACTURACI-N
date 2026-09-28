package hub

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

func servidor(t *testing.T, auth Autenticar) (*Hub, string) {
	t.Helper()
	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return time.Now().UTC() })
	srv := httptest.NewServer(h.Handler(auth))
	t.Cleanup(srv.Close)
	return h, "ws" + strings.TrimPrefix(srv.URL, "http")
}

func todos(*http.Request) (Cliente, error) { return Cliente{Tipo: "PRUEBA"}, nil }

func esperarConectados(t *testing.T, h *Hub, n int) {
	t.Helper()
	for range 500 {
		if h.Conectados() == n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("conectados = %d, quiero %d", h.Conectados(), n)
}

// RF-02-08.4: ≥ 50 dispositivos con p95 de difusión ≤ 200 ms.
func TestDifusionA60Dispositivos(t *testing.T) {
	const clientes, eventosN = 60, 40
	h, url := servidor(t, todos)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var mu sync.Mutex
	var lat []time.Duration
	var wg sync.WaitGroup
	for range clientes {
		c, _, err := websocket.Dial(ctx, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.CloseNow() }()
		wg.Go(func() {
			for range eventosN {
				_, raw, err := c.Read(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				var s eventos.Sobre
				_ = json.Unmarshal(raw, &s)
				d := time.Since(s.TS)
				mu.Lock()
				lat = append(lat, d)
				mu.Unlock()
			}
		})
	}
	esperarConectados(t, h, clientes)
	for range eventosN {
		if err := h.Difundir(eventos.TableLocked{TableID: ids.New(), ByUserID: ids.New(), ByName: "Carlos", ExpiresAt: time.Now().Add(45 * time.Second)}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	wg.Wait()
	slices.Sort(lat)
	if len(lat) != clientes*eventosN {
		t.Fatalf("recibidos %d de %d", len(lat), clientes*eventosN)
	}
	p95 := lat[len(lat)*95/100]
	t.Logf("%d dispositivos · %d mensajes · p50 %v · p95 %v · máx %v", clientes, len(lat), lat[len(lat)/2], p95, lat[len(lat)-1])
	if p95 > 200*time.Millisecond {
		t.Fatalf("p95 = %v > 200 ms", p95)
	}
}

// Un teléfono que no lee (mala señal) se desconecta sin frenar a los demás.
func TestClienteLentoNoFrenaALosDemas(t *testing.T) {
	h, url := servidor(t, todos)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	lento, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lento.CloseNow() }()
	rapido, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rapido.CloseNow() }()
	esperarConectados(t, h, 2)
	total := colaPorCliente * 8
	recibidos := make(chan int, 1)
	go func() {
		n := 0
		for n < total {
			if _, _, err := rapido.Read(ctx); err != nil {
				break
			}
			n++
		}
		recibidos <- n
	}()
	grande := strings.Repeat("x", 4000)
	for range total {
		_ = h.Difundir(eventos.PrinterStatus{PrinterID: ids.New(), Name: grande, Status: "OK"})
	}
	if n := <-recibidos; n != total {
		t.Fatalf("el cliente rápido recibió %d de %d", n, total)
	}
	esperarConectados(t, h, 1) // el lento quedó fuera
}

func TestMensajesEntrantes(t *testing.T) {
	h, url := servidor(t, todos)
	var got []string
	var mu sync.Mutex
	h.Recibir = func(_ context.Context, _ *Cliente, s eventos.Sobre) {
		mu.Lock()
		got = append(got, s.Type)
		mu.Unlock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow() }()
	s, _ := eventos.Nuevo(eventos.TableHeartbeat{TableID: ids.New(), DeviceID: ids.New()}, time.Now())
	ok, _ := json.Marshal(s)
	for _, m := range []string{string(ok), "basura", `{"v":1,"id":"01a0da6b-e870-74e7-984b-266e0803f192","type":"no.existe","ts":"2026-01-01T00:00:00Z","data":{}}`, `{"v":9,"type":"table.heartbeat","ts":"2026-01-01T00:00:00Z","data":{}}`} {
		if err := c.Write(ctx, websocket.MessageText, []byte(m)); err != nil {
			t.Fatal(err)
		}
	}
	var errores []string
	for range 3 {
		_, raw, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		errores = append(errores, string(raw))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0] != "table.heartbeat" {
		t.Fatalf("recibidos: %v", got)
	}
	todo := strings.Join(errores, " ")
	for _, want := range []string{"mensaje inválido", "desconocido", "no soportada"} {
		if !strings.Contains(todo, want) {
			t.Errorf("falta el error %q en %s", want, todo)
		}
	}
}

func TestRechazaNoAutorizados(t *testing.T) {
	_, url := servidor(t, func(*http.Request) (Cliente, error) { return Cliente{}, ErrNoAutorizado })
	_, res, err := websocket.Dial(context.Background(), url, nil)
	if err == nil || res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("se aceptó una conexión no autorizada: %v", err)
	}
}
