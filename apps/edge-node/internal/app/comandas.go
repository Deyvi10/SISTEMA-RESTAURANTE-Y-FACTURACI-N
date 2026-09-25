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
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// ---------- Entrada ----------

// LineaIn es un plato de la comanda tal como lo envía la app o la caja.
type LineaIn struct {
	ID            *ids.ID  `json:"id"`
	ProductoID    ids.ID   `json:"productoId"`
	Cantidad      string   `json:"cantidad"`
	Modificadores []string `json:"modificadores"`
	Nota          string   `json:"nota"`
	Tiempo        string   `json:"tiempo"`
}

// ComandaIn es el envío de una comanda (RF-02-04). IdempotencyKey evita imprimir dos veces
// si el mesero toca «Enviar» otra vez o la red reintenta.
type ComandaIn struct {
	IdempotencyKey string    `json:"idempotencyKey"`
	Mesa           string    `json:"mesa"`
	Mesero         string    `json:"mesero"`
	MeseroID       *ids.ID   `json:"meseroId"`
	Lineas         []LineaIn `json:"lineas"`
}

// Linea es lo que queda guardado: copia del nombre y la estación resuelta (nunca la IP).
type Linea struct {
	ID            ids.ID   `json:"id"`
	ProductoID    ids.ID   `json:"productoId"`
	Producto      string   `json:"producto"`
	Cantidad      string   `json:"cantidad"`
	Modificadores []string `json:"modificadores,omitempty"`
	Nota          string   `json:"nota,omitempty"`
	Tiempo        string   `json:"tiempo,omitempty"`
	EstacionID    ids.ID   `json:"estacionId"`
}

// Envio describe a dónde fue cada parte de la comanda (para mostrarlo al mesero).
type Envio struct {
	EstacionID ids.ID   `json:"estacionId"`
	Estacion   string   `json:"estacion"`
	Lineas     int      `json:"lineas"`
	Impresoras []string `json:"impresoras"`
	Aviso      string   `json:"aviso,omitempty"`
}

type ComandaOut struct {
	ID        ids.ID    `json:"id"`
	Numero    int       `json:"numero"`
	EnviadaAt time.Time `json:"enviadaAt"`
	Repetida  bool      `json:"repetida"`
	Envios    []Envio   `json:"envios"`
}

var tiemposValidos = map[string]bool{"": true, "BEBIDA": true, "ENTRADA": true, "FUERTE": true, "POSTRE": true}

func invalido(detalle string) error {
	return problema(http.StatusUnprocessableEntity, "DATOS_INVALIDOS", detalle)
}

func (in *ComandaIn) validar() error {
	in.Mesa, in.Mesero, in.IdempotencyKey = strings.TrimSpace(in.Mesa), strings.TrimSpace(in.Mesero), strings.TrimSpace(in.IdempotencyKey)
	switch {
	case len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 100:
		return invalido("Falta la clave de idempotencia (8 a 100 caracteres).")
	case in.Mesa == "" || len([]rune(in.Mesa)) > 40:
		return invalido("Indica la mesa u orden (hasta 40 caracteres).")
	case in.Mesero == "" || len([]rune(in.Mesero)) > 40:
		return invalido("Indica el nombre del mesero (hasta 40 caracteres).")
	case len(in.Lineas) == 0 || len(in.Lineas) > 100:
		return invalido("La comanda debe tener entre 1 y 100 platos.")
	}
	for i := range in.Lineas {
		l := &in.Lineas[i]
		q, err := decimal.NewFromString(strings.TrimSpace(l.Cantidad))
		if err != nil || !q.IsPositive() || q.GreaterThan(decimal.NewFromInt(999)) || q.Exponent() < -3 {
			return invalido(fmt.Sprintf("Plato %d: la cantidad debe ser un número mayor a 0, con hasta 3 decimales.", i+1))
		}
		l.Cantidad = q.String()
		l.Nota = strings.TrimSpace(l.Nota)
		l.Tiempo = strings.ToUpper(strings.TrimSpace(l.Tiempo))
		if len([]rune(l.Nota)) > 140 {
			return invalido(fmt.Sprintf("Plato %d: la nota admite hasta 140 caracteres.", i+1))
		}
		if !tiemposValidos[l.Tiempo] {
			return invalido(fmt.Sprintf("Plato %d: el tiempo debe ser BEBIDA, ENTRADA, FUERTE o POSTRE.", i+1))
		}
		if len(l.Modificadores) > 15 {
			return invalido(fmt.Sprintf("Plato %d: demasiados modificadores.", i+1))
		}
		for j, m := range l.Modificadores {
			m = strings.TrimSpace(m)
			if m == "" || len([]rune(m)) > 60 {
				return invalido(fmt.Sprintf("Plato %d: modificador vacío o demasiado largo.", i+1))
			}
			l.Modificadores[j] = m
		}
	}
	return nil
}

