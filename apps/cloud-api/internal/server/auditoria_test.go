package server_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	cloudaud "github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// auditar simula un registro de la cadena del nodo y lo envía por el push.
func (n *nodoSim) auditar(tenant ids.ID, seq int64, anterior string, alterar func(*auditoria.Registro)) auditoria.Registro {
	n.e.t.Helper()
	u := ids.New()
	r := auditoria.Registro{ID: ids.New(), Seq: seq, TenantID: tenant, UsuarioID: &u, Accion: "LINEA_ANULADA", Entidad: "orden", EntidadID: ids.New().String(),
		Monto: "3.50", Motivo: "Se enfrió", Detalle: auditoria.Compactar(map[string]any{"lineas": 1}), CreatedAt: time.Now()}
	r.Sellar(anterior)
	if alterar != nil {
		alterar(&r)
	}
	ev := n.evento(cloudaud.EventoNodo)
	ev.Payload, _ = json.Marshal(r)
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: []edgesync.Event{ev}}, true, 200, nil)
	return r
}

func TestAuditoriaInmutableConCadena(t *testing.T) {
	e := newEnv(t)
	c, res := e.restaurante("1790011674001", "a@a.ec")
	n := c.e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	ctx := context.Background()
	marcas := func(id ids.ID) (hashOK, cadenaOK bool) {
		if err := e.tdb.Admin.QueryRow(ctx, `SELECT hash_valido, cadena_valida FROM auditoria WHERE id = $1`, id).Scan(&hashOK, &cadenaOK); err != nil {
			t.Fatal(err)
		}
		return
	}

	// La cadena del nodo llega en orden y se verifica completa.
	r1 := n.auditar(res.TenantID, 1, "", nil)
	r2 := n.auditar(res.TenantID, 2, r1.Hash, nil)
	var cadena []auditoria.Registro
	if err := e.tdb.App.InTenant(ctx, res.TenantID, func(tx pgx.Tx) error {
		var err error
		cadena, err = cloudaud.Cadena(ctx, tx, res.TenantID, &n.id)
		return err
	}); err != nil || len(cadena) != 2 || auditoria.Verificar(cadena) != -1 {
		t.Fatalf("cadena del nodo: %v %d %d", err, len(cadena), auditoria.Verificar(cadena))
	}
	if h, c := marcas(r2.ID); !h || !c {
		t.Fatalf("registro íntegro marcado mal: %v %v", h, c)
	}
	// Un registro con el monto cambiado después de sellarlo, y otro que no sigue al último.
	alterado := n.auditar(res.TenantID, 3, r2.Hash, func(r *auditoria.Registro) { r.Monto = "0.50" })
	if h, _ := marcas(alterado.ID); h {
		t.Fatal("un registro alterado pasó como íntegro")
	}
	salteado := n.auditar(res.TenantID, 4, "hash-que-no-existe", nil)
	if h, c := marcas(salteado.ID); !h || c {
		t.Fatalf("eslabón roto: hash %v cadena %v", h, c)
	}

	// La cadena propia de la nube (acciones del panel web).
	for i := range 2 {
		if err := e.tdb.App.InTenant(ctx, res.TenantID, func(tx pgx.Tx) error {
			return cloudaud.Registrar(ctx, tx, res.TenantID, auditoria.Registro{Accion: "PERMISO_CAMBIADO", Entidad: "usuario", EntidadID: ids.New().String(),
				Despues: auditoria.Compactar(map[string]any{"concedido": i == 0})}, time.Now())
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.tdb.App.InTenant(ctx, res.TenantID, func(tx pgx.Tx) error {
		var err error
		cadena, err = cloudaud.Cadena(ctx, tx, res.TenantID, nil)
		return err
	}); err != nil || len(cadena) != 2 || cadena[1].Seq != 2 || auditoria.Verificar(cadena) != -1 {
		t.Fatalf("cadena de la nube: %v %+v", err, cadena)
	}

	// QA-10: la aplicación no puede cambiar ni borrar; el dueño de la base tampoco (trigger).
	for _, q := range []string{`UPDATE auditoria SET monto = 0`, `DELETE FROM auditoria`} {
		if err := e.tdb.App.InTenant(ctx, res.TenantID, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, q); return err }); err == nil {
			t.Fatalf("la aplicación pudo: %s", q)
		}
		if _, err := e.tdb.Admin.Exec(ctx, q); err == nil {
			t.Fatalf("el dueño de la base pudo: %s", q)
		}
	}
	for _, tabla := range []string{"auditoria", "sync_eventos", "cierres_z"} {
		if _, err := e.tdb.Admin.Exec(ctx, `TRUNCATE `+tabla+` CASCADE`); err == nil {
			t.Fatalf("TRUNCATE %s se permitió", tabla)
		}
	}
}
