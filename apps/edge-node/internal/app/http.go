package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/web"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeProblem responde errores en formato RFC 9457, igual que la API de la nube.
func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "code": code, "detail": detail})
}

func withRecover(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				log.Error("pánico en petición", "ruta", r.URL.Path, "panic", p, "stack", string(debug.Stack()))
				writeProblem(w, http.StatusInternalServerError, "ERROR_INTERNO", "Ocurrió un error inesperado en el nodo.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// pagina sirve una página estática embebida sin caché (cambia con cada versión del nodo).
func pagina(nombre string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		http.ServeFileFS(w, r, web.Static(), nombre)
	}
}

// inicio lleva a la activación si el nodo aún no tiene dueño, o al estado si ya lo tiene.
func (a *App) inicio(w http.ResponseWriter, r *http.Request) {
	id, err := a.Identidad(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "ERROR_INTERNO", "No se pudo leer la configuración del nodo.")
		return
	}
	destino := "/estado"
	if !id.Activo() {
		destino = "/activar"
	}
	http.Redirect(w, r, destino, http.StatusFound)
}

func (a *App) handleActivar(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Codigo string `json:"codigo"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
		writeProblem(w, http.StatusBadRequest, "JSON_INVALIDO", "La solicitud no es válida.")
		return
	}
	id, err := a.Activar(r.Context(), in.Codigo)
	var p *Problema
	switch {
	case errors.As(err, &p):
		writeProblem(w, p.Status, p.Code, p.Detail)
	case err != nil:
		a.Log.Error("activación fallida", "err", err)
		writeProblem(w, http.StatusInternalServerError, "ERROR_INTERNO", "No se pudo activar el nodo. Intenta de nuevo.")
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"nodoId": id.NodoID, "tenantId": id.TenantID, "localId": id.LocalID,
			"nombreLocal": id.NombreLocal, "nombreComercial": id.NombreComercial,
		})
	}
}
