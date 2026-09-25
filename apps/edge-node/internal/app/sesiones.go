package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/secreto"
)

const (
	// SesionUsuarioTTL: la sesión por PIN dura como mucho un turno largo (F3-04).
	SesionUsuarioTTL = 12 * time.Hour
	// Bloqueo por dispositivo tras 5 PIN erróneos seguidos.
	maxFallosPIN  = 5
	bloqueoPIN    = 5 * time.Minute
	ventanaAlerta = 10 * time.Minute
	alertaPIN     = 20
	// AutorizacionTTL: el PIN de un supervisor autoriza UNA acción durante un minuto.
	AutorizacionTTL = time.Minute
)

func decodeJSON(raw []byte, v any) error { return json.Unmarshal(raw, v) }

// Persona es alguien del personal que puede entrar con su PIN en este local.
type Persona struct {
	ID        ids.ID  `json:"id"`
	Nombre    string  `json:"nombre"`
	Rol       string  `json:"rol"`
	AvatarURL *string `json:"avatarUrl"`
}

// Personal lista a quienes pueden entrar con PIN en este local (cuadrícula de la app).
func (a *App) Personal(ctx context.Context) ([]Persona, error) {
	id, err := a.Identidad(ctx)
	if err != nil {
		return nil, err
	}
	local := ""
	if id != nil {
		local = id.LocalID.String()
	}
	rows, err := a.Store.Read().QueryContext(ctx, `SELECT u.id, u.nombre_mostrar, u.rol, u.avatar_key FROM usuarios u
		WHERE u.activo = 1 AND u.pin_hash IS NOT NULL AND u.rol IN ('ADMIN','CAJERO','MESERO')
		  AND (NOT EXISTS (SELECT 1 FROM usuario_locales x WHERE x.usuario_id = u.id)
		       OR EXISTS (SELECT 1 FROM usuario_locales x WHERE x.usuario_id = u.id AND x.local_id = ?))
		ORDER BY CASE u.rol WHEN 'MESERO' THEN 0 WHEN 'CAJERO' THEN 1 ELSE 2 END, u.nombre_mostrar`, local)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Persona{}
	for rows.Next() {
		var p Persona
		var sid string
		var avatar sql.NullString
		if err := rows.Scan(&sid, &p.Nombre, &p.Rol, &avatar); err != nil {
			return nil, err
		}
		p.ID, _ = ids.Parse(sid)
		if avatar.Valid && avatar.String != "" {
			u := "/media/" + avatar.String + "/sm.webp"
			p.AvatarURL = &u
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Usuario es quien está usando el dispositivo tras su PIN.
type Usuario struct {
	ID       ids.ID         `json:"id"`
	Nombre   string         `json:"nombre"`
	Rol      string         `json:"rol"`
	Permisos []rbac.Permiso `json:"permisos"`
	permisos map[rbac.Permiso]bool
}

func (u Usuario) Puede(p rbac.Permiso) bool { return u.permisos[p] }

func (a *App) cargarUsuario(ctx context.Context, q queryer, id ids.ID) (Usuario, bool, error) {
	var u Usuario
	var activo int
	err := q.QueryRowContext(ctx, `SELECT nombre_mostrar, rol, activo FROM usuarios WHERE id = ?`, id.String()).Scan(&u.Nombre, &u.Rol, &activo)
	if err != nil {
		return u, false, err
	}
	u.ID = id
	ajustes := map[rbac.Permiso]bool{}
	rows, err := q.QueryContext(ctx, `SELECT permiso, concedido FROM permisos_usuario WHERE usuario_id = ?`, id.String())
	if err != nil {
		return u, false, err
	}
	for rows.Next() {
		var p string
		var c int
		if rows.Scan(&p, &c) == nil {
			ajustes[rbac.Permiso(p)] = c == 1
		}
	}
	_ = rows.Close()
	u.permisos = rbac.Efectivos(rbac.Rol(u.Rol), ajustes)
	for p, ok := range u.permisos {
		if ok {
			u.Permisos = append(u.Permisos, p)
		}
	}
	sort.Slice(u.Permisos, func(i, j int) bool { return u.Permisos[i] < u.Permisos[j] })
	return u, activo == 1, nil
}

// pepperPIN devuelve el pepper de PIN del restaurante (lo pide a la nube si falta).
func (a *App) pepperPIN(ctx context.Context) ([]byte, error) {
	var p []byte
	if err := a.Store.Read().QueryRowContext(ctx, `SELECT pin_pepper FROM nodo WHERE id = 1`).Scan(&p); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if len(p) > 0 {
		return p, nil
	}
	if err := a.traerPepper(ctx); err != nil {
		return nil, problema(http.StatusServiceUnavailable, "SIN_CLAVE_PIN", "El nodo necesita conectarse a internet una vez para poder validar los PIN. Revisa la conexión de la PC de caja.")
	}
	err := a.Store.Read().QueryRowContext(ctx, `SELECT pin_pepper FROM nodo WHERE id = 1`).Scan(&p)
	return p, err
}

// traerPepper pide a la nube el pepper de PIN de este restaurante y lo guarda.
func (a *App) traerPepper(ctx context.Context) error {
	a.syncMu.Lock()
	cli := a.nube
	a.syncMu.Unlock()
	if cli == nil {
		return errors.New("nodo sin sincronización")
	}
	var res struct {
		Pepper string `json:"pepper"`
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := cli.Do(cctx, http.MethodGet, "/v1/nodos/secreto-pin", nil, &res, true); err != nil {
		return err
	}
	p, err := base64.StdEncoding.DecodeString(res.Pepper)
	if err != nil || len(p) < 16 {
		return errors.New("pepper inválido")
	}
	return a.Store.Write(ctx, func(tx *store.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE nodo SET pin_pepper = ? WHERE id = 1`, p)
		return err
	})
}

type EntrarIn struct {
	UsuarioID ids.ID `json:"usuarioId"`
	PIN       string `json:"pin"`
}

type SesionUsuario struct {
	Token    string    `json:"token"`
	ExpiraAt time.Time `json:"expiraAt"`
	Usuario  Usuario   `json:"usuario"`
}

var errPIN = problema(http.StatusUnauthorized, "PIN_INCORRECTO", "PIN incorrecto.")

// verificarPIN aplica el bloqueo por dispositivo y devuelve el usuario si el PIN es correcto.
func (a *App) verificarPIN(ctx context.Context, d Dispositivo, usuario ids.ID, pin string) (Usuario, error) {
	now := a.Clock.Now()
	var hasta sql.NullString
	_ = a.Store.Read().QueryRowContext(ctx, `SELECT bloqueado_hasta FROM intentos_pin WHERE dispositivo_id = ?`, d.ID.String()).Scan(&hasta)
	if hasta.Valid {
		if t, _ := time.Parse(time.RFC3339Nano, hasta.String); now.Before(t) {
			seg := int(t.Sub(now).Seconds()) + 1
			return Usuario{}, problema(http.StatusTooManyRequests, "PIN_BLOQUEADO", "Demasiados intentos. Espera "+itoa((seg+59)/60)+" min o pide ayuda al administrador.")
		}
	}
	pepper, err := a.pepperPIN(ctx)
	if err != nil {
		return Usuario{}, err
	}
	var hash sql.NullString
	err = a.Store.Read().QueryRowContext(ctx, `SELECT pin_hash FROM usuarios WHERE id = ? AND activo = 1`, usuario.String()).Scan(&hash)
	ok := false
	if err == nil && hash.Valid {
		ok, _ = secreto.VerifyPIN(pepper, usuario, strings.TrimSpace(pin), hash.String)
	} else {
		// Mismo costo de tiempo aunque el usuario no exista.
		_, _ = secreto.VerifyPIN(pepper, usuario, pin, "$argon2id$v=19$m=65536,t=2,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	}
	if !ok {
		a.registrarFalloPIN(ctx, d, usuario, now)
		return Usuario{}, errPIN
	}
	if err := a.Store.Write(ctx, func(tx *store.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE intentos_pin SET fallos = 0, bloqueado_hasta = NULL WHERE dispositivo_id = ?`, d.ID.String())
		return err
	}); err != nil {
		return Usuario{}, err
	}
	u, activo, err := a.cargarUsuario(ctx, a.Store.Read(), usuario)
	if err != nil || !activo {
		return Usuario{}, errPIN
	}
	return u, nil
}

func (a *App) registrarFalloPIN(ctx context.Context, d Dispositivo, usuario ids.ID, now time.Time) {
	ts := now.Format(time.RFC3339Nano)
	var fallos, enVentana int
	var desde string
	err := a.Store.Write(ctx, func(tx *store.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT fallos, ventana_desde, en_ventana FROM intentos_pin WHERE dispositivo_id = ?`, d.ID.String()).Scan(&fallos, &desde, &enVentana)
		if errors.Is(err, sql.ErrNoRows) {
			desde = ts
		} else if err != nil {
			return err
		}
		if t, _ := time.Parse(time.RFC3339Nano, desde); now.Sub(t) > ventanaAlerta {
			desde, enVentana = ts, 0
		}
		fallos++
		enVentana++
		var bloq any
		if fallos >= maxFallosPIN {
			bloq, fallos = now.Add(bloqueoPIN).Format(time.RFC3339Nano), 0
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO intentos_pin (dispositivo_id, fallos, bloqueado_hasta, ventana_desde, en_ventana) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (dispositivo_id) DO UPDATE SET fallos = excluded.fallos, bloqueado_hasta = coalesce(excluded.bloqueado_hasta, bloqueado_hasta),
			ventana_desde = excluded.ventana_desde, en_ventana = excluded.en_ventana`, d.ID.String(), fallos, bloq, desde, enVentana); err != nil {
			return err
		}
		if bloq != nil || enVentana == alertaPIN {
			accion := "PIN_BLOQUEO_DISPOSITIVO"
			if enVentana == alertaPIN {
				accion = "PIN_ALERTA_INTENTOS"
			}
			return auditar(ctx, tx, accion, "dispositivo", d.ID, nil, map[string]any{"usuarioIntentado": usuario, "intentosEnVentana": enVentana}, now)
		}
		return nil
	})
	if err != nil {
		a.Log.Error("no se pudo registrar el intento de PIN", "err", err)
	}
	if enVentana == alertaPIN {
		a.Log.Warn("alerta: 20 PIN erróneos en 10 min en un dispositivo", "dispositivo", d.ID)
	}
}

// Entrar valida el PIN en el nodo (nunca en el teléfono) y abre la sesión del usuario.
func (a *App) Entrar(ctx context.Context, d Dispositivo, in EntrarIn) (SesionUsuario, error) {
	u, err := a.verificarPIN(ctx, d, in.UsuarioID, in.PIN)
	if err != nil {
		return SesionUsuario{}, err
	}
	now := a.Clock.Now()
	s := SesionUsuario{Token: tokenAleatorio(), ExpiraAt: now.Add(SesionUsuarioTTL), Usuario: u}
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		// Cambio rápido de usuario: la sesión anterior del mismo dispositivo se cierra.
		if _, err := tx.ExecContext(ctx, `UPDATE sesiones_usuario SET cerrada_at = ? WHERE dispositivo_id = ? AND cerrada_at IS NULL`, now.Format(time.RFC3339Nano), d.ID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sesiones_usuario (token_hash, usuario_id, dispositivo_id, creada_at, expira_at) VALUES (?, ?, ?, ?, ?)`,
			hashToken(s.Token), u.ID.String(), d.ID.String(), now.Format(time.RFC3339Nano), s.ExpiraAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		uid := u.ID
		return auditar(ctx, tx, "INICIO_SESION_PIN", "dispositivo", d.ID, &uid, map[string]any{}, now)
	})
	return s, err
}

// Salir cierra la sesión del usuario en el dispositivo.
func (a *App) Salir(ctx context.Context, r *http.Request) error {
	tok := tokenDe(r, "Usuario")
	if tok == "" {
		return nil
	}
	return a.Store.Write(ctx, func(tx *store.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE sesiones_usuario SET cerrada_at = ? WHERE token_hash = ? AND cerrada_at IS NULL`, a.now(), hashToken(tok))
		return err
	})
}

