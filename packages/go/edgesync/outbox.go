package edgesync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// OutboxSchema crea la tabla del outbox en SQLite (docs/04 §9).
// AUTOINCREMENT garantiza que un seq nunca se reutiliza aunque se purguen filas.
const OutboxSchema = `
CREATE TABLE IF NOT EXISTS outbox (
  seq         INTEGER PRIMARY KEY AUTOINCREMENT,
  evento_id   TEXT    NOT NULL UNIQUE,
  tipo        TEXT    NOT NULL,
  version     INTEGER NOT NULL CHECK (version >= 1),
  agregado_id TEXT    NOT NULL,
  payload     TEXT    NOT NULL CHECK (json_valid(payload)),
  created_at  TEXT    NOT NULL,
  enviado_at  TEXT
);
CREATE INDEX IF NOT EXISTS outbox_pendientes ON outbox (seq) WHERE enviado_at IS NULL;
`

// Outbox es la bandeja de salida del nodo.
type Outbox struct {
	db *sql.DB
}

// NewOutbox usa una base SQLite ya abierta (ver OpenSQLite) y asegura el esquema.
func NewOutbox(ctx context.Context, db *sql.DB) (*Outbox, error) {
	if _, err := db.ExecContext(ctx, OutboxSchema); err != nil {
		return nil, fmt.Errorf("edgesync: crear outbox: %w", err)
	}
	return &Outbox{db: db}, nil
}

// NewEvent prepara un evento; el seq lo asigna Append.
func NewEvent(typ string, version int, aggregate ids.ID, payload any, now time.Time) (Event, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("edgesync: payload: %w", err)
	}
	return Event{ID: ids.New(), Type: typ, Version: version, AggregateID: aggregate, Payload: b, CreatedAt: now.UTC()}, nil
}

// Append guarda el evento DENTRO de la transacción del cambio de negocio y devuelve su seq.
// Si la transacción hace rollback, el evento tampoco existe.
func (o *Outbox) Append(ctx context.Context, tx *sql.Tx, e Event) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO outbox (evento_id, tipo, version, agregado_id, payload, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID.String(), e.Type, e.Version, e.AggregateID.String(), string(e.Payload), e.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("edgesync: append: %w", err)
	}
	return res.LastInsertId()
}

// Pending devuelve los eventos no confirmados más antiguos, respetando los límites de lote.
// Siempre incluye al menos un evento aunque supere maxBytes, para no atascarse.
func (o *Outbox) Pending(ctx context.Context, maxEvents, maxBytes int) ([]Event, error) {
	rows, err := o.db.QueryContext(ctx,
		`SELECT seq, evento_id, tipo, version, agregado_id, payload, created_at FROM outbox WHERE enviado_at IS NULL ORDER BY seq LIMIT ?`, maxEvents)
	if err != nil {
		return nil, fmt.Errorf("edgesync: pendientes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Event
	size := 0
	for rows.Next() {
		var e Event
		var id, agg, payload, created string
		if err := rows.Scan(&e.NodeSeq, &id, &e.Type, &e.Version, &agg, &payload, &created); err != nil {
			return nil, err
		}
		if size += len(payload) + 200; size > maxBytes && len(out) > 0 {
			break
		}
		if e.ID, err = ids.Parse(id); err != nil {
			return nil, err
		}
		if e.AggregateID, err = ids.Parse(agg); err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(payload)
		if e.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// MarkSent marca como confirmado todo lo que tenga seq ≤ upTo.
func (o *Outbox) MarkSent(ctx context.Context, upTo int64, now time.Time) error {
	_, err := o.db.ExecContext(ctx, `UPDATE outbox SET enviado_at = ? WHERE seq <= ? AND enviado_at IS NULL`, now.UTC().Format(time.RFC3339Nano), upTo)
	return err
}

// Stats sirve para la telemetría de salud (tamaño y edad del outbox, docs/03 §5.7).
type Stats struct {
	Pending      int
	OldestUnsent time.Time
}

func (o *Outbox) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	var oldest sql.NullString
	err := o.db.QueryRowContext(ctx, `SELECT COUNT(*), MIN(created_at) FROM outbox WHERE enviado_at IS NULL`).Scan(&s.Pending, &oldest)
	if err == nil && oldest.Valid {
		s.OldestUnsent, err = time.Parse(time.RFC3339Nano, oldest.String)
	}
	return s, err
}
