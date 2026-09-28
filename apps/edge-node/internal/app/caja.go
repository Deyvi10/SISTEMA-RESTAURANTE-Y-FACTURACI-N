package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/cierrez"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
)

// ---------- Configuración replicada de la nube ----------

type Caja struct {
	ID         ids.ID  `json:"id"`
	Nombre     string  `json:"nombre"`
	EstacionID *ids.ID `json:"estacionId"`
}

type MetodoPago struct {
	ID             ids.ID `json:"id"`
	Nombre         string `json:"nombre"`
	Tipo           string `json:"tipo"`
	CodigoSRI      string `json:"codigoSri"`
	AbreCajon      bool   `json:"abreCajon"`
	PideReferencia bool   `json:"pideReferencia"`
	Icono          string `json:"icono"`
}

type MotivoDescuento struct {
	ID     ids.ID `json:"id"`
	Nombre string `json:"nombre"`
	Tipo   string `json:"tipo"` // DESCUENTO, CORTESIA
}

// ConfigCaja es lo que la caja necesita para cobrar sin internet.
type ConfigCaja struct {
	Cajas                 []Caja            `json:"cajas"`
	Metodos               []MetodoPago      `json:"metodos"`
	Motivos               []MotivoDescuento `json:"motivos"`
	ConsumidorFinalMaximo string            `json:"consumidorFinalMaximo"`
	PropinaActiva         bool              `json:"propinaActiva"`
	PropinaPorcentaje     string            `json:"propinaPorcentaje"`
	// Billetes y monedas del asistente de cierre (la lista vive en un solo lugar: cierrez).
	Denominaciones []cierrez.Denominacion `json:"denominaciones"`
}

// consumidorFinalPorDefecto se usa si el nodo aún no recibió el parámetro global (DP-07).
const consumidorFinalPorDefecto = "50.00"

func (a *App) ConfigCaja(ctx context.Context) (ConfigCaja, error) {
	q := a.Store.Read()
	c := ConfigCaja{Cajas: []Caja{}, Metodos: []MetodoPago{}, Motivos: []MotivoDescuento{}, ConsumidorFinalMaximo: consumidorFinalPorDefecto, PropinaPorcentaje: "10",
		Denominaciones: cierrez.Denominaciones}
	rows, err := q.QueryContext(ctx, `SELECT id, nombre, estacion_id FROM cajas WHERE deleted_at IS NULL AND activa = 1 ORDER BY nombre`)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var x Caja
		var id string
		var est sql.NullString
		if err := rows.Scan(&id, &x.Nombre, &est); err != nil {
			_ = rows.Close()
			return c, err
		}
		x.ID, _ = ids.Parse(id)
		if e, err := ids.Parse(est.String); err == nil {
			x.EstacionID = &e
		}
		c.Cajas = append(c.Cajas, x)
	}
	_ = rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT id, nombre, tipo, codigo_forma_pago_sri, abre_cajon, pide_referencia, coalesce(icono, 'caja') FROM metodos_pago
		WHERE deleted_at IS NULL AND activo = 1 ORDER BY orden, nombre`)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var m MetodoPago
		var id string
		if err := rows.Scan(&id, &m.Nombre, &m.Tipo, &m.CodigoSRI, &m.AbreCajon, &m.PideReferencia, &m.Icono); err != nil {
			_ = rows.Close()
			return c, err
		}
		m.ID, _ = ids.Parse(id)
		c.Metodos = append(c.Metodos, m)
	}
	_ = rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT id, nombre, tipo FROM motivos_descuento WHERE deleted_at IS NULL AND activo = 1 ORDER BY orden, nombre`)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var m MotivoDescuento
		var id string
		if err := rows.Scan(&id, &m.Nombre, &m.Tipo); err != nil {
			_ = rows.Close()
			return c, err
		}
		m.ID, _ = ids.Parse(id)
		c.Motivos = append(c.Motivos, m)
	}
	_ = rows.Close()
	c.ConsumidorFinalMaximo = consumidorFinalMaximo(ctx, q).String()
	var activa int
	var pct string
	if err := q.QueryRowContext(ctx, `SELECT propina_legal_activa, propina_porcentaje FROM locales LIMIT 1`).Scan(&activa, &pct); err == nil {
		c.PropinaActiva, c.PropinaPorcentaje = activa == 1, pct
	}
	return c, nil
}

