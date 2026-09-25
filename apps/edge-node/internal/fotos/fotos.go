// Package fotos mantiene en el Nodo Local una copia de las fotos del menú (F2-15, ADR-0013):
// la caja y la app de meseros las ven sin internet y la nube no las sirve una y otra vez.
// Las claves de imagen son inmutables (cambiar la foto cambia la clave), así que un archivo
// guardado nunca queda viejo: solo se descargan claves nuevas y se borran las que ya no se usan.
package fotos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Tamaños que se guardan: sm para listas y md para el detalle (lg solo en el backoffice).
var Tamanos = []string{"sm", "md"}

// claveRe acepta solo claves de imagen de la plataforma (evita rutas fuera del directorio).
// Mismo formato que en la nube (apps/cloud-api/internal/imagenes).
var claveRe = regexp.MustCompile(`^(t/[0-9a-f-]{36}/[0-9a-f-]{36}|biblioteca/[a-z0-9-]{1,40})$`)

// ClaveValida indica si la clave tiene la forma esperada.
func ClaveValida(k string) bool { return claveRe.MatchString(k) }

const maxFoto = 2 << 20 // una WebP del menú pesa decenas de KB; 2 MB es de sobra

type Cache struct {
	Dir     string // <datos>/media
	NubeURL string
	HTTP    *http.Client
	Log     *slog.Logger
	// Pausa entre descargas: no satura el internet del local en hora pico.
	Pausa time.Duration

	mu sync.Mutex // una sincronización a la vez
}

func (c *Cache) ruta(clave, tam string) string {
	return filepath.Join(c.Dir, filepath.FromSlash(clave), tam+".webp")
}

// Sincronizar descarga lo que falta de las claves dadas y borra lo que ya no se usa.
// Devuelve cuántos archivos descargó. Nunca bloquea la operación: corre en segundo plano.
func (c *Cache) Sincronizar(ctx context.Context, claves []string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	vigentes := map[string]bool{}
	nuevos := 0
	var errs []error
	for _, k := range claves {
		if !ClaveValida(k) {
			continue
		}
		for _, tam := range Tamanos {
			p := c.ruta(k, tam)
			vigentes[p] = true
			if _, err := os.Stat(p); err == nil {
				continue
			}
			if err := c.descargar(ctx, k, tam, p); err != nil {
				errs = append(errs, err)
				continue
			}
			nuevos++
			if c.Pausa > 0 {
				select {
				case <-ctx.Done():
					return nuevos, ctx.Err()
				case <-time.After(c.Pausa):
				}
			}
		}
	}
	c.limpiar(vigentes)
	if len(errs) > 0 {
		return nuevos, fmt.Errorf("fotos: %d no se pudieron descargar (se reintentará): %w", len(errs), errs[0])
	}
	return nuevos, nil
}

func (c *Cache) descargar(ctx context.Context, clave, tam, destino string) error {
	url := strings.TrimRight(c.NubeURL, "/") + "/media/" + clave + "/" + tam + ".webp"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: la nube respondió %d", clave, res.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o750); err != nil {
		return err
	}
	// Escribir a un temporal y renombrar: un corte a mitad nunca deja una foto rota.
	tmp, err := os.CreateTemp(filepath.Dir(destino), ".descarga-*")
	if err != nil {
		return err
	}
	n, err := io.Copy(tmp, io.LimitReader(res.Body, maxFoto+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxFoto {
		err = errors.New("foto demasiado grande")
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), destino)
}

// limpiar borra los archivos que ya no corresponden a ningún producto activo.
func (c *Cache) limpiar(vigentes map[string]bool) {
	_ = filepath.WalkDir(c.Dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // un directorio ilegible no detiene la limpieza
		}
		if !vigentes[p] {
			_ = os.Remove(p)
		}
		return nil
	})
	// Directorios vacíos (de la más profunda a la raíz).
	var dirs []string
	_ = filepath.WalkDir(c.Dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && p != c.Dir {
			dirs = append(dirs, p)
		}
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i]) // falla si no está vacío: está bien
	}
}

// Tamano devuelve cuánto ocupa la caché (para la página de estado).
func (c *Cache) Tamano() (archivos int, bytes int64) {
	_ = filepath.WalkDir(c.Dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				archivos++
				bytes += fi.Size()
			}
		}
		return nil
	})
	return archivos, bytes
}

// Handler sirve /media/<clave>/<tam>.webp desde la caché. Si no está, responde 404 y el
// cliente muestra el icono de la categoría.
func (c *Cache) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ruta := strings.TrimPrefix(r.URL.Path, "/media/")
		i := strings.LastIndex(ruta, "/")
		if i < 0 {
			http.NotFound(w, r)
			return
		}
		clave, archivo := ruta[:i], ruta[i+1:]
		tam := strings.TrimSuffix(archivo, ".webp")
		if !ClaveValida(clave) || (tam != "sm" && tam != "md") || !strings.HasSuffix(archivo, ".webp") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Content-Type", "image/webp")
		http.ServeFile(w, r, c.ruta(clave, tam))
	})
}
