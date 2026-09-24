package edgesync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// ReceiverSchema crea las tablas de sincronización en PostgreSQL.
// En la Fase 2 (F2-04) pasan a db/cloud/migrations con tenant_id y RLS.
const ReceiverSchema = `
CREATE TABLE IF NOT EXISTS sync_cursores (
  nodo_id     uuid PRIMARY KEY,
  ultimo_seq  bigint NOT NULL DEFAULT 0 CHECK (ultimo_seq >= 0),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS sync_eventos (
  evento_id   uuid PRIMARY KEY,
  nodo_id     uuid   NOT NULL,
  node_seq    bigint NOT NULL,
  tipo        text   NOT NULL,
  version     int    NOT NULL,
  agregado_id uuid   NOT NULL,
  payload     jsonb  NOT NULL,
  orden       bigserial,
  recibido_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (nodo_id, node_seq)
);
`

// Applier aplica un evento a las tablas de negocio dentro de la transacción del lote.
// Debe ser idempotente por evento (p. ej. UPSERT por id).
type Applier func(ctx context.Context, tx pgx.Tx, nodeID ids.ID, e Event) error

// StoreEvent es el Applier base: guarda el evento crudo (bitácora de sincronización).
func StoreEvent(ctx context.Context, tx pgx.Tx, nodeID ids.ID, e Event) error {
	_, err := tx.Exec(ctx, `INSERT INTO sync_eventos (evento_id, nodo_id, node_seq, tipo, version, agregado_id, payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (evento_id) DO NOTHING`,
		e.ID, nodeID, e.NodeSeq, e.Type, e.Version, e.AggregateID, []byte(e.Payload))
	return err
}

// Receiver aplica lotes de un nodo en orden y exactamente una vez.
type Receiver struct {
	Pool  *pgxpool.Pool
	Apply Applier
	Log   *slog.Logger
}

// Push aplica un lote validado y devuelve el último seq aplicado.
//
// Reglas: seq ≤ cursor → duplicado, se ignora (idempotencia). seq = cursor+1 → se aplica.
// seq > cursor+1 → hay un hueco: se detiene y se responde el cursor para que el nodo
// reenvíe desde ahí (orden). Todo en una transacción con el cursor bloqueado FOR UPDATE,
// así dos envíos simultáneos del mismo nodo no pueden intercalarse.
func (r *Receiver) Push(ctx context.Context, req PushRequest) (PushResponse, error) {
	if err := req.Validate(); err != nil {
		return PushResponse{}, err
	}
	apply := r.Apply
	if apply == nil {
		apply = StoreEvent
	}
	var last int64
	err := pgx.BeginFunc(ctx, r.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO sync_cursores (nodo_id) VALUES ($1) ON CONFLICT DO NOTHING`, req.NodeID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT ultimo_seq FROM sync_cursores WHERE nodo_id = $1 FOR UPDATE`, req.NodeID).Scan(&last); err != nil {
			return err
		}
		start := last
		for _, e := range req.Events {
			if e.NodeSeq <= last {
				continue
			}
			if e.NodeSeq != last+1 {
				break
			}
			if err := apply(ctx, tx, req.NodeID, e); err != nil {
				return fmt.Errorf("aplicar evento seq %d (%s): %w", e.NodeSeq, e.Type, err)
			}
			last = e.NodeSeq
		}
		if last == start {
			return nil
		}
		_, err := tx.Exec(ctx, `UPDATE sync_cursores SET ultimo_seq = $2, updated_at = now() WHERE nodo_id = $1`, req.NodeID, last)
		return err
	})
	return PushResponse{LastApplied: last}, err
}

// Handler expone POST /v1/sync/push. La identidad del nodo (mTLS o firma ed25519, F2-02)
// y el tenant se validan en un middleware anterior.
func (r *Receiver) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body PushRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 2*MaxBatchBytes)).Decode(&body); err != nil {
			writeProblem(w, http.StatusBadRequest, "El lote no es JSON válido o supera 2 MB.")
			return
		}
		res, err := r.Push(req.Context(), body)
		switch {
		case errors.Is(err, ErrInvalidBatch):
			writeProblem(w, http.StatusUnprocessableEntity, err.Error())
		case err != nil:
			r.log().Error("sync: error al aplicar lote", "nodo", body.NodeID, "err", err)
			writeProblem(w, http.StatusInternalServerError, "No se pudo aplicar el lote; el nodo lo reenviará.")
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(res)
		}
	})
}

func (r *Receiver) log() *slog.Logger {
	if r.Log == nil {
		return slog.Default()
	}
	return r.Log
}

func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"title": http.StatusText(status), "status": status, "detail": detail})
}
