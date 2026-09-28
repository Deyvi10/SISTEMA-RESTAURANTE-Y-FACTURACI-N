// Package auth implementa el inicio de sesión web (RF-01-02): contraseña Argon2id, bloqueo
// por intentos, access token corto + refresh rotativo con detección de reutilización,
// recuperación por correo y cambio obligatorio de la contraseña temporal.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/limite"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

const (
	// graciaRotacion: un refresh recién rotado que llega dentro de esta ventana es una carrera
	// benigna (recargar mientras se renovaba, dos pestañas), no un robo.
	graciaRotacion = 30 * time.Second
	maxFallos      = 5
	bloqueo        = 15 * time.Minute
	resetTTL       = 30 * time.Minute
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
	return s.abrirSesion(ctx, tenantID, userID, ids.New(), ids.New(), cli)
}

// abrirSesion crea una sesión (o la siguiente de una familia al rotar) y emite los tokens.
func (s *Service) abrirSesion(ctx context.Context, tenantID, userID, familia, sid ids.ID, cli Cliente) (Sesion, error) {
	return s.abrirSesionTras(ctx, tenantID, userID, familia, sid, cli, nil)
}

// abrirSesionTras ejecuta antes (p. ej. revocar la sesión que se rota) en la MISMA
// transacción que crea la nueva: nunca existe un instante en que la familia no tenga una
// sesión viva, así una renovación simultánea siempre encuentra la continuación.
func (s *Service) abrirSesionTras(ctx context.Context, tenantID, userID, familia, sid ids.ID, cli Cliente, antes func(db.Tx) error) (Sesion, error) {
	var out Sesion
	refresh, hash := NewOpaqueToken()
	now := s.Clock.Now()
	err := s.DB.InTenant(ctx, tenantID, func(tx db.Tx) error {
		if antes != nil {
			if err := antes(tx); err != nil {
				return err
			}
		}
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

// Refresh rota el refresh token. Si llega uno ya rotado fuera de la ventana de gracia
// (robado y reutilizado), se revoca toda la familia: atacante y usuario legítimo deben
// volver a iniciar sesión. Dentro de la ventana se trata como una carrera benigna.
func (s *Service) Refresh(ctx context.Context, refresh string, cli Cliente) (Sesion, error) {
	if refresh == "" {
		return Sesion{}, errSesion
	}
	var sid, tenantID, userID, familia ids.ID
	var expira time.Time
	var revocada *time.Time
	var reemplazada *ids.ID
	err := s.DB.Global(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT sesion_id, tenant_id, usuario_id, familia, expira_at, revocada_at, reemplazada_por FROM auth_buscar_sesion($1)`, HashToken(refresh)).
			Scan(&sid, &tenantID, &userID, &familia, &expira, &revocada, &reemplazada)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Sesion{}, errSesion
	}
	if err != nil {
		return Sesion{}, err
	}
	now := s.Clock.Now()
	if now.After(expira) {
		return Sesion{}, errSesion
	}
	nueva := ids.New()
	if revocada != nil {
		if reemplazada != nil && now.Sub(*revocada) <= graciaRotacion {
			return s.reabrirEnGracia(ctx, tenantID, userID, familia, nueva, cli)
		}
		if err := s.revocarFamilia(ctx, tenantID, familia); err != nil {
			return Sesion{}, err
		}
		slog.WarnContext(ctx, "auth: refresh reutilizado, familia revocada", "tenant_id", tenantID, "user_id", userID)
		return Sesion{}, errSesion
	}
	// Revocar la actual (anotando su reemplazo) y abrir la siguiente de la misma familia,
	// en una sola transacción.
	out, err := s.abrirSesionTras(ctx, tenantID, userID, familia, nueva, cli, func(tx db.Tx) error {
		var activo bool
		if err := tx.QueryRow(ctx, `SELECT activo FROM usuarios WHERE id = $1`, userID).Scan(&activo); err != nil {
			return err
		}
		if !activo {
			return errSesion
		}
		// Si otra renovación ya rotó este token, el UPDATE espera a que confirme (bloqueo
		// de fila) y luego no afecta filas: su sesión nueva ya está visible.
		tag, err := tx.Exec(ctx, `UPDATE sesiones SET revocada_at = $2, reemplazada_por = $3 WHERE id = $1 AND revocada_at IS NULL`, sid, now, nueva)
		if err == nil && tag.RowsAffected() == 0 {
			return errRotadaEnParalelo
		}
		return err
	})
	if errors.Is(err, errRotadaEnParalelo) {
		// Dos renovaciones simultáneas con el mismo token (recarga durante una renovación):
		// la otra ya rotó hace instantes, así que es la misma carrera benigna.
		return s.reabrirEnGracia(ctx, tenantID, userID, familia, ids.New(), cli)
	}
	return out, err
}

var errRotadaEnParalelo = errors.New("auth: sesión rotada por otra petición")

// reabrirEnGracia abre otra sesión en la familia si sigue viva. Tras un logout o si el
// usuario fue desactivado, no se reabre nada.
func (s *Service) reabrirEnGracia(ctx context.Context, tenantID, userID, familia, nueva ids.ID, cli Cliente) (Sesion, error) {
	var viva bool
	if err := s.DB.InTenant(ctx, tenantID, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sesiones s JOIN usuarios u ON u.id = s.usuario_id
			WHERE s.familia = $1 AND s.revocada_at IS NULL AND u.activo)`, familia).Scan(&viva)
	}); err != nil {
		return Sesion{}, err
	}
	if !viva {
		return Sesion{}, errSesion
	}
	return s.abrirSesion(ctx, tenantID, userID, familia, nueva, cli)
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
		return tx.QueryRow(ctx, `SELECT tenant_id, familia FROM auth_buscar_sesion($1)`, HashToken(refresh)).Scan(&tenantID, &familia)
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
	return s.abrirSesion(ctx, p.TenantID, p.UserID, ids.New(), ids.New(), cli)
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

func lockKey(login, ip string) string { return limite.Clave(login, ip) }

func (s *Service) limiter() *limite.Limiter {
	return &limite.Limiter{DB: s.DB, Now: s.Clock.Now, Max: maxFallos, Bloqueo: bloqueo}
}

func (s *Service) bloqueado(ctx context.Context, clave string) (bool, error) {
	return s.limiter().Bloqueado(ctx, clave)
}

func (s *Service) registrarFallo(ctx context.Context, clave string) error {
	return s.limiter().Fallo(ctx, clave)
}

func (s *Service) limpiarFallos(ctx context.Context, clave string) error {
	return s.limiter().Limpiar(ctx, clave)
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
