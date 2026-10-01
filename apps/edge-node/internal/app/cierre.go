package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/cierrez"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
)

// EventoCierreZ lleva el Cierre Z completo a la nube (que lo guarda y lo envía al dueño).
const EventoCierreZ = "caja.cierre_z"

// impresorasDeCaja: las impresoras de la estación de la caja (o de la primera estación de
// tipo Caja si no se indica). Si no tiene, la primera impresora activa, cuyo nombre se
// devuelve como respaldo para avisar al cajero.
func (a *App) impresorasDeCaja(ctx context.Context, tx *store.Tx, estacion *ids.ID) (imps []impresion.Impresora, respaldo string, err error) {
	if estacion == nil {
		var id string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM estaciones WHERE tipo = 'CAJA' AND deleted_at IS NULL ORDER BY orden LIMIT 1`).Scan(&id); err == nil {
			if e, err := ids.Parse(id); err == nil {
				estacion = &e
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, "", err
		}
	}
	if estacion != nil {
		if imps, err = impresorasDe(ctx, tx, *estacion); err != nil || len(imps) > 0 {
			return imps, "", err
		}
	}
	todas, err := a.impresorasActivas(ctx)
	if err != nil || len(todas) == 0 {
		return nil, "", err
	}
	return todas[:1], todas[0].Nombre, nil
}

type DeclaradoIn struct {
	MetodoID ids.ID `json:"metodoId"`
	Monto    string `json:"monto"`
}

// CerrarTurnoIn es lo que declara el cajero sin ver lo esperado (cierre ciego).
type CerrarTurnoIn struct {
	CajaID         ids.ID           `json:"cajaId"`
	Conteo         []cierrez.Conteo `json:"conteo"`
	Declarado      []DeclaradoIn    `json:"declarado"` // vouchers de tarjeta, transferencias…
	IdempotencyKey string           `json:"idempotencyKey"`
}

type CierreOut struct {
	Cierre     cierrez.Cierre `json:"cierre"`
	Impresoras []string       `json:"impresoras"`
	Aviso      string         `json:"aviso,omitempty"`
}

// CerrarTurno calcula en el nodo lo esperado, lo compara con lo declarado, genera el Cierre Z
// inmutable, cierra el turno (la caja queda bloqueada hasta abrir otro), lo imprime y lo deja
// en el outbox para la nube, todo en una transacción.
func (a *App) CerrarTurno(ctx context.Context, u Usuario, in CerrarTurnoIn) (CierreOut, error) {
	var out CierreOut
	if !u.Puede(rbac.GestionarTurno) {
		return out, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para cerrar turnos de caja.")
	}
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 64 {
		return out, invalido("Falta la clave de idempotencia del cierre.")
	}
	contado, err := cierrez.TotalConteo(in.Conteo)
	if err != nil {
		return out, invalido("El conteo de billetes y monedas no es válido: " + err.Error() + ".")
	}
	declarado := map[ids.ID]money.Money{}
	for _, d := range in.Declarado {
		m, err := montoNoNegativo(d.Monto, "El total declarado")
		if err != nil {
			return out, err
		}
		if _, repetido := declarado[d.MetodoID]; repetido {
			return out, invalido("Un método de pago aparece dos veces en lo declarado.")
		}
		declarado[d.MetodoID] = m
	}
	now, loc := a.Clock.Now(), a.zonaLocal(ctx)
	var despertar []ids.ID
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		// Reintento del mismo cierre (se cortó la red): devuelve el ya generado.
		var datos string
		err := tx.QueryRowContext(ctx, `SELECT datos FROM cierres_z WHERE idempotency_key = ?`, in.IdempotencyKey).Scan(&datos)
		if err == nil {
			return json.Unmarshal([]byte(datos), &out.Cierre)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		t, err := turnoAbiertoDe(ctx, tx, in.CajaID)
		if err != nil {
			return err
		}
		c := cierrez.Cierre{ID: ids.New(), TurnoID: t.ID, CajaID: t.CajaID, JornadaID: t.JornadaID, CajeroID: t.CajeroID, Cajero: t.CajeroNombre,
			CerradoPor: u.Nombre, AbiertoAt: t.AbiertoAt.UTC(), CerradoAt: now.UTC(), Conteo: in.Conteo, EfectivoTotal: contado}
		var estacion sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT c.nombre, c.estacion_id, j.fecha_negocio, coalesce((SELECT nombre FROM locales LIMIT 1), '')
			FROM cajas c, jornadas j WHERE c.id = ? AND j.id = ?`, t.CajaID.String(), t.JornadaID.String()).Scan(&c.Caja, &estacion, &c.FechaNegocio, &c.Local); err != nil {
			return err
		}
		if c.FondoInicial, err = money.Parse(t.FondoInicial); err != nil {
			return err
		}
		metodos, err := metodosDelCierre(ctx, tx, t.ID)
		if err != nil {
			return err
		}
		for id := range declarado {
			if !metodoDeclarable(metodos, id) {
				return invalido("Lo declarado incluye un método de pago que no existe o es efectivo.")
			}
		}
		cobrado, err := sumarPorMetodo(ctx, tx, t.ID)
		if err != nil {
			return err
		}
		if err := sumarMovimientos(ctx, tx, t.ID, &c.Movimientos); err != nil {
			return err
		}
		c.Lineas, c.Resultado = cierrez.Calcular(metodos, c.Movimientos, cobrado, contado, declarado)
		// Numeración por caja y cadena de hash con el cierre anterior de la misma caja.
		if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(numero), 0) + 1, coalesce((SELECT hash FROM cierres_z WHERE caja_id = ? ORDER BY numero DESC LIMIT 1), '')
			FROM cierres_z WHERE caja_id = ?`, t.CajaID.String(), t.CajaID.String()).Scan(&c.Numero, &c.HashAnterior); err != nil {
			return err
		}
		c.Hash = c.CalcularHash()
		raw, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cierres_z (id, turno_id, caja_id, numero, resultado, datos, hash, hash_anterior, idempotency_key, generado_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, c.ID.String(), t.ID.String(), t.CajaID.String(), c.Numero, c.Resultado, string(raw), c.Hash, c.HashAnterior,
			in.IdempotencyKey, now.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE turnos_caja SET estado = 'CERRADO', cerrado_at = ? WHERE id = ?`, now.Format(time.RFC3339Nano), t.ID.String()); err != nil {
			return err
		}
		// Impresión en la estación de la caja (cajón incluido).
		var est *ids.ID
		if e, err := ids.Parse(estacion.String); err == nil {
			est = &e
		}
		imps, respaldo, err := a.impresorasDeCaja(ctx, tx, est)
		if err != nil {
			return err
		}
		switch {
		case respaldo != "":
			out.Aviso = "La caja no tiene impresora: el Cierre Z salió en «" + respaldo + "»."
		case len(imps) == 0:
			out.Aviso = "No hay impresoras configuradas: el Cierre Z quedó guardado, pero no se imprimió."
		}
		out.Impresoras = []string{}
		for _, imp := range imps {
			tr := impresion.Trabajo{ID: ids.New(), ImpresoraID: imp.ID, Tipo: "CIERRE_Z"}
			if err := impresion.Encolar(ctx, tx, tr, nil, escpos.ImprimirCierreZ(imp.Ancho, c, loc), nil, now); err != nil {
				return err
			}
			out.Impresoras = append(out.Impresoras, imp.Nombre)
			despertar = append(despertar, imp.ID)
		}
		uid := u.ID
		if err := auditar(ctx, tx, "CIERRE_Z", "turno", t.ID, &uid, map[string]any{"cierre": c.ID, "numero": c.Numero, "resultado": c.Resultado}, now); err != nil {
			return err
		}
		out.Cierre = c
		return a.eventoCaja(ctx, tx, EventoCierreZ, c.ID, c, now)
	})
	if err != nil {
		return CierreOut{}, err
	}
	a.motor.Despertar(despertar...)
	a.notificarPush()
	return out, nil
}

