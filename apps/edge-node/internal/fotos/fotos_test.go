package fotos

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCacheDescargaSirveYLimpia(t *testing.T) {
	var pedidos atomic.Int32
	nube := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos.Add(1)
		if strings.Contains(r.URL.Path, "no-existe") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("RIFF-webp-" + r.URL.Path))
	}))
	defer nube.Close()
	c := &Cache{Dir: t.TempDir(), NubeURL: nube.URL, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx := context.Background()

	n, err := c.Sincronizar(ctx, []string{"biblioteca/ceviche", "biblioteca/cerveza", "../../etc/passwd"})
	if err != nil || n != 4 {
		t.Fatalf("descargadas %d, %v", n, err)
	}
	// Segunda vez: nada nuevo que bajar (claves inmutables).
	antes := pedidos.Load()
	if n, _ := c.Sincronizar(ctx, []string{"biblioteca/ceviche", "biblioteca/cerveza"}); n != 0 || pedidos.Load() != antes {
		t.Fatalf("volvió a descargar: %d", n)
	}
	// Se sirve desde la caché aunque la nube se caiga.
	nube.Close()
	srv := httptest.NewServer(c.Handler())
	defer srv.Close()
	res, err := http.Get(srv.URL + "/media/biblioteca/ceviche/md.webp")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 200 || !strings.HasSuffix(string(body), "/media/biblioteca/ceviche/md.webp") || res.Header.Get("Content-Type") != "image/webp" {
		t.Fatalf("servir: %d %q", res.StatusCode, body)
	}
	for _, mala := range []string{"/media/biblioteca/ceviche/lg.webp", "/media/../nodo.db", "/media/biblioteca/otro/sm.webp"} {
		if res, _ := http.Get(srv.URL + mala); res.StatusCode != 404 {
			t.Errorf("%s = %d", mala, res.StatusCode)
		}
	}
	// La cerveza sale del menú: su foto se borra.
	if _, err := c.Sincronizar(ctx, []string{"biblioteca/ceviche"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.ruta("biblioteca/cerveza", "sm")); !os.IsNotExist(err) {
		t.Fatal("no borró la foto que ya no se usa")
	}
	if archivos, bytes := c.Tamano(); archivos != 2 || bytes == 0 {
		t.Fatalf("tamaño: %d archivos, %d bytes", archivos, bytes)
	}
}

func TestErroresNoDejanFotosRotas(t *testing.T) {
	nube := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "caída", 503) }))
	defer nube.Close()
	c := &Cache{Dir: t.TempDir(), NubeURL: nube.URL}
	if _, err := c.Sincronizar(context.Background(), []string{"biblioteca/ceviche"}); err == nil {
		t.Fatal("un 503 no se informó")
	}
	if archivos, _ := c.Tamano(); archivos != 0 {
		t.Fatal("quedó un archivo a medias")
	}
}
