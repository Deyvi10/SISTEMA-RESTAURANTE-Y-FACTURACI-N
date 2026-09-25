package server_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/catalogo"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/server"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// QA-11: toda ruta declara su acceso, y las públicas son solo las previstas.
func TestQA11TodaRutaDeclaraAcceso(t *testing.T) {
	publicasPermitidas := map[string]bool{
		"GET /health": true, "GET /ready": true, "GET /media/{ruta...}": true,
		"POST /v1/auth/login": true, "POST /v1/auth/refresh": true, "POST /v1/auth/logout": true,
		"POST /v1/auth/password/olvide": true, "POST /v1/auth/password/restablecer": true,
		"POST /v1/nodos/activar": true,
	}
	routes := server.Routes(server.Deps{})
	if len(routes) < 40 {
		t.Fatalf("solo %d rutas: ¿se perdió alguna?", len(routes))
	}
	vistas := map[string]bool{}
	for _, r := range routes {
		k := r.Method + " " + r.Path
		if vistas[k] {
			t.Errorf("ruta duplicada: %s", k)
		}
		vistas[k] = true
		if r.Access == nil {
			t.Errorf("%s no declara acceso", k)
			continue
		}
		if r.Access.Public && !publicasPermitidas[k] {
			t.Errorf("%s es pública sin estar en la lista permitida", k)
		}
		if strings.HasPrefix(r.Path, "/v1/") && !r.Access.Public && !r.Access.Nodo && r.Access.Permiso == "" && !strings.Contains("GET /v1/me POST /v1/auth/password/cambiar GET /v1/resumen GET /v1/tarifas-iva GET /v1/iconos-categoria GET /v1/permisos POST /v1/imagenes", k) {
			t.Errorf("%s solo exige sesión: declara un permiso concreto", k)
		}
	}
}