// ---------- Estaciones e impresoras desde la réplica ----------

type estacion struct {
	ID     ids.ID
	Nombre string
}

// resolverEstacion: producto → su categoría → estación por defecto → cualquier estación de
// producción. Nunca se pierde una comanda (RF-02-03.4).
func resolverEstacion(ctx context.Context, q queryer, productoEst, categoriaEst *string) (estacion, error) {
	for _, cand := range []*string{productoEst, categoriaEst} {
		if cand == nil {
			continue
		}
		if e, err := estacionPorID(ctx, q, *cand); err == nil {
			return e, nil
		}
	}
	var id, nombre string
	err := q.QueryRowContext(ctx, `SELECT id, nombre FROM estaciones WHERE deleted_at IS NULL AND tipo = 'PRODUCCION'
		ORDER BY es_defecto DESC, orden, nombre LIMIT 1`).Scan(&id, &nombre)
	if errors.Is(err, sql.ErrNoRows) {
		return estacion{}, problema(http.StatusConflict, "SIN_ESTACIONES", "Este local no tiene estaciones de producción. Créalas en el backoffice (Salón › Estaciones).")
	}
	if err != nil {
		return estacion{}, err
	}
	pid, err := ids.Parse(id)
	return estacion{ID: pid, Nombre: nombre}, err
}

func estacionPorID(ctx context.Context, q queryer, id string) (estacion, error) {
	var nombre string
	if err := q.QueryRowContext(ctx, `SELECT nombre FROM estaciones WHERE id = ? AND deleted_at IS NULL AND tipo = 'PRODUCCION'`, id).Scan(&nombre); err != nil {
		return estacion{}, err
	}
	pid, err := ids.Parse(id)
	return estacion{ID: pid, Nombre: nombre}, err
}

type queryer interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}

// impresorasDe devuelve dónde imprime una estación: la redirección si existe, si no las
// impresoras asignadas (activas).
func impresorasDe(ctx context.Context, q queryer, est ids.ID) ([]impresion.Impresora, error) {
	var redir string
	err := q.QueryRowContext(ctx, `SELECT impresora_id FROM redirecciones WHERE estacion_id = ?`, est.String()).Scan(&redir)
	var rows *sql.Rows
	switch {
	case err == nil:
		rows, err = q.QueryContext(ctx, `SELECT id, nombre, coalesce(host, ''), coalesce(puerto, 9100), ancho_papel, `+colaWindows+` FROM impresoras
			WHERE id = ? AND deleted_at IS NULL AND activa = 1`, redir)
	case errors.Is(err, sql.ErrNoRows):
		rows, err = q.QueryContext(ctx, `SELECT i.id, i.nombre, coalesce(i.host, ''), coalesce(i.puerto, 9100), i.ancho_papel, `+colaWindowsI+` FROM impresoras i
			JOIN estacion_impresoras ei ON ei.impresora_id = i.id
			WHERE ei.estacion_id = ? AND i.deleted_at IS NULL AND i.activa = 1 ORDER BY i.nombre`, est.String())
	}
	if err != nil {
		return nil, err
	}
	return leerImpresoras(rows)
}

// Cola de Windows solo si la impresora se usa por el spooler (las de red van directo).
const (
	colaWindows  = `CASE WHEN conexion = 'WINDOWS' THEN coalesce(nombre_windows, '') ELSE '' END`
	colaWindowsI = `CASE WHEN i.conexion = 'WINDOWS' THEN coalesce(i.nombre_windows, '') ELSE '' END`
)

