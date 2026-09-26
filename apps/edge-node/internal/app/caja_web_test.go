package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestCajaWeb(t *testing.T) {
	archivos := fstest.MapFS{
		"index.html":           {Data: []byte("<div id=root></div>")},
		"assets/app-a1b2.js":   {Data: []byte("console.log(1)")},
		"manifest.webmanifest": {Data: []byte("{}")},
	}
	h := caja(archivos)
	pedir := func(ruta string) *http.Response {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", ruta, nil))
		return w.Result()
	}
	cuerpo := func(r *http.Response) string { b, _ := io.ReadAll(r.Body); return string(b) }

	// Archivos con hash: para siempre en la caché del navegador.
	if r := pedir("/pos/assets/app-a1b2.js"); r.StatusCode != 200 || r.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset: %d %q", r.StatusCode, r.Header.Get("Cache-Control"))
	}
	// Rutas de React y la raíz: index.html sin caché, con CSP.
	for _, ruta := range []string{"/pos/", "/pos/mesas/12"} {
		r := pedir(ruta)
		if r.StatusCode != 200 || cuerpo(r) != "<div id=root></div>" || r.Header.Get("Cache-Control") != "no-cache" || r.Header.Get("Content-Security-Policy") == "" {
			t.Fatalf("%s: %d %v", ruta, r.StatusCode, r.Header)
		}
	}
	if r := pedir("/pos/manifest.webmanifest"); r.StatusCode != 200 || r.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("manifest: %d", r.StatusCode)
	}

	// Nodo compilado sin la caja: lo explica en vez de dar 404.
	vacio := caja(fstest.MapFS{".gitkeep": {}})
	w := httptest.NewRecorder()
	vacio.ServeHTTP(w, httptest.NewRequest("GET", "/pos/", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("sin compilar: %d", w.Code)
	}
}
