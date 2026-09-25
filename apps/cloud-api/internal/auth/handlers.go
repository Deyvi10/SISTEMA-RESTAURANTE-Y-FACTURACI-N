package auth

import (
	"net"
	"net/http"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
)

const refreshCookie = "restpos_refresh"

// Handlers expone los endpoints /v1/auth.
type Handlers struct {
	Svc          *Service
	CookieSecure bool
}

func (h *Handlers) setRefresh(w http.ResponseWriter, token string, maxAge time.Duration) {
	// Secure es configurable solo para desarrollo local por http; en otros entornos es obligatorio (config).
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure viene de config, HttpOnly y SameSite=Strict fijos
		Name: refreshCookie, Value: token, Path: "/v1/auth", MaxAge: int(maxAge.Seconds()),
		HttpOnly: true, Secure: h.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func cliente(r *http.Request) Cliente {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return Cliente{IP: ip, UserAgent: r.UserAgent()}
}

func (h *Handlers) responder(w http.ResponseWriter, s Sesion) {
	h.setRefresh(w, s.RefreshToken, RefreshTTL)
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, s)
}

// POST /v1/auth/login
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Usuario  string `json:"usuario"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	s, err := h.Svc.Login(r.Context(), body.Usuario, body.Password, cliente(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.responder(w, s)
}

// POST /v1/auth/refresh (cookie)
func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie(refreshCookie)
	token := ""
	if c != nil {
		token = c.Value
	}
	s, err := h.Svc.Refresh(r.Context(), token, cliente(r))
	if err != nil {
		h.setRefresh(w, "", -1)
		httpx.Error(w, r, err)
		return
	}
	h.responder(w, s)
}

// POST /v1/auth/logout
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if c, _ := r.Cookie(refreshCookie); c != nil {
		if err := h.Svc.Logout(r.Context(), c.Value); err != nil {
			httpx.Error(w, r, err)
			return
		}
	}
	h.setRefresh(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/auth/password/olvide
func (h *Handlers) Olvide(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.Svc.SolicitarRecuperacion(r.Context(), body.Email); err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// POST /v1/auth/password/restablecer
func (h *Handlers) Restablecer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.Svc.Restablecer(r.Context(), body.Token, body.Password); err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/auth/password/cambiar
func (h *Handlers) Cambiar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Actual string `json:"actual"`
		Nueva  string `json:"nueva"`
	}
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	s, err := h.Svc.CambiarPassword(r.Context(), MustPrincipal(r.Context()), body.Actual, body.Nueva, cliente(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.responder(w, s)
}

// GET /v1/me
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	u, err := h.Svc.Me(r.Context(), MustPrincipal(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	permisos := []rbac.Permiso{}
	for _, i := range rbac.Catalogo() {
		if Tiene(r.Context(), i.Permiso) {
			permisos = append(permisos, i.Permiso)
		}
	}
	httpx.JSON(w, http.StatusOK, struct {
		Usuario
		Permisos []rbac.Permiso `json:"permisos"`
	}{u, permisos})
}
