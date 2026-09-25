// Package auth implementa el inicio de sesión web (RF-01-02): contraseña Argon2id, bloqueo
// por intentos, access token corto + refresh rotativo con detección de reutilización,
// recuperación por correo y cambio obligatorio de la contraseña temporal.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

const (
	maxFallos = 5
	bloqueo   = 15 * time.Minute
	resetTTL  = 30 * time.Minute
)

var (
	errCredenciales = apperr.New(apperr.Unauthorized, "CREDENCIALES_INVALIDAS", "El correo o la contraseña no coinciden. Revisa e intenta de nuevo.")
	errBloqueado    = apperr.New(apperr.TooMany, "CUENTA_BLOQUEADA", "Demasiados intentos fallidos. Espera 15 minutos o recupera tu contraseña.")
	errSesion       = apperr.New(apperr.Unauthorized, "SESION_VENCIDA", "Tu sesión terminó. Vuelve a iniciar sesión.")
	errTokenReset   = apperr.New(apperr.Invalid, "ENLACE_INVALIDO", "El enlace para cambiar la contraseña ya se usó o venció. Pide uno nuevo.")
)

type Service struct {
	DB            *db.DB
	Signer        *Signer
	Mail          mail.Sender
	Clock         clock.Clock
	BackofficeURL string
}

// Usuario es lo que el backoffice necesita saber de quien inició sesión.
type Usuario struct {
	ID                  ids.ID `json:"id"`
	TenantID            ids.ID `json:"tenantId"`
	NombreMostrar       string `json:"nombreMostrar"`
	Email               string `json:"email"`
	Rol                 string `json:"rol"`
	NombreComercial     string `json:"nombreComercial"`
	DebeCambiarPassword bool   `json:"debeCambiarPassword"`
}

// Sesion es el resultado de iniciar sesión o renovar.
type Sesion struct {
	AccessToken  string  `json:"accessToken"`
	ExpiresIn    int     `json:"expiresIn"`
	Usuario      Usuario `json:"usuario"`
	RefreshToken string  `json:"-"` // va en cookie HttpOnly, nunca en el cuerpo
}

type Cliente struct {
	IP        string
	UserAgent string
}

