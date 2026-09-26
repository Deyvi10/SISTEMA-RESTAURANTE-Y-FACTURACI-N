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

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
)

// ---------- Lectura ----------

type ModLinea struct {
	ModificadorID   ids.ID `json:"modificadorId"`
	Nombre          string `json:"nombre"`
	PrecioAdicional string `json:"precioAdicional"`
}

type LineaOrden struct {
	ID             ids.ID     `json:"id"`
	ProductoID     ids.ID     `json:"productoId"`
	Producto       string     `json:"producto"`
	PrecioUnitario string     `json:"precioUnitario"`
	Cantidad       string     `json:"cantidad"`
	Modificadores  []ModLinea `json:"modificadores"`
	Nota           string     `json:"nota"`
	Tiempo         string     `json:"tiempo"`
	EstacionID     ids.ID     `json:"estacionId"`
	Estado         string     `json:"estado"` // EN_ESPERA, ENVIADA, ANULADA
	Total          string     `json:"total"`
	CreadaPor      ids.ID     `json:"creadaPor"`
	CreadaAt       time.Time  `json:"creadaAt"`
	AnuladaMotivo  *string    `json:"anuladaMotivo,omitempty"`
}

type Orden struct {
	ID           ids.ID       `json:"id"`
	MesaID       *ids.ID      `json:"mesaId"`
	Mesa         string       `json:"mesa"`
	Tipo         string       `json:"tipo"`
	MeseroID     ids.ID       `json:"meseroId"`
	MeseroNombre string       `json:"meseroNombre"`
	Numero       int          `json:"numero"`
	Estado       string       `json:"estado"`
	Comensales   *int         `json:"comensales"`
	AbiertaAt    time.Time    `json:"abiertaAt"`
	Lineas       []LineaOrden `json:"lineas"`
	Total        string       `json:"total"`
	Version      int          `json:"version"`
}

var errOrdenNoExiste = problema(http.StatusNotFound, "NO_ENCONTRADO", "Esa orden ya no existe o está cerrada.")

// totalLinea = (precio + modificadores) × cantidad, redondeado a centavos.
func totalLinea(precio string, mods []ModLinea, cantidad string) (money.Money, error) {
	p, err := money.Parse(precio)
	if err != nil {
		return money.Money{}, err
	}
	for _, m := range mods {
		x, err := money.Parse(m.PrecioAdicional)
		if err != nil {
			return money.Money{}, err
		}
		p = p.Add(x)
	}
	q, err := decimal.NewFromString(cantidad)
	if err != nil {
		return money.Money{}, err
	}
	return p.Mul(q).Round2(), nil
}

