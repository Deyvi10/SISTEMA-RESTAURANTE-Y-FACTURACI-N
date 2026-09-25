// Package personal gestiona el personal del restaurante (RF-01-04): un mesero se crea en
// 10 segundos con nombre y PIN; los usuarios nunca se borran, solo se desactivan.
package personal

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/imagenes"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/rbac"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

type Service struct {
	DB            *db.DB
	Mail          mail.Sender
	Pepper        []byte
	BackofficeURL string
	ImagenURL     func(key, tam string) string
}

type Usuario struct {
	ID            ids.ID  `json:"id"`
	NombreMostrar string  `json:"nombreMostrar"`
	Rol           string  `json:"rol"`
	Email         *string `json:"email"`
	EsDueno       bool    `json:"esDueno"`
	TienePIN      bool    `json:"tienePin"`
	AccesoWeb     bool    `json:"accesoWeb"`
	Activo        bool    `json:"activo"`
	AvatarKey     *string `json:"avatarKey"`
	AvatarURL     *string `json:"avatarUrl"`
}

type UsuarioInput struct {
	ID            *ids.ID `json:"id"`
	NombreMostrar string  `json:"nombreMostrar"`
	Rol           string  `json:"rol"`
	PIN           string  `json:"pin"`
	Email         *string `json:"email"`
	AvatarKey     *string `json:"avatarKey"`
}

// Los roles operativos usan la app o el POS con PIN (docs/06 §2).
func requierePIN(rol rbac.Rol) bool { return rol != rbac.Admin }

func (in *UsuarioInput) normalizar() {
	in.NombreMostrar = strings.TrimSpace(in.NombreMostrar)
	in.PIN = strings.TrimSpace(in.PIN)
	if in.Email != nil {
		e := strings.ToLower(strings.TrimSpace(*in.Email))
		in.Email = &e
		if e == "" {
			in.Email = nil
		}
	}
}

func (in UsuarioInput) validar(tenant ids.ID, nuevo bool) error {
	var v apperr.Validation
	if in.AvatarKey != nil {
		v.Check(imagenes.ClaveValida(tenant, *in.AvatarKey) && strings.HasPrefix(*in.AvatarKey, "t/"), "avatarKey", "Esa foto no existe. Súbela de nuevo.")
	}
	rol := rbac.Rol(in.Rol)
	v.Check(len([]rune(in.NombreMostrar)) >= 1 && len([]rune(in.NombreMostrar)) <= 40, "nombreMostrar", "Escribe el nombre como aparecerá en la pantalla, por ejemplo «Carlos M.».")
	v.Check(rol.Valido(), "rol", "Elige un rol: Administrador, Cajero, Mesero, Cocina o Bodega.")
	if in.PIN != "" {
		if m := auth.ValidarPIN(in.PIN); m != "" {
			v.Add("pin", m)
		}
	} else if nuevo && requierePIN(rol) {
		v.Add("pin", "Asigna un PIN de 4 a 6 dígitos: con él entra en la app.")
	}
	if in.Email != nil {
		v.Check(strings.Count(*in.Email, "@") == 1 && len(*in.Email) >= 5 && len(*in.Email) <= 120, "email", "Escribe un correo válido.")
	}
	if rol == rbac.Admin {
		v.Check(in.Email != nil, "email", "El administrador necesita correo para entrar al panel.")
	}
	if in.Email != nil && rol != rbac.Admin && rol != rbac.Cajero {
		v.Add("email", "Solo administradores y cajeros entran al panel web. Los demás usan su PIN.")
	}
	return v.Err()
}

const cols = `id, nombre_mostrar, rol, email::text, es_dueno, pin_hash IS NOT NULL, password_hash IS NOT NULL, activo, avatar_key`

func (s *Service) scan(r pgx.CollectableRow) (Usuario, error) {
	var u Usuario
	err := r.Scan(&u.ID, &u.NombreMostrar, &u.Rol, &u.Email, &u.EsDueno, &u.TienePIN, &u.AccesoWeb, &u.Activo, &u.AvatarKey)
	if err == nil && u.AvatarKey != nil && s.ImagenURL != nil {
		url := s.ImagenURL(*u.AvatarKey, "sm")
		u.AvatarURL = &url
	}
	return u, err
}

func (s *Service) Listar(ctx context.Context, p auth.Principal) ([]Usuario, error) {
	var out []Usuario
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+cols+` FROM usuarios ORDER BY activo DESC, es_dueno DESC, nombre_mostrar`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, s.scan)
		return err
	})
	return out, err
}