// Login valida credenciales. Mismo mensaje y mismo tiempo exista o no el usuario.
func (s *Service) Login(ctx context.Context, login, password string, cli Cliente) (Sesion, error) {
	login = strings.TrimSpace(strings.ToLower(login))
	if login == "" || password == "" {
		return Sesion{}, errCredenciales
	}
	clave := lockKey(login, cli.IP)
	if bloqueado, err := s.bloqueado(ctx, clave); err != nil {
		return Sesion{}, err
	} else if bloqueado {
		return Sesion{}, errBloqueado
	}

	var userID, tenantID ids.ID
	var hash string
	var activo, debeCambiar bool
	var estado string
	err := s.DB.Global(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT usuario_id, tenant_id, password_hash, activo, debe_cambiar_password, estado_tenant FROM auth_buscar_login($1)`, login).
			Scan(&userID, &tenantID, &hash, &activo, &debeCambiar, &estado)
	})
	encontrado := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Sesion{}, err
	}
	if !encontrado {
		hash = dummyHash
	}
	ok, err := VerifyPassword(password, hash)
	if err != nil {
		return Sesion{}, err
	}
	if !ok || !encontrado || !activo {
		if err := s.registrarFallo(ctx, clave); err != nil {
			return Sesion{}, err
		}
		return Sesion{}, errCredenciales
	}
	if err := s.limpiarFallos(ctx, clave); err != nil {
		return Sesion{}, err
	}
	return s.abrirSesion(ctx, tenantID, userID, ids.New(), cli)
}

// abrirSesion crea una sesión (o la siguiente de una familia al rotar) y emite los tokens.
func (s *Service) abrirSesion(ctx context.Context, tenantID, userID, familia ids.ID, cli Cliente) (Sesion, error) {
	var out Sesion
	refresh, hash := NewOpaqueToken()
	sid := ids.New()
	now := s.Clock.Now()
	err := s.DB.InTenant(ctx, tenantID, func(tx db.Tx) error {
		u, err := cargarUsuario(ctx, tx, userID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO sesiones (id, tenant_id, usuario_id, familia, refresh_hash, expira_at, user_agent, ip)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, sid, tenantID, userID, familia, hash, now.Add(RefreshTTL), truncar(cli.UserAgent, 300), ipOrNil(cli.IP)); err != nil {
			return err
		}
		out.Usuario = u
		return nil
	})
	if err != nil {
		return Sesion{}, err
	}
	out.AccessToken, err = s.Signer.Issue(Principal{UserID: userID, TenantID: tenantID, SessionID: sid, Rol: out.Usuario.Rol, DebeCambiar: out.Usuario.DebeCambiarPassword})
	out.ExpiresIn, out.RefreshToken = int(AccessTTL.Seconds()), refresh
	return out, err
}

// Refresh rota el refresh token. Si llega uno ya rotado (robado y reutilizado), se revoca
// toda la familia: el atacante y el usuario legítimo deben volver a iniciar sesión.
func (s *Service) Refresh(ctx context.Context, refresh string, cli Cliente) (Sesion, error) {
	if refresh == "" {
		return Sesion{}, errSesion
	}
	var sid, tenantID, userID, familia ids.ID
	var expira time.Time
	var revocada *time.Time
	err := s.DB.Global(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT sesion_id, tenant_id, usuario_id, familia, expira_at, revocada_at FROM auth_buscar_sesion($1)`, HashToken(refresh)).
			Scan(&sid, &tenantID, &userID, &familia, &expira, &revocada)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Sesion{}, errSesion
	}
	if err != nil {
		return Sesion{}, err
	}
	now := s.Clock.Now()
	if revocada != nil {
		if err := s.revocarFamilia(ctx, tenantID, familia); err != nil {
			return Sesion{}, err
		}
		slog.WarnContext(ctx, "auth: refresh reutilizado, familia revocada", "tenant_id", tenantID, "user_id", userID)
		return Sesion{}, errSesion
	}
	if now.After(expira) {
		return Sesion{}, errSesion
	}
	// Revocar la actual y abrir la siguiente de la misma familia.
	var activo bool
	err = s.DB.InTenant(ctx, tenantID, func(tx db.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT activo FROM usuarios WHERE id = $1`, userID).Scan(&activo); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE sesiones SET revocada_at = $2 WHERE id = $1 AND revocada_at IS NULL`, sid, now)
		if err == nil && tag.RowsAffected() == 0 {
			return errSesion // otra petición la rotó al mismo tiempo
		}
		return err
	})
	if err != nil {
		return Sesion{}, err
	}
	if !activo {
		return Sesion{}, errSesion
	}
	return s.abrirSesion(ctx, tenantID, userID, familia, cli)
}