var errSesionUsuario = problema(http.StatusUnauthorized, "SESION_USUARIO", "Tu sesión terminó. Entra otra vez con tu PIN.")

// usuarioDe resuelve el usuario de la petición (cabecera «Authorization: Usuario <token>»,
// que viaja junto a la del dispositivo). Un usuario desactivado pierde la sesión al instante.
func (a *App) usuarioDe(r *http.Request, d Dispositivo) (Usuario, error) {
	tok := tokenDe(r, "Usuario")
	if tok == "" {
		return Usuario{}, errSesionUsuario
	}
	var uid, disp, expira string
	var cerrada sql.NullString
	err := a.Store.Read().QueryRowContext(r.Context(), `SELECT usuario_id, dispositivo_id, expira_at, cerrada_at FROM sesiones_usuario WHERE token_hash = ?`, hashToken(tok)).
		Scan(&uid, &disp, &expira, &cerrada)
	if err != nil || cerrada.Valid || disp != d.ID.String() {
		return Usuario{}, errSesionUsuario
	}
	if t, _ := time.Parse(time.RFC3339Nano, expira); a.Clock.Now().After(t) {
		return Usuario{}, errSesionUsuario
	}
	id, err := ids.Parse(uid)
	if err != nil {
		return Usuario{}, errSesionUsuario
	}
	u, activo, err := a.cargarUsuario(r.Context(), a.Store.Read(), id)
	if err != nil || !activo {
		return Usuario{}, errSesionUsuario
	}
	return u, nil
}

