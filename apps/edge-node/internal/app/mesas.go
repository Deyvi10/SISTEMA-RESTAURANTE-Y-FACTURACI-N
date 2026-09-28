package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

const (
	// BloqueoMesaTTL: sin latido en 45 s la mesa se libera sola (el teléfono murió).
	BloqueoMesaTTL = 45 * time.Second
	// EsperaCuentaAlerta: una mesa que pidió la cuenta hace más de 10 min se marca demorada.
	EsperaCuentaAlerta = 10 * time.Minute
)

// Bloqueo es quién está editando una mesa.
type Bloqueo struct {
	UsuarioID     ids.ID    `json:"usuarioId"`
	UsuarioNombre string    `json:"usuarioNombre"`
	DispositivoID ids.ID    `json:"dispositivoId"`
	ExpiraAt      time.Time `json:"expiraAt"`
}

var errMesaNoExiste = problema(http.StatusNotFound, "NO_ENCONTRADO", "Esa mesa ya no existe. Actualiza el salón.")

func mesaExiste(ctx context.Context, q queryer, mesa ids.ID) (string, error) {
	var nombre string
	err := q.QueryRowContext(ctx, `SELECT nombre FROM mesas WHERE id = ? AND deleted_at IS NULL AND activa = 1`, mesa.String()).Scan(&nombre)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errMesaNoExiste
	}
	return nombre, err
}

func leerBloqueo(ctx context.Context, q queryer, mesa ids.ID, now time.Time) (*Bloqueo, error) {
	var b Bloqueo
	var uid, did, latido string
	err := q.QueryRowContext(ctx, `SELECT usuario_id, usuario_nombre, dispositivo_id, ultimo_heartbeat_at FROM bloqueos_mesa WHERE mesa_id = ?`, mesa.String()).
		Scan(&uid, &b.UsuarioNombre, &did, &latido)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t, _ := time.Parse(time.RFC3339Nano, latido)
	b.ExpiraAt = t.Add(BloqueoMesaTTL)
	if !now.Before(b.ExpiraAt) {
		return nil, nil // vencido: como si no existiera
	}
	b.UsuarioID, _ = ids.Parse(uid)
	b.DispositivoID, _ = ids.Parse(did)
	return &b, nil
}

func errOcupada(b *Bloqueo) error {
	return problema(http.StatusConflict, "LOCKED_BY", "La mesa la está editando "+b.UsuarioNombre+". Espera a que termine.")
}

// BloquearMesa: de dos pedidos simultáneos gana exactamente uno (el escritor único de
// SQLite serializa la comprobación y la escritura). El mismo usuario en el mismo teléfono
// puede volver a pedirla (renueva).
func (a *App) BloquearMesa(ctx context.Context, d Dispositivo, u Usuario, mesa ids.ID) (Bloqueo, error) {
	now := a.Clock.Now()
	var res Bloqueo
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		if _, err := mesaExiste(ctx, tx, mesa); err != nil {
			return err
		}
		b, err := leerBloqueo(ctx, tx, mesa, now)
		if err != nil {
			return err
		}
		if b != nil && (b.UsuarioID != u.ID || b.DispositivoID != d.ID) {
			return errOcupada(b)
		}
		ts := now.Format(time.RFC3339Nano)
		_, err = tx.ExecContext(ctx, `INSERT INTO bloqueos_mesa (mesa_id, usuario_id, usuario_nombre, dispositivo_id, adquirido_at, ultimo_heartbeat_at) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (mesa_id) DO UPDATE SET usuario_id = excluded.usuario_id, usuario_nombre = excluded.usuario_nombre, dispositivo_id = excluded.dispositivo_id,
			adquirido_at = CASE WHEN bloqueos_mesa.usuario_id = excluded.usuario_id THEN bloqueos_mesa.adquirido_at ELSE excluded.adquirido_at END,
			ultimo_heartbeat_at = excluded.ultimo_heartbeat_at`, mesa.String(), u.ID.String(), u.Nombre, d.ID.String(), ts, ts)
		res = Bloqueo{UsuarioID: u.ID, UsuarioNombre: u.Nombre, DispositivoID: d.ID, ExpiraAt: now.Add(BloqueoMesaTTL)}
		return err
	})
	if err != nil {
		return Bloqueo{}, err
	}
	did := d.ID
	_ = a.hub.Difundir(eventos.TableLocked{TableID: mesa, ByUserID: u.ID, ByName: u.Nombre, DeviceID: &did, ExpiresAt: res.ExpiraAt})
	return res, nil
}

