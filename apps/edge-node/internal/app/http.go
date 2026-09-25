package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
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