func leerOrden(ctx context.Context, q queryer, id ids.ID) (Orden, error) {
	var o Orden
	var mesa sql.NullString
	var abierta, mesero string
	var com sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT o.mesa_id, coalesce(m.nombre, 'Orden ' || o.numero_corto), o.tipo, o.mesero_id, o.mesero_nombre, o.numero_corto, o.estado, o.comensales, o.abierta_at, o.version
		FROM ordenes o LEFT JOIN mesas m ON m.id = o.mesa_id WHERE o.id = ?`, id.String()).
		Scan(&mesa, &o.Mesa, &o.Tipo, &mesero, &o.MeseroNombre, &o.Numero, &o.Estado, &com, &abierta, &o.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return o, errOrdenNoExiste
	}
	if err != nil {
		return o, err
	}
	o.ID = id
	o.MeseroID, _ = ids.Parse(mesero)
	if mesa.Valid {
		m, _ := ids.Parse(mesa.String)
		o.MesaID = &m
	}
	if com.Valid {
		c := int(com.Int64)
		o.Comensales = &c
	}
	o.AbiertaAt, _ = time.Parse(time.RFC3339Nano, abierta)
	rows, err := q.QueryContext(ctx, `SELECT id, producto_id, producto_nombre, precio_unitario, cantidad, nota, tiempo, estacion_id, estado, creada_por, creada_at, anulada_motivo
		FROM orden_lineas WHERE orden_id = ? ORDER BY creada_at, rowid`, id.String())
	if err != nil {
		return o, err
	}
	o.Lineas = []LineaOrden{}
	for rows.Next() {
		var l LineaOrden
		var lid, pid, est, por, creada string
		var motivo sql.NullString
		if err := rows.Scan(&lid, &pid, &l.Producto, &l.PrecioUnitario, &l.Cantidad, &l.Nota, &l.Tiempo, &est, &l.Estado, &por, &creada, &motivo); err != nil {
			_ = rows.Close()
			return o, err
		}
		l.ID, _ = ids.Parse(lid)
		l.ProductoID, _ = ids.Parse(pid)
		l.EstacionID, _ = ids.Parse(est)
		l.CreadaPor, _ = ids.Parse(por)
		l.CreadaAt, _ = time.Parse(time.RFC3339Nano, creada)
		if motivo.Valid {
			l.AnuladaMotivo = &motivo.String
		}
		o.Lineas = append(o.Lineas, l)
	}
	_ = rows.Close()
	total := money.Money{}
	for i := range o.Lineas {
		l := &o.Lineas[i]
		mods, err := leerMods(ctx, q, l.ID)
		if err != nil {
			return o, err
		}
		l.Modificadores = mods
		t, err := totalLinea(l.PrecioUnitario, mods, l.Cantidad)
		if err != nil {
			return o, err
		}
		l.Total = t.String()
		if l.Estado != "ANULADA" {
			total = total.Add(t)
		}
	}
	o.Total = total.String()
	return o, nil
}

func leerMods(ctx context.Context, q queryer, linea ids.ID) ([]ModLinea, error) {
	rows, err := q.QueryContext(ctx, `SELECT modificador_id, nombre, precio_adicional FROM orden_linea_modificadores WHERE orden_linea_id = ? ORDER BY rowid`, linea.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ModLinea{}
	for rows.Next() {
		var m ModLinea
		var id string
		if err := rows.Scan(&id, &m.Nombre, &m.PrecioAdicional); err != nil {
			return nil, err
		}
		m.ModificadorID, _ = ids.Parse(id)
		out = append(out, m)
	}
	return out, rows.Err()
}

func totalOrden(ctx context.Context, q queryer, id ids.ID) (string, error) {
	o, err := leerOrden(ctx, q, id)
	return o.Total, err
}

// OrdenDeMesa devuelve la orden abierta de una mesa (404 si está libre).
func (a *App) OrdenDeMesa(ctx context.Context, mesa ids.ID) (Orden, error) {
	var id string
	err := a.Store.Read().QueryRowContext(ctx, `SELECT id FROM ordenes WHERE mesa_id = ? AND estado IN ('ABIERTA','PRECUENTA')`, mesa.String()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Orden{}, problema(http.StatusNotFound, "MESA_LIBRE", "La mesa está libre.")
	}
	if err != nil {
		return Orden{}, err
	}
	oid, _ := ids.Parse(id)
	return leerOrden(ctx, a.Store.Read(), oid)
}

// ---------- Enviar (F3-08…F3-10) ----------

type LineaNueva struct {
	ID            ids.ID   `json:"id"` // UUID v7 generado en el teléfono
	ProductoID    ids.ID   `json:"productoId"`
	Cantidad      string   `json:"cantidad"`
	Modificadores []ids.ID `json:"modificadores"`
	Nota          string   `json:"nota"`
	Tiempo        string   `json:"tiempo"`
	EnEspera      bool     `json:"enEspera"` // «Enviar y mantener»: se imprime al «marchar» su tiempo
}

type EnviarOrdenIn struct {
	IdempotencyKey string       `json:"idempotencyKey"`
	OrdenID        ids.ID       `json:"ordenId"` // UUID v7 del teléfono si la mesa está libre
	MesaID         ids.ID       `json:"mesaId"`
	Comensales     *int         `json:"comensales"`
	Lineas         []LineaNueva `json:"lineas"`
}

type EnviarOrdenOut struct {
	Orden    Orden   `json:"orden"`
	Numero   int     `json:"comandaNumero"`
	Envios   []Envio `json:"envios"`
	Repetida bool    `json:"repetida"`
}

type grupoMods struct {
	nombre      string
	obligatorio bool
	min, max    int
	elegidos    int
}

// resolverMods valida los modificadores elegidos contra los grupos del producto
// (obligatorio, mínimo y máximo) y devuelve su copia con nombre y precio (snapshot).
func resolverMods(ctx context.Context, tx *store.Tx, producto ids.ID, elegidos []ids.ID, nombreProd string) ([]ModLinea, error) {
	grupos := map[string]*grupoMods{}
	rows, err := tx.QueryContext(ctx, `SELECT g.id, g.nombre, g.obligatorio, g.min, g.max FROM producto_grupos_modificadores pg
		JOIN grupos_modificadores g ON g.id = pg.grupo_id AND g.deleted_at IS NULL WHERE pg.producto_id = ?`, producto.String())
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		g := &grupoMods{}
		var ob int
		if err := rows.Scan(&id, &g.nombre, &ob, &g.min, &g.max); err != nil {
			_ = rows.Close()
			return nil, err
		}
		g.obligatorio = ob == 1
		grupos[id] = g
	}
	_ = rows.Close()
	out := []ModLinea{}
	vistos := map[ids.ID]bool{}
	for _, m := range elegidos {
		if vistos[m] {
			continue
		}
		vistos[m] = true
		var grupo, nombre, precio string
		var activo int
		err := tx.QueryRowContext(ctx, `SELECT grupo_id, nombre, precio_adicional, activo FROM modificadores WHERE id = ?`, m.String()).Scan(&grupo, &nombre, &precio, &activo)
		g, ok := grupos[grupo]
		if err != nil || !ok || activo == 0 {
			return nil, invalido("«" + nombreProd + "»: una opción elegida ya no está disponible. Actualiza el menú.")
		}
		g.elegidos++
		out = append(out, ModLinea{ModificadorID: m, Nombre: nombre, PrecioAdicional: precio})
	}
	for _, g := range grupos {
		min := g.min
		if g.obligatorio && min < 1 {
			min = 1
		}
		if g.elegidos < min {
			return nil, invalido(fmt.Sprintf("«%s»: elige %s (%d como mínimo).", nombreProd, strings.ToLower(g.nombre), min))
		}
		if g.elegidos > g.max {
			return nil, invalido(fmt.Sprintf("«%s»: en %s puedes elegir hasta %d.", nombreProd, strings.ToLower(g.nombre), g.max))
		}
	}
	return out, nil
}

func (in *EnviarOrdenIn) validar() error {
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 100 {
		return invalido("Falta la clave de idempotencia (8 a 100 caracteres).")
	}
	if in.MesaID == ids.Nil {
		return invalido("Indica la mesa.")
	}
	if len(in.Lineas) == 0 || len(in.Lineas) > 100 {
		return invalido("Agrega entre 1 y 100 platos.")
	}
	if in.Comensales != nil && (*in.Comensales < 1 || *in.Comensales > 99) {
		return invalido("Los comensales van de 1 a 99.")
	}
	for i := range in.Lineas {
		l := &in.Lineas[i]
		if l.ID.Version() != 7 {
			return invalido(fmt.Sprintf("Plato %d: falta su identificador.", i+1))
		}
		var err error
		if l.Cantidad, l.Nota, l.Tiempo, err = validarLinea(i, l.Cantidad, l.Nota, l.Tiempo); err != nil {
			return err
		}
		if l.EnEspera && l.Tiempo == "" {
			return invalido(fmt.Sprintf("Plato %d: para mantenerlo en espera indica su tiempo (Entrada, Fuerte o Postre).", i+1))
		}
	}
	return nil
}

// EnviarOrden guarda los platos nuevos de una mesa y manda a producción los que no quedan
// en espera, todo en una transacción. Idempotente por IdempotencyKey (QA-08).
func (a *App) EnviarOrden(ctx context.Context, d Dispositivo, u Usuario, in EnviarOrdenIn) (EnviarOrdenOut, error) {
	if !u.Puede(rbac.TomarPedido) {
		return EnviarOrdenOut{}, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para tomar pedidos.")
	}
	if err := in.validar(); err != nil {
		return EnviarOrdenOut{}, err
	}
	now := a.Clock.Now()
	loc := a.zonaLocal(ctx)
	var out EnviarOrdenOut
	var despertar []ids.ID
	var ordenID ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		// ¿Ya se procesó este envío? (el teléfono reintenta tras perder la respuesta)
		var cid, oid sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT id, orden_id, numero FROM comandas WHERE idempotency_key = ?`, in.IdempotencyKey).Scan(&cid, &oid, &out.Numero)
		if err == nil {
			out.Repetida = true
			ordenID, _ = ids.Parse(oid.String)
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		mesaNombre, err := mesaExiste(ctx, tx, in.MesaID)
		if err != nil {
			return err
		}
		if b, err := leerBloqueo(ctx, tx, in.MesaID, now); err != nil {
			return err
		} else if b != nil && (b.UsuarioID != u.ID || b.DispositivoID != d.ID) {
			return errOcupada(b)
		}
		fecha := now.In(loc).Format("2006-01-02")
		// Orden abierta de la mesa o una nueva.
		var abierta, estado string
		err = tx.QueryRowContext(ctx, `SELECT id, estado FROM ordenes WHERE mesa_id = ? AND estado IN ('ABIERTA','PRECUENTA')`, in.MesaID.String()).Scan(&abierta, &estado)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			ordenID = in.OrdenID
			if ordenID.Version() != 7 {
				ordenID = ids.New()
			}
			var numero int
			if err := tx.QueryRowContext(ctx, `INSERT INTO contadores (clave, valor) VALUES (?, 1) ON CONFLICT (clave) DO UPDATE SET valor = valor + 1 RETURNING valor`, "orden:"+fecha).Scan(&numero); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO ordenes (id, mesa_id, tipo, mesero_id, mesero_nombre, numero_corto, fecha_negocio, comensales, abierta_at) VALUES (?, ?, 'MESA', ?, ?, ?, ?, ?, ?)`,
				ordenID.String(), in.MesaID.String(), u.ID.String(), u.Nombre, numero, fecha, in.Comensales, now.Format(time.RFC3339Nano)); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			ordenID, _ = ids.Parse(abierta)
			if estado == "PRECUENTA" {
				// Pidieron algo más tras la pre-cuenta: la mesa vuelve a ocupada.
				if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET estado = 'ABIERTA', precuenta_at = NULL, version = version + 1 WHERE id = ?`, abierta); err != nil {
					return err
				}
			}
			if in.Comensales != nil {
				if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET comensales = ?, version = version + 1 WHERE id = ?`, *in.Comensales, abierta); err != nil {
					return err
				}
			}
		}
		// Líneas: snapshot de nombre, precio, IVA y estación.
		var aImprimir []Linea
		estaciones := map[ids.ID]estacion{}
		for i, l := range in.Lineas {
			var nombre, precio, tarifa string
			var activo int
			var prodEst, catEst *string
			err := tx.QueryRowContext(ctx, `SELECT p.nombre, p.precio, p.tarifa_iva_id, p.activo, p.estacion_id, c.estacion_id FROM productos p
				LEFT JOIN categorias c ON c.id = p.categoria_id AND c.deleted_at IS NULL WHERE p.id = ? AND p.deleted_at IS NULL`, l.ProductoID.String()).
				Scan(&nombre, &precio, &tarifa, &activo, &prodEst, &catEst)
			if errors.Is(err, sql.ErrNoRows) {
				return invalido(fmt.Sprintf("Plato %d: ya no está en el menú. Actualiza la app.", i+1))
			}
			if err != nil {
				return err
			}
			if activo == 0 {
				return invalido("«" + nombre + "» está agotado o desactivado.")
			}
			var porcentaje string
			if err := tx.QueryRowContext(ctx, `SELECT porcentaje FROM tarifas_iva WHERE id = ?`, tarifa).Scan(&porcentaje); err != nil {
				porcentaje = "0"
			}
			mods, err := resolverMods(ctx, tx, l.ProductoID, l.Modificadores, nombre)
			if err != nil {
				return err
			}
			est, err := resolverEstacion(ctx, tx, prodEst, catEst)
			if err != nil {
				return err
			}
			estaciones[est.ID] = est
			estado := "ENVIADA"
			if l.EnEspera {
				estado = "EN_ESPERA"
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO orden_lineas (id, orden_id, producto_id, producto_nombre, precio_unitario, tarifa_iva_id, porcentaje_iva, cantidad, nota, tiempo, estacion_id, estado, creada_por, creada_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				l.ID.String(), ordenID.String(), l.ProductoID.String(), nombre, precio, tarifa, porcentaje, l.Cantidad, l.Nota, l.Tiempo, est.ID.String(), estado, u.ID.String(), now.Format(time.RFC3339Nano)); err != nil {
				if strings.Contains(err.Error(), "UNIQUE") {
					return problema(http.StatusConflict, "LINEA_REPETIDA", "Un plato ya estaba enviado. Actualiza la orden.")
				}
				return err
			}
			nombresMods := make([]string, 0, len(mods))
			for _, m := range mods {
				if _, err := tx.ExecContext(ctx, `INSERT INTO orden_linea_modificadores (id, orden_linea_id, modificador_id, nombre, precio_adicional) VALUES (?, ?, ?, ?, ?)`,
					ids.New().String(), l.ID.String(), m.ModificadorID.String(), m.Nombre, m.PrecioAdicional); err != nil {
					return err
				}
				nombresMods = append(nombresMods, m.Nombre)
			}
			if !l.EnEspera {
				aImprimir = append(aImprimir, Linea{ID: l.ID, ProductoID: l.ProductoID, Producto: nombre, Cantidad: l.Cantidad, Modificadores: nombresMods, Nota: l.Nota, Tiempo: l.Tiempo, EstacionID: est.ID})
			}
		}
		comanda, envios, desp, numero, err := a.comandaDeLineas(ctx, tx, in.IdempotencyKey, ordenID, mesaNombre, u, aImprimir, estaciones, loc, now, fecha)
		if err != nil {
			return err
		}
		out.Envios, out.Numero, despertar = envios, numero, desp
		if comanda != ids.Nil && len(aImprimir) > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE orden_lineas SET comanda_id = ? WHERE orden_id = ? AND estado = 'ENVIADA' AND comanda_id IS NULL`, comanda.String(), ordenID.String()); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET version = version + 1 WHERE id = ?`, ordenID.String()); err != nil {
			return err
		}
		if err := liberarEnTx(ctx, tx, in.MesaID); err != nil {
			return err
		}
		ev, err := edgesync.NewEvent("orden.lineas_enviadas", 1, ordenID, map[string]any{"ordenId": ordenID, "mesaId": in.MesaID, "comandaId": comanda, "lineas": in.Lineas, "meseroId": u.ID}, now)
		if err != nil {
			return err
		}
		_, err = a.outbox.Append(ctx, tx, ev)
		return err
	})
	if err != nil {
		return EnviarOrdenOut{}, err
	}
	a.motor.Despertar(despertar...)
	a.notificarPush()
	o, err := leerOrden(ctx, a.Store.Read(), ordenID)
	if err != nil {
		return out, err
	}
	out.Orden = o
	if !out.Repetida {
		_ = a.hub.Difundir(eventos.TableUnlocked{TableID: in.MesaID, Reason: "RELEASED"})
		mesa := in.MesaID
		_ = a.hub.Difundir(eventos.OrderSubmitted{OrderID: ordenID, ComandaID: ordenID, TableID: &mesa, Number: int64(out.Numero), Lines: int64(len(in.Lineas)), ByUserID: u.ID})
		a.avisarMesa(ctx, in.MesaID)
	}
	return out, nil
}

// comandaDeLineas registra la comanda (siempre, para la idempotencia) y encola los tickets
// de las líneas a imprimir. Devuelve el id de la comanda y su número del día.
func (a *App) comandaDeLineas(ctx context.Context, tx *store.Tx, clave string, orden ids.ID, mesa string, u Usuario, lineas []Linea,
	estaciones map[ids.ID]estacion, loc *time.Location, now time.Time, fecha string,
) (ids.ID, []Envio, []ids.ID, int, error) {
	id := ids.New()
	var numero int
	if err := tx.QueryRowContext(ctx, `INSERT INTO contadores (clave, valor) VALUES (?, 1) ON CONFLICT (clave) DO UPDATE SET valor = valor + 1 RETURNING valor`, "comanda:"+fecha).Scan(&numero); err != nil {
		return ids.Nil, nil, nil, 0, err
	}
	raw, err := json.Marshal(lineas)
	if err != nil {
		return ids.Nil, nil, nil, 0, err
	}
	if lineas == nil {
		raw = []byte("[]")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO comandas (id, idempotency_key, numero, fecha_negocio, mesa, mesero_id, mesero, lineas, enviada_at, orden_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), clave, numero, fecha, mesa, u.ID.String(), u.Nombre, string(raw), now.Format(time.RFC3339Nano), orden.String()); err != nil {
		return ids.Nil, nil, nil, 0, err
	}
	if len(lineas) == 0 {
		return id, []Envio{}, nil, numero, nil
	}
	envios, desp, err := a.encolarPorEstacion(ctx, tx, id, "COMANDA", documento{Mesa: mesa, Mesero: u.Nombre, Numero: numero, Hora: now}, lineas, estaciones, loc, now)
	return id, envios, desp, numero, err
}

// ---------- Marchar tiempos ----------

type MarcharIn struct {
	IdempotencyKey string `json:"idempotencyKey"`
	Tiempo         string `json:"tiempo"` // vacío = todo lo que está en espera
}

// Marchar manda a producción los platos que estaban en espera («Marchar fuerte»).
func (a *App) Marchar(ctx context.Context, u Usuario, orden ids.ID, in MarcharIn) (EnviarOrdenOut, error) {
	if !u.Puede(rbac.TomarPedido) {
		return EnviarOrdenOut{}, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para tomar pedidos.")
	}
	if len(strings.TrimSpace(in.IdempotencyKey)) < 8 {
		return EnviarOrdenOut{}, invalido("Falta la clave de idempotencia.")
	}
	in.Tiempo = strings.ToUpper(strings.TrimSpace(in.Tiempo))
	now := a.Clock.Now()
	loc := a.zonaLocal(ctx)
	var out EnviarOrdenOut
	var despertar []ids.ID
	var mesa *ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		var ya string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM comandas WHERE idempotency_key = ?`, in.IdempotencyKey).Scan(&ya); err == nil {
			out.Repetida = true
			return nil
		}
		o, err := leerOrden(ctx, tx, orden)
		if err != nil {
			return err
		}
		if o.Estado != "ABIERTA" && o.Estado != "PRECUENTA" {
			return errOrdenNoExiste
		}
		mesa = o.MesaID
		var lineas []Linea
		estaciones := map[ids.ID]estacion{}
		for _, l := range o.Lineas {
			if l.Estado != "EN_ESPERA" || (in.Tiempo != "" && l.Tiempo != in.Tiempo) {
				continue
			}
			nm := make([]string, 0, len(l.Modificadores))
			for _, m := range l.Modificadores {
				nm = append(nm, m.Nombre)
			}
			lineas = append(lineas, Linea{ID: l.ID, ProductoID: l.ProductoID, Producto: l.Producto, Cantidad: l.Cantidad, Modificadores: nm, Nota: l.Nota, Tiempo: l.Tiempo, EstacionID: l.EstacionID})
			e, err := estacionPorID(ctx, tx, l.EstacionID.String())
			if err != nil {
				e = estacion{ID: l.EstacionID, Nombre: "Producción"}
			}
			estaciones[l.EstacionID] = e
		}
		if len(lineas) == 0 {
			return invalido("No hay platos en espera para marchar.")
		}
		comanda, envios, desp, numero, err := a.comandaDeLineas(ctx, tx, in.IdempotencyKey, orden, o.Mesa, u, lineas, estaciones, loc, now, now.In(loc).Format("2006-01-02"))
		if err != nil {
			return err
		}
		out.Envios, out.Numero, despertar = envios, numero, desp
		for _, l := range lineas {
			if _, err := tx.ExecContext(ctx, `UPDATE orden_lineas SET estado = 'ENVIADA', comanda_id = ? WHERE id = ?`, comanda.String(), l.ID.String()); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE ordenes SET version = version + 1 WHERE id = ?`, orden.String())
		return err
	})
	if err != nil {
		return out, err
	}
	a.motor.Despertar(despertar...)
	out.Orden, err = leerOrden(ctx, a.Store.Read(), orden)
	if mesa != nil {
		a.avisarMesa(ctx, *mesa)
	}
	return out, err
}

// ---------- Pre-cuenta (F3-12) ----------

// Totales de una orden según la configuración del local (informativo; el cálculo fiscal
// definitivo llega con el motor de impuestos de F5-04).
type Totales struct {
	Subtotal string `json:"subtotal"` // base sin IVA
	IVA      string `json:"iva"`
	Propina  string `json:"propina"`
	Total    string `json:"total"`
}

func (a *App) calcularTotales(ctx context.Context, q queryer, o Orden) (Totales, money.Money, money.Money, money.Money, money.Money, error) {
	var incluye, propActiva int
	var propPct string
	if err := q.QueryRowContext(ctx, `SELECT precios_incluyen_iva, propina_legal_activa, propina_porcentaje FROM locales LIMIT 1`).Scan(&incluye, &propActiva, &propPct); err != nil {
		incluye, propActiva, propPct = 1, 0, "0"
	}
	porTarifa := map[string]money.Money{}
	for _, l := range o.Lineas {
		if l.Estado == "ANULADA" {
			continue
		}
		var pct string
		if err := q.QueryRowContext(ctx, `SELECT porcentaje_iva FROM orden_lineas WHERE id = ?`, l.ID.String()).Scan(&pct); err != nil {
			return Totales{}, money.Money{}, money.Money{}, money.Money{}, money.Money{}, err
		}
		t, err := money.Parse(l.Total)
		if err != nil {
			return Totales{}, money.Money{}, money.Money{}, money.Money{}, money.Money{}, err
		}
		porTarifa[pct] = porTarifa[pct].Add(t)
	}
	var base, iva money.Money
	claves := make([]string, 0, len(porTarifa))
	for k := range porTarifa {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	for _, k := range claves {
		monto := porTarifa[k]
		p, err := decimal.NewFromString(k)
		if err != nil {
			p = decimal.Zero
		}
		f := p.Div(decimal.NewFromInt(100))
		if incluye == 1 {
			b := money.FromDecimal(monto.Decimal().Div(decimal.NewFromInt(1).Add(f))).Round2()
			base, iva = base.Add(b), iva.Add(monto.Sub(b))
		} else {
			base, iva = base.Add(monto), iva.Add(monto.Mul(f).Round2())
		}
	}
	propina := money.Money{}
	if propActiva == 1 {
		p, err := decimal.NewFromString(propPct)
		if err == nil {
			propina = base.Mul(p.Div(decimal.NewFromInt(100))).Round2()
		}
	}
	total := base.Add(iva).Add(propina)
	return Totales{Subtotal: base.String(), IVA: iva.String(), Propina: propina.String(), Total: total.String()}, base, iva, propina, total, nil
}

type PrecuentaIn struct {
	Personas int `json:"personas"`
}

type PrecuentaOut struct {
	Orden      Orden    `json:"orden"`
	Totales    Totales  `json:"totales"`
	Impresoras []string `json:"impresoras"`
	Aviso      string   `json:"aviso,omitempty"`
}

// Precuenta imprime el detalle informativo en la estación de caja y pasa la mesa a «Por pagar».
func (a *App) Precuenta(ctx context.Context, u Usuario, orden ids.ID, in PrecuentaIn) (PrecuentaOut, error) {
	if !u.Puede(rbac.ImprimirPrecuenta) {
		return PrecuentaOut{}, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para imprimir la pre-cuenta.")
	}
	if in.Personas < 0 || in.Personas > 50 {
		return PrecuentaOut{}, invalido("Las personas para dividir van de 1 a 50.")
	}
	now := a.Clock.Now()
	loc := a.zonaLocal(ctx)
	var out PrecuentaOut
	var despertar []ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		o, err := leerOrden(ctx, tx, orden)
		if err != nil {
			return err
		}
		if o.Estado != "ABIERTA" && o.Estado != "PRECUENTA" {
			return errOrdenNoExiste
		}
		tot, base, iva, propina, total, err := a.calcularTotales(ctx, tx, o)
		if err != nil {
			return err
		}
		out.Orden, out.Totales = o, tot
		var local string
		_ = tx.QueryRowContext(ctx, `SELECT nombre FROM locales LIMIT 1`).Scan(&local)
		pc := escpos.PreCuenta{Local: local, Mesa: o.Mesa, Mesero: o.MeseroNombre, Hora: now.In(loc), Subtotal: base, IVA: iva, Propina: propina, Total: total, Personas: in.Personas}
		pc.Lineas = lineasCuenta(o.Lineas)
		// Estación de caja: sus impresoras; si no tiene, la primera impresora activa.
		var cajaID string
		err = tx.QueryRowContext(ctx, `SELECT id FROM estaciones WHERE tipo = 'CAJA' AND deleted_at IS NULL ORDER BY orden LIMIT 1`).Scan(&cajaID)
		var imps []impresion.Impresora
		if err == nil {
			cid, _ := ids.Parse(cajaID)
			imps, err = impresorasDe(ctx, tx, cid)
			if err != nil {
				return err
			}
		}
		if len(imps) == 0 {
			todas, err := a.impresorasActivas(ctx)
			if err != nil {
				return err
			}
			if len(todas) > 0 {
				imps = todas[:1]
				out.Aviso = "La estación de caja no tiene impresora: la pre-cuenta salió en «" + todas[0].Nombre + "»."
			} else {
				out.Aviso = "No hay impresoras configuradas: la mesa quedó por pagar, pero no se imprimió nada."
			}
		}
		out.Impresoras = []string{}
		oid := orden.String()
		for _, imp := range imps {
			t := impresion.Trabajo{ID: ids.New(), ImpresoraID: imp.ID, Tipo: "PRECUENTA", ComandaID: &oid}
			if err := impresion.Encolar(ctx, tx, t, nil, escpos.ImprimirPreCuenta(imp.Ancho, pc), nil, now); err != nil {
				return err
			}
			out.Impresoras = append(out.Impresoras, imp.Nombre)
			despertar = append(despertar, imp.ID)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET estado = 'PRECUENTA', precuenta_at = coalesce(precuenta_at, ?), version = version + 1 WHERE id = ?`,
			now.Format(time.RFC3339Nano), orden.String()); err != nil {
			return err
		}
		uid := u.ID
		return auditar(ctx, tx, "PRECUENTA", "orden", orden, &uid, map[string]any{"total": tot.Total, "personas": in.Personas}, now)
	})
	if err != nil {
		return out, err
	}
	a.motor.Despertar(despertar...)
	if out.Orden.MesaID != nil {
		a.avisarMesa(ctx, *out.Orden.MesaID)
	}
	out.Orden, _ = leerOrden(ctx, a.Store.Read(), orden)
	return out, nil
}