func leerImpresoras(rows *sql.Rows) ([]impresion.Impresora, error) {
	defer func() { _ = rows.Close() }()
	var out []impresion.Impresora
	for rows.Next() {
		var id string
		var x impresion.Impresora
		var ancho int
		if err := rows.Scan(&id, &x.Nombre, &x.Host, &x.Puerto, &ancho, &x.ColaWindows); err != nil {
			return nil, err
		}
		x.ID, _ = ids.Parse(id)
		x.Ancho = escpos.Paper(ancho)
		out = append(out, x)
	}
	return out, rows.Err()
}

// impresorasActivas es la configuración del motor (impresoras de red activas).
func (a *App) impresorasActivas(ctx context.Context) ([]impresion.Impresora, error) {
	rows, err := a.Store.Read().QueryContext(ctx, `SELECT id, nombre, coalesce(host, ''), coalesce(puerto, 9100), ancho_papel, `+colaWindows+` FROM impresoras
		WHERE deleted_at IS NULL AND activa = 1 AND (
			(conexion = 'TCP' AND host IS NOT NULL AND puerto IS NOT NULL) OR (conexion = 'WINDOWS' AND nombre_windows IS NOT NULL))`)
	if err != nil {
		return nil, err
	}
	return leerImpresoras(rows)
}

// zona del local para imprimir la hora (la base guarda UTC).
func (a *App) zonaLocal(ctx context.Context) *time.Location {
	var z string
	if err := a.Store.Read().QueryRowContext(ctx, `SELECT zona_horaria FROM locales LIMIT 1`).Scan(&z); err == nil {
		if loc, err := time.LoadLocation(z); err == nil {
			return loc
		}
	}
	return clock.Guayaquil
}

// ---------- Enviar ----------

// EnviarComanda guarda la comanda, la separa por estación y encola un ticket por impresora,
// todo en una transacción. Luego despierta las impresoras: imprimen en paralelo.
func (a *App) EnviarComanda(ctx context.Context, in ComandaIn) (ComandaOut, error) {
	if err := in.validar(); err != nil {
		return ComandaOut{}, err
	}
	now := a.Clock.Now()
	loc := a.zonaLocal(ctx)
	out := ComandaOut{ID: ids.New(), EnviadaAt: now}
	var despertar []ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		// Idempotencia: la misma clave devuelve la comanda original sin reimprimir.
		var id string
		var enviada string
		err := tx.QueryRowContext(ctx, `SELECT id, numero, enviada_at FROM comandas WHERE idempotency_key = ?`, in.IdempotencyKey).Scan(&id, &out.Numero, &enviada)
		if err == nil {
			out.ID, _ = ids.Parse(id)
			out.EnviadaAt, _ = time.Parse(time.RFC3339Nano, enviada)
			out.Repetida = true
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		lineas := make([]Linea, 0, len(in.Lineas))
		estaciones := map[ids.ID]estacion{}
		for i, l := range in.Lineas {
			var nombre string
			var activo int
			var prodEst, catEst *string
			err := tx.QueryRowContext(ctx, `SELECT p.nombre, p.activo, p.estacion_id, c.estacion_id FROM productos p
				LEFT JOIN categorias c ON c.id = p.categoria_id AND c.deleted_at IS NULL
				WHERE p.id = ? AND p.deleted_at IS NULL`, l.ProductoID.String()).Scan(&nombre, &activo, &prodEst, &catEst)
			if errors.Is(err, sql.ErrNoRows) {
				return invalido(fmt.Sprintf("Plato %d: ese producto ya no está en el menú. Actualiza la app.", i+1))
			}
			if err != nil {
				return err
			}
			if activo == 0 {
				return invalido(fmt.Sprintf("«%s» está desactivado en el menú.", nombre))
			}
			est, err := resolverEstacion(ctx, tx, prodEst, catEst)
			if err != nil {
				return err
			}
			estaciones[est.ID] = est
			lid := ids.New()
			if l.ID != nil && l.ID.Version() == 7 {
				lid = *l.ID
			}
			lineas = append(lineas, Linea{ID: lid, ProductoID: l.ProductoID, Producto: nombre, Cantidad: l.Cantidad,
				Modificadores: l.Modificadores, Nota: l.Nota, Tiempo: l.Tiempo, EstacionID: est.ID})
		}
		fecha := now.In(loc).Format("2006-01-02")
		if err := tx.QueryRowContext(ctx, `INSERT INTO contadores (clave, valor) VALUES (?, 1)
			ON CONFLICT (clave) DO UPDATE SET valor = valor + 1 RETURNING valor`, "comanda:"+fecha).Scan(&out.Numero); err != nil {
			return err
		}
		raw, err := json.Marshal(lineas)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO comandas (id, idempotency_key, numero, fecha_negocio, mesa, mesero_id, mesero, lineas, enviada_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, out.ID.String(), in.IdempotencyKey, out.Numero, fecha, in.Mesa, idOrNil(in.MeseroID), in.Mesero, string(raw), now.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		c := documento{Mesa: in.Mesa, Mesero: in.Mesero, Numero: out.Numero, Hora: now}
		envios, desp, err := a.encolarPorEstacion(ctx, tx, out.ID, "COMANDA", c, lineas, estaciones, loc, now)
		if err != nil {
			return err
		}
		out.Envios, despertar = envios, desp
		ev, err := edgesync.NewEvent("comanda.enviada", 1, out.ID, map[string]any{"id": out.ID, "numero": out.Numero, "mesa": in.Mesa, "lineas": lineas}, now)
		if err != nil {
			return err
		}
		_, err = a.outbox.Append(ctx, tx, ev)
		return err
	})
	if err != nil {
		return ComandaOut{}, err
	}
	a.motor.Despertar(despertar...)
	a.notificarPush()
	return out, nil
}

