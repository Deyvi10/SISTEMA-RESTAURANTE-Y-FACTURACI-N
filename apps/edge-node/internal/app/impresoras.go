package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// VigenciaComando: una orden de la nube más vieja que esto no se ejecuta (una prueba de
// impresión de hace una hora ya no le sirve a nadie).
const VigenciaComando = 10 * time.Minute

// nuevoMotor arma el motor de impresión con sus avisos: estado por el hub y resultado de
// las pruebas pedidas desde la nube por el outbox.
func (a *App) nuevoMotor(t impresion.Transporte) *impresion.Motor {
	return &impresion.Motor{
		DB: a.Store, T: t, Log: a.Log, Now: a.Clock.Now,
		AlCambiarEstado: func(e impresion.Estado) {
			a.pedirHeartbeat()
			_ = a.hub.Difundir(eventos.PrinterStatus{PrinterID: e.ID, Name: e.Nombre, Status: e.Estado, Queue: int64(e.Cola), StationIDs: a.estacionesDeImpresora(e.ID)})
		},
		AlImprimir: func(t impresion.Trabajo) {
			if t.ComandoID != nil {
				a.informarComando(*t.ComandoID, true, "Impreso")
			}
		},
		AlFallar: func(t impresion.Trabajo, motivo string) bool {
			if t.ComandoID == nil {
				return false // las comandas esperan: nunca se descartan
			}
			a.informarComando(*t.ComandoID, false, motivo)
			return true
		},
	}
}

func (a *App) estacionesDeImpresora(id ids.ID) []ids.ID {
	rows, err := a.Store.Read().Query(`SELECT estacion_id FROM estacion_impresoras WHERE impresora_id = ?`, id.String())
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()
	var out []ids.ID
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			if x, err := ids.Parse(s); err == nil {
				out = append(out, x)
			}
		}
	}
	return out
}

// sincronizarImpresoras lleva al motor la configuración de la réplica.
func (a *App) sincronizarImpresoras(ctx context.Context) {
	lista, err := a.impresorasActivas(ctx)
	if err != nil {
		a.Log.Error("no se pudo leer la configuración de impresoras", "err", err)
		return
	}
	a.motor.Sincronizar(lista)
}

// ---------- Órdenes de la nube ----------

// ejecutarComandos corre las órdenes nuevas de la nube (p. ej. «imprimir prueba»).
func (a *App) ejecutarComandos(ctx context.Context) {
	rows, err := a.Store.Read().QueryContext(ctx, `SELECT c.id, c.tipo, c.datos, c.created_at FROM comandos_nodo c
		LEFT JOIN comandos_ejecutados e ON e.id = c.id WHERE e.id IS NULL AND c.ejecutado_at IS NULL ORDER BY c.created_at`)
	if err != nil {
		a.Log.Error("comandos de la nube", "err", err)
		return
	}
	type cmd struct{ id, tipo, datos, creado string }
	var lista []cmd
	for rows.Next() {
		var c cmd
		if rows.Scan(&c.id, &c.tipo, &c.datos, &c.creado) == nil {
			lista = append(lista, c)
		}
	}
	_ = rows.Close()
	now := a.Clock.Now()
	var despertar []ids.ID
	for _, c := range lista {
		creado, _ := time.Parse(time.RFC3339Nano, c.creado)
		resultado := ""
		err := a.Store.Write(ctx, func(tx *store.Tx) error {
			switch {
			case now.Sub(creado) > VigenciaComando:
				resultado = "vencido"
			case c.tipo == "IMPRIMIR_PRUEBA":
				var d struct {
					ImpresoraID ids.ID `json:"impresoraId"`
				}
				_ = json.Unmarshal([]byte(c.datos), &d)
				imp, err := impresoraPorID(ctx, tx, d.ImpresoraID)
				if err != nil {
					resultado = "La impresora no está configurada en este nodo."
					break
				}
				t := impresion.Trabajo{ID: ids.New(), ImpresoraID: imp.ID, Tipo: "PRUEBA", ComandoID: &c.id}
				conexion := "Red · " + net.JoinHostPort(imp.Host, strconv.Itoa(imp.Puerto))
				if err := impresion.Encolar(ctx, tx, t, nil, escpos.ImprimirPrueba(imp.Ancho, imp.Nombre, conexion, now.In(a.zonaLocal(ctx))), nil, now); err != nil {
					return err
				}
				despertar = append(despertar, imp.ID)
			default:
				resultado = "Orden no soportada por esta versión del nodo."
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO comandos_ejecutados (id, ejecutado_at, resultado) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
				c.id, now.Format(time.RFC3339Nano), map[bool]string{true: "ENCOLADO", false: resultado}[resultado == ""])
			return err
		})
		if err != nil {
			a.Log.Error("no se pudo ejecutar la orden de la nube", "comando", c.id, "err", err)
			continue
		}
		if resultado != "" {
			a.informarComando(c.id, false, resultado)
		}
	}
	a.motor.Despertar(despertar...)
}

