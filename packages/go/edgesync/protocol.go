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

// Heartbeat es la telemetría de salud que el nodo envía cada 60 s (F2-04, docs/03 §10).
// Sin datos personales: solo números y estados.
type Heartbeat struct {
	Version          string           `json:"version"`
	HoraNodo         time.Time        `json:"horaNodo"`
	ArranqueAt       time.Time        `json:"arranqueAt"`
	DiscoLibreMB     int64            `json:"discoLibreMb"`
	BaseMB           int64            `json:"baseMb"`
	OutboxPendientes int              `json:"outboxPendientes"`
	OutboxAntiguedad int64            `json:"outboxAntiguedadSeg"` // del evento más viejo sin confirmar
	Impresoras       []ImpresoraSalud `json:"impresoras"`
}

type ImpresoraSalud struct {
	ID     ids.ID `json:"id"`
	Estado string `json:"estado"` // OK, SIN_PAPEL, TAPA_ABIERTA, SIN_CONEXION…
	Cola   int    `json:"cola"`
}

// HeartbeatResponse devuelve la hora de la nube para medir la deriva del reloj del nodo.
type HeartbeatResponse struct {
	HoraNube       time.Time `json:"horaNube"`
	DerivaSegundos int64     `json:"derivaSegundos"` // hora del nodo − hora de la nube
	AlertaReloj    bool      `json:"alertaReloj"`
}

// MaxDerivaReloj: la fecha va dentro de la clave de acceso SRI, así que más de 60 s de
// diferencia se alerta (F2-04).
const MaxDerivaReloj = 60 * time.Second

// ---------- Nube → Nodo (F2-03) ----------

// Tablas que la nube replica al nodo, en el orden en que se aplica un volcado completo.
// Es el contrato entre el trigger de db/cloud (registrar_cambio) y la réplica del nodo.
var TablasReplica = []string{
	"locales", "estaciones", "zonas", "mesas", "categorias", "productos", "grupos_modificadores",
	"modificadores", "producto_grupos_modificadores", "notas_rapidas", "usuarios", "usuario_locales",
	"permisos_usuario", "tarifas_iva", "impresoras", "estacion_impresoras", "comandos_nodo",
}

// Cambio es una fila de la nube: op U = insertar o reemplazar, D = borrar.
type Cambio struct {
	Seq   int64           `json:"seq"`
	Tabla string          `json:"tabla"`
	Op    string          `json:"op"`
	Datos json.RawMessage `json:"datos"`
}

// Modos de respuesta del pull.
const (
	PullCompleto    = "COMPLETO"    // volcado de todo: el nodo reemplaza su réplica
	PullIncremental = "INCREMENTAL" // cambios desde el cursor
)

// PullResponse responde GET /v1/sync/pull?desde=N&esperar=S.
// Hasta es el cursor que el nodo guarda tras aplicar; Mas indica que hay más cambios.
type PullResponse struct {
	Modo    string   `json:"modo"`
	Hasta   int64    `json:"hasta"`
	Mas     bool     `json:"mas"`
	Cambios []Cambio `json:"cambios"`
}

// MaxPull es el máximo de cambios por respuesta incremental.
const MaxPull = 500
