package server

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/nodos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
)

func ipDe(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// POST /v1/nodos/activar (pública: el nodo aún no tiene identidad; la protege el código
// de un solo uso y el bloqueo por intentos).
func activarNodo(n *nodos.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in nodos.Activacion
		if err := httpx.Decode(w, r, &in); err != nil {
			httpx.Error(w, r, err)
			return
		}
		out, err := n.Activar(r.Context(), in, ipDe(r))
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, http.StatusOK, out)
	}
}

// POST /v1/nodos/heartbeat
func heartbeat(n *nodos.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var hb edgesync.Heartbeat
		if err := httpx.DecodeNodo(w, r, &hb); err != nil {
			httpx.Error(w, r, err)
			return
		}
		out, err := n.Heartbeat(r.Context(), auth.MustNodo(r.Context()), hb)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, out)
	}
}

var errOtroNodo = apperr.New(apperr.Forbidden, "NODO_AJENO", "El lote pertenece a otro nodo.")

// POST /v1/sync/push: lotes del outbox del nodo, aplicados en orden y una sola vez
// (ADR-0012) dentro del tenant del nodo autenticado.
func syncPush(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := auth.MustNodo(r.Context())
		var req edgesync.PushRequest
		if err := httpx.DecodeNodo(w, r, &req); err != nil {
			httpx.Error(w, r, err)
			return
		}
		if req.NodeID != n.ID {
			httpx.Error(w, r, errOtroNodo)
			return
		}
		rec := &edgesync.Receiver{Begin: func(ctx context.Context, fn func(pgx.Tx) error) error {
			return d.InTenant(ctx, n.TenantID, fn)
		}}
		res, err := rec.Push(r.Context(), req)
		if errors.Is(err, edgesync.ErrInvalidBatch) {
			err = apperr.New(apperr.Invalid, "LOTE_INVALIDO", err.Error())
		}
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, res)
	}
}
