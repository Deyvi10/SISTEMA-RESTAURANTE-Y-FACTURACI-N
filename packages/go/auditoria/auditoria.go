// Package auditoria es el registro inmutable de acciones sensibles (RF-08-06): cada registro
// lleva el hash del anterior, así un registro borrado, alterado o insertado a destiempo
// rompe la cadena. Lo escriben el Nodo Local (su propia cadena) y la nube (una cadena por
// restaurante para lo que ocurre en el panel web); la nube verifica las cadenas del nodo.
package auditoria

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Registro contiene lo que exige RF-08-06.2.
type Registro struct {
	ID            ids.ID          `json:"id"`
	Seq           int64           `json:"seq"` // posición en la cadena de su origen
	TenantID      ids.ID          `json:"tenantId"`
	LocalID       *ids.ID         `json:"localId,omitempty"`
	UsuarioID     *ids.ID         `json:"usuarioId,omitempty"`
	AutorizadoPor *ids.ID         `json:"autorizadoPor,omitempty"` // supervisor que dio su PIN
	DispositivoID *ids.ID         `json:"dispositivoId,omitempty"`
	Accion        string          `json:"accion"`
	Entidad       string          `json:"entidad"`
	EntidadID     string          `json:"entidadId,omitempty"`
	Antes         json.RawMessage `json:"antes,omitempty"`
	Despues       json.RawMessage `json:"despues,omitempty"`
	Monto         string          `json:"monto,omitempty"` // importe implicado, decimal exacto
	Motivo        string          `json:"motivo,omitempty"`
	Detalle       json.RawMessage `json:"detalle,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	HashAnterior  string          `json:"hashAnterior"`
	Hash          string          `json:"hash"`
}

// Compactar deja un JSON en la forma que produce encoding/json, para que el hash sea el mismo
// después de viajar (vacío o nulo = sin valor).
func Compactar(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" || string(b) == "{}" {
		return nil
	}
	return b
}

// CalcularHash: SHA-256 del registro en JSON sin su propio hash (incluye el anterior).
func (r Registro) CalcularHash() string {
	r.Hash = ""
	r.CreatedAt = r.CreatedAt.UTC()
	b, err := json.Marshal(r)
	if err != nil {
		panic(err) // solo tipos serializables
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// Sellar encadena el registro con el hash del anterior y calcula el suyo.
func (r *Registro) Sellar(anterior string) {
	r.HashAnterior = anterior
	r.CreatedAt = r.CreatedAt.UTC()
	r.Hash = r.CalcularHash()
}

// Integro indica si el hash corresponde al contenido.
func (r Registro) Integro() bool { return r.Hash != "" && r.Hash == r.CalcularHash() }

// Verificar recorre una cadena en orden y devuelve la posición del primer registro alterado
// o fuera de lugar, o -1 si está completa. Los registros sin hash son anteriores a la cadena.
func Verificar(rs []Registro) int {
	anterior := ""
	empezo := false
	for i, r := range rs {
		if r.Hash == "" && !empezo {
			continue
		}
		empezo = true
		if !r.Integro() || r.HashAnterior != anterior {
			return i
		}
		anterior = r.Hash
	}
	return -1
}