// consumidorFinalMaximo: límite vigente de una venta a consumidor final (parámetro global que
// llega de la nube; mientras no llegue, el valor por defecto).
func consumidorFinalMaximo(ctx context.Context, q queryer) money.Money {
	var v string
	if err := q.QueryRowContext(ctx, `SELECT valor FROM parametros_globales WHERE clave = 'consumidor_final_maximo'`).Scan(&v); err == nil {
		if m, err := money.Parse(v); err == nil {
			return m
		}
	}
	return money.MustParse(consumidorFinalPorDefecto)
}

// ---------- Jornada (F4-02) ----------

type Jornada struct {
	ID           ids.ID     `json:"id"`
	FechaNegocio string     `json:"fechaNegocio"`
	AbiertaAt    time.Time  `json:"abiertaAt"`
	AbiertaPor   ids.ID     `json:"abiertaPor"`
	CerradaAt    *time.Time `json:"cerradaAt,omitempty"`
}

var errSinJornada = problema(http.StatusConflict, "SIN_JORNADA", "No hay una jornada abierta.")

func leerJornadaAbierta(ctx context.Context, q queryer) (*Jornada, error) {
	var j Jornada
	var id, abierta, por string
	err := q.QueryRowContext(ctx, `SELECT id, fecha_negocio, abierta_at, abierta_por FROM jornadas WHERE cerrada_at IS NULL`).Scan(&id, &j.FechaNegocio, &abierta, &por)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	j.ID, _ = ids.Parse(id)
	j.AbiertaPor, _ = ids.Parse(por)
	j.AbiertaAt, _ = time.Parse(time.RFC3339Nano, abierta)
	return &j, nil
}

// abrirJornadaEnTx abre la jornada del día de hoy (fecha local). Una sola por fecha: si la de
// hoy ya se cerró, no se reabre (el día de negocio ya terminó).
func abrirJornadaEnTx(ctx context.Context, tx *store.Tx, u Usuario, now time.Time, loc *time.Location, motivo string) (*Jornada, error) {
	fecha := now.In(loc).Format("2006-01-02")
	var cerrada int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM jornadas WHERE fecha_negocio = ?`, fecha).Scan(&cerrada); err != nil {
		return nil, err
	}
	if cerrada > 0 {
		return nil, problema(http.StatusConflict, "JORNADA_CERRADA", "La jornada de hoy ya se cerró. La próxima se abre mañana.")
	}
	j := &Jornada{ID: ids.New(), FechaNegocio: fecha, AbiertaAt: now, AbiertaPor: u.ID}
	if _, err := tx.ExecContext(ctx, `INSERT INTO jornadas (id, fecha_negocio, abierta_at, abierta_por) VALUES (?, ?, ?, ?)`,
		j.ID.String(), fecha, now.Format(time.RFC3339Nano), u.ID.String()); err != nil {
		return nil, err
	}
	// Adopta las órdenes que se transfirieron al cerrar la jornada anterior.
	if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET jornada_id = ? WHERE jornada_id IS NULL AND estado IN ('ABIERTA','PRECUENTA')`, j.ID.String()); err != nil {
		return nil, err
	}
	uid := u.ID
	return j, auditar(ctx, tx, "JORNADA_ABIERTA", "jornada", j.ID, &uid, map[string]any{"fecha": fecha, "motivo": motivo}, now)
}

