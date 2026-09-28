package server_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	cloudaud "github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/personal"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/auditoria"
)

// F4-14: el dueño activa o desactiva los permisos ⚙️ de cada persona; el cambio queda en la
// auditoría de la nube y viaja al nodo por la réplica.
func TestPermisosGranulares(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "a@a.ec")
	ctx := context.Background()
	var cajero personal.Usuario
	c.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Luis P.", "rol": "CAJERO", "pin": "7391"}, 201, &cajero)
	base := "/v1/usuarios/" + cajero.ID.String() + "/permisos"
	permiso := func(ps personal.PermisosUsuario, p string) personal.Permiso {
		for _, x := range ps.Permisos {
			if string(x.Permiso) == p {
				return x
			}
		}
		t.Fatalf("falta %s", p)
		return personal.Permiso{}
	}

	var ps personal.PermisosUsuario
	c.do("GET", base, nil, 200, &ps)
	if a := permiso(ps, "ABRIR_CAJON"); !a.Configurable || a.Concedido || a.PorDefecto {
		t.Fatalf("abrir cajón para un cajero: %+v", a)
	}
	if s := permiso(ps, "CONFIGURAR_SRI"); s.Configurable || s.Concedido {
		t.Fatalf("configurar SRI: %+v", s)
	}
	if g := permiso(ps, "GESTIONAR_TURNO"); !g.Concedido || g.Configurable {
		t.Fatalf("gestionar turno viene con el rol: %+v", g)
	}

	c.do("PUT", base, map[string]any{"permiso": "ABRIR_CAJON", "concedido": true}, 200, &ps)
	if !permiso(ps, "ABRIR_CAJON").Concedido {
		t.Fatal("no quedó concedido")
	}
	c.do("PUT", base, map[string]any{"permiso": "ABRIR_CAJON", "concedido": true}, 200, nil) // igual: no se audita dos veces
	c.do("PUT", base, map[string]any{"permiso": "CONFIGURAR_SRI", "concedido": true}, 409, nil)
	c.do("PUT", base, map[string]any{"permiso": "NO_EXISTE", "concedido": true}, 404, nil)
	c.do("PUT", "/v1/usuarios/"+cajero.ID.String()+"/estado", map[string]any{"activo": false}, 200, nil)

	// Auditoría de la nube: alta, permiso y baja, en una cadena íntegra.
	var cadena []auditoria.Registro
	if err := e.tdb.App.InTenant(ctx, r.TenantID, func(tx pgx.Tx) error {
		var err error
		cadena, err = cloudaud.Cadena(ctx, tx, r.TenantID, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var acciones []string
	for _, x := range cadena {
		if x.EntidadID == cajero.ID.String() {
			acciones = append(acciones, x.Accion)
		}
	}
	if len(acciones) != 3 || acciones[0] != "USUARIO_ALTA" || acciones[1] != "PERMISO_CAMBIADO" || acciones[2] != "USUARIO_BAJA" || auditoria.Verificar(cadena) != -1 {
		t.Fatalf("auditoría: %v (cadena rota en %d)", acciones, auditoria.Verificar(cadena))
	}
	for _, x := range cadena {
		if x.Accion == "PERMISO_CAMBIADO" && (string(x.Antes) != `{"concedido":false,"permiso":"ABRIR_CAJON"}` || string(x.Despues) != `{"concedido":true,"permiso":"ABRIR_CAJON"}` || x.UsuarioID == nil || *x.UsuarioID != r.UsuarioID) {
			t.Fatalf("registro del permiso: %+v", x)
		}
	}
	// El cambio viaja al nodo por el feed de la réplica.
	var n int
	if err := e.tdb.Admin.QueryRow(ctx, `SELECT count(*) FROM sync_cambios WHERE tenant_id = $1 AND tabla = 'permisos_usuario'`, r.TenantID).Scan(&n); err != nil || n == 0 {
		t.Fatalf("réplica: %v %d", err, n)
	}
}