// lineasCuenta arma el detalle para el cliente: sin anuladas y juntando el mismo plato
// (mismos modificadores y precio unitario) aunque haya salido en comandas distintas.
func lineasCuenta(lineas []LineaOrden) []escpos.LineaCuenta {
	var out []escpos.LineaCuenta
	pos := map[string]int{}
	cant := map[string]decimal.Decimal{}
	for _, l := range lineas {
		if l.Estado == "ANULADA" {
			continue
		}
		t, _ := money.Parse(l.Total)
		c, err := decimal.NewFromString(l.Cantidad)
		nombre := l.Producto
		for _, m := range l.Modificadores {
			nombre += " + " + m.Nombre
		}
		if err != nil || c.IsZero() {
			out = append(out, escpos.LineaCuenta{Cantidad: l.Cantidad, Producto: nombre, Total: t})
			continue
		}
		clave := nombre + "|" + t.Decimal().Div(c).StringFixed(money.UnitPriceScale)
		if i, ok := pos[clave]; ok {
			cant[clave] = cant[clave].Add(c)
			out[i].Cantidad = cant[clave].String()
			out[i].Total = out[i].Total.Add(t)
			continue
		}
		pos[clave], cant[clave] = len(out), c
		out = append(out, escpos.LineaCuenta{Cantidad: l.Cantidad, Producto: nombre, Total: t})
	}
	return out
}