// informarComando envía a la nube el resultado de una orden.
func (a *App) informarComando(id string, ok bool, resultado string) {
	cid, err := ids.Parse(id)
	if err != nil {
		return
	}
	ctx := context.Background()
	ev, err := edgesync.NewEvent("comando.ejecutado", 1, cid, map[string]any{"comandoId": cid, "ok": ok, "resultado": resultado}, a.Clock.Now())
	if err != nil {
		return
	}
	if err := a.Store.Write(ctx, func(tx *store.Tx) error { _, err := a.outbox.Append(ctx, tx, ev); return err }); err != nil {
		a.Log.Error("no se pudo registrar el resultado de la orden", "err", err)
		return
	}
	a.notificarPush()
}

func impresoraPorID(ctx context.Context, q queryer, id ids.ID) (impresion.Impresora, error) {
	var x impresion.Impresora
	var ancho int
	err := q.QueryRowContext(ctx, `SELECT nombre, coalesce(host, ''), coalesce(puerto, 9100), ancho_papel FROM impresoras WHERE id = ? AND deleted_at IS NULL`, id.String()).
		Scan(&x.Nombre, &x.Host, &x.Puerto, &ancho)
	x.ID, x.Ancho = id, escpos.Paper(ancho)
	return x, err
}

// ---------- Redirección (RF-02-05.3) ----------

type RedirigirIn struct {
	ImpresoraID ids.ID  `json:"impresoraId"`
	UsuarioID   *ids.ID `json:"usuarioId"`
}

// Redirigir manda la cola de una estación a otra impresora con un toque (p. ej. la de cocina
// se quedó sin papel y el bar tiene). Lo pendiente se vuelve a armar para el ancho de papel
// de la impresora nueva.
func (a *App) Redirigir(ctx context.Context, estacionID ids.ID, in RedirigirIn) (int, error) {
	now := a.Clock.Now()
	movidos := 0
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		if _, err := estacionPorID(ctx, tx, estacionID.String()); err != nil {
			return problema(http.StatusNotFound, "NO_ENCONTRADO", "No existe esa estación de producción.")
		}
		dest, err := impresoraPorID(ctx, tx, in.ImpresoraID)
		if errors.Is(err, sql.ErrNoRows) {
			return invalido("Esa impresora no existe en este local.")
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO redirecciones (estacion_id, impresora_id, desde) VALUES (?, ?, ?)
			ON CONFLICT (estacion_id) DO UPDATE SET impresora_id = excluded.impresora_id, desde = excluded.desde`,
			estacionID.String(), dest.ID.String(), now.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT id, documento FROM trabajos_impresion WHERE estacion_id = ? AND estado = 'PENDIENTE' AND impresora_id <> ?`,
			estacionID.String(), dest.ID.String())
		if err != nil {
			return err
		}
		type pend struct {
			id  string
			doc sql.NullString
		}
		var ps []pend
		for rows.Next() {
			var p pend
			if rows.Scan(&p.id, &p.doc) == nil {
				ps = append(ps, p)
			}
		}
		_ = rows.Close()
		for _, p := range ps {
			if !p.doc.Valid {
				continue
			}
			var c escpos.Comanda
			if err := json.Unmarshal([]byte(p.doc.String), &c); err != nil {
				continue
			}
			if _, err := tx.ExecContext(ctx, `UPDATE trabajos_impresion SET impresora_id = ?, payload = ? WHERE id = ? AND estado = 'PENDIENTE'`,
				dest.ID.String(), escpos.ImprimirComanda(dest.Ancho, c), p.id); err != nil {
				return err
			}
			movidos++
		}
		return auditar(ctx, tx, "ESTACION_REDIRIGIDA", "estacion", estacionID, in.UsuarioID, map[string]any{"impresoraId": dest.ID, "trabajos": movidos}, now)
	})
	if err != nil {
		return 0, err
	}
	a.motor.Despertar(in.ImpresoraID)
	return movidos, nil
}