// QA-06: para CADA tabla con tenant_id, el tenant A no lee, no modifica, no borra y no
// inserta filas de B. Se descubren las tablas en el catálogo de PostgreSQL, así una tabla
// nueva queda cubierta sin tocar la prueba (y la prueba falla si no tiene RLS o datos).
func TestQA06AislamientoDeTenants(t *testing.T) {
	e := newEnv(t)
	a, ra := e.restaurante("1790011674001", "a@a.ec")
	b, rb := e.restaurante("1760001550001", "b@b.ec")
	// Datos en todas las tablas para ambos tenants.
	for _, c := range []*cliente{a, b} {
		var cats []catalogo.Categoria
		c.do("GET", "/v1/categorias", nil, 200, &cats)
		var tarifas []catalogo.TarifaIVA
		c.do("GET", "/v1/tarifas-iva", nil, 200, &tarifas)
		var g catalogo.Grupo
		c.do("POST", "/v1/grupos-modificadores", map[string]any{"nombre": "Extras", "modificadores": []map[string]string{{"nombre": "Queso"}}}, 201, &g)
		c.do("POST", "/v1/categorias", map[string]any{"nombre": "Con notas", "notasRapidas": []string{"Sin sal"}}, 201, nil)
		c.do("POST", "/v1/productos", map[string]any{"categoriaId": cats[0].ID, "nombre": "Plato", "precio": "5", "tarifaIvaId": tarifas[0].ID, "gruposModificadores": []any{g.ID}}, 201, nil)
		c.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Mesero", "rol": "MESERO", "pin": "8899"}, 201, nil)
		c.do("POST", "/v1/auth/password/olvide", map[string]string{"email": "noexiste@x.ec"}, 202, nil)
		c.nodoOperativo()
	}
	_ = e.ten // tokens_recuperacion: solicitud real para A
	a.do("POST", "/v1/auth/password/olvide", map[string]string{"email": "a@a.ec"}, 202, nil)
	b.do("POST", "/v1/auth/password/olvide", map[string]string{"email": "b@b.ec"}, 202, nil)

	ctx := context.Background()
	// permisos_usuario aún no tiene endpoint (RF-01-07 llega en F4): se siembra directo.
	for _, r := range []struct{ tenant, user ids.ID }{{ra.TenantID, ra.UsuarioID}, {rb.TenantID, rb.UsuarioID}} {
		if err := e.tdb.App.InTenant(ctx, r.tenant, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO permisos_usuario (tenant_id, usuario_id, permiso, concedido) VALUES ($1, $2, 'DAR_DESCUENTO', true)`, r.tenant, r.user)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := e.tdb.Admin.Query(ctx, `SELECT c.table_name, t.relrowsecurity, t.relforcerowsecurity,
		EXISTS (SELECT 1 FROM pg_policies p WHERE p.tablename = c.table_name)
		FROM information_schema.columns c JOIN pg_class t ON t.relname = c.table_name
		WHERE c.table_schema = 'public' AND c.column_name = 'tenant_id' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	type tabla struct {
		nombre             string
		rls, force, policy bool
	}
	tablas, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (tabla, error) {
		var x tabla
		return x, r.Scan(&x.nombre, &x.rls, &x.force, &x.policy)
	})
	if err != nil {
		t.Fatal(err)
	}
	tablas = append(tablas, tabla{nombre: "tenants", rls: true, force: true, policy: true})
	if len(tablas) < 15 {
		t.Fatalf("solo %d tablas con tenant_id", len(tablas))
	}
	for _, tb := range tablas {
		t.Run(tb.nombre, func(t *testing.T) {
			if !tb.rls || !tb.force || !tb.policy {
				t.Fatalf("sin RLS forzado o sin política (rls=%v force=%v policy=%v)", tb.rls, tb.force, tb.policy)
			}
			col := "tenant_id"
			if tb.nombre == "tenants" {
				col = "id"
			}
			for _, par := range []struct{ yo, otro ids.ID }{{ra.TenantID, rb.TenantID}, {rb.TenantID, ra.TenantID}} {
				err := e.tdb.App.InTenant(ctx, par.yo, func(tx pgx.Tx) error {
					var propias, ajenas int
					if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FILTER (WHERE %[1]s = $1), count(*) FILTER (WHERE %[1]s = $2) FROM %[2]s`, col, tb.nombre), par.yo, par.otro).Scan(&propias, &ajenas); err != nil {
						return err
					}
					if propias == 0 {
						return fmt.Errorf("el fixture no tiene filas propias en %s: agrega datos a la prueba", tb.nombre)
					}
					if ajenas != 0 {
						return fmt.Errorf("LEE %d filas de otro tenant", ajenas)
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				// UPDATE y DELETE sobre filas ajenas no afectan nada (y se revierte por si acaso).
				for _, stmt := range []string{
					fmt.Sprintf(`UPDATE %s SET %s = %s WHERE %s = $1`, tb.nombre, col, col, col),
					fmt.Sprintf(`DELETE FROM %s WHERE %s = $1`, tb.nombre, col),
				} {
					err := e.tdb.App.InTenant(ctx, par.yo, func(tx pgx.Tx) error {
						tag, err := tx.Exec(ctx, stmt, par.otro)
						if err != nil && !strings.Contains(err.Error(), "permission denied") {
							return err
						}
						if err == nil && tag.RowsAffected() != 0 {
							return fmt.Errorf("%q afectó %d filas de otro tenant", stmt, tag.RowsAffected())
						}
						return errRollback
					})
					if err != nil && !errors.Is(err, errRollback) {
						t.Fatal(err)
					}
				}
				// INSERT de una fila con el tenant ajeno: la política WITH CHECK la rechaza.
				err = e.tdb.App.InTenant(ctx, par.yo, func(tx pgx.Tx) error {
					_, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s SELECT (jsonb_populate_record(NULL::%[1]s, to_jsonb(r) || jsonb_build_object('%[2]s', $1::uuid, 'id', gen_random_uuid()))).* FROM %[1]s r LIMIT 1`, tb.nombre, col), par.otro)
					if err == nil {
						return fmt.Errorf("pudo INSERTAR una fila de otro tenant")
					}
					if !strings.Contains(err.Error(), "row-level security") && !strings.Contains(err.Error(), "permission denied") && !strings.Contains(err.Error(), "violates") {
						return fmt.Errorf("error inesperado al insertar: %w", err)
					}
					return errRollback
				})
				if err != nil && !errors.Is(err, errRollback) {
					t.Fatal(err)
				}
			}
		})
	}
}

var errRollback = fmt.Errorf("rollback intencional")

// El rol de la aplicación no es dueño de las tablas ni puede saltarse RLS (ADR-0008).
func TestRolDeAppSinPrivilegios(t *testing.T) {
	e := newEnv(t)
	var super, bypass bool
	var duenas int
	err := e.tdb.App.Global(context.Background(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(context.Background(), `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&super, &bypass); err != nil {
			return err
		}
		return tx.QueryRow(context.Background(), `SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tableowner = current_user`).Scan(&duenas)
	})
	if err != nil {
		t.Fatal(err)
	}
	if super || bypass || duenas != 0 {
		t.Fatalf("rol de app con privilegios: super=%v bypass=%v dueña de %d tablas", super, bypass, duenas)
	}
}
