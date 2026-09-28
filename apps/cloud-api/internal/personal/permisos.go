package personal

import (
	"context"
	"strings"
	"time"

	"github.com/shopspring/decimal"

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
	// Límite propio de descuento sin autorización (nil = el del local). F4-09.
	DescuentoMaximoPct *string `json:"descuentoMaximoPct"`
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
	if err := tx.QueryRow(ctx, `SELECT descuento_maximo_pct::text FROM usuarios WHERE id = $1`, id).Scan(&out.DescuentoMaximoPct); err != nil {
		return PermisosUsuario{}, err
	}
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

type LimiteDescuento struct {
	Porcentaje *string `json:"porcentaje"` // nil o vacío = usar el del local
}

// CambiarLimiteDescuento fija (o quita) el descuento que la persona puede dar sin
// autorización (RF-04-07.3). Queda auditado y llega al nodo por la réplica de usuarios.
func (s *Service) CambiarLimiteDescuento(ctx context.Context, p auth.Principal, id ids.ID, in LimiteDescuento) (PermisosUsuario, error) {
	var nuevo *decimal.Decimal
	if in.Porcentaje != nil && strings.TrimSpace(*in.Porcentaje) != "" {
		d, err := decimal.NewFromString(strings.TrimSpace(*in.Porcentaje))
		if err != nil || d.IsNegative() || d.GreaterThan(decimal.NewFromInt(100)) || d.Exponent() < -2 {
			return PermisosUsuario{}, fieldErr("porcentaje", "Escribe un porcentaje entre 0 y 100, o déjalo vacío para usar el del local.")
		}
		nuevo = &d
	}
	var out PermisosUsuario
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		antes, err := s.leerPermisos(ctx, tx, id)
		if err != nil {
			return err
		}
		if antes.Rol == string(rbac.Admin) {
			return apperr.New(apperr.Conflict, "SIN_LIMITE", "El administrador no tiene límite de descuento.")
		}
		if _, err := tx.Exec(ctx, `UPDATE usuarios SET descuento_maximo_pct = $2 WHERE id = $1`, id, nuevo); err != nil {
			return err
		}
		if out, err = s.leerPermisos(ctx, tx, id); err != nil {
			return err
		}
		if !mismoTexto(antes.DescuentoMaximoPct, out.DescuentoMaximoPct) {
			return s.auditar(ctx, tx, p, "LIMITE_DESCUENTO_CAMBIADO", id, map[string]any{"porcentaje": antes.DescuentoMaximoPct}, map[string]any{"porcentaje": out.DescuentoMaximoPct})
		}
		return nil
	})
	return out, err
}

func mismoTexto(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// auditar registra en la cadena de la nube una acción sobre una persona (RF-08-06.1).
func (s *Service) auditar(ctx context.Context, tx db.Tx, p auth.Principal, accion string, usuario ids.ID, antes, despues any) error {
	autor := p.UserID
	return auditoria.Registrar(ctx, tx, p.TenantID, aud.Registro{UsuarioID: &autor, Accion: accion, Entidad: "usuario", EntidadID: usuario.String(),
		Antes: aud.Compactar(antes), Despues: aud.Compactar(despues)}, time.Now())
}
