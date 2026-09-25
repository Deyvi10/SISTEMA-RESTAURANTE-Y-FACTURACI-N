package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/nube"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
)

const (
	flujoNube    = "nube"
	flujoVolcado = "nube-volcado"
	// EsperaPull: long-poll; la nube responde apenas hay un cambio.
	EsperaPull = 25 * time.Second
	// VolcadoCada: una vez al día se pide todo de nuevo para corregir cualquier deriva
	// (y traer las tarifas de IVA, que son globales y no pasan por el feed de cambios).
	VolcadoCada = 24 * time.Hour
)

// CambiosAplicados describe lo que cambió en la réplica (lo usa el hub de tiempo real).
type CambiosAplicados struct {
	Completo bool
	Tablas   map[string]int
	Cambios  []edgesync.Cambio
}

// cursorNube devuelve el último cambio aplicado y cuándo fue el último volcado completo
// (cero si nunca hubo volcado).
func (a *App) cursorNube(ctx context.Context) (cursor int64, volcado time.Time, err error) {
	rows, err := a.Store.Read().QueryContext(ctx, `SELECT flujo, cursor, updated_at FROM inbox_cursores WHERE flujo IN (?, ?)`, flujoNube, flujoVolcado)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var flujo, upd string
		var c int64
		if err := rows.Scan(&flujo, &c, &upd); err != nil {
			return 0, time.Time{}, err
		}
		if flujo == flujoNube {
			cursor = c
		} else if volcado, err = time.Parse(time.RFC3339Nano, upd); err != nil {
			return 0, time.Time{}, err
		}
	}
	return cursor, volcado, rows.Err()
}

// PullOnce trae y aplica un lote de cambios de la nube. Devuelve si quedan más.
func (a *App) PullOnce(ctx context.Context, esperar time.Duration) (bool, error) {
	desde, ultimoVolcado, err := a.cursorNube(ctx)
	if err != nil {
		return false, err
	}
	pedirVolcado := ultimoVolcado.IsZero() || a.Clock.Now().Sub(ultimoVolcado) > VolcadoCada
	cctx, cancel := context.WithTimeout(ctx, esperar+20*time.Second)
	defer cancel()
	var res edgesync.PullResponse
	path := fmt.Sprintf("/v1/sync/pull?desde=%d&esperar=%d", desde, int(esperar/time.Second))
	if pedirVolcado {
		path += "&volcado=1"
	}
	if err := a.nube.Do(cctx, http.MethodGet, path, nil, &res, true); err != nil {
		return false, err
	}
	aplicados := CambiosAplicados{Completo: res.Modo == edgesync.PullCompleto, Tablas: map[string]int{}}
	if len(res.Cambios) == 0 && res.Hasta == desde && !aplicados.Completo {
		return false, nil
	}
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		if aplicados.Completo {
			if err := a.replica.Vaciar(ctx, tx); err != nil {
				return err
			}
		}
		for _, c := range res.Cambios {
			if !a.replica.Conoce(c.Tabla) {
				continue
			}
			if err := a.replica.Aplicar(ctx, tx, c); err != nil {
				return err
			}
			aplicados.Tablas[c.Tabla]++
			aplicados.Cambios = append(aplicados.Cambios, c)
		}
		now := a.now()
		if _, err := tx.ExecContext(ctx, `INSERT INTO inbox_cursores (flujo, cursor, updated_at) VALUES (?, ?, ?)
			ON CONFLICT (flujo) DO UPDATE SET cursor = excluded.cursor, updated_at = excluded.updated_at`, flujoNube, res.Hasta, now); err != nil {
			return err
		}
		if aplicados.Completo {
			_, err := tx.ExecContext(ctx, `INSERT INTO inbox_cursores (flujo, cursor, updated_at) VALUES (?, ?, ?)
				ON CONFLICT (flujo) DO UPDATE SET cursor = excluded.cursor, updated_at = excluded.updated_at`, flujoVolcado, res.Hasta, now)
			return err
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if len(aplicados.Tablas) > 0 || aplicados.Completo {
		a.Log.Info("cambios de la nube aplicados", "completo", aplicados.Completo, "tablas", aplicados.Tablas, "cursor", res.Hasta)
		if a.alAplicar != nil {
			a.alAplicar(aplicados)
		}
	}
	return res.Mas, nil
}

// pullLoop mantiene la réplica al día. Antes de aceptar cambios de catálogo empuja lo
// operativo pendiente (docs/03 §5.6): así un cambio de precio no se cruza con ventas
// que la nube aún no conoce.
func (a *App) pullLoop(ctx context.Context) {
	fallos := 0
	for ctx.Err() == nil {
		a.vaciarOutbox(ctx)
		mas, err := a.PullOnce(ctx, EsperaPull)
		switch {
		case ctx.Err() != nil:
			return
		case nube.EsRevocado(err):
			a.revocado(ctx)
			return
		case err != nil:
			fallos++
			espera := min(time.Second<<min(fallos-1, 5), 30*time.Second)
			a.salud.fallo("Sin conexión con la nube")
			a.Log.Warn("pull fallido, se reintentará", "err", err, "espera", espera)
			select {
			case <-ctx.Done():
				return
			case <-time.After(espera):
			}
		default:
			fallos = 0
			_ = mas // si hay más, el siguiente ciclo pide de inmediato
		}
	}
}

// vaciarOutbox envía lo pendiente (hasta 10 lotes) antes de pedir cambios.
func (a *App) vaciarOutbox(ctx context.Context) {
	for range 10 {
		n, err := a.pusher.PushOnce(ctx)
		if err != nil || n == 0 {
			return
		}
	}
}