// TotalesDe calcula los totales de una orden sin imprimir (para la pantalla de la mesa).
func (a *App) TotalesDe(ctx context.Context, orden ids.ID) (Totales, error) {
	o, err := leerOrden(ctx, a.Store.Read(), orden)
	if err != nil {
		return Totales{}, err
	}
	t, _, _, _, _, err := a.calcularTotales(ctx, a.Store.Read(), o)
	return t, err
}

// ---------- Transferencias y uniones (F3-13) ----------

func (a *App) exigirTransferir(u Usuario) error {
	if !u.Puede(rbac.TransferirMesa) {
		return problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para mover o unir mesas.")
	}
	return nil
}

func ordenAbierta(ctx context.Context, tx *store.Tx, orden ids.ID) (Orden, error) {
	o, err := leerOrden(ctx, tx, orden)
	if err != nil {
		return o, err
	}
	if o.Estado != "ABIERTA" && o.Estado != "PRECUENTA" {
		return o, errOrdenNoExiste
	}
	return o, nil
}

func mesaLibreParaMover(ctx context.Context, tx *store.Tx, d Dispositivo, u Usuario, mesa ids.ID, now time.Time) error {
	if _, err := mesaExiste(ctx, tx, mesa); err != nil {
		return err
	}
	if b, err := leerBloqueo(ctx, tx, mesa, now); err != nil {
		return err
	} else if b != nil && (b.UsuarioID != u.ID || b.DispositivoID != d.ID) {
		return errOcupada(b)
	}
	return nil
}

