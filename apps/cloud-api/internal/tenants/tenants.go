// Package tenants da de alta restaurantes (RF-01-01) con datos semilla, para que el dueño
// no empiece con un sistema en blanco.
package tenants

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

// Planes con UUID fijo (sembrados por la migración 202609250001).
var Planes = map[string]ids.ID{
	"EMPRENDEDOR": uuid.MustParse("0192a000-0000-7000-8000-000000000001"),
	"RESTAURANTE": uuid.MustParse("0192a000-0000-7000-8000-000000000002"),
	"PRO":         uuid.MustParse("0192a000-0000-7000-8000-000000000003"),
}

type Alta struct {
	RUC             string
	RazonSocial     string
	NombreComercial string
	NombreDueno     string
	EmailDueno      string
	TelefonoDueno   string
	Plan            string
}

type Resultado struct {
	TenantID         ids.ID
	LocalID          ids.ID
	UsuarioID        ids.ID
	PasswordTemporal string // se envía por correo; se devuelve solo para el CLI del Super Admin
	EstacionCocina   ids.ID
	EstacionBar      ids.ID
	ZonaID           ids.ID
}

type Service struct {
	DB            *db.DB
	Mail          mail.Sender
	BackofficeURL string
}

func (a Alta) validar() error {
	var v apperr.Validation
	if r := sri.ValidarRUCEmisor(a.RUC); !r.Valida {
		v.Add("ruc", r.Motivo)
	}
	v.Check(len(strings.TrimSpace(a.RazonSocial)) > 0, "razonSocial", "Escribe la razón social tal como aparece en el RUC.")
	v.Check(len(strings.TrimSpace(a.NombreComercial)) > 0, "nombreComercial", "Escribe el nombre con el que te conocen tus clientes.")
	v.Check(len(strings.TrimSpace(a.NombreDueno)) > 0, "nombreDueno", "Escribe el nombre del dueño.")
	v.Check(strings.Count(a.EmailDueno, "@") == 1 && len(a.EmailDueno) >= 5, "emailDueno", "Escribe un correo válido: ahí llegará la contraseña temporal.")
	_, ok := Planes[a.Plan]
	v.Check(ok, "plan", "Elige un plan: EMPRENDEDOR, RESTAURANTE o PRO.")
	return v.Err()
}

// Crear da de alta el restaurante en una sola transacción: o queda todo o nada.
func (s *Service) Crear(ctx context.Context, a Alta) (Resultado, error) {
	a.EmailDueno = strings.ToLower(strings.TrimSpace(a.EmailDueno))
	if err := a.validar(); err != nil {
		return Resultado{}, err
	}
	r := Resultado{
		TenantID: ids.New(), LocalID: ids.New(), UsuarioID: ids.New(), PasswordTemporal: auth.PasswordTemporal(),
		EstacionCocina: ids.New(), EstacionBar: ids.New(), ZonaID: ids.New(),
	}
	hash, err := auth.HashPassword(r.PasswordTemporal)
	if err != nil {
		return Resultado{}, err
	}
	err = s.DB.InTenant(ctx, r.TenantID, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, ruc, razon_social, nombre_comercial, email_dueno, telefono_dueno, plan_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`, r.TenantID, a.RUC, strings.TrimSpace(a.RazonSocial), strings.TrimSpace(a.NombreComercial), a.EmailDueno, nilIfEmpty(a.TelefonoDueno), Planes[a.Plan]); err != nil {
			if db.IsUniqueViolation(err, "tenants_ruc_activo") {
				return apperr.New(apperr.Conflict, "RUC_REGISTRADO", "Ya existe un restaurante activo con ese RUC.")
			}
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO locales (id, tenant_id, nombre, codigo_establecimiento) VALUES ($1,$2,$3,'001')`,
			r.LocalID, r.TenantID, strings.TrimSpace(a.NombreComercial)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO usuarios (id, tenant_id, nombre_mostrar, rol, es_dueno, email, password_hash, debe_cambiar_password)
			VALUES ($1,$2,$3,'ADMIN',true,$4,$5,true)`, r.UsuarioID, r.TenantID, truncate(strings.TrimSpace(a.NombreDueno), 40), a.EmailDueno, hash); err != nil {
			if db.IsUniqueViolation(err, "usuarios_email") {
				return &apperr.Error{
					Kind: apperr.Conflict, Code: "EMAIL_REGISTRADO", Message: "Ese correo ya se usa en otra cuenta.",
					Fields: []apperr.FieldError{{Campo: "emailDueno", Mensaje: "Ese correo ya se usa en otra cuenta."}},
				}
			}
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO usuario_locales (tenant_id, usuario_id, local_id) VALUES ($1,$2,$3)`, r.TenantID, r.UsuarioID, r.LocalID); err != nil {
			return err
		}
		return sembrar(ctx, tx, r)
	})
	if err != nil {
		return Resultado{}, err
	}
	if err := s.enviarBienvenida(ctx, a, r); err != nil {
		return r, fmt.Errorf("restaurante creado, pero no se pudo enviar el correo: %w", err)
	}
	return r, nil
}