// ---------- Autorización de supervisor (F3-14) ----------

type AutorizarIn struct {
	UsuarioID  ids.ID       `json:"usuarioId"`
	PIN        string       `json:"pin"`
	Accion     rbac.Permiso `json:"accion"`
	Referencia string       `json:"referencia"`
}

type Autorizacion struct {
	Token      string    `json:"token"`
	ExpiraAt   time.Time `json:"expiraAt"`
	Autorizado string    `json:"autorizadoPor"`
}

// Autorizar: un supervisor escribe su PIN en el mismo dispositivo y autoriza UNA acción
// concreta (p. ej. anular los platos de una orden) durante un minuto.
func (a *App) Autorizar(ctx context.Context, d Dispositivo, in AutorizarIn) (Autorizacion, error) {
	if strings.TrimSpace(in.Referencia) == "" {
		return Autorizacion{}, invalido("Falta a qué se aplica la autorización.")
	}
	u, err := a.verificarPIN(ctx, d, in.UsuarioID, in.PIN)
	if err != nil {
		return Autorizacion{}, err
	}
	if !u.Puede(in.Accion) {
		return Autorizacion{}, problema(http.StatusForbidden, "SIN_PERMISO", u.Nombre+" no tiene permiso para autorizar esto.")
	}
	now := a.Clock.Now()
	au := Autorizacion{Token: tokenAleatorio(), ExpiraAt: now.Add(AutorizacionTTL), Autorizado: u.Nombre}
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO autorizaciones (token_hash, accion, referencia, autorizado_por, dispositivo_id, expira_at) VALUES (?, ?, ?, ?, ?, ?)`,
			hashToken(au.Token), string(in.Accion), in.Referencia, u.ID.String(), d.ID.String(), au.ExpiraAt.Format(time.RFC3339Nano))
		return err
	})
	return au, err
}

// consumirAutorizacion valida y gasta una autorización para esa acción y referencia.
func consumirAutorizacion(ctx context.Context, tx *store.Tx, token string, d Dispositivo, accion rbac.Permiso, ref string, now time.Time) (ids.ID, error) {
	var por, expira string
	var usado sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT autorizado_por, expira_at, usado_at FROM autorizaciones WHERE token_hash = ? AND accion = ? AND referencia = ? AND dispositivo_id = ?`,
		hashToken(token), string(accion), ref, d.ID.String()).Scan(&por, &expira, &usado)
	sinAut := problema(http.StatusForbidden, "AUTORIZACION_INVALIDA", "La autorización del supervisor no es válida o ya se usó. Pídela de nuevo.")
	if err != nil || usado.Valid {
		return ids.Nil, sinAut
	}
	if t, _ := time.Parse(time.RFC3339Nano, expira); now.After(t) {
		return ids.Nil, sinAut
	}
	if _, err := tx.ExecContext(ctx, `UPDATE autorizaciones SET usado_at = ? WHERE token_hash = ?`, now.Format(time.RFC3339Nano), hashToken(token)); err != nil {
		return ids.Nil, err
	}
	return ids.Parse(por)
}
