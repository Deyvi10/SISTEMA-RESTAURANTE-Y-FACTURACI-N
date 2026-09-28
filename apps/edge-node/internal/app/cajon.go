package app

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
)

// EventoCajonAbierto informa a la nube cada apertura sin venta (el dueño la ve en la
// auditoría y en la matriz de fugas de F8-05).
const EventoCajonAbierto = "caja.cajon_abierto"

type AbrirCajonIn struct {
	CajaID       ids.ID `json:"cajaId"`
	Motivo       string `json:"motivo"`
	Autorizacion string `json:"autorizacion"` // token del PIN de supervisor si el usuario no tiene el permiso
}

type CajonOut struct {
	Impresora     string `json:"impresora"`
	AutorizadoPor string `json:"autorizadoPor,omitempty"`
	Aviso         string `json:"aviso,omitempty"`
}

// AbrirCajon abre el cajón sin una venta (RF-02-07): exige el permiso ABRIR_CAJON o la
// autorización de un supervisor para esta caja, un motivo, y queda auditado. El pulso sale
// por la impresora de la caja junto con un comprobante de la apertura.
func (a *App) AbrirCajon(ctx context.Context, d Dispositivo, u Usuario, in AbrirCajonIn) (CajonOut, error) {
	var out CajonOut
	in.Motivo = strings.Join(strings.Fields(in.Motivo), " ")
	if n := len([]rune(in.Motivo)); n < 3 || n > 200 {
		return out, invalido("Escribe por qué abres el cajón (de 3 a 200 caracteres).")
	}
	now, loc := a.Clock.Now(), a.zonaLocal(ctx)
	var despertar ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		var autoriza *ids.ID
		if !u.Puede(rbac.AbrirCajon) {
			if in.Autorizacion == "" {
				return problema(http.StatusForbidden, "REQUIERE_SUPERVISOR", "Abrir el cajón sin una venta requiere el PIN de un supervisor.")
			}
			por, err := consumirAutorizacion(ctx, tx, in.Autorizacion, d, rbac.AbrirCajon, in.CajaID.String(), now)
			if err != nil {
				return err
			}
			autoriza = &por
			_ = tx.QueryRowContext(ctx, `SELECT nombre_mostrar FROM usuarios WHERE id = ?`, por.String()).Scan(&out.AutorizadoPor)
		}
		caja, err := cajaExiste(ctx, tx, in.CajaID)
		if err != nil {
			return err
		}
		var estacion sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT estacion_id FROM cajas WHERE id = ?`, in.CajaID.String()).Scan(&estacion); err != nil {
			return err
		}
		var est *ids.ID
		if e, err := ids.Parse(estacion.String); err == nil {
			est = &e
		}
		imps, respaldo, err := a.impresorasDeCaja(ctx, tx, est)
		if err != nil {
			return err
		}
		if len(imps) == 0 {
			return problema(http.StatusConflict, "SIN_IMPRESORA", "El cajón se abre por la impresora de la caja y no hay ninguna configurada.")
		}
		if respaldo != "" {
			out.Aviso = "La caja no tiene impresora propia: el pulso salió por «" + respaldo + "». Revisa a cuál está conectado el cajón."
		}
		imp := imps[0] // el cajón cuelga de una sola impresora
		out.Impresora = imp.Nombre
		var local string
		_ = tx.QueryRowContext(ctx, `SELECT nombre FROM locales LIMIT 1`).Scan(&local)
		ticket := escpos.ImprimirAperturaCajon(imp.Ancho, escpos.AperturaCajon{Local: local, Caja: caja, Usuario: u.Nombre, AutorizadoPor: out.AutorizadoPor, Motivo: in.Motivo, Hora: now.In(loc)})
		if err := impresion.Encolar(ctx, tx, impresion.Trabajo{ID: ids.New(), ImpresoraID: imp.ID, Tipo: "CAJON"}, nil, ticket, nil, now); err != nil {
			return err
		}
		despertar = imp.ID
		// El turno abierto, si lo hay, para cruzar la apertura con el cierre.
		var turno sql.NullString
		_ = tx.QueryRowContext(ctx, `SELECT id FROM turnos_caja WHERE caja_id = ? AND estado = 'ABIERTO'`, in.CajaID.String()).Scan(&turno)
		uid := u.ID
		detalle := map[string]any{"caja": in.CajaID, "motivo": in.Motivo, "autorizadoPor": autoriza, "turno": turno.String}
		if err := auditar(ctx, tx, "CAJON_ABIERTO_SIN_VENTA", "caja", in.CajaID, &uid, detalle, now); err != nil {
			return err
		}
		return a.eventoCaja(ctx, tx, EventoCajonAbierto, in.CajaID, map[string]any{"cajaId": in.CajaID, "usuarioId": u.ID, "usuario": u.Nombre,
			"autorizadoPor": autoriza, "motivo": in.Motivo, "turnoId": turno.String, "at": now.UTC().Format(time.RFC3339Nano)}, now)
	})
	if err != nil {
		return CajonOut{}, err
	}
	a.motor.Despertar(despertar)
	a.notificarPush()
	return out, nil
}