type MoverIn struct {
	MesaID ids.ID   `json:"mesaId"`
	Lineas []ids.ID `json:"lineas"` // vacío = toda la orden
}

// Mover pasa toda la orden a una mesa libre, o solo algunas líneas a otra mesa (que puede
// tener su propia orden). Una orden que queda sin platos se anula.
func (a *App) Mover(ctx context.Context, d Dispositivo, u Usuario, orden ids.ID, in MoverIn) (Orden, error) {
	if err := a.exigirTransferir(u); err != nil {
		return Orden{}, err
	}
	now := a.Clock.Now()
	loc := a.zonaLocal(ctx)
	var origenMesa *ids.ID
	var destinoOrden ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		o, err := ordenAbierta(ctx, tx, orden)
		if err != nil {
			return err
		}
		origenMesa = o.MesaID
		if o.MesaID != nil && *o.MesaID == in.MesaID {
			return invalido("Elige una mesa distinta.")
		}
		if err := mesaLibreParaMover(ctx, tx, d, u, in.MesaID, now); err != nil {
			return err
		}
		var destino string
		errDest := tx.QueryRowContext(ctx, `SELECT id FROM ordenes WHERE mesa_id = ? AND estado IN ('ABIERTA','PRECUENTA')`, in.MesaID.String()).Scan(&destino)
		uid := u.ID
		if len(in.Lineas) == 0 {
			if errDest == nil {
				return problema(http.StatusConflict, "MESA_OCUPADA", "Esa mesa ya tiene una orden. Usa «Unir mesas».")
			}
			if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET mesa_id = ?, version = version + 1 WHERE id = ?`, in.MesaID.String(), orden.String()); err != nil {
				return err
			}
			destinoOrden = orden
			return auditar(ctx, tx, "ORDEN_MOVIDA", "orden", orden, &uid, map[string]any{"desde": o.MesaID, "hacia": in.MesaID}, now)
		}
		if errDest != nil && !errors.Is(errDest, sql.ErrNoRows) {
			return errDest
		}
		if errors.Is(errDest, sql.ErrNoRows) {
			destinoOrden = ids.New()
			fecha := now.In(loc).Format("2006-01-02")
			var numero int
			if err := tx.QueryRowContext(ctx, `INSERT INTO contadores (clave, valor) VALUES (?, 1) ON CONFLICT (clave) DO UPDATE SET valor = valor + 1 RETURNING valor`, "orden:"+fecha).Scan(&numero); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO ordenes (id, mesa_id, tipo, mesero_id, mesero_nombre, numero_corto, fecha_negocio, abierta_at) VALUES (?, ?, 'MESA', ?, ?, ?, ?, ?)`,
				destinoOrden.String(), in.MesaID.String(), o.MeseroID.String(), o.MeseroNombre, numero, fecha, now.Format(time.RFC3339Nano)); err != nil {
				return err
			}
		} else {
			destinoOrden, _ = ids.Parse(destino)
		}
		propias := map[ids.ID]bool{}
		for _, l := range o.Lineas {
			if l.Estado != "ANULADA" {
				propias[l.ID] = true
			}
		}
		for _, l := range in.Lineas {
			if !propias[l] {
				return invalido("Un plato no pertenece a esta orden o está anulado.")
			}
			if _, err := tx.ExecContext(ctx, `UPDATE orden_lineas SET orden_id = ? WHERE id = ?`, destinoOrden.String(), l.String()); err != nil {
				return err
			}
		}
		if err := anularSiVacia(ctx, tx, orden, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET version = version + 1 WHERE id IN (?, ?)`, orden.String(), destinoOrden.String()); err != nil {
			return err
		}
		return auditar(ctx, tx, "LINEAS_MOVIDAS", "orden", orden, &uid, map[string]any{"lineas": in.Lineas, "hacia": in.MesaID, "ordenDestino": destinoOrden}, now)
	})
	if err != nil {
		return Orden{}, err
	}
	if origenMesa != nil {
		a.avisarMesa(ctx, *origenMesa)
	}
	a.avisarMesa(ctx, in.MesaID)
	a.registrarEventoOrden(ctx, "orden.movida", orden, map[string]any{"ordenId": orden, "mesaId": in.MesaID, "lineas": in.Lineas, "ordenDestino": destinoOrden})
	return leerOrden(ctx, a.Store.Read(), destinoOrden)
}

// anularSiVacia cierra como ANULADA una orden que se quedó sin platos vigentes.
func anularSiVacia(ctx context.Context, tx *store.Tx, orden ids.ID, now time.Time) error {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM orden_lineas WHERE orden_id = ? AND estado <> 'ANULADA'`, orden.String()).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `UPDATE ordenes SET estado = 'ANULADA', cerrada_at = ? WHERE id = ?`, now.Format(time.RFC3339Nano), orden.String())
	return err
}

