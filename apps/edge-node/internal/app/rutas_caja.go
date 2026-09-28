package app

import (
	"context"
	"net/http"
)

var errNoEsCaja = problema(http.StatusForbidden, "SOLO_CAJA", "Esta acción solo se hace desde una caja.")

// enCaja: como sesion, pero solo para la caja (la PC del nodo o una PC emparejada como POS).
// Un teléfono de mesero no abre turnos ni mueve efectivo.
func (a *App) enCaja(fn conSesion) http.Handler {
	return a.sesion(func(ctx context.Context, d Dispositivo, u Usuario, r *http.Request) (any, error) {
		if d.Tipo != "POS" {
			return nil, errNoEsCaja
		}
		return fn(ctx, d, u, r)
	})
}

func (a *App) rutasCaja() {
	m := a.mux
	m.Handle("GET /v1/caja/config", a.enCaja(func(ctx context.Context, _ Dispositivo, _ Usuario, _ *http.Request) (any, error) {
		return a.ConfigCaja(ctx)
	}))
	m.Handle("GET /v1/jornada", a.enCaja(func(ctx context.Context, _ Dispositivo, _ Usuario, _ *http.Request) (any, error) {
		j, err := a.JornadaActual(ctx)
		return map[string]any{"jornada": j}, err
	}))
	m.Handle("POST /v1/jornada/abrir", a.enCaja(func(ctx context.Context, _ Dispositivo, u Usuario, _ *http.Request) (any, error) {
		return a.AbrirJornada(ctx, u)
	}))
	m.Handle("POST /v1/jornada/cerrar", a.enCaja(func(ctx context.Context, _ Dispositivo, u Usuario, r *http.Request) (any, error) {
		in, err := leer[CerrarJornadaIn](nil, r)
		if err != nil {
			return nil, err
		}
		return a.CerrarJornada(ctx, u, in)
	}))
	m.Handle("GET /v1/cajas/{id}/estado", a.enCaja(func(ctx context.Context, _ Dispositivo, _ Usuario, r *http.Request) (any, error) {
		id, err := idRuta(r)
		if err != nil {
			return nil, err
		}
		return a.EstadoDeCaja(ctx, id)
	}))
	m.Handle("POST /v1/turnos", a.enCaja(func(ctx context.Context, _ Dispositivo, u Usuario, r *http.Request) (any, error) {
		in, err := leer[AbrirTurnoIn](nil, r)
		if err != nil {
			return nil, err
		}
		return a.AbrirTurno(ctx, u, in)
	}))
	m.Handle("POST /v1/turnos/cerrar", a.enCaja(func(ctx context.Context, _ Dispositivo, u Usuario, r *http.Request) (any, error) {
		in, err := leer[CerrarTurnoIn](nil, r)
		if err != nil {
			return nil, err
		}
		return a.CerrarTurno(ctx, u, in)
	}))
	m.Handle("GET /v1/clientes/buscar", a.enCaja(func(ctx context.Context, _ Dispositivo, _ Usuario, r *http.Request) (any, error) {
		q := r.URL.Query()
		return a.BuscarCliente(ctx, q.Get("tipo"), q.Get("identificacion"))
	}))
	m.Handle("GET /v1/ordenes/sin-mesa", a.enCaja(func(ctx context.Context, _ Dispositivo, _ Usuario, _ *http.Request) (any, error) {
		return a.OrdenesSinMesa(ctx)
	}))
	m.Handle("POST /v1/ordenes/{id}/propina", a.enCaja(func(ctx context.Context, _ Dispositivo, u Usuario, r *http.Request) (any, error) {
		id, err := idRuta(r)
		if err != nil {
			return nil, err
		}
		in, err := leer[PropinaIn](nil, r)
		if err != nil {
			return nil, err
		}
		return a.CambiarPropina(ctx, u, id, in)
	}))
	m.Handle("POST /v1/ordenes/{id}/cobrar", a.enCaja(func(ctx context.Context, _ Dispositivo, u Usuario, r *http.Request) (any, error) {
		id, err := idRuta(r)
		if err != nil {
			return nil, err
		}
		in, err := leer[CobrarIn](nil, r)
		if err != nil {
			return nil, err
		}
		return a.Cobrar(ctx, u, id, in)
	}))
	m.Handle("POST /v1/caja/movimientos", a.enCaja(func(ctx context.Context, _ Dispositivo, u Usuario, r *http.Request) (any, error) {
		in, err := leer[MovimientoIn](nil, r)
		if err != nil {
			return nil, err
		}
		return a.RegistrarMovimiento(ctx, u, in)
	}))
	m.Handle("GET /v1/turnos/{id}/movimientos", a.enCaja(func(ctx context.Context, _ Dispositivo, _ Usuario, r *http.Request) (any, error) {
		id, err := idRuta(r)
		if err != nil {
			return nil, err
		}
		return a.MovimientosDeTurno(ctx, id)
	}))
}
