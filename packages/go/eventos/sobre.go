// Package eventos define los mensajes de tiempo real entre el Nodo Local y sus clientes
// (caja, KDS, app de meseros). Los tipos de cada evento se generan desde contracts/events
// (eventos.gen.go); aquí va el sobre común.
package eventos

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Sobre es el formato de todo mensaje WebSocket: {v, id, type, ts, data} (docs/03 §6).
type Sobre struct {
	V    int             `json:"v"`
	ID   ids.ID          `json:"id"`
	Type string          `json:"type"`
	TS   time.Time       `json:"ts"`
	Data json.RawMessage `json:"data"`
}

// Evento es cualquier tipo generado desde los contratos.
type Evento interface{ Tipo() string }

// Nuevo envuelve un evento con su versión vigente y un id v7.
func Nuevo(e Evento, now time.Time) (Sobre, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return Sobre{}, err
	}
	v, ok := Versiones[e.Tipo()]
	if !ok {
		return Sobre{}, fmt.Errorf("eventos: tipo desconocido %q", e.Tipo())
	}
	return Sobre{V: v, ID: ids.New(), Type: e.Tipo(), TS: now, Data: data}, nil
}

// Abrir decodifica los datos del sobre en el tipo esperado, verificando el tipo.
func Abrir[T Evento](s Sobre) (T, error) {
	var out T
	if s.Type != out.Tipo() {
		return out, fmt.Errorf("eventos: se esperaba %s y llegó %s", out.Tipo(), s.Type)
	}
	err := json.Unmarshal(s.Data, &out)
	return out, err
}