type UnirIn struct {
	OrdenDestino ids.ID `json:"ordenDestino"`
}

// Unir junta dos mesas en una sola orden (la de destino); la de origen queda anulada y
// apuntando a la que la absorbió.
func (a *App) Unir(ctx context.Context, u Usuario, orden ids.ID, in UnirIn) (Orden, error) {
	if err := a.exigirTransferir(u); err != nil {
		return Orden{}, err
	}
	if orden == in.OrdenDestino {
		return Orden{}, invalido("Elige otra mesa para unir.")
	}
	now := a.Clock.Now()
	var mesas []ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		o, err := ordenAbierta(ctx, tx, orden)
		if err != nil {
			return err
		}
		dest, err := ordenAbierta(ctx, tx, in.OrdenDestino)
		if err != nil {
			return err
		}
		for _, m := range []*ids.ID{o.MesaID, dest.MesaID} {
			if m != nil {
				mesas = append(mesas, *m)
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE orden_lineas SET orden_id = ? WHERE orden_id = ?`, dest.ID.String(), orden.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET estado = 'ANULADA', unida_a = ?, cerrada_at = ?, version = version + 1 WHERE id = ?`, dest.ID.String(), now.Format(time.RFC3339Nano), orden.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET comensales = CASE WHEN comensales IS NULL AND ? IS NULL THEN NULL ELSE coalesce(comensales, 0) + coalesce(?, 0) END, version = version + 1 WHERE id = ?`,
			o.Comensales, o.Comensales, dest.ID.String()); err != nil {
			return err
		}
		uid := u.ID
		return auditar(ctx, tx, "MESAS_UNIDAS", "orden", dest.ID, &uid, map[string]any{"unida": orden}, now)
	})
	if err != nil {
		return Orden{}, err
	}
	for _, m := range mesas {
		a.avisarMesa(ctx, m)
	}
	a.registrarEventoOrden(ctx, "orden.unida", orden, map[string]any{"ordenId": orden, "destino": in.OrdenDestino})
	return leerOrden(ctx, a.Store.Read(), in.OrdenDestino)
}

type TransferirIn struct {
	MeseroID ids.ID `json:"meseroId"`
}

// Transferir pasa la mesa a otro mesero (cambio de turno).
func (a *App) Transferir(ctx context.Context, u Usuario, orden ids.ID, in TransferirIn) (Orden, error) {
	if err := a.exigirTransferir(u); err != nil {
		return Orden{}, err
	}
	now := a.Clock.Now()
	var mesa *ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		o, err := ordenAbierta(ctx, tx, orden)
		if err != nil {
			return err
		}
		mesa = o.MesaID
		nuevo, activo, err := a.cargarUsuario(ctx, tx, in.MeseroID)
		if err != nil || !activo {
			return invalido("Esa persona no está activa.")
		}
		if !nuevo.Puede(rbac.TomarPedido) {
			return invalido(nuevo.Nombre + " no puede atender mesas.")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET mesero_id = ?, mesero_nombre = ?, version = version + 1 WHERE id = ?`, nuevo.ID.String(), nuevo.Nombre, orden.String()); err != nil {
			return err
		}
		uid := u.ID
		return auditar(ctx, tx, "MESA_TRANSFERIDA", "orden", orden, &uid, map[string]any{"de": o.MeseroID, "a": nuevo.ID}, now)
	})
	if err != nil {
		return Orden{}, err
	}
	if mesa != nil {
		a.avisarMesa(ctx, *mesa)
	}
	a.registrarEventoOrden(ctx, "orden.transferida", orden, map[string]any{"ordenId": orden, "meseroId": in.MeseroID})
	return leerOrden(ctx, a.Store.Read(), orden)
}

