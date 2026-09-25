// Package server arma el router HTTP: cada ruta declara su acceso (QA-11) y comparte los
// middlewares transversales (request id, logs, recover, CORS, cabeceras de seguridad).
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/catalogo"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/imagenes"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/nodos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/personal"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/rbac"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/salon"
)

// Route es una ruta de la API. Access nil = ruta mal declarada (la prueba QA-11 falla).
type Route struct {
	Method, Path string
	Access       *auth.Access
	Handler      http.HandlerFunc
}

var (
	publico     = &auth.Access{Public: true}
	autenticado = &auth.Access{}
	tempPwd     = &auth.Access{AllowTempPwd: true}
	nodo        = &auth.Access{Nodo: true}
)

func con(p rbac.Permiso) *auth.Access { return &auth.Access{Permiso: p} }

type Deps struct {
	DB            *db.DB
	Signer        *auth.Signer
	Now           func() time.Time
	Auth          *auth.Handlers
	Salon         *salon.Service
	Catalogo      *catalogo.Service
	Personal      *personal.Service
	Imagenes      *imagenes.Service
	Nodos         *nodos.Service
	BackofficeURL string
}

// Routes devuelve la tabla completa de rutas.
func Routes(d Deps) []Route {
	s, c, pe := d.Salon, d.Catalogo, d.Personal
	menu, sal, per := con(rbac.ConfigurarMenu), con(rbac.ConfigurarSalon), con(rbac.GestionarPersonal)
	return []Route{
		{"GET", "/health", publico, func(w http.ResponseWriter, r *http.Request) { httpx.JSON(w, 200, map[string]string{"estado": "ok"}) }},
		{"GET", "/ready", publico, ready(d.DB)},
		{"GET", "/media/{ruta...}", publico, d.Imagenes.HandleMedia},

		{"POST", "/v1/auth/login", publico, d.Auth.Login},
		{"POST", "/v1/auth/refresh", publico, d.Auth.Refresh},
		{"POST", "/v1/auth/logout", publico, d.Auth.Logout},
		{"POST", "/v1/auth/password/olvide", publico, d.Auth.Olvide},
		{"POST", "/v1/auth/password/restablecer", publico, d.Auth.Restablecer},
		{"POST", "/v1/auth/password/cambiar", tempPwd, d.Auth.Cambiar},
		{"GET", "/v1/me", tempPwd, d.Auth.Me},

		{"GET", "/v1/resumen", autenticado, resumen(d.DB)},
		{"GET", "/v1/tarifas-iva", autenticado, list(func(ctx context.Context, _ auth.Principal) ([]catalogo.TarifaIVA, error) {
			return c.TarifasVigentes(ctx)
		})},
		{"GET", "/v1/iconos-categoria", autenticado, func(w http.ResponseWriter, r *http.Request) { httpx.JSON(w, 200, catalogo.IconosCategoria) }},
		{"GET", "/v1/permisos", autenticado, func(w http.ResponseWriter, r *http.Request) { httpx.JSON(w, 200, rbac.Catalogo()) }},

		{"GET", "/v1/locales", sal, list(s.Locales)},
		{"PATCH", "/v1/locales/{id}", sal, update(s.ActualizarLocal)},
		{"GET", "/v1/estaciones", sal, list(s.Estaciones)},
		{"POST", "/v1/estaciones", sal, create(s.CrearEstacion)},
		{"PUT", "/v1/estaciones/{id}", sal, update(s.ActualizarEstacion)},
		{"DELETE", "/v1/estaciones/{id}", sal, remove(s.EliminarEstacion)},
		{"GET", "/v1/zonas", sal, list(s.Zonas)},
		{"POST", "/v1/zonas", sal, create(s.CrearZona)},
		{"PATCH", "/v1/zonas/{id}", sal, update(func(ctx context.Context, p auth.Principal, id idT, in struct {
			Nombre string `json:"nombre"`
		},
		) (salon.Zona, error) {
			return s.RenombrarZona(ctx, p, id, in.Nombre)
		})},
		{"DELETE", "/v1/zonas/{id}", sal, remove(s.EliminarZona)},
		{"GET", "/v1/mesas", sal, list(s.Mesas)},
		{"POST", "/v1/mesas", sal, create(s.CrearMesa)},
		{"POST", "/v1/mesas/lote", sal, create(func(ctx context.Context, p auth.Principal, in struct {
			ZonaID    idT `json:"zonaId"`
			Cantidad  int `json:"cantidad"`
			Capacidad int `json:"capacidad"`
		},
		) ([]salon.Mesa, error) {
			return s.CrearLote(ctx, p, in.ZonaID, in.Cantidad, in.Capacidad)
		})},
		{"PUT", "/v1/mesas/{id}", sal, update(s.ActualizarMesa)},
		{"DELETE", "/v1/mesas/{id}", sal, remove(s.EliminarMesa)},

		{"GET", "/v1/categorias", menu, list(c.Categorias)},
		{"POST", "/v1/categorias", menu, create(c.CrearCategoria)},
		{"PUT", "/v1/categorias/orden", menu, action(func(ctx context.Context, p auth.Principal, in struct {
			Orden []idT `json:"orden"`
		},
		) error {
			return c.OrdenarCategorias(ctx, p, in.Orden)
		})},
		{"PUT", "/v1/categorias/{id}", menu, update(c.ActualizarCategoria)},
		{"DELETE", "/v1/categorias/{id}", menu, remove(c.EliminarCategoria)},
		{"GET", "/v1/productos", menu, productos(c)},
		{"GET", "/v1/productos/{id}", menu, get(c.Producto)},
		{"POST", "/v1/productos", menu, create(c.CrearProducto)},
		{"PUT", "/v1/productos/{id}", menu, update(c.ActualizarProducto)},
		{"DELETE", "/v1/productos/{id}", menu, remove(c.EliminarProducto)},
		{"GET", "/v1/grupos-modificadores", menu, list(c.Grupos)},
		{"POST", "/v1/grupos-modificadores", menu, create(c.CrearGrupo)},
		{"PUT", "/v1/grupos-modificadores/{id}", menu, update(c.ActualizarGrupo)},
		{"DELETE", "/v1/grupos-modificadores/{id}", menu, remove(c.EliminarGrupo)},
		{"GET", "/v1/galeria", menu, d.Imagenes.HandleGaleria},
		// Subir fotos sirve al menú y a los avatares: se exige uno de los dos permisos.
		{"POST", "/v1/imagenes", autenticado, func(w http.ResponseWriter, r *http.Request) {
			if !auth.Tiene(r.Context(), rbac.ConfigurarMenu) && !auth.Tiene(r.Context(), rbac.GestionarPersonal) {
				httpx.Error(w, r, errSinPermiso)
				return
			}
			d.Imagenes.HandleSubir(w, r)
		}},

		{"GET", "/v1/nodos", sal, list(d.Nodos.Listar)},
		{"POST", "/v1/nodos/codigos", sal, create(d.Nodos.GenerarCodigo)},
		{"POST", "/v1/nodos/{id}/revocar", sal, remove(d.Nodos.Revocar)},
		{"POST", "/v1/nodos/activar", publico, activarNodo(d.Nodos)},
		{"POST", "/v1/nodos/heartbeat", nodo, heartbeat(d.Nodos)},
		{"POST", "/v1/sync/push", nodo, syncPush(d.DB)},
		{"GET", "/v1/sync/pull", nodo, syncPull(d.Nodos)},

		{"GET", "/v1/usuarios", per, list(pe.Listar)},
		{"POST", "/v1/usuarios", per, create(pe.Crear)},
		{"PUT", "/v1/usuarios/{id}", per, update(pe.Actualizar)},
		{"PUT", "/v1/usuarios/{id}/pin", per, updateNoContent(func(ctx context.Context, p auth.Principal, id idT, in struct {
			PIN string `json:"pin"`
		},
		) error {
			return pe.CambiarPIN(ctx, p, id, in.PIN)
		})},
		{"PUT", "/v1/usuarios/{id}/estado", per, update(func(ctx context.Context, p auth.Principal, id idT, in struct {
			Activo bool `json:"activo"`
		},
		) (personal.Usuario, error) {
			return pe.CambiarEstado(ctx, p, id, in.Activo)
		})},
	}
}

// Handler arma el mux con guardas por ruta y middlewares globales.
func Handler(d Deps, routes []Route) http.Handler {
	mux := http.NewServeMux()
	for _, rt := range routes {
		mux.Handle(rt.Method+" "+rt.Path, auth.Guard(d.DB, d.Signer, d.Now, *rt.Access, rt.Handler))
	}
	return httpx.Chain(mux, httpx.WithRequestID, httpx.WithRecover, httpx.WithLogging, httpx.WithSecurityHeaders, httpx.WithCORS(d.BackofficeURL))
}

func ready(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.Pool.Ping(ctx); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"estado": "sin base de datos"})
			return
		}
		httpx.JSON(w, 200, map[string]string{"estado": "listo"})
	}
}