// jornadaParaOperar devuelve la jornada abierta o la abre (el primer pedido del día no espera
// a nadie: el restaurante nunca se bloquea por un trámite).
func jornadaParaOperar(ctx context.Context, tx *store.Tx, u Usuario, now time.Time, loc *time.Location) (*Jornada, error) {
	j, err := leerJornadaAbierta(ctx, tx)
	if err != nil || j != nil {
		return j, err
	}
	return abrirJornadaEnTx(ctx, tx, u, now, loc, "automática con la primera operación del día")
}

// fechaNegocio: la fecha de la jornada abierta o, si no hay, la fecha local de hoy.
func fechaNegocio(ctx context.Context, q queryer, now time.Time, loc *time.Location) string {
	if j, err := leerJornadaAbierta(ctx, q); err == nil && j != nil {
		return j.FechaNegocio
	}
	return now.In(loc).Format("2006-01-02")
}

func (a *App) JornadaActual(ctx context.Context) (*Jornada, error) {
	return leerJornadaAbierta(ctx, a.Store.Read())
}

func (a *App) AbrirJornada(ctx context.Context, u Usuario) (*Jornada, error) {
	if !u.Puede(rbac.GestionarTurno) {
		return nil, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para abrir la jornada.")
	}
	now, loc := a.Clock.Now(), a.zonaLocal(ctx)
	var j *Jornada
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		abierta, err := leerJornadaAbierta(ctx, tx)
		if err != nil {
			return err
		}
		if abierta != nil {
			return problema(http.StatusConflict, "JORNADA_ABIERTA", "Ya hay una jornada abierta (del "+abierta.FechaNegocio+").")
		}
		if j, err = abrirJornadaEnTx(ctx, tx, u, now, loc, "manual"); err != nil {
			return err
		}
		return a.eventoCaja(ctx, tx, "jornada.abierta", j.ID, j, now)
	})
	if err == nil {
		a.notificarPush()
	}
	return j, err
}

type CerrarJornadaIn struct {
	// TransferirOrdenes deja las órdenes abiertas para la próxima jornada (RF-04-01).
	TransferirOrdenes bool `json:"transferirOrdenes"`
}

