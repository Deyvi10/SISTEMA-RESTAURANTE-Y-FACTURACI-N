package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
)

// AsignacionIn: un plato en una cuenta con su peso (la pizza entre 3: peso 1 en cada una).
type AsignacionIn struct {
	LineaID ids.ID `json:"lineaId"`
	Peso    int64  `json:"peso"`
}

// CuentaIn es una cuenta abierta de la división.
type CuentaIn struct {
	Asignaciones []AsignacionIn `json:"asignaciones"`
}

// DivisionIn trae las cuentas ABIERTAS; las pagadas no se tocan. Una sola cuenta sin pagadas
// (o ninguna) deshace la división.
type DivisionIn struct {
	Cuentas []CuentaIn `json:"cuentas"`
}

// CuentaVista es una cuenta tal como la ve la caja.
type CuentaVista struct {
	ID           ids.ID            `json:"id"`
	Numero       int               `json:"numero"`
	Estado       string            `json:"estado"` // ABIERTA, PAGADA
	Asignaciones []AsignacionIn    `json:"asignaciones"`
	Lineas       map[ids.ID]string `json:"lineas"` // lo que lleva de cada plato
	Subtotal     string            `json:"subtotal"`
	IVA          string            `json:"iva"`
	Propina      string            `json:"propina"`
	Total        string            `json:"total"`
	Documento    string            `json:"documento,omitempty"` // INT-000123 si ya se cobró
}

type DivisionOut struct {
	Cuentas   []CuentaVista `json:"cuentas"`
	SinCuenta []ids.ID      `json:"sinCuenta"`
	Total     string        `json:"total"`
}