// sembrar crea lo mínimo para operar (RF-01-01.3): estaciones, salón y categorías con icono.
func sembrar(ctx context.Context, tx db.Tx, r Resultado) error {
	cajaID := ids.New()
	stmts := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO estaciones (id, tenant_id, local_id, nombre, tipo, icono, color, orden, es_defecto) VALUES ($1,$2,$3,'Cocina','PRODUCCION','cocina','red',1,true)`, []any{r.EstacionCocina, r.TenantID, r.LocalID}},
		{`INSERT INTO estaciones (id, tenant_id, local_id, nombre, tipo, icono, color, orden) VALUES ($1,$2,$3,'Bar','PRODUCCION','bar','purple',2)`, []any{r.EstacionBar, r.TenantID, r.LocalID}},
		{`INSERT INTO estaciones (id, tenant_id, local_id, nombre, tipo, icono, color, orden) VALUES ($1,$2,$3,'Caja','CAJA','factura','gray',3)`, []any{cajaID, r.TenantID, r.LocalID}},
		{`INSERT INTO zonas (id, tenant_id, local_id, nombre, orden) VALUES ($1,$2,$3,'Salón',1)`, []any{r.ZonaID, r.TenantID, r.LocalID}},
		{`INSERT INTO mesas (id, tenant_id, local_id, zona_id, nombre, capacidad) VALUES ($1,$2,$3,$4,'Mesa 1',4)`, []any{ids.New(), r.TenantID, r.LocalID, r.ZonaID}},
		{`INSERT INTO categorias (id, tenant_id, nombre, orden, estacion_id, color, icono) VALUES ($1,$2,'Entradas',1,$3,'green','salad')`, []any{ids.New(), r.TenantID, r.EstacionCocina}},
		{`INSERT INTO categorias (id, tenant_id, nombre, orden, estacion_id, color, icono) VALUES ($1,$2,'Platos fuertes',2,$3,'orange','utensils')`, []any{ids.New(), r.TenantID, r.EstacionCocina}},
		{`INSERT INTO categorias (id, tenant_id, nombre, orden, estacion_id, color, icono) VALUES ($1,$2,'Bebidas',3,$3,'blue','cup-soda')`, []any{ids.New(), r.TenantID, r.EstacionBar}},
	}
	for _, st := range stmts {
		if _, err := tx.Exec(ctx, st.sql, st.args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) enviarBienvenida(ctx context.Context, a Alta, r Resultado) error {
	text, html, err := mail.Render(mail.Contenido{
		Titulo: fmt.Sprintf("¡Bienvenido, %s!", a.NombreComercial),
		Parrafos: []string{
			"Tu restaurante ya está listo. Tu sistema sigue funcionando aunque se caiga el internet, y factura al SRI sin hacer esperar a tu cajero.",
			fmt.Sprintf("Entra con tu correo %s o con tu RUC y esta contraseña temporal. Te pediremos cambiarla la primera vez:", a.EmailDueno),
		},
		Destacado: r.PasswordTemporal,
		Boton:     "Configurar mi restaurante", BotonURL: s.BackofficeURL + "/login",
		Pie: "Te toma unos 10 minutos: menú, mesas y personal. Si necesitas ayuda, responde este correo.",
	})
	if err != nil {
		return err
	}
	return s.Mail.Send(ctx, mail.Message{To: a.EmailDueno, Subject: "Tu restaurante está listo", Text: text, HTML: html})
}

func nilIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
