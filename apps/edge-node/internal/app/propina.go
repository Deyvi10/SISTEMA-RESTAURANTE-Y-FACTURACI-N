package app

import (
	"context"
	"net/http"
	"strings"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
)

type PropinaIn struct {
	Retirar bool   `json:"retirar"`
	Motivo  string `json:"motivo"` // por qué la rechazó el cliente (opcional, queda en la auditoría)
}

// CambiarPropina quita (o repone) el 10 % de servicio de una orden abierta cuando el cliente
// lo rechaza (RF-04-08.3). Queda auditado con el usuario, el monto y el motivo.
func (a *App) CambiarPropina(ctx context.Context, u Usuario, orden ids.ID, in PropinaIn) (Totales, error) {
	if !u.Puede(rbac.Cobrar) {
		return Totales{}, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para quitar el servicio.")
	}
	in.Motivo = recortar(strings.Join(strings.Fields(in.Motivo), " "), 200)
	now := a.Clock.Now()
	var tot Totales
	var mesa *ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		o, err := ordenAbierta(ctx, tx, orden)
		if err != nil {
			return err
		}
		if err := bloqueoPorCuentas(ctx, tx, orden); err != nil {
			return err
		}
		antes, _, _, _, _, err := a.calcularTotales(ctx, tx, o)
		if err != nil {
			return err
		}
		if !antes.PropinaActiva {
			return problema(http.StatusConflict, "SIN_SERVICIO", "Este local no cobra servicio.")
		}
		if o.PropinaRetirada == in.Retirar {
			tot = antes
			return nil // ya estaba así: no se audita dos veces
		}
		var motivo any
		if in.Retirar && in.Motivo != "" {
			motivo = in.Motivo
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET propina_retirada = ?, propina_motivo = ?, version = version + 1 WHERE id = ?`, in.Retirar, motivo, orden.String()); err != nil {
			return err
		}
		o.PropinaRetirada = in.Retirar
		if tot, _, _, _, _, err = a.calcularTotales(ctx, tx, o); err != nil {
			return err
		}
		accion, monto := "PROPINA_REPUESTA", tot.Propina
		if in.Retirar {
			accion, monto = "PROPINA_RETIRADA", antes.Propina
		}
		uid := u.ID
		mesa = o.MesaID
		return auditar(ctx, tx, accion, "orden", orden, &uid, map[string]any{"monto": monto, "motivo": in.Motivo, "mesero": o.MeseroNombre}, now)
	})
	if err == nil && mesa != nil {
		a.avisarMesa(ctx, *mesa) // el total de la mesa cambió
	}
	return tot, err
}