// leerCuentas trae las cuentas de la orden: las pagadas con lo que pagaron (congelado) y las
// abiertas con sus pesos. codigos: cuenta → código del documento de las pagadas.
func leerCuentas(ctx context.Context, q queryer, orden ids.ID) ([]cuentaReparto, map[ids.ID]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT c.id, c.numero, c.estado, c.pagado, coalesce(json_extract(d.datos, '$.codigo'), '')
		FROM cuentas c LEFT JOIN documentos_venta d ON d.id = c.documento_id WHERE c.orden_id = ? ORDER BY c.numero`, orden.String())
	if err != nil {
		return nil, nil, err
	}
	var cs []cuentaReparto
	codigos := map[ids.ID]string{}
	for rows.Next() {
		var c cuentaReparto
		var id, estado, codigo string
		var pagado sql.NullString
		if err := rows.Scan(&id, &c.Numero, &estado, &pagado, &codigo); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		c.ID, _ = ids.Parse(id)
		c.Pagada = estado == "PAGADA"
		if c.Pagada {
			var p CuentaTotales
			if err := json.Unmarshal([]byte(pagado.String), &p); err != nil {
				_ = rows.Close()
				return nil, nil, err
			}
			c.PagadoLineas, c.PagadoTarifas, c.PagadoPropina = p.Lineas, p.PorTarifa, p.Propina
			codigos[c.ID] = codigo
		}
		cs = append(cs, c)
	}
	_ = rows.Close()
	for i := range cs {
		if cs[i].Pagada {
			continue
		}
		cs[i].Pesos = map[ids.ID]int64{}
		rows, err := q.QueryContext(ctx, `SELECT orden_linea_id, peso FROM cuenta_asignaciones WHERE cuenta_id = ?`, cs[i].ID.String())
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var l string
			var p int64
			if err := rows.Scan(&l, &p); err != nil {
				_ = rows.Close()
				return nil, nil, err
			}
			lid, _ := ids.Parse(l)
			cs[i].Pesos[lid] = p
		}
		_ = rows.Close()
	}
	return cs, codigos, nil
}

// cuentasDe calcula las cuentas de una orden (nil si no está dividida).
func (a *App) cuentasDe(ctx context.Context, q queryer, o Orden) (Totales, []cuentaReparto, []CuentaTotales, map[ids.ID]string, error) {
	tot, _, _, _, _, err := a.calcularTotales(ctx, q, o)
	if err != nil {
		return tot, nil, nil, nil, err
	}
	cs, codigos, err := leerCuentas(ctx, q, o.ID)
	if err != nil || len(cs) == 0 {
		return tot, cs, nil, codigos, err
	}
	res, err := repartirEnCuentas(tot.reparto, tot.PorTarifa, tot.propinaM, tot.incluyeIVA, append([]cuentaReparto(nil), cs...))
	return tot, cs, res, codigos, err
}

func vista(cs []cuentaReparto, res []CuentaTotales, codigos map[ids.ID]string) []CuentaVista {
	porID := map[ids.ID]cuentaReparto{}
	for _, c := range cs {
		porID[c.ID] = c
	}
	out := make([]CuentaVista, 0, len(res))
	for _, r := range res {
		c := porID[r.ID]
		v := CuentaVista{ID: r.ID, Numero: r.Numero, Estado: map[bool]string{true: "PAGADA", false: "ABIERTA"}[r.Pagada], Asignaciones: []AsignacionIn{},
			Lineas: map[ids.ID]string{}, Subtotal: r.Subtotal.String(), IVA: r.IVA.String(), Propina: r.Propina.String(), Total: r.Total.String(), Documento: codigos[r.ID]}
		for l, m := range r.Lineas {
			v.Lineas[l] = m.String()
		}
		for l, p := range c.Pesos {
			v.Asignaciones = append(v.Asignaciones, AsignacionIn{LineaID: l, Peso: p})
		}
		sort.Slice(v.Asignaciones, func(i, j int) bool { return v.Asignaciones[i].LineaID.String() < v.Asignaciones[j].LineaID.String() })
		out = append(out, v)
	}
	return out
}

// errDivision traduce los errores del reparto a mensajes para el cajero.
func errDivision(err error, o Orden) error {
	var sc errSinCuenta
	switch {
	case errors.As(err, &sc):
		nombres := map[ids.ID]string{}
		for _, l := range o.Lineas {
			nombres[l.ID] = l.Cantidad + " " + l.Producto
		}
		var faltan []string
		for _, l := range sc.lineas {
			faltan = append(faltan, nombres[l])
		}
		return problema(http.StatusUnprocessableEntity, "PLATOS_SIN_CUENTA", "Estos platos no están en ninguna cuenta: "+strings.Join(faltan, ", ")+".")
	case errors.Is(err, errPagadoSupera):
		return problema(http.StatusConflict, "PAGADO_SUPERA", "Lo ya cobrado supera lo que queda de la orden. Revisa los platos anulados o los descuentos.")
	}
	return err
}

// Cuentas devuelve la división de una orden.
func (a *App) Cuentas(ctx context.Context, orden ids.ID) (DivisionOut, error) {
	q := a.Store.Read()
	o, err := leerOrden(ctx, q, orden)
	if err != nil {
		return DivisionOut{}, err
	}
	tot, cs, res, codigos, err := a.cuentasDe(ctx, q, o)
	out := DivisionOut{Cuentas: []CuentaVista{}, SinCuenta: []ids.ID{}, Total: tot.Total}
	var sc errSinCuenta
	if errors.As(err, &sc) {
		// Se informa qué falta asignar, y las cuentas sin montos hasta que se complete.
		out.SinCuenta = sc.lineas
		for _, c := range cs {
			out.Cuentas = append(out.Cuentas, vista([]cuentaReparto{c}, []CuentaTotales{{ID: c.ID, Numero: c.Numero, Pagada: c.Pagada, Lineas: c.PagadoLineas}}, codigos)...)
		}
		return out, nil
	}
	if err != nil {
		return out, errDivision(err, o)
	}
	out.Cuentas = vista(cs, res, codigos)
	return out, nil
}

// Dividir reemplaza las cuentas abiertas de la orden en una sola transacción (RF-04-06.4):
// las pagadas no cambian. Todos los platos con saldo deben quedar en alguna cuenta.
func (a *App) Dividir(ctx context.Context, u Usuario, orden ids.ID, in DivisionIn) (DivisionOut, error) {
	if !u.Puede(rbac.DividirCuenta) {
		return DivisionOut{}, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para dividir cuentas.")
	}
	if len(in.Cuentas) > 30 {
		return DivisionOut{}, invalido("Una orden se divide en 30 cuentas como máximo.")
	}
	now := a.Clock.Now()
	var out DivisionOut
	var mesa *ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		o, err := ordenAbierta(ctx, tx, orden)
		if err != nil {
			return err
		}
		vivas := map[ids.ID]bool{}
		for _, l := range o.Lineas {
			if l.Estado != "ANULADA" {
				vivas[l.ID] = true
			}
		}
		actuales, _, err := leerCuentas(ctx, tx, orden)
		if err != nil {
			return err
		}
		pagadas, maxPagada := 0, 0
		for _, c := range actuales {
			if c.Pagada {
				pagadas++
				maxPagada = max(maxPagada, c.Numero)
			}
		}
		// Se borran las abiertas y se crean las nuevas (las pagadas quedan como están).
		if _, err := tx.ExecContext(ctx, `DELETE FROM cuenta_asignaciones WHERE cuenta_id IN (SELECT id FROM cuentas WHERE orden_id = ? AND estado = 'ABIERTA')`, orden.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM cuentas WHERE orden_id = ? AND estado = 'ABIERTA'`, orden.String()); err != nil {
			return err
		}
		deshacer := pagadas == 0 && len(in.Cuentas) <= 1
		if !deshacer {
			if len(in.Cuentas) == 0 {
				return invalido("Quedan platos por cobrar: arma al menos una cuenta.")
			}
			ts := now.Format(time.RFC3339Nano)
			for i, c := range in.Cuentas {
				if len(c.Asignaciones) == 0 {
					return invalido(fmt.Sprintf("La cuenta %d está vacía.", maxPagada+i+1))
				}
				id := ids.New()
				if _, err := tx.ExecContext(ctx, `INSERT INTO cuentas (id, orden_id, numero, created_at) VALUES (?, ?, ?, ?)`, id.String(), orden.String(), maxPagada+i+1, ts); err != nil {
					return err
				}
				vistos := map[ids.ID]bool{}
				for _, as := range c.Asignaciones {
					if !vivas[as.LineaID] {
						return invalido("Un plato de la división ya no está en la orden. Actualiza la pantalla.")
					}
					if as.Peso < 1 || as.Peso > 1000 || vistos[as.LineaID] {
						return invalido("Cada plato va una vez por cuenta, con un peso de 1 a 1000.")
					}
					vistos[as.LineaID] = true
					if _, err := tx.ExecContext(ctx, `INSERT INTO cuenta_asignaciones (cuenta_id, orden_linea_id, peso) VALUES (?, ?, ?)`, id.String(), as.LineaID.String(), as.Peso); err != nil {
						return err
					}
				}
			}
			// La división tiene que cerrar: cada plato con saldo en alguna cuenta y nada negativo.
			_, cs, res, codigos, err := a.cuentasDe(ctx, tx, o)
			if err != nil {
				return errDivision(err, o)
			}
			out.Cuentas = vista(cs, res, codigos)
		}
		tot, _, _, _, _, err := a.calcularTotales(ctx, tx, o)
		if err != nil {
			return err
		}
		out.Total, out.SinCuenta = tot.Total, []ids.ID{}
		if out.Cuentas == nil {
			out.Cuentas = []CuentaVista{}
		}
		uid := u.ID
		mesa = o.MesaID
		accion := map[bool]string{true: "DIVISION_DESHECHA", false: "CUENTA_DIVIDIDA"}[deshacer]
		return auditar(ctx, tx, accion, "orden", orden, &uid, map[string]any{"cuentas": len(in.Cuentas), "pagadas": pagadas}, now)
	})
	if err == nil && mesa != nil {
		a.avisarMesa(ctx, *mesa)
	}
	return out, err
}