// LatidoMesa mantiene el bloqueo mientras la pantalla de la mesa esté abierta (cada 10 s).
func (a *App) LatidoMesa(ctx context.Context, d Dispositivo, u Usuario, mesa ids.ID) (Bloqueo, error) {
	now := a.Clock.Now()
	var res Bloqueo
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		b, err := leerBloqueo(ctx, tx, mesa, now)
		if err != nil {
			return err
		}
		if b == nil {
			return problema(http.StatusConflict, "BLOQUEO_PERDIDO", "Perdiste la mesa por inactividad. Ábrela de nuevo.")
		}
		if b.UsuarioID != u.ID || b.DispositivoID != d.ID {
			return errOcupada(b)
		}
		_, err = tx.ExecContext(ctx, `UPDATE bloqueos_mesa SET ultimo_heartbeat_at = ? WHERE mesa_id = ?`, now.Format(time.RFC3339Nano), mesa.String())
		b.ExpiraAt = now.Add(BloqueoMesaTTL)
		res = *b
		return err
	})
	return res, err
}

// LiberarMesa suelta el bloqueo. forzar: un administrador libera la mesa de otro (auditado).
func (a *App) LiberarMesa(ctx context.Context, d Dispositivo, u Usuario, mesa ids.ID, forzar bool) error {
	now := a.Clock.Now()
	liberada := false
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		b, err := leerBloqueo(ctx, tx, mesa, now)
		if err != nil || b == nil {
			return err
		}
		ajeno := b.UsuarioID != u.ID || b.DispositivoID != d.ID
		if ajeno && !forzar {
			return errOcupada(b)
		}
		if ajeno && u.Rol != "ADMIN" {
			return problema(http.StatusForbidden, "SIN_PERMISO", "Solo un administrador puede liberar la mesa de otro mesero.")
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM bloqueos_mesa WHERE mesa_id = ?`, mesa.String()); err != nil {
			return err
		}
		liberada = true
		if ajeno {
			uid := u.ID
			return auditar(ctx, tx, "MESA_LIBERADA_FORZADA", "mesa", mesa, &uid, map[string]any{"teniaBloqueo": b.UsuarioNombre}, now)
		}
		return nil
	})
	if err == nil && liberada {
		razon := "RELEASED"
		if forzar {
			razon = "FORCED"
		}
		_ = a.hub.Difundir(eventos.TableUnlocked{TableID: mesa, Reason: razon})
	}
	return err
}

// liberarEnTx suelta el bloqueo de quien envía la comanda (dentro de su transacción).
func liberarEnTx(ctx context.Context, tx *store.Tx, mesa ids.ID) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM bloqueos_mesa WHERE mesa_id = ?`, mesa.String())
	return err
}

// barrerBloqueos libera las mesas cuyo teléfono dejó de latir (cada 5 s).
func (a *App) barrerBloqueos(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		a.BarrerBloqueosVencidos(ctx)
	}
}