func (s *Service) obtener(ctx context.Context, tx db.Tx, id ids.ID) (Usuario, error) {
	rows, err := tx.Query(ctx, `SELECT `+cols+` FROM usuarios WHERE id=$1`, id)
	if err != nil {
		return Usuario{}, err
	}
	u, err := pgx.CollectExactlyOneRow(rows, s.scan)
	return u, db.NotFound(err)
}

// Crear da de alta a una persona. Con correo (admin/cajero) recibe invitación y contraseña temporal.
func (s *Service) Crear(ctx context.Context, p auth.Principal, in UsuarioInput) (Usuario, error) {
	in.normalizar()
	if err := in.validar(p.TenantID, true); err != nil {
		return Usuario{}, err
	}
	id := db.IDOrNew(in.ID)
	var pinHash, fingerprint, pwHash *string
	if in.PIN != "" {
		h, err := auth.HashPIN(s.Pepper, id, in.PIN)
		if err != nil {
			return Usuario{}, err
		}
		fp := auth.FingerprintPIN(s.Pepper, in.PIN)
		pinHash, fingerprint = &h, &fp
	}
	temporal := ""
	if in.Email != nil {
		temporal = auth.PasswordTemporal()
		h, err := auth.HashPassword(temporal)
		if err != nil {
			return Usuario{}, err
		}
		pwHash = &h
	}
	var u Usuario
	var negocio string
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO usuarios (id, tenant_id, nombre_mostrar, rol, email, password_hash, debe_cambiar_password, pin_hash, pin_fingerprint, avatar_key, created_by)
			VALUES ($1,$2,$3,$4,$5,$6::text,$6::text IS NOT NULL,$7,$8,$9,$10) ON CONFLICT (id) DO NOTHING`,
			id, p.TenantID, in.NombreMostrar, in.Rol, in.Email, pwHash, pinHash, fingerprint, in.AvatarKey, p.UserID)
		if err != nil {
			return traducir(err)
		}
		if tag.RowsAffected() == 1 {
			if _, err := tx.Exec(ctx, `INSERT INTO usuario_locales (tenant_id, usuario_id, local_id) SELECT $1, $2, id FROM locales WHERE deleted_at IS NULL`, p.TenantID, id); err != nil {
				return err
			}
		} else {
			temporal = "" // reintento idempotente: no se reenvía la invitación
		}
		if err := tx.QueryRow(ctx, `SELECT nombre_comercial FROM tenants`).Scan(&negocio); err != nil {
			return err
		}
		u, err = s.obtener(ctx, tx, id)
		return err
	})
	if err != nil {
		return Usuario{}, err
	}
	if temporal != "" {
		if err := s.invitar(ctx, *in.Email, in.NombreMostrar, negocio, temporal); err != nil {
			return u, fmt.Errorf("usuario creado, pero no se pudo enviar la invitación: %w", err)
		}
	}
	return u, nil
}

func (s *Service) invitar(ctx context.Context, email, nombre, negocio, temporal string) error {
	text, html, err := mail.Render(mail.Contenido{
		Titulo:    fmt.Sprintf("%s te invitó a su equipo", negocio),
		Parrafos:  []string{fmt.Sprintf("Hola, %s. Ya puedes entrar al panel con tu correo y esta contraseña temporal. Te pediremos cambiarla la primera vez:", nombre)},
		Destacado: temporal,
		Boton:     "Entrar al panel", BotonURL: s.BackofficeURL + "/login",
		Pie: "Si no esperabas esta invitación, ignora este correo.",
	})
	if err != nil {
		return err
	}
	return s.Mail.Send(ctx, mail.Message{To: email, Subject: "Te invitaron a " + negocio, Text: text, HTML: html})
}

// Actualizar cambia nombre, rol, correo o avatar. El PIN se cambia aparte.
func (s *Service) Actualizar(ctx context.Context, p auth.Principal, id ids.ID, in UsuarioInput) (Usuario, error) {
	in.normalizar()
	pin := in.PIN
	in.PIN = ""
	if err := in.validar(p.TenantID, false); err != nil {
		return Usuario{}, err
	}
	var u Usuario
	var invitar, negocio string
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		actual, err := s.obtener(ctx, tx, id)
		if err != nil {
			return err
		}
		if in.Email != nil && !actual.AccesoWeb {
			// Recién obtiene acceso al panel: contraseña temporal e invitación.
			invitar = auth.PasswordTemporal()
			h, err := auth.HashPassword(invitar)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE usuarios SET password_hash=$2, debe_cambiar_password=true WHERE id=$1`, id, h); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, `SELECT nombre_comercial FROM tenants`).Scan(&negocio); err != nil {
				return err
			}
		}
		if actual.EsDueno && in.Rol != string(rbac.Admin) {
			return apperr.New(apperr.Conflict, "DUENO_ADMIN", "El dueño siempre es administrador.")
		}
		if requierePIN(rbac.Rol(in.Rol)) && !actual.TienePIN && pin == "" {
			return fieldErr("pin", "Este rol entra con PIN: asígnale uno.")
		}
		if in.Email == nil && actual.AccesoWeb && rbac.Rol(in.Rol) != rbac.Admin {
			// Quitar el correo quita el acceso web.
			if _, err := tx.Exec(ctx, `UPDATE usuarios SET password_hash = NULL WHERE id=$1`, id); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE usuarios SET nombre_mostrar=$2, rol=$3, email=$4, avatar_key=$5 WHERE id=$1`, id, in.NombreMostrar, in.Rol, in.Email, in.AvatarKey); err != nil {
			return traducir(err)
		}
		if pin != "" {
			if err := s.guardarPIN(ctx, tx, id, pin); err != nil {
				return err
			}
		}
		u, err = s.obtener(ctx, tx, id)
		return err
	})
	if err == nil && invitar != "" {
		if err := s.invitar(ctx, *in.Email, in.NombreMostrar, negocio, invitar); err != nil {
			return u, fmt.Errorf("cambios guardados, pero no se pudo enviar la invitación: %w", err)
		}
	}
	return u, err
}

// CambiarPIN asigna un PIN nuevo (el anterior deja de servir).
func (s *Service) CambiarPIN(ctx context.Context, p auth.Principal, id ids.ID, pin string) error {
	if m := auth.ValidarPIN(strings.TrimSpace(pin)); m != "" {
		return fieldErr("pin", m)
	}
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		if _, err := s.obtener(ctx, tx, id); err != nil {
			return err
		}
		return s.guardarPIN(ctx, tx, id, strings.TrimSpace(pin))
	})
}

func (s *Service) guardarPIN(ctx context.Context, tx db.Tx, id ids.ID, pin string) error {
	h, err := auth.HashPIN(s.Pepper, id, pin)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE usuarios SET pin_hash=$2, pin_fingerprint=$3 WHERE id=$1`, id, h, auth.FingerprintPIN(s.Pepper, pin))
	return traducir(err)
}

