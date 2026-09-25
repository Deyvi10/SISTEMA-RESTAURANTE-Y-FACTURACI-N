package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func puertoLibre(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().String()
}

// El nodo arranca en menos de 10 s, responde /health y se apaga ordenadamente.
func TestArranqueYApagado(t *testing.T) {
	addr := puertoLibre(t)
	cfg := Config{DataDir: t.TempDir(), HTTPAddr: addr, NubeURL: "http://nube.test"}
	ctx, cancel := context.WithCancel(context.Background())
	a, err := New(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err := http.Get("http://" + addr + "/health")
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatalf("/health = %d", res.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("el nodo no respondió en 10 s")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("el apagado no terminó")
	}
}

func TestPuertoOcupadoDaMensajeClaro(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer func() { _ = l.Close() }()
	cfg := Config{DataDir: t.TempDir(), HTTPAddr: l.Addr().String(), NubeURL: "http://nube.test"}
	a, err := New(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Run(context.Background()); err == nil {
		t.Fatal("debió fallar con el puerto ocupado")
	}
}
