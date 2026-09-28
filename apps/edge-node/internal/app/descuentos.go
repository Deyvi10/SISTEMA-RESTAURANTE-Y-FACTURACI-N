package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
)

// Descuento vigente de una orden (F4-09): de una línea o de la cuenta completa.
type Descuento struct {
	ID            ids.ID  `json:"id"`
	LineaID       *ids.ID `json:"lineaId"` // nil = toda la cuenta
	Tipo          string  `json:"tipo"`    // PORCENTAJE, MONTO
	Valor         string  `json:"valor"`
	Cortesia      bool    `json:"cortesia"`
	Motivo        string  `json:"motivo"`
	UsuarioNombre string  `json:"usuarioNombre"`
	AutorizadoPor *ids.ID `json:"autorizadoPor,omitempty"`
	Monto         string  `json:"monto"` // cuánto descuenta hoy (con IVA si los precios lo incluyen)
}

func leerDescuentos(ctx context.Context, q queryer, orden ids.ID) ([]Descuento, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, linea_id, tipo, valor, cortesia, motivo, usuario_nombre, autorizado_por FROM descuentos
		WHERE orden_id = ? AND quitado_at IS NULL ORDER BY linea_id IS NULL, created_at`, orden.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Descuento
	for rows.Next() {
		var d Descuento
		var id string
		var linea, aut sql.NullString
		if err := rows.Scan(&id, &linea, &d.Tipo, &d.Valor, &d.Cortesia, &d.Motivo, &d.UsuarioNombre, &aut); err != nil {
			return nil, err
		}
		d.ID, _ = ids.Parse(id)
		d.LineaID, d.AutorizadoPor = idDe(linea.String), idDe(aut.String)
		out = append(out, d)
	}
	return out, rows.Err()
}

// montoDescuento: lo que descuenta sobre un importe (nunca más que el importe).
func montoDescuento(d Descuento, sobre money.Money) money.Money {
	if d.Cortesia {
		return sobre
	}
	v, err := money.Parse(d.Valor)
	if err != nil || v.IsNegative() {
		return money.Money{}
	}
	m := v
	if d.Tipo == "PORCENTAJE" {
		m = sobre.Mul(v.Decimal().Div(decimal.NewFromInt(100))).Round2()
	}
	if m.GreaterThan(sobre) {
		return sobre
	}
	return m
}

// aplicarDescuentos calcula el importe final de cada línea: primero su propio descuento y
// luego el de la cuenta, prorrateado por el mayor residuo para que la base y el IVA por tarifa
// salgan exactos (y el XML de F5 tenga el descuento por detalle). Devuelve cuánto descuenta
// cada descuento. Σ finales + Σ descuentos = Σ líneas, al centavo, y ninguna línea queda negativa.
func aplicarDescuentos(lineas []LineaOrden, ds []Descuento) (finales map[ids.ID]money.Money, conMonto []Descuento) {
	finales = map[ids.ID]money.Money{}
	var orden []ids.ID
	porLinea := map[ids.ID]int{}
	cuenta := -1
	for i, d := range ds {
		if d.LineaID == nil {
			cuenta = i
		} else {
			porLinea[*d.LineaID] = i
		}
	}
	conMonto = append([]Descuento(nil), ds...)
	for _, l := range lineas {
		if l.Estado == "ANULADA" {
			continue
		}
		t, _ := money.Parse(l.Total)
		if i, ok := porLinea[l.ID]; ok {
			m := montoDescuento(ds[i], t)
			conMonto[i].Monto = m.String()
			t = t.Sub(m)
		}
		finales[l.ID] = t
		orden = append(orden, l.ID)
	}
	for i := range conMonto {
		if conMonto[i].Monto == "" {
			conMonto[i].Monto = "0.00" // descuento de una línea anulada
		}
	}
	if cuenta < 0 || len(orden) == 0 {
		return finales, conMonto
	}
	suma := money.Money{}
	pesos := make([]decimal.Decimal, len(orden))
	for i, id := range orden {
		suma = suma.Add(finales[id])
		pesos[i] = finales[id].Decimal()
	}
	m := montoDescuento(ds[cuenta], suma)
	conMonto[cuenta].Monto = m.String()
	if m.IsZero() {
		return finales, conMonto
	}
	partes, err := money.Allocate(m, pesos)
	if err != nil { // todas las líneas en cero: no hay qué descontar
		conMonto[cuenta].Monto = "0.00"
		return finales, conMonto
	}
	for i, id := range orden {
		finales[id] = finales[id].Sub(partes[i])
	}
	return finales, conMonto
}

// limiteDescuento: el porcentaje que la persona puede dar sin autorización. El administrador
// no tiene tope; los demás, el suyo propio o el del local (10 % si no llegó la configuración).
func limiteDescuento(ctx context.Context, q queryer, u Usuario) (money.Money, bool) {
	if u.Rol == string(rbac.Admin) {
		return money.Money{}, false
	}
	var propio, local sql.NullString
	_ = q.QueryRowContext(ctx, `SELECT descuento_maximo_pct FROM usuarios WHERE id = ?`, u.ID.String()).Scan(&propio)
	_ = q.QueryRowContext(ctx, `SELECT descuento_maximo_pct FROM locales LIMIT 1`).Scan(&local)
	for _, v := range []sql.NullString{propio, local} {
		if m, err := money.Parse(v.String); v.Valid && err == nil {
			return m, true
		}
	}
	return money.MustParse("10"), true
}

type DescuentoIn struct {
	LineaID      *ids.ID `json:"lineaId"` // nil = toda la cuenta
	Tipo         string  `json:"tipo"`    // PORCENTAJE, MONTO (una cortesía no lo necesita)
	Valor        string  `json:"valor"`
	Cortesia     bool    `json:"cortesia"`
	MotivoID     ids.ID  `json:"motivoId"`
	Autorizacion string  `json:"autorizacion"` // token del PIN de supervisor si hace falta
}

var errDescuentoSupervisor = problema(http.StatusForbidden, "REQUIERE_SUPERVISOR", "Este descuento necesita la autorización de un supervisor.")

// AplicarDescuento pone (o reemplaza) el descuento de una línea o de la cuenta (RF-04-07):
// con motivo de la lista, dentro del límite de la persona o con el PIN de un supervisor; la
// cortesía (100 %) siempre pide autorización salvo al administrador. Queda auditado.
func (a *App) AplicarDescuento(ctx context.Context, d Dispositivo, u Usuario, orden ids.ID, in DescuentoIn) (Totales, error) {
	in.Tipo = strings.ToUpper(strings.TrimSpace(in.Tipo))
	if in.Cortesia {
		in.Tipo, in.Valor = "PORCENTAJE", "100"
	}
	if in.Tipo != "PORCENTAJE" && in.Tipo != "MONTO" {
		return Totales{}, invalido("El descuento es en porcentaje o en monto.")
	}
	valor, err := montoNoNegativo(in.Valor, "El descuento")
	if err != nil {
		return Totales{}, err
	}
	if valor.IsZero() || (in.Tipo == "PORCENTAJE" && valor.GreaterThan(money.MustParse("100"))) {
		return Totales{}, invalido("Escribe un descuento mayor que cero (hasta 100 %).")
	}
	now := a.Clock.Now()
	var tot Totales
	var mesa *ids.ID
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		o, err := ordenAbierta(ctx, tx, orden)
		if err != nil {
			return err
		}
		var motivo, tipoMotivo string
		err = tx.QueryRowContext(ctx, `SELECT nombre, tipo FROM motivos_descuento WHERE id = ? AND deleted_at IS NULL AND activo = 1`, in.MotivoID.String()).Scan(&motivo, &tipoMotivo)
		if errors.Is(err, sql.ErrNoRows) {
			return invalido("Elige el motivo del descuento de la lista.")
		}
		if err != nil {
			return err
		}
		if in.Cortesia != (tipoMotivo == "CORTESIA") {
			return invalido("Ese motivo es para " + map[bool]string{true: "cortesías", false: "descuentos"}[tipoMotivo == "CORTESIA"] + ".")
		}
		// El importe sobre el que se aplica, para comparar con el límite.
		sobre, err := money.Parse(o.Total)
		if err != nil {
			return err
		}
		if in.LineaID != nil {
			var ok bool
			for _, l := range o.Lineas {
				if l.ID == *in.LineaID && l.Estado != "ANULADA" {
					sobre, _ = money.Parse(l.Total)
					ok = true
				}
			}
			if !ok {
				return invalido("Ese plato no está en la orden o está anulado.")
			}
		}
		pct := valor
		if in.Tipo == "MONTO" {
			if sobre.IsZero() {
				return invalido("No hay nada que descontar.")
			}
			pct = money.FromDecimal(valor.Decimal().Mul(decimal.NewFromInt(100)).Div(sobre.Decimal())).Round2()
		}
		// ¿Hace falta un supervisor? Sin el permiso, por encima del límite o por ser cortesía.
		limite, conTope := limiteDescuento(ctx, tx, u)
		necesita := !u.Puede(rbac.DarDescuento) || (conTope && pct.GreaterThan(limite)) || (in.Cortesia && u.Rol != string(rbac.Admin))
		var autoriza *ids.ID
		if necesita {
			if in.Autorizacion == "" {
				return errDescuentoSupervisor
			}
			por, err := consumirAutorizacion(ctx, tx, in.Autorizacion, d, rbac.DarDescuento, orden.String(), now)
			if err != nil {
				return err
			}
			autoriza = &por
		}
		ts := now.Format(time.RFC3339Nano)
		// Un descuento por línea y uno por cuenta: el nuevo reemplaza al vigente.
		if _, err := tx.ExecContext(ctx, `UPDATE descuentos SET quitado_at = ?, quitado_por = ? WHERE orden_id = ? AND coalesce(linea_id, '') = ? AND quitado_at IS NULL`,
			ts, u.ID.String(), orden.String(), idTexto(in.LineaID)); err != nil {
			return err
		}
		id := ids.New()
		if _, err := tx.ExecContext(ctx, `INSERT INTO descuentos (id, orden_id, linea_id, tipo, valor, cortesia, motivo_id, motivo, usuario_id, usuario_nombre, autorizado_por, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id.String(), orden.String(), idOrNil(in.LineaID), in.Tipo, valor.String(), in.Cortesia,
			in.MotivoID.String(), motivo, u.ID.String(), u.Nombre, idOrNil(autoriza), ts); err != nil {
			return err
		}
		if tot, _, _, _, _, err = a.calcularTotales(ctx, tx, o); err != nil {
			return err
		}
		var monto string
		for _, x := range tot.Descuentos {
			if x.ID == id {
				monto = x.Monto
			}
		}
		accion := map[bool]string{true: "CORTESIA_APLICADA", false: "DESCUENTO_APLICADO"}[in.Cortesia]
		uid := u.ID
		mesa = o.MesaID
		return auditar(ctx, tx, accion, "orden", orden, &uid, map[string]any{"monto": monto, "motivo": motivo, "tipo": in.Tipo, "valor": valor.String(),
			"lineaId": in.LineaID, "autorizadoPor": autoriza, "porcentaje": pct.String()}, now)
	})
	if err == nil && mesa != nil {
		a.avisarMesa(ctx, *mesa)
	}
	return tot, err
}