// documento son los datos comunes de un ticket de comanda (sin la estación ni las líneas).
type documento struct {
	Mesa, Mesero string
	Numero       int
	Hora         time.Time
	Reimpresion  bool
	Anulacion    bool
	Motivo       string
	Impreso      time.Time
}

func (d documento) comanda(est string, lineas []Linea, loc *time.Location) escpos.Comanda {
	c := escpos.Comanda{Estacion: est, Mesa: d.Mesa, Mesero: d.Mesero, Numero: d.Numero, Hora: d.Hora.In(loc),
		Reimpresion: d.Reimpresion, Anulacion: d.Anulacion, Motivo: d.Motivo}
	if !d.Impreso.IsZero() {
		c.Impreso = d.Impreso.In(loc)
	}
	for _, l := range lineas {
		c.Lineas = append(c.Lineas, escpos.LineaComanda{Cantidad: l.Cantidad, Producto: l.Producto, Modificadores: l.Modificadores, Nota: l.Nota, Tiempo: l.Tiempo})
	}
	return c
}

// encolarPorEstacion agrupa las líneas por estación y encola un ticket por impresora.
func (a *App) encolarPorEstacion(ctx context.Context, tx *store.Tx, comanda ids.ID, tipo string, d documento, lineas []Linea,
	estaciones map[ids.ID]estacion, loc *time.Location, now time.Time,
) ([]Envio, []ids.ID, error) {
	porEst := map[ids.ID][]Linea{}
	for _, l := range lineas {
		porEst[l.EstacionID] = append(porEst[l.EstacionID], l)
	}
	orden := make([]ids.ID, 0, len(porEst))
	for id := range porEst {
		orden = append(orden, id)
	}
	sort.Slice(orden, func(i, j int) bool { return estaciones[orden[i]].Nombre < estaciones[orden[j]].Nombre })
	var envios []Envio
	var despertar []ids.ID
	cid := comanda.String()
	for _, estID := range orden {
		est := estaciones[estID]
		ls := porEst[estID]
		env := Envio{EstacionID: estID, Estacion: est.Nombre, Lineas: len(ls), Impresoras: []string{}}
		imps, err := impresorasDe(ctx, tx, estID)
		if err != nil {
			return nil, nil, err
		}
		if len(imps) == 0 {
			env.Aviso = "La estación «" + est.Nombre + "» no tiene impresora asignada. Asígnale una en el backoffice (Impresoras)."
		}
		doc := d.comanda(est.Nombre, ls, loc)
		docJSON, err := json.Marshal(doc)
		if err != nil {
			return nil, nil, err
		}
		for _, imp := range imps {
			t := impresion.Trabajo{ID: ids.New(), ImpresoraID: imp.ID, Tipo: tipo, ComandaID: &cid}
			if err := impresion.Encolar(ctx, tx, t, &estID, escpos.ImprimirComanda(imp.Ancho, doc), docJSON, now); err != nil {
				return nil, nil, err
			}
			env.Impresoras = append(env.Impresoras, imp.Nombre)
			despertar = append(despertar, imp.ID)
		}
		envios = append(envios, env)
	}
	return envios, despertar, nil
}

