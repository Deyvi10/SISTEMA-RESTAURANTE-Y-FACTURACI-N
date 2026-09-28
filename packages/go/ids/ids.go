// Package ids genera los identificadores universales del sistema (ADR-0005).
//
// Todas las claves primarias son UUID v7 (RFC 9562): los primeros 48 bits son el
// instante en milisegundos, así los índices B-tree reciben inserciones casi ordenadas,
// y el resto es aleatorio, así la nube, el nodo y los teléfonos pueden crear registros
// sin conexión y sin colisiones.
//
// El UUID no es la «hora oficial» de nada: para eso existen los campos created_at.
package ids

import (
	"fmt"

	"github.com/google/uuid"
)

// ID es un UUID v7.
type ID = uuid.UUID

// Nil es el UUID vacío; nunca es un identificador válido de negocio.
var Nil = uuid.Nil

// New genera un UUID v7. Entra en pánico solo si el sistema no tiene fuente de
// aleatoriedad, situación en la que no es seguro seguir operando.
func New() ID {
	return uuid.Must(uuid.NewV7())
}

// Parse lee un UUID y exige que sea versión 7.
func Parse(s string) (ID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return Nil, fmt.Errorf("ids: %q no es un UUID: %w", s, err)
	}
	if id.Version() != 7 {
		return Nil, fmt.Errorf("ids: %q es UUID v%d, se esperaba v7", s, id.Version())
	}
	return id, nil
}

// MustParse es Parse para constantes del código: entra en pánico si no es un UUID v7.
func MustParse(s string) ID {
	id, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}
