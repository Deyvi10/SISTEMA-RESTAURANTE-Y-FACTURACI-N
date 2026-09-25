package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/rbac"
)

type ctxKey int

const (
	principalKey ctxKey = iota
	permisosKey
)

// FromContext devuelve el usuario autenticado. Solo existe en rutas protegidas.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// MustPrincipal es FromContext para handlers que solo se montan tras Authenticate.
func MustPrincipal(ctx context.Context) Principal {
	p, ok := FromContext(ctx)
	if !ok {
		panic("auth: handler protegido sin principal")
	}
	return p
}

// Tiene indica si el usuario de la petición tiene el permiso efectivo.
func Tiene(ctx context.Context, p rbac.Permiso) bool {
	m, _ := ctx.Value(permisosKey).(map[rbac.Permiso]bool)
	return m[p]
}

var (
	errNoAutenticado = apperr.New(apperr.Unauthorized, "NO_AUTENTICADO", "Inicia sesión para continuar.")
	errDebeCambiar   = apperr.New(apperr.Forbidden, "DEBE_CAMBIAR_PASSWORD", "Antes de continuar, cambia tu contraseña temporal.")
)

// Access describe quién puede usar una ruta. Toda ruta DEBE declarar uno (QA-11).
type Access struct {
	Public       bool         // sin sesión (login, recuperación)
	Permiso      rbac.Permiso // "" = cualquier usuario autenticado
	AllowTempPwd bool         // permitida aunque deba cambiar la contraseña
}

// Guard verifica token, sesión viva, usuario activo y permiso. La consulta revisa la
// sesión en cada petición: revocar un dispositivo o desactivar un usuario surte efecto
// al instante aunque su access token no haya vencido (hallazgo X-05).
func Guard(d *db.DB, signer *Signer, now func() time.Time, a Access, next http.Handler) http.Handler {
	if a.Public {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			httpx.Error(w, r, errNoAutenticado)
			return
		}
		p, err := signer.Parse(raw)
		if err != nil {
			httpx.Error(w, r, errSesion)
			return
		}
		var rol string
		var activo bool
		var revocada *time.Time
		ajustes := map[rbac.Permiso]bool{}
		err = d.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
			if err := tx.QueryRow(ctx, `SELECT u.rol, u.activo, s.revocada_at FROM usuarios u JOIN sesiones s ON s.usuario_id = u.id
				WHERE u.id = $1 AND s.id = $2`, p.UserID, p.SessionID).Scan(&rol, &activo, &revocada); err != nil {
				return err
			}
			rows, err := tx.Query(ctx, `SELECT permiso, concedido FROM permisos_usuario WHERE usuario_id = $1`, p.UserID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var perm string
				var ok bool
				if err := rows.Scan(&perm, &ok); err != nil {
					return err
				}
				ajustes[rbac.Permiso(perm)] = ok
			}
			return rows.Err()
		})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!activo || revocada != nil)) {
			httpx.Error(w, r, errSesion)
			return
		}
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		if p.DebeCambiar && !a.AllowTempPwd {
			httpx.Error(w, r, errDebeCambiar)
			return
		}
		permisos := rbac.Efectivos(rbac.Rol(rol), ajustes)
		if a.Permiso != "" && !permisos[a.Permiso] {
			httpx.Error(w, r, apperr.ErrForbidden)
			return
		}
		p.Rol = rol
		ctx = context.WithValue(ctx, principalKey, p)
		ctx = context.WithValue(ctx, permisosKey, permisos)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