func (s *Service) revocarFamilia(ctx context.Context, tenantID, familia ids.ID) error {
	return s.DB.InTenant(ctx, tenantID, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE sesiones SET revocada_at = coalesce(revocada_at, $2) WHERE familia = $1`, familia, s.Clock.Now())
		return err
	})
}

// Logout revoca la sesión del refresh token (si existe). Siempre termina bien.
func (s *Service) Logout(ctx context.Context, refresh string) error {
	if refresh == "" {
		return nil
	}
	var tenantID, familia ids.ID
	err := s.DB.Global(ctx, func(tx db.Tx) error {
		var sid, uid ids.ID
		var exp time.Time
		var rev *time.Time
		return tx.QueryRow(ctx, `SELECT sesion_id, tenant_id, usuario_id, familia, expira_at, revocada_at FROM auth_buscar_sesion($1)`, HashToken(refresh)).
			Scan(&sid, &tenantID, &uid, &familia, &exp, &rev)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.revocarFamilia(ctx, tenantID, familia)
}

// SolicitarRecuperacion envía un enlace de un solo uso válido 30 min (RF-01-02.3).
// Responde igual exista o no el correo, para no revelar qué cuentas existen.
func (s *Service) SolicitarRecuperacion(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if !strings.Contains(email, "@") {
		return nil
	}
	var userID, tenantID ids.ID
	err := s.DB.Global(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT usuario_id, tenant_id FROM auth_buscar_por_email($1)`, email).Scan(&userID, &tenantID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	token, hash := NewOpaqueToken()
	var nombre string
	err = s.DB.InTenant(ctx, tenantID, func(tx db.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT nombre_mostrar FROM usuarios WHERE id = $1`, userID).Scan(&nombre); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO tokens_recuperacion (id, tenant_id, usuario_id, token_hash, expira_at) VALUES ($1,$2,$3,$4,$5)`,
			ids.New(), tenantID, userID, hash, s.Clock.Now().Add(resetTTL))
		return err
	})
	if err != nil {
		return err
	}
	text, html, err := mail.Render(mail.Contenido{
		Titulo:   "Cambia tu contraseña",
		Parrafos: []string{fmt.Sprintf("Hola, %s. Recibimos una solicitud para cambiar la contraseña de tu restaurante.", nombre), "El enlace sirve una sola vez durante 30 minutos."},
		Boton:    "Crear contraseña nueva", BotonURL: s.BackofficeURL + "/restablecer?token=" + token,
		Pie: "Si no fuiste tú, ignora este correo: tu contraseña sigue igual.",
	})
	if err != nil {
		return err
	}
	return s.Mail.Send(ctx, mail.Message{To: email, Subject: "Cambia tu contraseña", Text: text, HTML: html})
}

// Restablecer cambia la contraseña con un token de recuperación y cierra todas las sesiones.
func (s *Service) Restablecer(ctx context.Context, token, nueva string) error {
	var tokenID, tenantID, userID ids.ID
	var expira time.Time
	var usado *time.Time
	err := s.DB.Global(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT token_id, tenant_id, usuario_id, expira_at, usado_at FROM auth_buscar_token_recuperacion($1)`, HashToken(token)).
			Scan(&tokenID, &tenantID, &userID, &expira, &usado)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errTokenReset
	}
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	if usado != nil || now.After(expira) {
		return errTokenReset
	}
	return s.DB.InTenant(ctx, tenantID, func(tx db.Tx) error {
		var email string
		if err := tx.QueryRow(ctx, `SELECT coalesce(email::text, '') FROM usuarios WHERE id = $1`, userID).Scan(&email); err != nil {
			return err
		}
		if m := ValidarPassword(nueva, email); m != "" {
			return fieldErr("password", m)
		}
		tag, err := tx.Exec(ctx, `UPDATE tokens_recuperacion SET usado_at = $2 WHERE id = $1 AND usado_at IS NULL`, tokenID, now)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errTokenReset
		}
		return s.guardarPassword(ctx, tx, userID, nueva, now)
	})
}

// CambiarPassword cambia la contraseña conociendo la actual (incluida la temporal).
// Revoca todas las sesiones (cualquier otro dispositivo queda fuera) y abre una nueva para
// quien hizo el cambio, así no tiene que volver a escribir la contraseña.
func (s *Service) CambiarPassword(ctx context.Context, p Principal, actual, nueva string, cli Cliente) (Sesion, error) {
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var hash, email string
		if err := tx.QueryRow(ctx, `SELECT coalesce(password_hash, ''), coalesce(email::text, '') FROM usuarios WHERE id = $1`, p.UserID).Scan(&hash, &email); err != nil {
			return err
		}
		if ok, _ := VerifyPassword(actual, hash); !ok || hash == "" {
			return fieldErr("actual", "La contraseña actual no es correcta.")
		}
		if actual == nueva {
			return fieldErr("nueva", "La contraseña nueva debe ser distinta de la actual.")
		}
		if m := ValidarPassword(nueva, email); m != "" {
			return fieldErr("nueva", m)
		}
		return s.guardarPassword(ctx, tx, p.UserID, nueva, s.Clock.Now())
	})
	if err != nil {
		return Sesion{}, err
	}
	return s.abrirSesion(ctx, p.TenantID, p.UserID, ids.New(), cli)
}

// guardarPassword actualiza el hash y revoca todas las sesiones del usuario.
func (s *Service) guardarPassword(ctx context.Context, tx db.Tx, userID ids.ID, nueva string, now time.Time) error {
	h, err := HashPassword(nueva)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE usuarios SET password_hash = $2, debe_cambiar_password = false WHERE id = $1`, userID, h); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE sesiones SET revocada_at = $2 WHERE usuario_id = $1 AND revocada_at IS NULL`, userID, now)
	return err
}

// Me devuelve el usuario de la sesión.
func (s *Service) Me(ctx context.Context, p Principal) (Usuario, error) {
	var u Usuario
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var err error
		u, err = cargarUsuario(ctx, tx, p.UserID)
		return err
	})
	return u, err
}

func cargarUsuario(ctx context.Context, tx db.Tx, id ids.ID) (Usuario, error) {
	var u Usuario
	err := tx.QueryRow(ctx, `SELECT u.id, u.tenant_id, u.nombre_mostrar, coalesce(u.email::text, ''), u.rol, t.nombre_comercial, u.debe_cambiar_password
		FROM usuarios u JOIN tenants t ON t.id = u.tenant_id WHERE u.id = $1`, id).
		Scan(&u.ID, &u.TenantID, &u.NombreMostrar, &u.Email, &u.Rol, &u.NombreComercial, &u.DebeCambiarPassword)
	return u, err
}

// ---------- bloqueo por intentos ----------

func lockKey(login, ip string) string {
	h := sha256.Sum256([]byte(login + "|" + ip))
	return hex.EncodeToString(h[:])
}

func (s *Service) bloqueado(ctx context.Context, clave string) (bool, error) {
	var hasta *time.Time
	err := s.DB.Global(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT bloqueado_hasta FROM intentos_login WHERE clave = $1`, clave).Scan(&hasta)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil && hasta != nil && s.Clock.Now().Before(*hasta), err
}