func idOrNil(id *ids.ID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

// cargarComanda lee una comanda guardada.
func cargarComanda(ctx context.Context, q queryer, id ids.ID) (documento, []Linea, error) {
	var d documento
	var raw, enviada string
	err := q.QueryRowContext(ctx, `SELECT mesa, mesero, numero, lineas, enviada_at FROM comandas WHERE id = ?`, id.String()).Scan(&d.Mesa, &d.Mesero, &d.Numero, &raw, &enviada)
	if errors.Is(err, sql.ErrNoRows) {
		return d, nil, problema(http.StatusNotFound, "NO_ENCONTRADO", "No existe esa comanda.")
	}
	if err != nil {
		return d, nil, err
	}
	d.Hora, _ = time.Parse(time.RFC3339Nano, enviada)
	var ls []Linea
	return d, ls, json.Unmarshal([]byte(raw), &ls)
}

func estacionesDe(ctx context.Context, q queryer, ls []Linea) map[ids.ID]estacion {
	m := map[ids.ID]estacion{}
	for _, l := range ls {
		if _, ok := m[l.EstacionID]; ok {
			continue
		}
		var nombre string
		if err := q.QueryRowContext(ctx, `SELECT nombre FROM estaciones WHERE id = ?`, l.EstacionID.String()).Scan(&nombre); err != nil {
			nombre = "Producción"
		}
		m[l.EstacionID] = estacion{ID: l.EstacionID, Nombre: nombre}
	}
	return m
}

// ---------- Anular y reimprimir ----------

type AnularIn struct {
	Lineas        []ids.ID `json:"lineas"`
	Motivo        string   `json:"motivo"`
	UsuarioID     *ids.ID  `json:"usuarioId"`
	AutorizadoPor *ids.ID  `json:"autorizadoPor"`
}

// AnularLineas imprime el ticket de ANULACIÓN en la estación de cada línea anulada
// (RF-02-04.5) y deja rastro en auditoría. Una línea solo se anula una vez.
func (a *App) AnularLineas(ctx context.Context, comanda ids.ID, in AnularIn) ([]Envio, error) {
	in.Motivo = strings.TrimSpace(in.Motivo)
	if len(in.Lineas) == 0 || len([]rune(in.Motivo)) < 3 || len([]rune(in.Motivo)) > 140 {
		return nil, invalido("Indica qué platos se anulan y el motivo (3 a 140 caracteres).")
	}
	now := a.Clock.Now()
	loc := a.zonaLocal(ctx)
	var envios []Envio
	var despertar []ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		d, ls, err := cargarComanda(ctx, tx, comanda)
		if err != nil {
			return err
		}
		porID := map[ids.ID]Linea{}
		for _, l := range ls {
			porID[l.ID] = l
		}
		var anuladas []Linea
		for _, lid := range in.Lineas {
			l, ok := porID[lid]
			if !ok {
				return invalido("Uno de los platos no pertenece a esta comanda.")
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO anulaciones_comanda (comanda_id, linea_id, motivo, usuario_id, autorizado_por, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
				comanda.String(), lid.String(), in.Motivo, idOrNil(in.UsuarioID), idOrNil(in.AutorizadoPor), now.Format(time.RFC3339Nano)); err != nil {
				if strings.Contains(err.Error(), "UNIQUE") {
					return problema(http.StatusConflict, "YA_ANULADA", "«"+l.Producto+"» ya estaba anulado.")
				}
				return err
			}
			anuladas = append(anuladas, l)
		}
		d.Anulacion, d.Motivo, d.Impreso = true, in.Motivo, now
		envios, despertar, err = a.encolarPorEstacion(ctx, tx, comanda, "ANULACION", d, anuladas, estacionesDe(ctx, tx, anuladas), loc, now)
		if err != nil {
			return err
		}
		if err := auditar(ctx, tx, "LINEA_ANULADA", "comanda", comanda, in.UsuarioID, map[string]any{"lineas": in.Lineas, "motivo": in.Motivo, "autorizadoPor": in.AutorizadoPor}, now); err != nil {
			return err
		}
		ev, err := edgesync.NewEvent("comanda.lineas_anuladas", 1, comanda, map[string]any{"comandaId": comanda, "lineas": in.Lineas, "motivo": in.Motivo, "usuarioId": in.UsuarioID, "autorizadoPor": in.AutorizadoPor}, now)
		if err != nil {
			return err
		}
		_, err = a.outbox.Append(ctx, tx, ev)
		return err
	})
	if err != nil {
		return nil, err
	}
	a.motor.Despertar(despertar...)
	a.notificarPush()
	for _, lid := range in.Lineas {
		_ = a.hub.Difundir(eventos.OrderLineVoided{OrderID: comanda, LineID: lid, ByUserID: derefID(in.UsuarioID), AuthorizedBy: in.AutorizadoPor, Reason: in.Motivo})
	}
	return envios, nil
}

