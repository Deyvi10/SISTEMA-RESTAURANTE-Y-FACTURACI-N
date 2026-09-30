package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// PuntoEmision es la serie de una caja (F5-02).
type PuntoEmision struct {
	ID              ids.ID
	Establecimiento string // "001"
	Punto           string // "002"
}

// Serie es «001002», como va en la clave de acceso.
func (p PuntoEmision) Serie() string { return p.Establecimiento + p.Punto }

var errSinPunto = problema(http.StatusConflict, "CAJA_SIN_PUNTO_EMISION", "Esta caja no tiene punto de emisión. Asígnalo en el panel, en «Facturación SRI».")

// puntoDeCaja devuelve el punto de emisión de la caja si este nodo es su dueño.
func puntoDeCaja(ctx context.Context, tx *store.Tx, caja ids.ID) (PuntoEmision, error) {
	var p PuntoEmision
	var id, dueno sql.NullString
	var yo string
	if err := tx.QueryRowContext(ctx, `SELECT nodo_id FROM nodo WHERE id = 1`).Scan(&yo); err != nil {
		return p, err
	}
	err := tx.QueryRowContext(ctx, `SELECT p.id, p.codigo_establecimiento, p.codigo_punto, p.nodo_id FROM cajas c
		JOIN puntos_emision p ON p.id = c.punto_emision_id AND p.deleted_at IS NULL
		WHERE c.id = ?`, caja.String()).Scan(&id, &p.Establecimiento, &p.Punto, &dueno)
	if errors.Is(err, sql.ErrNoRows) {
		return p, errSinPunto
	}
	if err != nil {
		return p, err
	}
	// Un único dueño por punto: si la nube se lo dio a otro nodo, este no numera (docs/04 §7).
	if !dueno.Valid || dueno.String != yo {
		return p, problema(http.StatusConflict, "PUNTO_DE_OTRO_NODO", "El punto de emisión de esta caja pertenece a otro equipo. Revisa «Facturación SRI» en el panel.")
	}
	p.ID, _ = ids.Parse(id.String)
	return p, nil
}

// siguienteSecuencial toma el número siguiente del punto, tipo y ambiente dentro de la
// transacción de quien llama (la del cobro): si esa transacción falla, el número no se consume.
// Parte del mayor entre lo que ya numeró este nodo y lo último que conoce la nube (un nodo que
// reemplaza a otro no repite números).
func siguienteSecuencial(ctx context.Context, tx *store.Tx, punto ids.ID, tipo string, ambiente int) (int64, error) {
	var raw string
	if err := tx.QueryRowContext(ctx, `SELECT ultimos_secuenciales FROM puntos_emision WHERE id = ?`, punto.String()).Scan(&raw); err != nil {
		return 0, err
	}
	nube := map[string]int64{}
	_ = json.Unmarshal([]byte(raw), &nube) // mal formado: se ignora, manda el contador local
	base := nube[fmt.Sprintf("%s-%d", tipo, ambiente)]
	var n int64
	err := tx.QueryRowContext(ctx, `INSERT INTO secuenciales (punto_emision_id, tipo_comprobante, ambiente, ultimo) VALUES (?, ?, ?, ? + 1)
		ON CONFLICT (punto_emision_id, tipo_comprobante, ambiente) DO UPDATE SET ultimo = max(ultimo, ?) + 1
		RETURNING ultimo`, punto.String(), tipo, ambiente, base, base).Scan(&n)
	if err != nil {
		// El CHECK de la tabla corta en 999 999 999: la serie se agotó.
		return 0, fmt.Errorf("secuencial del punto %s: %w", punto, err)
	}
	return n, nil
}
