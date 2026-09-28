package server

import (
	"context"
	"net/http"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/catalogo"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Adaptadores genéricos: los servicios no saben de HTTP y los handlers no repiten código.

type idT = ids.ID

var (
	errSinPermiso = apperr.ErrForbidden
	errIDInvalido = apperr.New(apperr.NotFound, "NO_ENCONTRADO", "No encontramos lo que buscas.")
)

func pathID(r *http.Request) (ids.ID, error) {
	id, err := ids.Parse(r.PathValue("id"))
	if err != nil {
		return ids.Nil, errIDInvalido
	}
	return id, nil
}

func list[Out any](fn func(context.Context, auth.Principal) (Out, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := fn(r.Context(), auth.MustPrincipal(r.Context()))
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, out)
	}
}

func get[Out any](fn func(context.Context, auth.Principal, ids.ID) (Out, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		out, err := fn(r.Context(), auth.MustPrincipal(r.Context()), id)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, out)
	}
}

func create[In, Out any](fn func(context.Context, auth.Principal, In) (Out, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in In
		if err := httpx.Decode(w, r, &in); err != nil {
			httpx.Error(w, r, err)
			return
		}
		out, err := fn(r.Context(), auth.MustPrincipal(r.Context()), in)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, out)
	}
}

func update[In, Out any](fn func(context.Context, auth.Principal, ids.ID, In) (Out, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		var in In
		if err := httpx.Decode(w, r, &in); err != nil {
			httpx.Error(w, r, err)
			return
		}
		out, err := fn(r.Context(), auth.MustPrincipal(r.Context()), id, in)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, out)
	}
}

func updateNoContent[In any](fn func(context.Context, auth.Principal, ids.ID, In) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		var in In
		if err := httpx.Decode(w, r, &in); err != nil {
			httpx.Error(w, r, err)
			return
		}
		if err := fn(r.Context(), auth.MustPrincipal(r.Context()), id, in); err != nil {
			httpx.Error(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func action[In any](fn func(context.Context, auth.Principal, In) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in In
		if err := httpx.Decode(w, r, &in); err != nil {
			httpx.Error(w, r, err)
			return
		}
		if err := fn(r.Context(), auth.MustPrincipal(r.Context()), in); err != nil {
			httpx.Error(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func remove(fn func(context.Context, auth.Principal, ids.ID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		if err := fn(r.Context(), auth.MustPrincipal(r.Context()), id); err != nil {
			httpx.Error(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// GET /v1/productos?categoria=…&q=…
func productos(c *catalogo.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var f catalogo.Filtro
		if v := r.URL.Query().Get("categoria"); v != "" {
			id, err := ids.Parse(v)
			if err != nil {
				httpx.Error(w, r, apperr.New(apperr.Invalid, "FILTRO_INVALIDO", "La categoría del filtro no es válida."))
				return
			}
			f.CategoriaID = &id
		}
		f.Buscar = r.URL.Query().Get("q")
		out, err := c.Productos(r.Context(), auth.MustPrincipal(r.Context()), f)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, out)
	}
}
