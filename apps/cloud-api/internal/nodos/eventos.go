package nodos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Tipos de eventos Nodo → Nube que la nube aplica además de guardarlos en la bitácora.
const (
	EventoComandoEjecutado   = "comando.ejecutado"
	EventoImpresoraDetectada = "impresora.detectada"
)

// ComandoEjecutado informa el resultado de una orden de la nube (p. ej. imprimir prueba).
type ComandoEjecutado struct {
	ComandoID ids.ID `json:"comandoId"`
	OK        bool   `json:"ok"`
	Resultado string `json:"resultado"`
}

// ImpresoraDetectada es el hardware que el nodo encontró en la red o por USB (F2-08).
// El nodo genera el id; si la impresora ya existía (misma MAC o misma IP:puerto) la nube
// actualiza sus datos de red en lugar de duplicarla: así una IP que cambió por DHCP se
// reubica sola (RF-02-02.3).
type ImpresoraDetectada struct {
	ID       ids.ID  `json:"id"`
	Conexion string  `json:"conexion"`
	Host     *string `json:"host"`
	Puerto   *int    `json:"puerto"`
	MAC      *string `json:"mac"`
	UsbID    *string `json:"usbId"`
	Modelo   string  `json:"modelo"`
}

var macRe = regexp.MustCompile(`^[0-9a-f]{2}(:[0-9a-f]{2}){5}$`)

// Aplicador guarda cada evento del nodo y aplica los que la nube entiende. Idempotente:
// un evento repetido no cambia nada (el receptor además descarta seq ya aplicados).
func (s *Service) Aplicador(n auth.Nodo) edgesync.Applier {
	return func(ctx context.Context, tx pgx.Tx, nodeID ids.ID, e edgesync.Event) error {
		if err := edgesync.StoreEvent(ctx, tx, nodeID, e); err != nil {
			return err
		}
		switch e.Type {
		case EventoComandoEjecutado:
			var c ComandoEjecutado
			if err := json.Unmarshal(e.Payload, &c); err != nil {
				return nil // payload inválido: queda en la bitácora, no detiene la sync
			}
			res := strings.TrimSpace(c.Resultado)
			if len([]rune(res)) > 300 {
				res = string([]rune(res)[:300])
			}
			if !c.OK && res == "" {
				res = "Falló"
			}
			_, err := tx.Exec(ctx, `UPDATE comandos_nodo SET ejecutado_at = $3, resultado = $4 WHERE id = $1 AND local_id = $2 AND ejecutado_at IS NULL`,
				c.ComandoID, n.LocalID, e.CreatedAt, map[bool]string{true: "OK: ", false: "ERROR: "}[c.OK]+res)
			return err
		case EventoImpresoraDetectada:
			var d ImpresoraDetectada
			if err := json.Unmarshal(e.Payload, &d); err != nil || d.ID == ids.Nil {
				return nil
			}
			return s.registrarDetectada(ctx, tx, n, d)
		}
		return nil
	}
}

func (s *Service) registrarDetectada(ctx context.Context, tx pgx.Tx, n auth.Nodo, d ImpresoraDetectada) error {
	if d.MAC != nil {
		m := strings.ToLower(*d.MAC)
		if !macRe.MatchString(m) {
			d.MAC = nil
		} else {
			d.MAC = &m
		}
	}
	switch d.Conexion {
	case "TCP":
		if d.Host == nil || d.Puerto == nil {
			return nil
		}
	case "USB":
		if d.UsbID == nil {
			return nil
		}
	default:
		return nil
	}
	modelo := d.Modelo
	if len([]rune(modelo)) > 80 {
		modelo = string([]rune(modelo)[:80])
	}
	// ¿Ya existe? por id, por MAC o por IP:puerto (en ese orden).
	var existente ids.ID
	err := tx.QueryRow(ctx, `SELECT id FROM impresoras WHERE local_id = $1 AND deleted_at IS NULL AND (
			id = $2 OR ($3::text IS NOT NULL AND mac = $3) OR (conexion = 'TCP' AND host = $4 AND puerto = $5) OR (conexion = 'USB' AND usb_id = $6))
		ORDER BY (id = $2) DESC, (mac IS NOT DISTINCT FROM $3) DESC LIMIT 1`,
		n.LocalID, d.ID, d.MAC, d.Host, d.Puerto, d.UsbID).Scan(&existente)
	switch {
	case err == nil:
		// Reubicar (IP nueva por DHCP) sin tocar lo que decidió el dueño.
		_, err = tx.Exec(ctx, `UPDATE impresoras SET host = coalesce($2, host), puerto = coalesce($3, puerto), mac = coalesce($4, mac),
				usb_id = coalesce($5, usb_id), modelo = CASE WHEN $6 = '' THEN modelo ELSE $6 END
			WHERE id = $1 AND (host IS DISTINCT FROM coalesce($2, host) OR puerto IS DISTINCT FROM coalesce($3, puerto)
				OR mac IS DISTINCT FROM coalesce($4, mac) OR usb_id IS DISTINCT FROM coalesce($5, usb_id) OR ($6 <> '' AND modelo <> $6))`,
			existente, d.Host, d.Puerto, d.MAC, d.UsbID, modelo)
		return err
	case !errors.Is(err, pgx.ErrNoRows):
		return err
	}
	var n2 int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM impresoras WHERE local_id = $1 AND deleted_at IS NULL`, n.LocalID).Scan(&n2); err != nil {
		return err
	}
	for i := n2 + 1; ; i++ {
		nombre := fmt.Sprintf("Impresora %d", i)
		tag, err := tx.Exec(ctx, `INSERT INTO impresoras (id, tenant_id, local_id, nombre, conexion, host, puerto, mac, usb_id, modelo, origen)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'DETECTADA')
			ON CONFLICT DO NOTHING`, d.ID, n.TenantID, n.LocalID, nombre, d.Conexion, d.Host, d.Puerto, d.MAC, d.UsbID, modelo)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 || i > n2+50 {
			return nil
		}
	}
}
