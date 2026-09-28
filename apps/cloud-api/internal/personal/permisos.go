package personal

import (
	"context"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	aud "github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
)

// Permiso es una fila de la pantalla de interruptores (RF-01-07, F4-14).
type Permiso struct {
	Permiso      rbac.Permiso `json:"permiso"`
	Nombre       string       `json:"nombre"`
	Descripcion  string       `json:"descripcion"`
	Concedido    bool         `json:"concedido"`    // lo que vale hoy para esta persona
	Configurable bool         `json:"configurable"` // ⚙️: el dueño lo puede cambiar para su rol
	PorDefecto   bool         `json:"porDefecto"`   // lo que trae el rol sin ajustes
}

type PermisosUsuario struct {
	UsuarioID ids.ID    `json:"usuarioId"`
	Rol       string    `json:"rol"`
	Permisos  []Permiso `json:"permisos"`
}

func (s *Service) leerPermisos(ctx context.Context, tx db.Tx, id ids.ID) (PermisosUsuario, error) {
	u, err := s.obtener(ctx, tx, id)
	if err != nil {
		return PermisosUsuario{}, err
	}
	rows, err := tx.Query(ctx, `SELECT permiso, concedido FROM permisos_usuario WHERE usuario_id = $1`, id)
	if err != nil {
		return PermisosUsuario{}, err
	}
	ajustes := map[rbac.Permiso]bool{}
	for rows.Next() {
		var p string
		var c bool
		if err := rows.Scan(&p, &c); err != nil {
			rows.Close()
			return PermisosUsuario{}, err
		}
		ajustes[rbac.Permiso(p)] = c
	}
	rows.Close()
	rol := rbac.Rol(u.Rol)
	efectivos := rbac.Efectivos(rol, ajustes)
	out := PermisosUsuario{UsuarioID: id, Rol: u.Rol, Permisos: []Permiso{}}
	for _, info := range rbac.Catalogo() {
		out.Permisos = append(out.Permisos, Permiso{Permiso: info.Permiso, Nombre: info.Nombre, Descripcion: info.Descripcion,
			Concedido: efectivos[info.Permiso], Configurable: rbac.Configurable(rol, info.Permiso), PorDefecto: rbac.Defecto(rol, info.Permiso)})
	}
	return out, nil
}

// Permisos devuelve los permisos de una persona con lo que es configurable para su rol.
func (s *Service) Permisos(ctx context.Context, p auth.Principal, id ids.ID) (PermisosUsuario, error) {
	var out PermisosUsuario
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var err error
		out, err = s.leerPermisos(ctx, tx, id)
		return err
	})
	return out, err
}

type CambioPermiso struct {
	Permiso   rbac.Permiso `json:"permiso"`
	Concedido bool         `json:"concedido"`
}

// CambiarPermiso activa o desactiva un permiso configurable (⚙️) para una persona. Queda en
// la auditoría de la nube y llega al Nodo Local por la réplica de permisos_usuario.
func (s *Service) CambiarPermiso(ctx context.Context, p auth.Principal, id ids.ID, c CambioPermiso) (PermisosUsuario, error) {
	permiso := c.Permiso
	if !rbac.Existe(permiso) {
		return PermisosUsuario{}, apperr.ErrNotFound
	}
	var out PermisosUsuario
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		antes, err := s.leerPermisos(ctx, tx, id)
		if err != nil {
			return err
		}
		if !rbac.Configurable(rbac.Rol(antes.Rol), permiso) {
			return apperr.New(apperr.Conflict, "NO_CONFIGURABLE", "Ese permiso no se puede cambiar para este rol.")
		}
		var previo bool
		for _, x := range antes.Permisos {
			if x.Permiso == permiso {
				previo = x.Concedido
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO permisos_usuario (tenant_id, usuario_id, permiso, concedido) VALUES ($1, $2, $3, $4)
			ON CONFLICT (usuario_id, permiso) DO UPDATE SET concedido = excluded.concedido, updated_at = now()`, p.TenantID, id, string(permiso), c.Concedido); err != nil {
			return err
		}
		if previo != c.Concedido {
			if err := s.auditar(ctx, tx, p, "PERMISO_CAMBIADO", id, map[string]any{"permiso": permiso, "concedido": previo},
				map[string]any{"permiso": permiso, "concedido": c.Concedido}); err != nil {
				return err
			}
		}
		out, err = s.leerPermisos(ctx, tx, id)
		return err
	})
	return out, err
}

// auditar registra en la cadena de la nube una acción sobre una persona (RF-08-06.1).
func (s *Service) auditar(ctx context.Context, tx db.Tx, p auth.Principal, accion string, usuario ids.ID, antes, despues any) error {
	autor := p.UserID
	return auditoria.Registrar(ctx, tx, p.TenantID, aud.Registro{UsuarioID: &autor, Accion: accion, Entidad: "usuario", EntidadID: usuario.String(),
		Antes: aud.Compactar(antes), Despues: aud.Compactar(despues)}, time.Now())
}