func (a *App) CerrarJornada(ctx context.Context, u Usuario, in CerrarJornadaIn) (*Jornada, error) {
	if !u.Puede(rbac.GestionarTurno) {
		return nil, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para cerrar la jornada.")
	}
	now := a.Clock.Now()
	var j *Jornada
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		var err error
		if j, err = leerJornadaAbierta(ctx, tx); err != nil {
			return err
		}
		if j == nil {
			return errSinJornada
		}
		var turnos, ordenes int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM turnos_caja WHERE jornada_id = ? AND estado = 'ABIERTO'`, j.ID.String()).Scan(&turnos); err != nil {
			return err
		}
		if turnos > 0 {
			return problema(http.StatusConflict, "TURNOS_ABIERTOS", "Primero cierra los turnos de caja abiertos.")
		}
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM ordenes WHERE jornada_id = ? AND estado IN ('ABIERTA','PRECUENTA')`, j.ID.String()).Scan(&ordenes); err != nil {
			return err
		}
		if ordenes > 0 {
			if !in.TransferirOrdenes {
				return problema(http.StatusConflict, "ORDENES_ABIERTAS", "Hay órdenes abiertas. Cóbralas o transfiérelas a la próxima jornada.")
			}
			if _, err := tx.ExecContext(ctx, `UPDATE ordenes SET jornada_id = NULL WHERE jornada_id = ? AND estado IN ('ABIERTA','PRECUENTA')`, j.ID.String()); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE jornadas SET cerrada_at = ?, cerrada_por = ? WHERE id = ?`, now.Format(time.RFC3339Nano), u.ID.String(), j.ID.String()); err != nil {
			return err
		}
		// Las sesiones de los meseros terminan con la jornada (RF-04-01).
		if _, err := tx.ExecContext(ctx, `DELETE FROM sesiones_usuario`); err != nil {
			return err
		}
		t := now
		j.CerradaAt = &t
		uid := u.ID
		if err := auditar(ctx, tx, "JORNADA_CERRADA", "jornada", j.ID, &uid, map[string]any{"fecha": j.FechaNegocio, "ordenesTransferidas": ordenes}, now); err != nil {
			return err
		}
		return a.eventoCaja(ctx, tx, "jornada.cerrada", j.ID, map[string]any{"id": j.ID, "fechaNegocio": j.FechaNegocio, "cerradaAt": now, "ordenesTransferidas": ordenes}, now)
	})
	if err == nil {
		a.notificarPush()
	}
	return j, err
}

// ---------- Turnos de caja (F4-03) ----------

type Turno struct {
	ID           ids.ID     `json:"id"`
	CajaID       ids.ID     `json:"cajaId"`
	JornadaID    ids.ID     `json:"jornadaId"`
	CajeroID     ids.ID     `json:"cajeroId"`
	CajeroNombre string     `json:"cajeroNombre"`
	FondoInicial string     `json:"fondoInicial"`
	AbiertoAt    time.Time  `json:"abiertoAt"`
	CerradoAt    *time.Time `json:"cerradoAt,omitempty"`
	Estado       string     `json:"estado"`
}

func leerTurno(ctx context.Context, q queryer, where string, arg any) (*Turno, error) {
	var t Turno
	var id, caja, jornada, cajero, abierto string
	var cerrado sql.NullString
	err := q.QueryRowContext(ctx, `SELECT id, caja_id, jornada_id, cajero_id, cajero_nombre, fondo_inicial, abierto_at, cerrado_at, estado FROM turnos_caja WHERE `+where, arg). //nolint:gosec // where fijo
																							Scan(&id, &caja, &jornada, &cajero, &t.CajeroNombre, &t.FondoInicial, &abierto, &cerrado, &t.Estado)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.ID, _ = ids.Parse(id)
	t.CajaID, _ = ids.Parse(caja)
	t.JornadaID, _ = ids.Parse(jornada)
	t.CajeroID, _ = ids.Parse(cajero)
	t.AbiertoAt, _ = time.Parse(time.RFC3339Nano, abierto)
	if c, err := time.Parse(time.RFC3339Nano, cerrado.String); err == nil {
		t.CerradoAt = &c
	}
	return &t, nil
}

// turnoAbiertoDe devuelve el turno abierto de una caja o el error «sin turno no se cobra».
func turnoAbiertoDe(ctx context.Context, q queryer, caja ids.ID) (*Turno, error) {
	t, err := leerTurno(ctx, q, `caja_id = ? AND estado = 'ABIERTO'`, caja.String())
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, problema(http.StatusConflict, "SIN_TURNO", "Esta caja no tiene un turno abierto. Ábrelo con el fondo inicial para cobrar.")
	}
	return t, nil
}

func cajaExiste(ctx context.Context, q queryer, id ids.ID) (string, error) {
	var nombre string
	err := q.QueryRowContext(ctx, `SELECT nombre FROM cajas WHERE id = ? AND deleted_at IS NULL AND activa = 1`, id.String()).Scan(&nombre)
	if errors.Is(err, sql.ErrNoRows) {
		return "", problema(http.StatusNotFound, "CAJA_NO_EXISTE", "Esa caja no existe o está desactivada.")
	}
	return nombre, err
}

// EstadoCaja: lo que ve la caja al entrar. Nunca incluye el esperado del cierre (cierre ciego).
type EstadoCaja struct {
	Caja    Caja     `json:"caja"`
	Jornada *Jornada `json:"jornada"`
	Turno   *Turno   `json:"turno"`
}

func (a *App) EstadoDeCaja(ctx context.Context, caja ids.ID) (EstadoCaja, error) {
	q := a.Store.Read()
	var e EstadoCaja
	nombre, err := cajaExiste(ctx, q, caja)
	if err != nil {
		return e, err
	}
	e.Caja = Caja{ID: caja, Nombre: nombre}
	if e.Jornada, err = leerJornadaAbierta(ctx, q); err != nil {
		return e, err
	}
	e.Turno, err = leerTurno(ctx, q, `caja_id = ? AND estado = 'ABIERTO'`, caja.String())
	return e, err
}

type AbrirTurnoIn struct {
	CajaID       ids.ID `json:"cajaId"`
	FondoInicial string `json:"fondoInicial"`
}

func (a *App) AbrirTurno(ctx context.Context, u Usuario, in AbrirTurnoIn) (*Turno, error) {
	if !u.Puede(rbac.GestionarTurno) {
		return nil, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para abrir turnos de caja.")
	}
	fondo, err := montoNoNegativo(in.FondoInicial, "El fondo inicial")
	if err != nil {
		return nil, err
	}
	now, loc := a.Clock.Now(), a.zonaLocal(ctx)
	var t *Turno
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		if _, err := cajaExiste(ctx, tx, in.CajaID); err != nil {
			return err
		}
		if abierto, err := leerTurno(ctx, tx, `caja_id = ? AND estado = 'ABIERTO'`, in.CajaID.String()); err != nil {
			return err
		} else if abierto != nil {
			return problema(http.StatusConflict, "TURNO_ABIERTO", "Esta caja ya tiene un turno abierto por "+abierto.CajeroNombre+".")
		}
		j, err := jornadaParaOperar(ctx, tx, u, now, loc)
		if err != nil {
			return err
		}
		t = &Turno{ID: ids.New(), CajaID: in.CajaID, JornadaID: j.ID, CajeroID: u.ID, CajeroNombre: u.Nombre, FondoInicial: fondo.String(), AbiertoAt: now, Estado: "ABIERTO"}
		if _, err := tx.ExecContext(ctx, `INSERT INTO turnos_caja (id, caja_id, jornada_id, cajero_id, cajero_nombre, fondo_inicial, abierto_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			t.ID.String(), t.CajaID.String(), j.ID.String(), u.ID.String(), u.Nombre, t.FondoInicial, now.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		uid := u.ID
		if err := auditar(ctx, tx, "TURNO_ABIERTO", "turno", t.ID, &uid, map[string]any{"caja": in.CajaID, "fondoInicial": t.FondoInicial}, now); err != nil {
			return err
		}
		return a.eventoCaja(ctx, tx, "turno.abierto", t.ID, t, now)
	})
	if err == nil {
		a.notificarPush()
	}
	return t, err
}