func derefID(id *ids.ID) ids.ID {
	if id == nil {
		return ids.Nil
	}
	return *id
}

type ReimprimirIn struct {
	EstacionID *ids.ID `json:"estacionId"`
	UsuarioID  *ids.ID `json:"usuarioId"`
}

// Reimprimir vuelve a imprimir la comanda (o solo la parte de una estación) con la marca
// «REIMPRESIÓN» y la hora original, sin los platos anulados. Queda auditado (RF-02-06).
func (a *App) Reimprimir(ctx context.Context, comanda ids.ID, in ReimprimirIn) ([]Envio, error) {
	now := a.Clock.Now()
	loc := a.zonaLocal(ctx)
	var envios []Envio
	var despertar []ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		d, ls, err := cargarComanda(ctx, tx, comanda)
		if err != nil {
			return err
		}
		anuladas := map[string]bool{}
		rows, err := tx.QueryContext(ctx, `SELECT linea_id FROM anulaciones_comanda WHERE comanda_id = ?`, comanda.String())
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				anuladas[id] = true
			}
		}
		_ = rows.Close()
		var vigentes []Linea
		for _, l := range ls {
			if !anuladas[l.ID.String()] && (in.EstacionID == nil || l.EstacionID == *in.EstacionID) {
				vigentes = append(vigentes, l)
			}
		}
		if len(vigentes) == 0 {
			return invalido("No hay platos para reimprimir (todos están anulados o no son de esa estación).")
		}
		d.Reimpresion, d.Impreso = true, now
		envios, despertar, err = a.encolarPorEstacion(ctx, tx, comanda, "REIMPRESION", d, vigentes, estacionesDe(ctx, tx, vigentes), loc, now)
		if err != nil {
			return err
		}
		return auditar(ctx, tx, "COMANDA_REIMPRESA", "comanda", comanda, in.UsuarioID, map[string]any{"estacionId": in.EstacionID}, now)
	})
	if err != nil {
		return nil, err
	}
	a.motor.Despertar(despertar...)
	return envios, nil
}

// auditar agrega una fila a la auditoría local (append-only; la cadena de hash llega en F4-15).
func auditar(ctx context.Context, tx *store.Tx, accion, entidad string, entidadID ids.ID, usuario *ids.ID, detalle any, now time.Time) error {
	raw, err := json.Marshal(detalle)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO auditoria (id, usuario_id, accion, entidad, entidad_id, detalle, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ids.New().String(), idOrNil(usuario), accion, entidad, entidadID.String(), string(raw), now.Format(time.RFC3339Nano))
	return err
}

// notificarPush despierta al envío a la nube si la sincronización está activa.
func (a *App) notificarPush() {
	a.syncMu.Lock()
	p := a.pusher
	a.syncMu.Unlock()
	if p != nil {
		p.Notify()
	}
}
