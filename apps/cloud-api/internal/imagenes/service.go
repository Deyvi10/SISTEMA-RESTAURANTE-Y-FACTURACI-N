package imagenes

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Las claves son t/<tenant>/<uuid> (fotos subidas) o biblioteca/<slug> (galería común).
var (
	claveRe = regexp.MustCompile(`^(t/[0-9a-f-]{36}/[0-9a-f-]{36}|biblioteca/[a-z0-9-]{1,40})$`)
	mediaRe = regexp.MustCompile(`^(t/[0-9a-f-]{36}/[0-9a-f-]{36}|biblioteca/[a-z0-9-]{1,40})/(sm|md|lg)\.webp$`)
)

// ClaveValida indica si una clave puede asignarse a un producto del tenant: una foto suya o
// una de la galería común. Evita que un tenant apunte a las fotos de otro.
func ClaveValida(tenant ids.ID, key string) bool {
	return claveRe.MatchString(key) && (strings.HasPrefix(key, "biblioteca/") || strings.HasPrefix(key, "t/"+tenant.String()+"/"))
}

type Service struct {
	Store     Store
	PublicURL string
}

// URL de un tamaño de una imagen, servida por /media (cacheable para siempre: es inmutable).
func (s *Service) URL(key, tam string) string {
	return s.PublicURL + "/media/" + key + "/" + tam + ".webp"
}

type Imagen struct {
	Key  string            `json:"key"`
	URLs map[string]string `json:"urls"`
}

func (s *Service) imagen(key string) Imagen {
	urls := map[string]string{}
	for _, t := range Tamanos {
		urls[t.Nombre] = s.URL(key, t.Nombre)
	}
	return Imagen{Key: key, URLs: urls}
}

// Subir procesa y guarda una foto del tenant.
func (s *Service) Subir(ctx context.Context, p auth.Principal, data []byte) (Imagen, error) {
	tams, err := Procesar(data)
	if err != nil {
		return Imagen{}, err
	}
	key := "t/" + p.TenantID.String() + "/" + ids.New().String()
	for nombre, b := range tams {
		if err := s.Store.Put(ctx, key+"/"+nombre+".webp", b, "image/webp"); err != nil {
			return Imagen{}, err
		}
	}
	return s.imagen(key), nil
}

// ---------- Galería de fotos de platos ----------

//go:embed biblioteca
var bibliotecaFS embed.FS

// Foto de la galería común. Todas son de dominio público o CC0 (sin obligación de
// atribución); igual se guarda el origen para transparencia.
type Foto struct {
	Slug      string `json:"slug"`
	Nombre    string `json:"nombre"`
	Categoria string `json:"categoria"`
	Palabras  string `json:"palabras"` // para sugerir la foto según el nombre del producto
	Licencia  string `json:"licencia"`
	Fuente    string `json:"fuente"`
	Autor     string `json:"autor"`
}

type FotoGaleria struct {
	Foto
	Imagen
}

func manifiesto() ([]Foto, error) {
	b, err := bibliotecaFS.ReadFile("biblioteca/biblioteca.json")
	if errors.Is(err, fsNotExist) || len(b) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var fotos []Foto
	return fotos, json.Unmarshal(b, &fotos)
}

// Galeria lista las fotos listas para usar en productos.
func (s *Service) Galeria() ([]FotoGaleria, error) {
	fotos, err := manifiesto()
	out := make([]FotoGaleria, 0, len(fotos))
	for _, f := range fotos {
		out = append(out, FotoGaleria{Foto: f, Imagen: s.imagen("biblioteca/" + f.Slug)})
	}
	return out, err
}

// SembrarGaleria procesa las fotos embebidas y las sube si aún no están (idempotente).
func (s *Service) SembrarGaleria(ctx context.Context) (int, error) {
	fotos, err := manifiesto()
	if err != nil {
		return 0, err
	}
	subidas := 0
	for _, f := range fotos {
		key := "biblioteca/" + f.Slug
		if ok, err := s.Store.Exists(ctx, key+"/lg.webp"); err != nil {
			return subidas, err
		} else if ok {
			continue
		}
		data, err := bibliotecaFS.ReadFile("biblioteca/" + f.Slug + ".jpg")
		if err != nil {
			return subidas, err
		}
		tams, err := Procesar(data)
		if err != nil {
			return subidas, err
		}
		for nombre, b := range tams {
			if err := s.Store.Put(ctx, key+"/"+nombre+".webp", b, "image/webp"); err != nil {
				return subidas, err
			}
		}
		subidas++
	}
	return subidas, nil
}

// ---------- HTTP ----------

// POST /v1/imagenes (multipart, campo «foto»)
func (s *Service) HandleSubir(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBytes+1<<20)
	f, _, err := r.FormFile("foto")
	if err != nil {
		httpx.Error(w, r, apperr.New(apperr.Invalid, "SIN_FOTO", "Adjunta la foto en el campo «foto» (máximo 10 MB)."))
		return
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	img, err := s.Subir(r.Context(), auth.MustPrincipal(r.Context()), data)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, img)
}

// GET /v1/galeria
func (s *Service) HandleGaleria(w http.ResponseWriter, r *http.Request) {
	g, err := s.Galeria()
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, g)
}

// GET /media/{ruta...}: público e inmutable (las claves llevan un UUID nuevo por foto).
func (s *Service) HandleMedia(w http.ResponseWriter, r *http.Request) {
	ruta := r.PathValue("ruta")
	if !mediaRe.MatchString(ruta) {
		http.NotFound(w, r)
		return
	}
	obj, err := s.Store.Get(r.Context(), ruta)
	if errors.Is(err, ErrNoExiste) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "media: error del almacenamiento", "err", err)
		http.Error(w, "no disponible", http.StatusBadGateway)
		return
	}
	defer func() { _ = obj.Close() }()
	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	_, _ = io.Copy(w, obj)
}