// bloqueoPorCuentas impide cambios que moverían lo ya cobrado en una cuenta pagada: descuentos
// o servicio de toda la cuenta, y descuentos o anulaciones de platos que alguien ya pagó.
func bloqueoPorCuentas(ctx context.Context, q queryer, orden ids.ID, lineas ...ids.ID) error {
	cs, _, err := leerCuentas(ctx, q, orden)
	if err != nil {
		return err
	}
	pagadas := false
	for _, c := range cs {
		if !c.Pagada {
			continue
		}
		pagadas = true
		for _, l := range lineas {
			if m, ok := c.PagadoLineas[l]; ok && !m.IsZero() {
				return problema(http.StatusConflict, "CUENTA_PAGADA", "Ese plato ya se cobró en una cuenta: no se puede cambiar.")
			}
		}
	}
	if pagadas && len(lineas) == 0 {
		return problema(http.StatusConflict, "CUENTAS_PAGADAS", "Ya se cobró parte de la orden: los cambios de toda la cuenta ya no se pueden hacer.")
	}
	return nil
}

// ordenDividida indica si la orden tiene cuentas (para operaciones que las romperían).
func ordenDividida(ctx context.Context, q queryer, orden ids.ID) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM cuentas WHERE orden_id = ?`, orden.String()).Scan(&n)
	return n > 0, err
}
