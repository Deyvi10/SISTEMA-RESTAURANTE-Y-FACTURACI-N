package app

import (
	"context"
	"net/http"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Rutas de la app de meseros (F3). Todas exigen un dispositivo emparejado (o esta PC) y,
// salvo el emparejamiento y el login, una sesión de usuario abierta con PIN.

type conSesion func(ctx context.Context, d Dispositivo, u Usuario, r *http.Request) (any, error)

func (a *App) sesion(fn conSesion) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := a.dispositivoDe(r)
		if err != nil {
			responderError(w, a.Log, err)
			return
		}
		u, err := a.usuarioDe(r, d)
		if err != nil {
			responderError(w, a.Log, err)
			return
		}
		out, err := fn(r.Context(), d, u, r)
		if err != nil {
			responderError(w, a.Log, err)
			return
		}
		if out == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
}

func (a *App) conDispositivo(fn func(ctx context.Context, d Dispositivo, r *http.Request) (any, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := a.dispositivoDe(r)
		if err != nil {
			responderError(w, a.Log, err)
			return
		}
		out, err := fn(r.Context(), d, r)
		if err != nil {
			responderError(w, a.Log, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
}

func leer[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var in T
	err := decodificarJSON(w, r, &in)
	return in, err
}

func idRuta(r *http.Request) (ids.ID, error) {
	id, err := ids.Parse(r.PathValue("id"))
	if err != nil {
		return ids.Nil, problema(http.StatusNotFound, "NO_ENCONTRADO", "No existe ese elemento.")
	}
	return id, nil
}

// conID combina el id de la ruta y un cuerpo JSON en una acción con sesión.
func conIDBody[In any](a *App, fn func(ctx context.Context, d Dispositivo, u Usuario, id ids.ID, in In) (any, error)) http.Handler {
	return a.sesion(func(ctx context.Context, d Dispositivo, u Usuario, r *http.Request) (any, error) {
		id, err := idRuta(r)
		if err != nil {
			return nil, err
		}
		in, err := leer[In](nil, r)
		if err != nil {
			return nil, err
		}
		return fn(ctx, d, u, id, in)
	})
}

func (a *App) rutasApp() {
	m := a.mux
	// Emparejamiento (F3-02).
	m.Handle("POST /v1/emparejamiento", soloLocal(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e, err := a.NuevoEmparejamiento(r.Context())
		if err != nil {
			responderError(w, a.Log, err)
			return
		}
		writeJSON(w, http.StatusOK, e)
	})))
	m.HandleFunc("GET /emparejar", pagina("emparejar.html"))
	m.HandleFunc("POST /v1/dispositivos/emparejar", func(w http.ResponseWriter, r *http.Request) {
		in, err := leer[EmparejarIn](w, r)
		if err == nil {
			var out EmparejadoOut
			if out, err = a.Emparejar(r.Context(), in, ipDe(r)); err == nil {
				writeJSON(w, http.StatusOK, out)
				return
			}
		}
		responderError(w, a.Log, err)
	})
	m.HandleFunc("GET /v1/dispositivos/desafio", func(w http.ResponseWriter, r *http.Request) {
		id, err := ids.Parse(r.URL.Query().Get("dispositivoId"))
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "DATOS_INVALIDOS", "Falta el dispositivo.")
			return
		}
		nonce, exp := a.NuevoDesafio(id)
		writeJSON(w, http.StatusOK, map[string]any{"nonce": nonce, "expiraAt": exp})
	})
	m.HandleFunc("POST /v1/dispositivos/sesion", func(w http.ResponseWriter, r *http.Request) {
		in, err := leer[SesionDispositivoIn](w, r)
		if err == nil {
			var out SesionDispositivo
			if out, err = a.AbrirSesionDispositivo(r.Context(), in); err == nil {
				writeJSON(w, http.StatusOK, out)
				return
			}
		}
		responderError(w, a.Log, err)
	})

	// Personal y PIN (F3-04).
	m.Handle("GET /v1/personal", a.conDispositivo(func(ctx context.Context, _ Dispositivo, _ *http.Request) (any, error) {
		return a.Personal(ctx)
	}))
	m.Handle("POST /v1/sesiones", a.conDispositivo(func(ctx context.Context, d Dispositivo, r *http.Request) (any, error) {
		in, err := leer[EntrarIn](nil, r)
		if err != nil {
			return nil, err
		}
		return a.Entrar(ctx, d, in)
	}))
	m.Handle("DELETE /v1/sesiones", a.conDispositivo(func(ctx context.Context, _ Dispositivo, r *http.Request) (any, error) {
		return map[string]bool{"ok": true}, a.Salir(ctx, r)
	}))
	m.Handle("GET /v1/yo", a.sesion(func(_ context.Context, _ Dispositivo, u Usuario, _ *http.Request) (any, error) { return u, nil }))
	m.Handle("POST /v1/autorizaciones", a.conDispositivo(func(ctx context.Context, d Dispositivo, r *http.Request) (any, error) {
		in, err := leer[AutorizarIn](nil, r)
		if err != nil {
			return nil, err
		}
		return a.Autorizar(ctx, d, in)
	}))

	// Catálogo para el teléfono (F3-05): lo lee de la réplica del nodo.
	m.Handle("GET /v1/catalogo", a.conDispositivo(func(ctx context.Context, _ Dispositivo, _ *http.Request) (any, error) {
		return a.Catalogo(ctx)
	}))

	// Salón y bloqueos (F3-06, F3-07).
	m.Handle("GET /v1/salon", a.sesion(func(ctx context.Context, _ Dispositivo, _ Usuario, _ *http.Request) (any, error) {
		return a.SalonVivo(ctx)
	}))
	m.Handle("POST /v1/mesas/{id}/bloqueo", a.sesion(func(ctx context.Context, d Dispositivo, u Usuario, r *http.Request) (any, error) {
		id, err := idRuta(r)
		if err != nil {
			return nil, err
		}
		return a.BloquearMesa(ctx, d, u, id)
	}))
	m.Handle("POST /v1/mesas/{id}/latido", a.sesion(func(ctx context.Context, d Dispositivo, u Usuario, r *http.Request) (any, error) {
		id, err := idRuta(r)
		if err != nil {
			return nil, err
		}
		return a.LatidoMesa(ctx, d, u, id)
	}))
	m.Handle("DELETE /v1/mesas/{id}/bloqueo", a.sesion(func(ctx context.Context, d Dispositivo, u Usuario, r *http.Request) (any, error) {
		id, err := idRuta(r)
		if err != nil {
			return nil, err
		}
		return nil, a.LiberarMesa(ctx, d, u, id, r.URL.Query().Get("forzar") == "1")
	}))
	m.Handle("GET /v1/mesas/{id}/orden", a.sesion(func(ctx context.Context, _ Dispositivo, _ Usuario, r *http.Request) (any, error) {
		id, err := idRuta(r)
		if err != nil {
			return nil, err
		}
		return a.OrdenDeMesa(ctx, id)
	}))

	// Órdenes (F3-08 … F3-14).
	m.Handle("POST /v1/ordenes/enviar", a.sesion(func(ctx context.Context, d Dispositivo, u Usuario, r *http.Request) (any, error) {
		in, err := leer[EnviarOrdenIn](nil, r)
		if err != nil {
			return nil, err
		}
		return a.EnviarOrden(ctx, d, u, in)
	}))
	m.Handle("GET /v1/ordenes/{id}", a.sesion(func(ctx context.Context, _ Dispositivo, _ Usuario, r *http.Request) (any, error) {
		id, err := idRuta(r)
		if err != nil {
			return nil, err
		}
		o, err := leerOrden(ctx, a.Store.Read(), id)
		if err != nil {
			return nil, err
		}
		t, err := a.TotalesDe(ctx, id)
		return map[string]any{"orden": o, "totales": t}, err
	}))
	m.Handle("POST /v1/ordenes/{id}/marchar", conIDBody(a, func(ctx context.Context, _ Dispositivo, u Usuario, id ids.ID, in MarcharIn) (any, error) {
		return a.Marchar(ctx, u, id, in)
	}))
	m.Handle("POST /v1/ordenes/{id}/precuenta", conIDBody(a, func(ctx context.Context, _ Dispositivo, u Usuario, id ids.ID, in PrecuentaIn) (any, error) {
		return a.Precuenta(ctx, u, id, in)
	}))
	m.Handle("POST /v1/ordenes/{id}/mover", conIDBody(a, func(ctx context.Context, d Dispositivo, u Usuario, id ids.ID, in MoverIn) (any, error) {
		return a.Mover(ctx, d, u, id, in)
	}))
	m.Handle("POST /v1/ordenes/{id}/unir", conIDBody(a, func(ctx context.Context, _ Dispositivo, u Usuario, id ids.ID, in UnirIn) (any, error) {
		return a.Unir(ctx, u, id, in)
	}))
	m.Handle("POST /v1/ordenes/{id}/transferir", conIDBody(a, func(ctx context.Context, _ Dispositivo, u Usuario, id ids.ID, in TransferirIn) (any, error) {
		return a.Transferir(ctx, u, id, in)
	}))
	m.Handle("POST /v1/ordenes/{id}/anular", conIDBody(a, func(ctx context.Context, d Dispositivo, u Usuario, id ids.ID, in AnularOrdenIn) (any, error) {
		return a.AnularLineasOrden(ctx, d, u, id, in)
	}))
}