// CambiarEstado activa o desactiva. Al desactivar se cierran sus sesiones al instante
// (RF-01-04.4); el Nodo Local lo quitará de la cuadrícula con el evento user.deactivated (F2-03).
func (s *Service) CambiarEstado(ctx context.Context, p auth.Principal, id ids.ID, activo bool) (Usuario, error) {
	var u Usuario
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		actual, err := s.obtener(ctx, tx, id)
		if err != nil {
			return err
		}
		if !activo && actual.EsDueno {
			return apperr.New(apperr.Conflict, "DUENO_ACTIVO", "La cuenta del dueño no se puede desactivar.")
		}
		if !activo && id == p.UserID {
			return apperr.New(apperr.Conflict, "AUTODESACTIVAR", "No puedes desactivar tu propia cuenta.")
		}
		if _, err := tx.Exec(ctx, `UPDATE usuarios SET activo=$2 WHERE id=$1`, id, activo); err != nil {
			return traducir(err)
		}
		if !activo {
			if _, err := tx.Exec(ctx, `UPDATE sesiones SET revocada_at = now() WHERE usuario_id=$1 AND revocada_at IS NULL`, id); err != nil {
				return err
			}
		}
		u, err = s.obtener(ctx, tx, id)
		return err
	})
	return u, err
}

func traducir(err error) error {
	switch {
	case db.IsUniqueViolation(err, "usuarios_pin"):
		return &apperr.Error{
			Kind: apperr.Conflict, Code: "PIN_REPETIDO", Message: "Ese PIN ya lo usa otra persona del restaurante. Elige otro.",
			Fields: []apperr.FieldError{{Campo: "pin", Mensaje: "Ese PIN ya lo usa otra persona. Elige otro."}},
		}
	case db.IsUniqueViolation(err, "usuarios_email"):
		return &apperr.Error{
			Kind: apperr.Conflict, Code: "EMAIL_REGISTRADO", Message: "Ese correo ya se usa en otra cuenta.",
			Fields: []apperr.FieldError{{Campo: "email", Mensaje: "Ese correo ya se usa en otra cuenta."}},
		}
	}
	return db.Translate(err, "", "", "")
}

func fieldErr(campo, msg string) error {
	return &apperr.Error{Kind: apperr.Invalid, Code: "DATOS_INVALIDOS", Message: msg, Fields: []apperr.FieldError{{Campo: campo, Mensaje: msg}}}
}