// QuitarDescuento retira un descuento vigente (auditado).
func (a *App) QuitarDescuento(ctx context.Context, u Usuario, orden, descuento ids.ID) (Totales, error) {
	if !u.Puede(rbac.Cobrar) && !u.Puede(rbac.DarDescuento) {
		return Totales{}, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para quitar descuentos.")
	}
	now := a.Clock.Now()
	var tot Totales
	var mesa *ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		o, err := ordenAbierta(ctx, tx, orden)
		if err != nil {
			return err
		}
		antes, _, _, _, _, err := a.calcularTotales(ctx, tx, o)
		if err != nil {
			return err
		}
		var quitado *Descuento
		for _, x := range antes.Descuentos {
			if x.ID == descuento {
				quitado = &x
			}
		}
		if quitado == nil {
			return problema(http.StatusNotFound, "NO_ENCONTRADO", "Ese descuento ya no está vigente.")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE descuentos SET quitado_at = ?, quitado_por = ? WHERE id = ?`, now.Format(time.RFC3339Nano), u.ID.String(), descuento.String()); err != nil {
			return err
		}
		if tot, _, _, _, _, err = a.calcularTotales(ctx, tx, o); err != nil {
			return err
		}
		uid := u.ID
		mesa = o.MesaID
		return auditar(ctx, tx, "DESCUENTO_QUITADO", "orden", orden, &uid, map[string]any{"monto": quitado.Monto, "motivo": quitado.Motivo, "cortesia": quitado.Cortesia}, now)
	})
	if err == nil && mesa != nil {
		a.avisarMesa(ctx, *mesa)
	}
	return tot, err
}

func idTexto(id *ids.ID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