// QuitarRedireccion devuelve la estación a sus impresoras asignadas.
func (a *App) QuitarRedireccion(ctx context.Context, estacionID ids.ID, usuario *ids.ID) error {
	return a.Store.Write(ctx, func(tx *store.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM redirecciones WHERE estacion_id = ?`, estacionID.String())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return problema(http.StatusNotFound, "NO_ENCONTRADO", "Esa estación no estaba redirigida.")
		}
		return auditar(ctx, tx, "REDIRECCION_QUITADA", "estacion", estacionID, usuario, map[string]any{}, a.Clock.Now())
	})
}

// ---------- Vista de impresoras para la caja ----------

type ImpresoraVista struct {
	impresion.Estado
	Estaciones []ids.ID `json:"estaciones"`
}

type EstacionVista struct {
	ID          ids.ID   `json:"id"`
	Nombre      string   `json:"nombre"`
	Impresoras  []ids.ID `json:"impresoras"`
	RedirigidaA *ids.ID  `json:"redirigidaA"`
}

func (a *App) vistaImpresion(ctx context.Context) (map[string]any, error) {
	var imps []ImpresoraVista
	for _, e := range a.motor.Estados(ctx) {
		imps = append(imps, ImpresoraVista{Estado: e, Estaciones: a.estacionesDeImpresora(e.ID)})
	}
	rows, err := a.Store.Read().QueryContext(ctx, `SELECT e.id, e.nombre, r.impresora_id FROM estaciones e
		LEFT JOIN redirecciones r ON r.estacion_id = e.id WHERE e.deleted_at IS NULL AND e.tipo = 'PRODUCCION' ORDER BY e.orden, e.nombre`)
	if err != nil {
		return nil, err
	}
	var ests []EstacionVista
	for rows.Next() {
		var id string
		var ev EstacionVista
		var redir sql.NullString
		if err := rows.Scan(&id, &ev.Nombre, &redir); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ev.ID, _ = ids.Parse(id)
		if redir.Valid {
			r, _ := ids.Parse(redir.String)
			ev.RedirigidaA = &r
		}
		ests = append(ests, ev)
	}
	_ = rows.Close()
	for i := range ests {
		for _, im := range imps {
			for _, e := range im.Estaciones {
				if e == ests[i].ID {
					ests[i].Impresoras = append(ests[i].Impresoras, im.ID)
				}
			}
		}
	}
	if imps == nil {
		imps = []ImpresoraVista{}
	}
	if ests == nil {
		ests = []EstacionVista{}
	}
	return map[string]any{"impresoras": imps, "estaciones": ests}, nil
}

func (a *App) saludImpresoras(ctx context.Context) []edgesync.ImpresoraSalud {
	out := []edgesync.ImpresoraSalud{}
	for _, e := range a.motor.Estados(ctx) {
		out = append(out, edgesync.ImpresoraSalud{ID: e.ID, Estado: e.Estado, Cola: e.Cola})
	}
	return out
}
