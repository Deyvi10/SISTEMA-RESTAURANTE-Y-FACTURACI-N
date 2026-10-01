package facturacion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// EventoComprobanteEmitido es el comprobante que el nodo emitió al cobrar (F5-05).
const EventoComprobanteEmitido = "comprobante.emitido"

// ComprobanteEmitido es el payload del evento: el XML sin firmar y su hash.
type ComprobanteEmitido struct {
	ID             ids.ID `json:"id"`
	DocumentoID    ids.ID `json:"documentoId"`
	Tipo           string `json:"tipo"`
	Ambiente       int    `json:"ambiente"`
	PuntoEmisionID ids.ID `json:"puntoEmisionId"`
	Serie          string `json:"serie"`
	Secuencial     int64  `json:"secuencial"`
	ClaveAcceso    string `json:"claveAcceso"`
	FechaEmision   string `json:"fechaEmision"`
	ImporteTotal   string `json:"importeTotal"`
	XML            string `json:"xml"`
	Hash           string `json:"hash"`
	// Nota de crédito (F5-13): la factura que modifica y si revierte todo lo que quedaba.
	SustentoID    *ids.ID `json:"sustentoId,omitempty"`
	SustentoClave string  `json:"sustentoClave,omitempty"`
	Total         bool    `json:"total,omitempty"`
}

// RegistrarComprobante guarda en la nube un comprobante del nodo (idempotente por clave de
// acceso). Si el hash no corresponde al XML queda en REQUIERE_ATENCION: nunca se firma ni se
// envía un contenido alterado. Actualiza el último secuencial conocido del punto, del que
// sigue un nodo que reemplaza a otro (F5-02). Un payload ilegible queda solo en la bitácora.
func RegistrarComprobante(ctx context.Context, tx pgx.Tx, tenant, local, nodo ids.ID, payload []byte) error {
	var c ComprobanteEmitido
	if err := json.Unmarshal(payload, &c); err != nil || c.ID == ids.Nil {
		return nil
	}
	clave, err := sri.ParseClaveAcceso(c.ClaveAcceso)
	if err != nil || clave.TipoComprobante() != c.Tipo || clave.Serie() != c.Serie || int(clave.Ambiente()) != c.Ambiente ||
		clave.Secuencial() != fmt.Sprintf("%09d", c.Secuencial) {
		return nil // clave incoherente: no es un comprobante que se pueda enviar
	}
	total, err := money.Parse(c.ImporteTotal)
	if err != nil {
		return nil
	}
	// Una nota de crédito llega con su factura, que el nodo subió antes (mismo outbox, en orden).
	if (c.Tipo == sri.TipoNotaCredito) != (c.SustentoID != nil) {
		return nil
	}
	if c.SustentoID != nil {
		var clave string
		if err := tx.QueryRow(ctx, `SELECT clave_acceso FROM comprobantes WHERE id = $1 AND tenant_id = $2`, *c.SustentoID, tenant).Scan(&clave); err != nil || clave != c.SustentoClave {
			return nil // sin su factura no se puede enviar: queda en la bitácora de sync_eventos
		}
	}
	suma := sha256.Sum256([]byte(c.XML))
	valido := hex.EncodeToString(suma[:]) == c.Hash
	estado := "EN_NUBE"
	if !valido {
		estado = "REQUIERE_ATENCION"
	}
	var documento *ids.ID
	if c.Tipo == sri.TipoFactura {
		documento = &c.DocumentoID
	}
	// El comprador, para la bóveda y el correo (el nodo pone el correo en «Email», F5-05).
	var idComprador, nombre, correo *string
	if f, err := sri.LeerComprobante([]byte(c.XML)); err == nil && valido {
		idComprador, nombre = &f.IdComprador, &f.RazonSocialComprador
		if e := f.Adicional("Email"); e != "" {
			correo = &e
		}
	}
	tag, err := tx.Exec(ctx, `INSERT INTO comprobantes (id, tenant_id, local_id, nodo_id, punto_emision_id, documento_id, tipo, ambiente, serie,
			secuencial, clave_acceso, fecha_emision, importe_total, xml, hash, hash_valido, estado, proximo_intento_at,
			comprador_identificacion, comprador_nombre, correo_comprador, sustento_id, revierte_todo)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, now(), $18, $19, $20, $21, $22)
		ON CONFLICT DO NOTHING`,
		c.ID, tenant, local, nodo, c.PuntoEmisionID, documento, c.Tipo, c.Ambiente, c.Serie, c.Secuencial, c.ClaveAcceso,
		c.FechaEmision, total.Decimal(), c.XML, c.Hash, valido, estado, idComprador, nombre, correo, c.SustentoID, c.Total)
	// Repetido (el nodo reintenta) o una serie+secuencial que ya existe con otra clave: no se
	// duplica ni se detiene la sincronización; el evento crudo queda en sync_eventos.
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	detalle := map[string]any{"nodo": nodo}
	if !valido {
		detalle["motivo"] = "el hash no corresponde al XML recibido"
	}
	d, _ := json.Marshal(detalle)
	if _, err := tx.Exec(ctx, `INSERT INTO comprobante_eventos (id, tenant_id, comprobante_id, estado, detalle) VALUES ($1, $2, $3, $4, $5)`,
		ids.New(), tenant, c.ID, estado, d); err != nil {
		return err
	}
	clavePunto := c.Tipo + "-" + strconv.Itoa(c.Ambiente)
	_, err = tx.Exec(ctx, `UPDATE puntos_emision SET ultimos_secuenciales = jsonb_set(ultimos_secuenciales, ARRAY[$2::text],
			to_jsonb(GREATEST(coalesce((ultimos_secuenciales->>$2)::bigint, 0), $3::bigint))), updated_at = now(), version = version + 1
		WHERE id = $1 AND coalesce((ultimos_secuenciales->>$2)::bigint, 0) < $3`, c.PuntoEmisionID, clavePunto, c.Secuencial)
	return err
}