// metodosDelCierre: los métodos activos más los que se usaron en el turno aunque ya no estén
// activos. Si el nodo aún no recibió los métodos, el efectivo igual se cuadra.
func metodosDelCierre(ctx context.Context, q queryer, turno ids.ID) ([]cierrez.Metodo, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, nombre, tipo FROM metodos_pago
		WHERE (deleted_at IS NULL AND activo = 1) OR id IN (SELECT metodo_pago_id FROM pagos WHERE turno_id = ?)
		ORDER BY tipo <> 'EFECTIVO', orden, nombre`, turno.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []cierrez.Metodo
	efectivo := false
	for rows.Next() {
		var m cierrez.Metodo
		var id string
		if err := rows.Scan(&id, &m.Nombre, &m.Tipo); err != nil {
			return nil, err
		}
		m.ID, _ = ids.Parse(id)
		efectivo = efectivo || m.Tipo == "EFECTIVO"
		out = append(out, m)
	}
	if !efectivo {
		out = append([]cierrez.Metodo{{Nombre: "Efectivo", Tipo: "EFECTIVO"}}, out...)
	}
	return out, rows.Err()
}

func metodoDeclarable(ms []cierrez.Metodo, id ids.ID) bool {
	for _, m := range ms {
		if m.ID == id && m.Tipo != "EFECTIVO" {
			return true
		}
	}
	return false
}

func sumarPorMetodo(ctx context.Context, q queryer, turno ids.ID) (map[ids.ID]money.Money, error) {
	rows, err := q.QueryContext(ctx, `SELECT metodo_pago_id, monto FROM pagos WHERE turno_id = ?`, turno.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[ids.ID]money.Money{}
	for rows.Next() {
		var id, monto string
		if err := rows.Scan(&id, &monto); err != nil {
			return nil, err
		}
		mid, err1 := ids.Parse(id)
		m, err2 := money.Parse(monto)
		if err := errors.Join(err1, err2); err != nil {
			return nil, err
		}
		out[mid] = out[mid].Add(m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Lo devuelto por notas de crédito sale de lo cobrado por ese método (F5-13).
	dev, err := q.QueryContext(ctx, `SELECT metodo_pago_id, monto FROM devoluciones_nc WHERE turno_id = ?`, turno.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = dev.Close() }()
	for dev.Next() {
		var id, monto string
		if err := dev.Scan(&id, &monto); err != nil {
			return nil, err
		}
		mid, err1 := ids.Parse(id)
		m, err2 := money.Parse(monto)
		if err := errors.Join(err1, err2); err != nil {
			return nil, err
		}
		out[mid] = out[mid].Sub(m)
	}
	return out, dev.Err()
}

func sumarMovimientos(ctx context.Context, q queryer, turno ids.ID, mov *cierrez.Movimientos) error {
	rows, err := q.QueryContext(ctx, `SELECT tipo, monto FROM movimientos_caja WHERE turno_id = ?`, turno.String())
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var tipo, monto string
		if err := rows.Scan(&tipo, &monto); err != nil {
			return err
		}
		m, err := money.Parse(monto)
		if err != nil {
			return err
		}
		switch tipo {
		case "INGRESO":
			mov.Ingresos = mov.Ingresos.Add(m)
		case "RETIRO":
			mov.Retiros = mov.Retiros.Add(m)
		case "GASTO":
			mov.Gastos = mov.Gastos.Add(m)
		}
	}
	return rows.Err()
}
