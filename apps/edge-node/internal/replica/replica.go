// Package replica aplica en SQLite los cambios que la nube envía al nodo (F2-03):
// catálogo, salón, personal y configuración del local. Es idempotente: aplicar dos veces
// el mismo cambio deja la base igual (UPSERT por clave primaria o DELETE).
package replica

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
)

type tabla struct {
	cols map[string]bool
	pk   []string
}

// Replica conoce las columnas de cada tabla local (se leen del esquema al arrancar).
type Replica struct {
	tablas map[string]tabla
}

// Cargar lee el esquema de las tablas replicadas desde SQLite.
func Cargar(ctx context.Context, db *sql.DB) (*Replica, error) {
	r := &Replica{tablas: map[string]tabla{}}
	for _, nombre := range edgesync.TablasReplica {
		rows, err := db.QueryContext(ctx, `SELECT name, pk FROM pragma_table_info(?) ORDER BY pk`, nombre)
		if err != nil {
			return nil, err
		}
		t := tabla{cols: map[string]bool{}}
		for rows.Next() {
			var col string
			var pk int
			if err := rows.Scan(&col, &pk); err != nil {
				_ = rows.Close()
				return nil, err
			}
			t.cols[col] = true
			if pk > 0 {
				t.pk = append(t.pk, col)
			}
		}
		_ = rows.Close()
		if len(t.cols) == 0 || len(t.pk) == 0 {
			return nil, fmt.Errorf("replica: la tabla %s no existe o no tiene clave primaria", nombre)
		}
		r.tablas[nombre] = t
	}
	return r, nil
}

// Conoce indica si la tabla se replica en este nodo (una nube más nueva puede enviar
// tablas que esta versión todavía no usa: se ignoran).
func (r *Replica) Conoce(nombre string) bool { _, ok := r.tablas[nombre]; return ok }

// Vaciar borra toda la réplica (antes de un volcado completo). Nunca toca lo operativo.
func (r *Replica) Vaciar(ctx context.Context, tx *sql.Tx) error {
	for _, nombre := range edgesync.TablasReplica {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+nombre); err != nil { //nolint:gosec // nombre de una lista fija
			return err
		}
	}
	return nil
}

// Aplicar ejecuta un cambio dentro de la transacción del lote.
func (r *Replica) Aplicar(ctx context.Context, tx *sql.Tx, c edgesync.Cambio) error {
	t, ok := r.tablas[c.Tabla]
	if !ok {
		return nil
	}
	fila, err := decodificar(c.Datos)
	if err != nil {
		return fmt.Errorf("replica: %s: %w", c.Tabla, err)
	}
	for _, k := range t.pk {
		if fila[k] == nil {
			return fmt.Errorf("replica: %s sin clave %s", c.Tabla, k)
		}
	}
	switch c.Op {
	case "D":
		cond := make([]string, len(t.pk))
		args := make([]any, len(t.pk))
		for i, k := range t.pk {
			cond[i], args[i] = k+" = ?", fila[k]
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM `+c.Tabla+` WHERE `+strings.Join(cond, " AND "), args...) //nolint:gosec // identificadores del esquema local
	case "U":
		var cols, marcas, sets []string
		var args []any
		for col, v := range fila {
			if !t.cols[col] {
				continue // columna que este nodo no conoce
			}
			cols, marcas, args = append(cols, col), append(marcas, "?"), append(args, v)
			if !slices.Contains(t.pk, col) {
				sets = append(sets, col+" = excluded."+col)
			}
		}
		q := `INSERT INTO ` + c.Tabla + ` (` + strings.Join(cols, ", ") + `) VALUES (` + strings.Join(marcas, ", ") + `) ON CONFLICT (` + strings.Join(t.pk, ", ") + `) DO `
		if len(sets) == 0 {
			q += "NOTHING"
		} else {
			q += "UPDATE SET " + strings.Join(sets, ", ")
		}
		_, err = tx.ExecContext(ctx, q, args...)
	default:
		return fmt.Errorf("replica: operación desconocida %q", c.Op)
	}
	if err != nil {
		return fmt.Errorf("replica: %s %s: %w", c.Op, c.Tabla, err)
	}
	return nil
}

// decodificar convierte la fila JSON a valores SQLite sin pasar por float: los números
// quedan como texto exacto ("12.500000") y los booleanos como 0/1.
func decodificar(raw json.RawMessage) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	for k, v := range m {
		switch x := v.(type) {
		case json.Number:
			m[k] = x.String()
		case bool:
			m[k] = map[bool]int{true: 1, false: 0}[x]
		case map[string]any, []any:
			b, err := json.Marshal(x)
			if err != nil {
				return nil, err
			}
			m[k] = string(b)
		}
	}
	return m, nil
}