// registrarEventoOrden deja el cambio en el outbox para la nube (fuera de la transacción
// del cambio solo en operaciones que no son de dinero ni fiscales).
func (a *App) registrarEventoOrden(ctx context.Context, tipo string, orden ids.ID, payload any) {
	ev, err := edgesync.NewEvent(tipo, 1, orden, payload, a.Clock.Now())
	if err != nil {
		return
	}
	if err := a.Store.Write(ctx, func(tx *store.Tx) error { _, err := a.outbox.Append(ctx, tx, ev); return err }); err != nil {
		a.Log.Error("outbox", "tipo", tipo, "err", err)
		return
	}
	a.notificarPush()
}

// ---------- Anular platos enviados (F3-14) ----------

type AnularOrdenIn struct {
	Lineas       []ids.ID `json:"lineas"`
	Motivo       string   `json:"motivo"`
	SePreparo    bool     `json:"sePreparo"`
	Autorizacion string   `json:"autorizacion"` // token del PIN de supervisor, si hace falta
}

// AnularLineasOrden anula platos ya enviados: exige el permiso o la autorización de un
// supervisor, motivo y «¿se preparó?», e imprime ANULACIÓN en su estación.
func (a *App) AnularLineasOrden(ctx context.Context, d Dispositivo, u Usuario, orden ids.ID, in AnularOrdenIn) (Orden, error) {
	in.Motivo = strings.TrimSpace(in.Motivo)
	if len(in.Lineas) == 0 || len([]rune(in.Motivo)) < 3 || len([]rune(in.Motivo)) > 140 {
		return Orden{}, invalido("Elige los platos a anular y escribe el motivo (3 a 140 caracteres).")
	}
	now := a.Clock.Now()
	loc := a.zonaLocal(ctx)
	var despertar []ids.ID
	var mesa *ids.ID
	var autoriza *ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		if !u.Puede(rbac.AnularItemEnviado) {
			if in.Autorizacion == "" {
				return problema(http.StatusForbidden, "REQUIERE_SUPERVISOR", "Anular un plato enviado requiere el PIN de un supervisor.")
			}
			por, err := consumirAutorizacion(ctx, tx, in.Autorizacion, d, rbac.AnularItemEnviado, orden.String(), now)
			if err != nil {
				return err
			}
			autoriza = &por
		}
		o, err := ordenAbierta(ctx, tx, orden)
		if err != nil {
			return err
		}
		mesa = o.MesaID
		porID := map[ids.ID]LineaOrden{}
		for _, l := range o.Lineas {
			porID[l.ID] = l
		}
		var imprimir []Linea
		estaciones := map[ids.ID]estacion{}
		for _, lid := range in.Lineas {
			l, ok := porID[lid]
			if !ok {
				return invalido("Un plato no pertenece a esta orden.")
			}
			if l.Estado == "ANULADA" {
				return problema(http.StatusConflict, "YA_ANULADA", "«"+l.Producto+"» ya estaba anulado.")
			}
			sePreparo := 0
			if in.SePreparo {
				sePreparo = 1
			}
			if _, err := tx.ExecContext(ctx, `UPDATE orden_lineas SET estado = 'ANULADA', anulada_por = ?, anulada_autoriza = ?, anulada_motivo = ?, anulada_at = ?, se_preparo = ? WHERE id = ?`,
				u.ID.String(), idOrNil(autoriza), in.Motivo, now.Format(time.RFC3339Nano), sePreparo, lid.String()); err != nil {
				return err
			}
			if l.Estado == "ENVIADA" { // lo que estaba en espera nunca llegó a cocina
				nm := make([]string, 0, len(l.Modificadores))
				for _, m := range l.Modificadores {
					nm = append(nm, m.Nombre)
				}
				imprimir = append(imprimir, Linea{ID: l.ID, ProductoID: l.ProductoID, Producto: l.Producto, Cantidad: l.Cantidad, Modificadores: nm, Nota: l.Nota, Tiempo: l.Tiempo, EstacionID: l.EstacionID})
				e, err := estacionPorID(ctx, tx, l.EstacionID.String())
				if err != nil {
					e = estacion{ID: l.EstacionID, Nombre: "Producción"}
				}
				estaciones[l.EstacionID] = e
			}
		}
		if len(imprimir) > 0 {
			var numero int
			_ = tx.QueryRowContext(ctx, `SELECT numero_corto FROM ordenes WHERE id = ?`, orden.String()).Scan(&numero)
			doc := documento{Mesa: o.Mesa, Mesero: u.Nombre, Numero: numero, Hora: now, Anulacion: true, Motivo: in.Motivo, Impreso: now}
			_, desp, err := a.encolarPorEstacion(ctx, tx, orden, "ANULACION", doc, imprimir, estaciones, loc, now)
			if err != nil {
				return err
			}
			despertar = desp
		}
		if err := anularSiVacia(ctx, tx, orden, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET version = version + 1 WHERE id = ?`, orden.String()); err != nil {
			return err
		}
		uid := u.ID
		if err := auditar(ctx, tx, "LINEA_ANULADA", "orden", orden, &uid, map[string]any{"lineas": in.Lineas, "motivo": in.Motivo, "sePreparo": in.SePreparo, "autorizadoPor": autoriza}, now); err != nil {
			return err
		}
		ev, err := edgesync.NewEvent("orden.lineas_anuladas", 1, orden, map[string]any{"ordenId": orden, "lineas": in.Lineas, "motivo": in.Motivo, "sePreparo": in.SePreparo, "usuarioId": u.ID, "autorizadoPor": autoriza}, now)
		if err != nil {
			return err
		}
		_, err = a.outbox.Append(ctx, tx, ev)
		return err
	})
	if err != nil {
		return Orden{}, err
	}
	a.motor.Despertar(despertar...)
	a.notificarPush()
	for _, lid := range in.Lineas {
		_ = a.hub.Difundir(eventos.OrderLineVoided{OrderID: orden, LineID: lid, ByUserID: u.ID, AuthorizedBy: autoriza, Reason: in.Motivo})
	}
	if mesa != nil {
		a.avisarMesa(ctx, *mesa)
	}
	return leerOrden(ctx, a.Store.Read(), orden)
}
