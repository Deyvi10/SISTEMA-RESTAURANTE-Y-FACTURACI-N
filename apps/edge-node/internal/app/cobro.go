package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
)

// EventoVentaCobrada lleva a la nube el documento y el pago de una venta (dinero: en la
// misma transacción del cobro).
const EventoVentaCobrada = "venta.cobrada"

// Identificación del consumidor final según el SRI (docs/05 §12).
const (
	consumidorFinalID     = "9999999999999"
	consumidorFinalNombre = "CONSUMIDOR FINAL"
)

type CobrarIn struct {
	CajaID          ids.ID `json:"cajaId"`
	MetodoID        ids.ID `json:"metodoId"`
	Recibido        string `json:"recibido"` // efectivo entregado; vacío = exacto
	ConsumidorFinal bool   `json:"consumidorFinal"`
	Referencia      string `json:"referencia"`
	Lote            string `json:"lote"`
	Ultimos4        string `json:"ultimos4"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

// DocumentoVenta es lo que devuelve el cobro: el documento emitido y el vuelto.
type DocumentoVenta struct {
	ID        ids.ID    `json:"id"`
	Tipo      string    `json:"tipo"` // INTERNO (en F5, el comprobante electrónico)
	Numero    int       `json:"numero"`
	Codigo    string    `json:"codigo"` // INT-000123
	OrdenID   ids.ID    `json:"ordenId"`
	Mesa      string    `json:"mesa"`
	Comprador string    `json:"comprador"`
	Totales   Totales   `json:"totales"`
	Metodo    string    `json:"metodo"`
	MetodoID  ids.ID    `json:"metodoId"`
	Recibido  string    `json:"recibido"`
	Vuelto    string    `json:"vuelto"`
	AbreCajon bool      `json:"abreCajon"`
	Cajero    string    `json:"cajero"`
	TurnoID   ids.ID    `json:"turnoId"`
	EmitidoAt time.Time `json:"emitidoAt"`
}

type CobroOut struct {
	Documento  DocumentoVenta `json:"documento"`
	Impresoras []string       `json:"impresoras"`
	Aviso      string         `json:"aviso,omitempty"`
}

var ultimos4Re = regexp.MustCompile(`^\d{4}$`)

func recortar(s string, max int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		return string(r[:max])
	}
	return s
}

// Cobrar cobra la orden completa con un método en un solo gesto (RF-04-03.3): registra el
// pago en el turno, emite el documento, cierra la orden (la mesa queda libre), imprime con el
// pulso del cajón si el método lo abre y deja el evento para la nube. Todo o nada.
func (a *App) Cobrar(ctx context.Context, u Usuario, orden ids.ID, in CobrarIn) (CobroOut, error) {
	var out CobroOut
	if !u.Puede(rbac.Cobrar) {
		return out, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para cobrar.")
	}
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 64 {
		return out, invalido("Falta la clave de idempotencia del cobro.")
	}
	if !in.ConsumidorFinal {
		return out, invalido("Por ahora solo se cobra a consumidor final; los datos del comprador llegan con la facturación (F4-07).")
	}
	in.Referencia, in.Lote, in.Ultimos4 = recortar(in.Referencia, 40), recortar(in.Lote, 20), strings.TrimSpace(in.Ultimos4)
	if in.Ultimos4 != "" && !ultimos4Re.MatchString(in.Ultimos4) {
		return out, invalido("Los últimos 4 dígitos de la tarjeta son 4 números.")
	}
	var recibido money.Money
	if strings.TrimSpace(in.Recibido) != "" {
		m, err := montoNoNegativo(in.Recibido, "El monto recibido")
		if err != nil {
			return out, err
		}
		recibido = m
	}
	now, loc := a.Clock.Now(), a.zonaLocal(ctx)
	var despertar []ids.ID
	var mesa *ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		// Reintento o doble toque con la misma clave: el mismo documento, sin cobrar dos veces.
		var datos string
		err := tx.QueryRowContext(ctx, `SELECT datos FROM documentos_venta WHERE idempotency_key = ?`, in.IdempotencyKey).Scan(&datos)
		if err == nil {
			return json.Unmarshal([]byte(datos), &out.Documento)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		t, err := turnoAbiertoDe(ctx, tx, in.CajaID)
		if err != nil {
			return err
		}
		o, err := leerOrden(ctx, tx, orden)
		if errors.Is(err, errOrdenNoExiste) || (err == nil && o.Estado != "ABIERTA" && o.Estado != "PRECUENTA") {
			return problema(http.StatusConflict, "ORDEN_CERRADA", "Esta orden ya se cobró o se cerró.")
		}
		if err != nil {
			return err
		}
		if o.MesaID != nil {
			b, err := leerBloqueo(ctx, tx, *o.MesaID, now)
			if err != nil {
				return err
			}
			if b != nil {
				return errOcupada(b)
			}
		}
		tot, base, iva, propina, total, err := a.calcularTotales(ctx, tx, o)
		if err != nil {
			return err
		}
		if !total.GreaterThan(money.Money{}) {
			return problema(http.StatusConflict, "ORDEN_VACIA", "La orden no tiene nada que cobrar.")
		}
		if max := consumidorFinalMaximo(ctx, tx); total.GreaterThan(max) {
			return problema(http.StatusUnprocessableEntity, "CONSUMIDOR_FINAL_EXCEDIDO",
				"El total supera $"+max.String()+", el máximo para consumidor final. Hacen falta los datos del comprador.")
		}
		var metodo, tipo string
		var abreCajon bool
		err = tx.QueryRowContext(ctx, `SELECT nombre, tipo, abre_cajon FROM metodos_pago WHERE id = ? AND deleted_at IS NULL AND activo = 1`, in.MetodoID.String()).
			Scan(&metodo, &tipo, &abreCajon)
		if errors.Is(err, sql.ErrNoRows) {
			return invalido("Ese método de pago no existe o está desactivado.")
		}
		if err != nil {
			return err
		}
		// Efectivo: lo recibido cubre el total y la diferencia es el vuelto. Otros: el total exacto.
		vuelto := money.Money{}
		if tipo == "EFECTIVO" {
			if recibido.IsZero() {
				recibido = total
			}
			if recibido.LessThan(total) {
				return invalido("Lo recibido ($" + recibido.String() + ") no alcanza para el total ($" + total.String() + ").")
			}
			vuelto = recibido.Sub(total)
		} else {
			if !recibido.IsZero() && !recibido.Equal(total) {
				return invalido("Con " + metodo + " se cobra el total exacto. Para combinar métodos usa el pago mixto.")
			}
			recibido = money.Money{}
		}
		var numero int
		if err := tx.QueryRowContext(ctx, `INSERT INTO contadores (clave, valor) VALUES ('documento:INTERNO', 1)
			ON CONFLICT (clave) DO UPDATE SET valor = valor + 1 RETURNING valor`).Scan(&numero); err != nil {
			return err
		}
		doc := DocumentoVenta{ID: ids.New(), Tipo: "INTERNO", Numero: numero, Codigo: escpos.NumeroInterno(numero), OrdenID: o.ID, Mesa: o.Mesa,
			Comprador: consumidorFinalNombre, Totales: tot, Metodo: metodo, MetodoID: in.MetodoID, Recibido: recibido.String(), Vuelto: vuelto.String(),
			AbreCajon: abreCajon, Cajero: u.Nombre, TurnoID: t.ID, EmitidoAt: now}
		raw, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		ts := now.Format(time.RFC3339Nano)
		if _, err := tx.ExecContext(ctx, `INSERT INTO documentos_venta (id, numero, orden_id, turno_id, comprador_tipo, comprador_identificacion, comprador_nombre,
			subtotal, iva, propina, total, datos, idempotency_key, emitido_por, created_at) VALUES (?, ?, ?, ?, 'CONSUMIDOR_FINAL', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			doc.ID.String(), numero, o.ID.String(), t.ID.String(), consumidorFinalID, consumidorFinalNombre,
			base.String(), iva.String(), propina.String(), total.String(), string(raw), in.IdempotencyKey, u.ID.String(), ts); err != nil {
			return err
		}
		var rec, vue any
		if tipo == "EFECTIVO" {
			rec, vue = recibido.String(), vuelto.String()
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO pagos (id, orden_id, turno_id, metodo_pago_id, monto, recibido, vuelto, referencia, lote, ultimos4, documento_id, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, nullif(?, ''), nullif(?, ''), nullif(?, ''), ?, ?)`,
			ids.New().String(), o.ID.String(), t.ID.String(), in.MetodoID.String(), total.String(), rec, vue, in.Referencia, in.Lote, in.Ultimos4, doc.ID.String(), ts); err != nil {
			return err
		}
		// La orden se cierra y la mesa queda libre; se suelta cualquier bloqueo vencido.
		if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET estado = 'CERRADA', cerrada_at = ?, version = version + 1 WHERE id = ?`, ts, o.ID.String()); err != nil {
			return err
		}
		if o.MesaID != nil {
			if _, err := tx.ExecContext(ctx, `DELETE FROM bloqueos_mesa WHERE mesa_id = ?`, o.MesaID.String()); err != nil {
				return err
			}
			mesa = o.MesaID
		}
		// Impresión en la estación de la caja, con el pulso del cajón delante.
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
		switch {
		case respaldo != "":
			out.Aviso = "La caja no tiene impresora: el documento salió en «" + respaldo + "»."
		case len(imps) == 0:
			out.Aviso = "No hay impresoras configuradas: la venta quedó registrada, pero no se imprimió."
		}
		var local string
		_ = tx.QueryRowContext(ctx, `SELECT nombre FROM locales LIMIT 1`).Scan(&local)
		ticket := escpos.DocumentoVenta{Local: local, Numero: numero, Hora: now.In(loc), Mesa: o.Mesa, Cajero: u.Nombre, Comprador: consumidorFinalNombre,
			Lineas: lineasCuenta(o.Lineas), Subtotal: base, IVA: iva, Propina: propina, Total: total, Metodo: metodo, AbrirCajon: abreCajon}
		if tipo == "EFECTIVO" {
			ticket.Recibido, ticket.Vuelto = recibido, vuelto
		}
		out.Impresoras = []string{}
		oid := o.ID.String()
		for i, imp := range imps {
			ticket.AbrirCajon = abreCajon && i == 0 // un solo pulso: el cajón cuelga de la primera
			tr := impresion.Trabajo{ID: ids.New(), ImpresoraID: imp.ID, Tipo: "VENTA", ComandaID: &oid}
			if err := impresion.Encolar(ctx, tx, tr, nil, escpos.ImprimirDocumentoVenta(imp.Ancho, ticket), nil, now); err != nil {
				return err
			}
			out.Impresoras = append(out.Impresoras, imp.Nombre)
			despertar = append(despertar, imp.ID)
		}
		uid := u.ID
		if err := auditar(ctx, tx, "COBRO", "orden", o.ID, &uid, map[string]any{"documento": doc.Codigo, "total": total.String(), "metodo": metodo, "turno": t.ID}, now); err != nil {
			return err
		}
		out.Documento = doc
		return a.eventoCaja(ctx, tx, EventoVentaCobrada, doc.ID, map[string]any{"documento": doc, "lineas": o.Lineas}, now)
	})
	if err != nil {
		return CobroOut{}, err
	}
	a.motor.Despertar(despertar...)
	if mesa != nil {
		_ = a.hub.Difundir(eventos.TableUnlocked{TableID: *mesa, Reason: "RELEASED"})
		a.avisarMesa(ctx, *mesa)
	}
	a.notificarPush()
	return out, nil
}