// BarrerBloqueosVencidos borra los bloqueos vencidos y lo avisa a todos.
func (a *App) BarrerBloqueosVencidos(ctx context.Context) {
	limite := a.Clock.Now().Add(-BloqueoMesaTTL).Format(time.RFC3339Nano)
	var vencidas []ids.ID
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT mesa_id FROM bloqueos_mesa WHERE ultimo_heartbeat_at <= ?`, limite)
		if err != nil {
			return err
		}
		for rows.Next() {
			var s string
			if rows.Scan(&s) == nil {
				if id, err := ids.Parse(s); err == nil {
					vencidas = append(vencidas, id)
				}
			}
		}
		_ = rows.Close()
		_, err = tx.ExecContext(ctx, `DELETE FROM bloqueos_mesa WHERE ultimo_heartbeat_at <= ?`, limite)
		return err
	})
	if err != nil {
		a.Log.Error("barrido de bloqueos", "err", err)
		return
	}
	for _, m := range vencidas {
		_ = a.hub.Difundir(eventos.TableUnlocked{TableID: m, Reason: "EXPIRED"})
	}
}

// ---------- Mapa del salón (F3-06) ----------

type ZonaMapa struct {
	ID     ids.ID `json:"id"`
	Nombre string `json:"nombre"`
	Orden  int    `json:"orden"`
}

type MesaMapa struct {
	ID           ids.ID     `json:"id"`
	ZonaID       ids.ID     `json:"zonaId"`
	Nombre       string     `json:"nombre"`
	Capacidad    int        `json:"capacidad"`
	Forma        string     `json:"forma"`
	PosX         int        `json:"posX"`
	PosY         int        `json:"posY"`
	Estado       string     `json:"estado"` // LIBRE, OCUPADA, POR_PAGAR, DEMORADA
	OrdenID      *ids.ID    `json:"ordenId"`
	NumeroOrden  *int       `json:"numeroOrden"`
	MeseroNombre *string    `json:"meseroNombre"`
	AbiertaAt    *time.Time `json:"abiertaAt"`
	PrecuentaAt  *time.Time `json:"precuentaAt"`
	Comensales   *int       `json:"comensales"`
	Platos       int        `json:"platos"`
	Total        string     `json:"total"`
	Bloqueo      *Bloqueo   `json:"bloqueo"`
}

type Salon struct {
	Zonas []ZonaMapa `json:"zonas"`
	Mesas []MesaMapa `json:"mesas"`
}

// SalonVivo arma el mapa con el estado derivado: orden abierta + bloqueo (docs/04 §5).
func (a *App) SalonVivo(ctx context.Context) (Salon, error) {
	now := a.Clock.Now()
	s := Salon{Zonas: []ZonaMapa{}, Mesas: []MesaMapa{}}
	q := a.Store.Read()
	rows, err := q.QueryContext(ctx, `SELECT id, nombre, orden FROM zonas WHERE deleted_at IS NULL ORDER BY orden, nombre`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var z ZonaMapa
		var id string
		if err := rows.Scan(&id, &z.Nombre, &z.Orden); err != nil {
			_ = rows.Close()
			return s, err
		}
		z.ID, _ = ids.Parse(id)
		s.Zonas = append(s.Zonas, z)
	}
	_ = rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT m.id, m.zona_id, m.nombre, m.capacidad, m.forma, m.pos_x, m.pos_y,
			o.id, o.numero_corto, o.mesero_nombre, o.abierta_at, o.precuenta_at, o.comensales, o.estado,
			(SELECT count(*) FROM orden_lineas l WHERE l.orden_id = o.id AND l.estado <> 'ANULADA'),
			b.usuario_id, b.usuario_nombre, b.dispositivo_id, b.ultimo_heartbeat_at
		FROM mesas m
		LEFT JOIN ordenes o ON o.mesa_id = m.id AND o.estado IN ('ABIERTA','PRECUENTA')
		LEFT JOIN bloqueos_mesa b ON b.mesa_id = m.id
		WHERE m.deleted_at IS NULL AND m.activa = 1
		ORDER BY m.pos_y, m.pos_x, m.nombre`)
	if err != nil {
		return s, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var m MesaMapa
		var id, zona string
		var oid, mnombre, abierta, precuenta, oestado, buid, bunombre, bdid, blatido sql.NullString
		var onum, ocom sql.NullInt64
		if err := rows.Scan(&id, &zona, &m.Nombre, &m.Capacidad, &m.Forma, &m.PosX, &m.PosY,
			&oid, &onum, &mnombre, &abierta, &precuenta, &ocom, &oestado, &m.Platos, &buid, &bunombre, &bdid, &blatido); err != nil {
			return s, err
		}
		m.ID, _ = ids.Parse(id)
		m.ZonaID, _ = ids.Parse(zona)
		m.Estado, m.Total = "LIBRE", "0.00"
		if oid.Valid {
			o, _ := ids.Parse(oid.String)
			n, nombre := int(onum.Int64), mnombre.String
			m.OrdenID, m.NumeroOrden, m.MeseroNombre = &o, &n, &nombre
			if t, err := time.Parse(time.RFC3339Nano, abierta.String); err == nil {
				m.AbiertaAt = &t
			}
			if ocom.Valid {
				c := int(ocom.Int64)
				m.Comensales = &c
			}
			m.Estado = "OCUPADA"
			if oestado.String == "PRECUENTA" {
				m.Estado = "POR_PAGAR"
				if t, err := time.Parse(time.RFC3339Nano, precuenta.String); err == nil {
					m.PrecuentaAt = &t
					if now.Sub(t) > EsperaCuentaAlerta {
						m.Estado = "DEMORADA"
					}
				}
			}
			if tot, err := totalOrden(ctx, q, o); err == nil {
				m.Total = tot
			}
		}
		if buid.Valid {
			t, _ := time.Parse(time.RFC3339Nano, blatido.String)
			if now.Before(t.Add(BloqueoMesaTTL)) {
				b := Bloqueo{UsuarioNombre: bunombre.String, ExpiraAt: t.Add(BloqueoMesaTTL)}
				b.UsuarioID, _ = ids.Parse(buid.String)
				b.DispositivoID, _ = ids.Parse(bdid.String)
				m.Bloqueo = &b
			}
		}
		s.Mesas = append(s.Mesas, m)
	}
	return s, rows.Err()
}

// avisarMesa difunde el estado vivo de una mesa tras un cambio de su orden (p95 ≤ 300 ms).
func (a *App) avisarMesa(ctx context.Context, mesa ids.ID) {
	s, err := a.SalonVivo(ctx)
	if err != nil {
		return
	}
	for _, m := range s.Mesas {
		if m.ID != mesa {
			continue
		}
		ev := eventos.TableStateChanged{TableID: m.ID, State: m.Estado, OrderID: m.OrdenID}
		if m.Comensales != nil {
			g := int64(*m.Comensales)
			ev.Guests = &g
		}
		_ = a.hub.Difundir(ev)
	}
}
