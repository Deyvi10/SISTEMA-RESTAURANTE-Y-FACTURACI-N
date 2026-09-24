// Package edgesync implementa la sincronización Nodo Local → Nube (ADR-0004, docs/03 §5):
//
//   - Outbox transaccional en SQLite: el evento se escribe en la MISMA transacción que
//     el cambio de negocio, así que si la venta existe, su evento también.
//   - Pusher: envía lotes en orden de node_seq y solo marca como enviado lo que la nube
//     confirma. Reintenta con espera exponencial y jitter.
//   - Receiver en PostgreSQL: aplica cada evento una sola vez y en orden, con un cursor
//     por nodo (último node_seq aplicado) actualizado en la misma transacción.
//
// Garantías: ningún evento se pierde (se reenvía hasta tener ACK), ninguno se aplica dos
// veces (seq ≤ cursor se ignora) y el orden por nodo se respeta (un hueco detiene el lote).
package edgesync

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Límites de un lote (docs/03 §5.5).
const (
	MaxBatchEvents = 500
	MaxBatchBytes  = 1 << 20
)

// Event es un cambio operativo generado en el nodo.
type Event struct {
	ID          ids.ID          `json:"id"`
	NodeSeq     int64           `json:"seq"`
	Type        string          `json:"type"`
	Version     int             `json:"version"`
	AggregateID ids.ID          `json:"aggregateId"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// PushRequest es el cuerpo de POST /v1/sync/push.
type PushRequest struct {
	NodeID ids.ID  `json:"nodeId"`
	Events []Event `json:"events"`
}

// PushResponse confirma hasta qué node_seq quedó aplicado en la nube.
// Si es menor que el último seq enviado, el nodo reenvía desde ahí.
type PushResponse struct {
	LastApplied int64 `json:"lastApplied"`
}

var ErrInvalidBatch = errors.New("edgesync: lote inválido")

// Validate revisa un lote antes de tocar la base de datos.
func (r PushRequest) Validate() error {
	if r.NodeID == ids.Nil {
		return fmt.Errorf("%w: falta nodeId", ErrInvalidBatch)
	}
	if len(r.Events) == 0 || len(r.Events) > MaxBatchEvents {
		return fmt.Errorf("%w: el lote debe tener entre 1 y %d eventos", ErrInvalidBatch, MaxBatchEvents)
	}
	prev := int64(0)
	for i, e := range r.Events {
		switch {
		case e.ID == ids.Nil || e.ID.Version() != 7:
			return fmt.Errorf("%w: evento %d sin UUID v7", ErrInvalidBatch, i)
		case e.NodeSeq <= prev:
			return fmt.Errorf("%w: node_seq debe ser positivo y creciente (evento %d)", ErrInvalidBatch, i)
		case e.Type == "" || e.Version < 1:
			return fmt.Errorf("%w: evento %d sin type o version", ErrInvalidBatch, i)
		case !json.Valid(e.Payload):
			return fmt.Errorf("%w: payload del evento %d no es JSON", ErrInvalidBatch, i)
		}
		prev = e.NodeSeq
	}
	return nil
}
