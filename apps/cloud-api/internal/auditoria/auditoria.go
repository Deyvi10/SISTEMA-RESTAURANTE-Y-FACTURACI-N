// Package auditoria guarda en la nube la auditoría inmutable (RF-08-06): la cadena que llega
// de cada Nodo Local, verificada al recibirla, y la cadena propia de la nube por restaurante
// para las acciones del panel web (cambios de permisos, precios, usuarios…).
package auditoria

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/auditoria"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// EventoNodo es el evento con el que el nodo envía cada registro.
const EventoNodo = "auditoria.registrada"

func jsonb(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return []byte(b)
}

func nulo(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func insertar(ctx context.Context, tx pgx.Tx, r auditoria.Registro, tenant ids.ID, origen string, nodo *ids.ID, datos []byte, hashValido, cadenaValida bool) error {
	_, err := tx.Exec(ctx, `INSERT INTO auditoria (id, tenant_id, local_id, origen, nodo_id, seq, usuario_id, autorizado_por, dispositivo_id, accion, entidad, entidad_id,
		antes, despues, monto, motivo, detalle, created_at, hash_anterior, hash, datos, hash_valido, cadena_valida)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15::numeric, $16, $17, $18, $19, $20, $21, $22, $23) ON CONFLICT (id) DO NOTHING`,
		r.ID, tenant, r.LocalID, origen, nodo, r.Seq, r.UsuarioID, r.AutorizadoPor, r.DispositivoID, r.Accion, r.Entidad, nulo(r.EntidadID),
		jsonb(r.Antes), jsonb(r.Despues), nulo(r.Monto), nulo(r.Motivo), jsonb(r.Detalle), r.CreatedAt, r.HashAnterior, r.Hash, string(datos), hashValido, cadenaValida)
	return err
}

// RegistrarDelNodo guarda un registro de la cadena de un nodo dentro de la transacción del
// push. Verifica que el hash corresponda al contenido y que el eslabón anterior sea el último
// recibido de ese nodo; si algo no cuadra se guarda igual, marcado, para que el dueño lo vea.
func RegistrarDelNodo(ctx context.Context, tx pgx.Tx, tenant, nodo ids.ID, payload []byte) error {
	var r auditoria.Registro
	if err := json.Unmarshal(payload, &r); err != nil || r.ID == ids.Nil || r.Seq < 1 || r.Accion == "" || r.Entidad == "" {
		return nil // queda en la bitácora de eventos; no detiene la sincronización
	}
	var anterior string
	err := tx.QueryRow(ctx, `SELECT hash FROM auditoria WHERE nodo_id = $1 AND origen = 'NODO' ORDER BY seq DESC LIMIT 1`, nodo).Scan(&anterior)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return insertar(ctx, tx, r, tenant, "NODO", &nodo, payload, r.Integro() && r.TenantID == tenant, r.HashAnterior == anterior)
}

// Registrar agrega a la cadena de la nube del restaurante una acción hecha en el panel web.
// Se serializa por restaurante (bloqueo de la transacción) para que la cadena no se bifurque.
func Registrar(ctx context.Context, tx pgx.Tx, tenant ids.ID, r auditoria.Registro, now time.Time) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('auditoria:' || $1::text, 0))`, tenant); err != nil {
		return err
	}
	var anterior string
	err := tx.QueryRow(ctx, `SELECT seq, hash FROM auditoria WHERE tenant_id = $1 AND origen = 'NUBE' ORDER BY seq DESC LIMIT 1`, tenant).Scan(&r.Seq, &anterior)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	r.Seq++
	if r.ID == ids.Nil {
		r.ID = ids.New()
	}
	r.TenantID, r.CreatedAt = tenant, now
	r.Sellar(anterior)
	datos, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return insertar(ctx, tx, r, tenant, "NUBE", nil, datos, true, true)
}

// Cadena lee la cadena de un origen (un nodo, o la nube si nodo es nil) desde el texto con
// que se selló cada registro, para verificarla con auditoria.Verificar.
func Cadena(ctx context.Context, tx pgx.Tx, tenant ids.ID, nodo *ids.ID) ([]auditoria.Registro, error) {
	q := `SELECT datos FROM auditoria WHERE tenant_id = $1 AND origen = 'NUBE' ORDER BY seq`
	args := []any{tenant}
	if nodo != nil {
		q = `SELECT datos FROM auditoria WHERE tenant_id = $1 AND nodo_id = $2 ORDER BY seq`
		args = append(args, *nodo)
	}
	rows, err := tx.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (auditoria.Registro, error) {
		var datos string
		var r auditoria.Registro
		if err := row.Scan(&datos); err != nil {
			return r, err
		}
		return r, json.Unmarshal([]byte(datos), &r)
	})
}