// ---------- Movimientos de caja (F4-11) ----------

type MovimientoIn struct {
	CajaID         ids.ID `json:"cajaId"`
	Tipo           string `json:"tipo"` // RETIRO, INGRESO, GASTO
	Monto          string `json:"monto"`
	Motivo         string `json:"motivo"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type Movimiento struct {
	ID            ids.ID    `json:"id"`
	TurnoID       ids.ID    `json:"turnoId"`
	Tipo          string    `json:"tipo"`
	Monto         string    `json:"monto"`
	Motivo        string    `json:"motivo"`
	UsuarioNombre string    `json:"usuarioNombre"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (a *App) RegistrarMovimiento(ctx context.Context, u Usuario, in MovimientoIn) (Movimiento, error) {
	if !u.Puede(rbac.GestionarTurno) {
		return Movimiento{}, problema(http.StatusForbidden, "SIN_PERMISO", "No tienes permiso para registrar movimientos de caja.")
	}
	in.Tipo, in.Motivo, in.IdempotencyKey = strings.ToUpper(strings.TrimSpace(in.Tipo)), strings.TrimSpace(in.Motivo), strings.TrimSpace(in.IdempotencyKey)
	if in.Tipo != "RETIRO" && in.Tipo != "INGRESO" && in.Tipo != "GASTO" {
		return Movimiento{}, invalido("El tipo de movimiento es RETIRO, INGRESO o GASTO.")
	}
	if len([]rune(in.Motivo)) < 3 || len([]rune(in.Motivo)) > 200 {
		return Movimiento{}, invalido("Escribe el motivo del movimiento (de 3 a 200 caracteres).")
	}
	if len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 64 {
		return Movimiento{}, invalido("Falta la clave de idempotencia del movimiento.")
	}
	monto, err := montoNoNegativo(in.Monto, "El monto")
	if err != nil {
		return Movimiento{}, err
	}
	if monto.IsZero() {
		return Movimiento{}, invalido("El monto debe ser mayor que cero.")
	}
	now := a.Clock.Now()
	m := Movimiento{Tipo: in.Tipo, Monto: monto.String(), Motivo: in.Motivo, UsuarioNombre: u.Nombre, CreatedAt: now}
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		// Reintento del mismo movimiento: devuelve el registrado.
		var id, turno, creado string
		err := tx.QueryRowContext(ctx, `SELECT id, turno_id, tipo, monto, motivo, usuario_nombre, created_at FROM movimientos_caja WHERE idempotency_key = ?`, in.IdempotencyKey).
			Scan(&id, &turno, &m.Tipo, &m.Monto, &m.Motivo, &m.UsuarioNombre, &creado)
		if err == nil {
			m.ID, _ = ids.Parse(id)
			m.TurnoID, _ = ids.Parse(turno)
			m.CreatedAt, _ = time.Parse(time.RFC3339Nano, creado)
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		t, err := turnoAbiertoDe(ctx, tx, in.CajaID)
		if err != nil {
			return err
		}
		m.ID, m.TurnoID = ids.New(), t.ID
		if _, err := tx.ExecContext(ctx, `INSERT INTO movimientos_caja (id, turno_id, tipo, monto, motivo, usuario_id, usuario_nombre, idempotency_key, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.ID.String(), t.ID.String(), m.Tipo, m.Monto, m.Motivo, u.ID.String(), u.Nombre, in.IdempotencyKey, now.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		uid := u.ID
		if err := auditar(ctx, tx, "MOVIMIENTO_CAJA", "turno", t.ID, &uid, map[string]any{"tipo": m.Tipo, "monto": m.Monto, "motivo": m.Motivo}, now); err != nil {
			return err
		}
		return a.eventoCaja(ctx, tx, "caja.movimiento", m.ID, m, now)
	})
	if err == nil {
		a.notificarPush()
	}
	return m, err
}

func (a *App) MovimientosDeTurno(ctx context.Context, turno ids.ID) ([]Movimiento, error) {
	rows, err := a.Store.Read().QueryContext(ctx, `SELECT id, tipo, monto, motivo, usuario_nombre, created_at FROM movimientos_caja WHERE turno_id = ? ORDER BY created_at`, turno.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Movimiento{}
	for rows.Next() {
		m := Movimiento{TurnoID: turno}
		var id, creado string
		if err := rows.Scan(&id, &m.Tipo, &m.Monto, &m.Motivo, &m.UsuarioNombre, &creado); err != nil {
			return nil, err
		}
		m.ID, _ = ids.Parse(id)
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, creado)
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---------- Utilidades ----------

// montoNoNegativo lee un importe en texto decimal exacto (nunca float) con máximo 2 decimales.
func montoNoNegativo(s, que string) (money.Money, error) {
	m, err := money.Parse(strings.TrimSpace(s))
	if err != nil {
		return money.Money{}, invalido(que + " no es un importe válido.")
	}
	if m.IsNegative() {
		return money.Money{}, invalido(que + " no puede ser negativo.")
	}
	if !m.Round2().Equal(m) {
		return money.Money{}, invalido(que + " tiene más de dos decimales.")
	}
	if m.GreaterThan(money.MustParse("100000")) {
		return money.Money{}, invalido(que + " es demasiado grande.")
	}
	return m, nil
}

// eventoCaja deja el cambio de caja en el outbox dentro de la misma transacción (es dinero).
func (a *App) eventoCaja(ctx context.Context, tx *store.Tx, tipo string, agregado ids.ID, payload any, now time.Time) error {
	ev, err := edgesync.NewEvent(tipo, 1, agregado, payload, now)
	if err != nil {
		return err
	}
	_, err = a.outbox.Append(ctx, tx, ev)
	return err
}