func (s *Service) registrarFallo(ctx context.Context, clave string) error {
	now := s.Clock.Now()
	return s.DB.Global(ctx, func(tx db.Tx) error {
		// Un bloqueo vencido reinicia el conteo.
		_, err := tx.Exec(ctx, `INSERT INTO intentos_login (clave, fallos, updated_at) VALUES ($1, 1, $2)
			ON CONFLICT (clave) DO UPDATE SET
			  fallos = CASE WHEN intentos_login.bloqueado_hasta IS NOT NULL AND intentos_login.bloqueado_hasta <= $2 THEN 1 ELSE intentos_login.fallos + 1 END,
			  bloqueado_hasta = CASE WHEN intentos_login.bloqueado_hasta IS NOT NULL AND intentos_login.bloqueado_hasta <= $2 THEN NULL
			                         WHEN intentos_login.fallos + 1 >= $3 THEN $2 + $4::interval ELSE intentos_login.bloqueado_hasta END,
			  updated_at = $2`, clave, now, maxFallos, fmt.Sprintf("%d seconds", int(bloqueo.Seconds())))
		return err
	})
}

func (s *Service) limpiarFallos(ctx context.Context, clave string) error {
	return s.DB.Global(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM intentos_login WHERE clave = $1`, clave)
		return err
	})
}

func fieldErr(campo, msg string) error {
	return &apperr.Error{Kind: apperr.Invalid, Code: "DATOS_INVALIDOS", Message: msg, Fields: []apperr.FieldError{{Campo: campo, Mensaje: msg}}}
}

func truncar(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func ipOrNil(ip string) any {
	if a, err := netip.ParseAddr(ip); err == nil {
		return a.String()
	}
	return nil
}
