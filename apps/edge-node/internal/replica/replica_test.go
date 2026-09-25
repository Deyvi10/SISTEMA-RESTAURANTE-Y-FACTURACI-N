package replica

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
)

func TestAplicarEsIdempotenteYExacto(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	r, err := Cargar(ctx, st.Read())
	if err != nil {
		t.Fatal(err)
	}
	prod := edgesync.Cambio{Tabla: "productos", Op: "U", Datos: []byte(`{"id":"p1","tenant_id":"t","categoria_id":"c","nombre":"Ceviche","precio":12.500000,"tarifa_iva_id":"iva","activo":true,"orden":3,"columna_del_futuro":"x"}`)}
	grupo := edgesync.Cambio{Tabla: "producto_grupos_modificadores", Op: "U", Datos: []byte(`{"tenant_id":"t","producto_id":"p1","grupo_id":"g1","orden":0}`)}
	desconocida := edgesync.Cambio{Tabla: "tabla_del_futuro", Op: "U", Datos: []byte(`{"id":"x"}`)}
	for range 2 { // dos veces: idempotente
		if err := st.Write(ctx, func(tx *store.Tx) error {
			for _, c := range []edgesync.Cambio{prod, grupo, desconocida} {
				if err := r.Aplicar(ctx, tx, c); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	var precio string
	var activo, orden, n int
	if err := st.Read().QueryRow(`SELECT precio, activo, orden, (SELECT count(*) FROM productos) FROM productos WHERE id='p1'`).Scan(&precio, &activo, &orden, &n); err != nil {
		t.Fatal(err)
	}
	if precio != "12.500000" || activo != 1 || orden != 3 || n != 1 {
		t.Fatalf("precio=%q activo=%d orden=%d filas=%d", precio, activo, orden, n)
	}
	// Actualizar y borrar (clave compuesta).
	if err := st.Write(ctx, func(tx *store.Tx) error {
		if err := r.Aplicar(ctx, tx, edgesync.Cambio{Tabla: "productos", Op: "U", Datos: []byte(`{"id":"p1","tenant_id":"t","categoria_id":"c","nombre":"Ceviche mixto","precio":13,"tarifa_iva_id":"iva","activo":false}`)}); err != nil {
			return err
		}
		return r.Aplicar(ctx, tx, edgesync.Cambio{Tabla: "producto_grupos_modificadores", Op: "D", Datos: grupo.Datos})
	}); err != nil {
		t.Fatal(err)
	}
	var nombre string
	_ = st.Read().QueryRow(`SELECT nombre, precio, activo, (SELECT count(*) FROM producto_grupos_modificadores) FROM productos`).Scan(&nombre, &precio, &activo, &n)
	if nombre != "Ceviche mixto" || precio != "13" || activo != 0 || n != 0 {
		t.Fatalf("tras actualizar: %q %q %d grupos=%d", nombre, precio, activo, n)
	}
	// Sin clave primaria: error claro, no se inserta basura.
	if err := st.Write(ctx, func(tx *store.Tx) error {
		return r.Aplicar(ctx, tx, edgesync.Cambio{Tabla: "productos", Op: "U", Datos: []byte(`{"nombre":"x"}`)})
	}); err == nil {
		t.Fatal("aceptó una fila sin id")
	}
}
