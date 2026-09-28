// Package caja administra la configuración de caja que decide el dueño en la nube y usa el
// Nodo Local sin internet: cajas del local, métodos de pago con su forma de pago del SRI y
// motivos de descuento (F4-03, F4-06, F4-09). La operación (turnos, cobros) vive en el nodo.
package caja

import (
	"context"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

type Service struct {
	DB *db.DB
}

// ---------- Cajas ----------

type Caja struct {
	ID         ids.ID  `json:"id"`
	LocalID    ids.ID  `json:"localId"`
	Nombre     string  `json:"nombre"`
	EstacionID *ids.ID `json:"estacionId"`
	Activa     bool    `json:"activa"`
}

type CajaInput struct {
	ID         *ids.ID `json:"id"`
	LocalID    ids.ID  `json:"localId"`
	Nombre     string  `json:"nombre"`
	EstacionID *ids.ID `json:"estacionId"`
	Activa     *bool   `json:"activa"`
}

func (in *CajaInput) validar(ctx context.Context, tx db.Tx) error {
	in.Nombre = strings.TrimSpace(in.Nombre)
	var v apperr.Validation
	v.Check(len([]rune(in.Nombre)) >= 1 && len([]rune(in.Nombre)) <= 40, "nombre", "Ponle un nombre de 1 a 40 caracteres, como «Caja 1» o «Barra».")
	if in.EstacionID != nil {
		var tipo string
		err := tx.QueryRow(ctx, `SELECT tipo FROM estaciones WHERE id = $1 AND deleted_at IS NULL`, *in.EstacionID).Scan(&tipo)
		v.Check(err == nil && tipo == "CAJA", "estacionId", "Elige una estación de tipo Caja: ahí se imprimen los documentos y se abre el cajón.")
	}
	return v.Err()
}

const colsCaja = `id, local_id, nombre, estacion_id, activa`

func (s *Service) Cajas(ctx context.Context, p auth.Principal) ([]Caja, error) {
	out := []Caja{}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+colsCaja+` FROM cajas WHERE deleted_at IS NULL ORDER BY nombre`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Caja])
		return err
	})
	return out, err
}

func (s *Service) CrearCaja(ctx context.Context, p auth.Principal, in CajaInput) (Caja, error) {
	id := db.IDOrNew(in.ID)
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		if err := in.validar(ctx, tx); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO cajas (id, tenant_id, local_id, nombre, estacion_id, created_by) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING`,
			id, p.TenantID, in.LocalID, in.Nombre, in.EstacionID, p.UserID)
		return db.Translate(err, "cajas_nombre", "nombre", "Ya existe una caja con ese nombre.")
	})
	if err != nil {
		return Caja{}, err
	}
	return s.caja(ctx, p, id)
}

func (s *Service) ActualizarCaja(ctx context.Context, p auth.Principal, id ids.ID, in CajaInput) (Caja, error) {
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		if err := in.validar(ctx, tx); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE cajas SET nombre = $2, estacion_id = $3, activa = coalesce($4, activa) WHERE id = $1 AND deleted_at IS NULL`, id, in.Nombre, in.EstacionID, in.Activa)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return db.Translate(err, "cajas_nombre", "nombre", "Ya existe una caja con ese nombre.")
	})
	if err != nil {
		return Caja{}, err
	}
	return s.caja(ctx, p, id)
}

// EliminarCaja la oculta; sus turnos y cierres ya registrados en el nodo no se tocan.
// La última caja de un local no se elimina: sin caja no se puede cobrar.
func (s *Service) EliminarCaja(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var otras int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM cajas c WHERE c.deleted_at IS NULL AND c.id <> $1
			AND c.local_id = (SELECT local_id FROM cajas WHERE id = $1 AND deleted_at IS NULL)`, id).Scan(&otras); err != nil {
			return err
		}
		if otras == 0 {
			var existe bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM cajas WHERE id = $1 AND deleted_at IS NULL)`, id).Scan(&existe); err != nil || !existe {
				return apperr.ErrNotFound
			}
			return apperr.New(apperr.Conflict, "ULTIMA_CAJA", "Es la única caja del local: sin ella no se puede cobrar. Crea otra antes de eliminarla.")
		}
		_, err := tx.Exec(ctx, `UPDATE cajas SET deleted_at = now() WHERE id = $1`, id)
		return err
	})
}

func (s *Service) caja(ctx context.Context, p auth.Principal, id ids.ID) (Caja, error) {
	var c Caja
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		return db.NotFound(tx.QueryRow(ctx, `SELECT `+colsCaja+` FROM cajas WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&c.ID, &c.LocalID, &c.Nombre, &c.EstacionID, &c.Activa))
	})
	return c, err
}

// ---------- Métodos de pago ----------

type MetodoPago struct {
	ID             ids.ID `json:"id"`
	Nombre         string `json:"nombre"`
	Tipo           string `json:"tipo"`
	CodigoSRI      string `json:"codigoSri"`
	AbreCajon      bool   `json:"abreCajon"`
	PideReferencia bool   `json:"pideReferencia"`
	Icono          string `json:"icono"`
	Orden          int    `json:"orden"`
	Activo         bool   `json:"activo"`
}

type MetodoInput struct {
	ID             *ids.ID `json:"id"`
	Nombre         string  `json:"nombre"`
	Tipo           string  `json:"tipo"`
	AbreCajon      bool    `json:"abreCajon"`
	PideReferencia bool    `json:"pideReferencia"`
	Activo         *bool   `json:"activo"`
}

// CodigoSRIDe: la forma de pago del SRI se deduce del tipo (docs/05 §7). Las billeteras
// (DeUna, Payphone…) mueven dinero por el sistema financiero: «20».
var CodigoSRIDe = map[string]string{
	"EFECTIVO": "01", "TARJETA_DEBITO": "16", "TARJETA_CREDITO": "19", "TRANSFERENCIA": "20", "BILLETERA": "20", "OTRO": "20",
}

var iconoDe = map[string]string{
	"EFECTIVO": "efectivo", "TARJETA_DEBITO": "tarjeta", "TARJETA_CREDITO": "tarjeta", "TRANSFERENCIA": "transferencia", "BILLETERA": "billetera", "OTRO": "caja",
}

func (in *MetodoInput) validar() error {
	in.Nombre, in.Tipo = strings.TrimSpace(in.Nombre), strings.ToUpper(strings.TrimSpace(in.Tipo))
	var v apperr.Validation
	v.Check(len([]rune(in.Nombre)) >= 1 && len([]rune(in.Nombre)) <= 30, "nombre", "Ponle un nombre de 1 a 30 caracteres, como «Tarjeta» o «DeUna».")
	_, ok := CodigoSRIDe[in.Tipo]
	v.Check(ok, "tipo", "Elige el tipo: efectivo, tarjeta de crédito o débito, transferencia, billetera u otro.")
	return v.Err()
}

const colsMetodo = `id, nombre, tipo, codigo_forma_pago_sri, abre_cajon, pide_referencia, icono, orden, activo`

func (s *Service) Metodos(ctx context.Context, p auth.Principal) ([]MetodoPago, error) {
	out := []MetodoPago{}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+colsMetodo+` FROM metodos_pago WHERE deleted_at IS NULL ORDER BY orden, nombre`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[MetodoPago])
		return err
	})
	return out, err
}

func traducirMetodo(err error) error {
	if db.IsUniqueViolation(err, "metodos_pago_efectivo") {
		var v apperr.Validation
		v.Add("tipo", "Ya tienes un método de efectivo: solo puede haber uno (es el que cuadra el cajón).")
		return v.Err()
	}
	return db.Translate(err, "metodos_pago_nombre", "nombre", "Ya existe un método de pago con ese nombre.")
}

func (s *Service) CrearMetodo(ctx context.Context, p auth.Principal, in MetodoInput) (MetodoPago, error) {
	if err := in.validar(); err != nil {
		return MetodoPago{}, err
	}
	id := db.IDOrNew(in.ID)
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO metodos_pago (id, tenant_id, nombre, tipo, codigo_forma_pago_sri, abre_cajon, pide_referencia, icono, orden)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, (SELECT coalesce(max(orden), 0) + 1 FROM metodos_pago WHERE deleted_at IS NULL)) ON CONFLICT (id) DO NOTHING`,
			id, p.TenantID, in.Nombre, in.Tipo, CodigoSRIDe[in.Tipo], in.AbreCajon || in.Tipo == "EFECTIVO", in.PideReferencia, iconoDe[in.Tipo])
		return traducirMetodo(err)
	})
	if err != nil {
		return MetodoPago{}, err
	}
	return s.metodo(ctx, p, id)
}

func (s *Service) ActualizarMetodo(ctx context.Context, p auth.Principal, id ids.ID, in MetodoInput) (MetodoPago, error) {
	if err := in.validar(); err != nil {
		return MetodoPago{}, err
	}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var tipo string
		if err := tx.QueryRow(ctx, `SELECT tipo FROM metodos_pago WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&tipo); err != nil {
			return db.NotFound(err)
		}
		if tipo == "EFECTIVO" && (in.Tipo != "EFECTIVO" || (in.Activo != nil && !*in.Activo)) {
			return apperr.New(apperr.Conflict, "EFECTIVO_FIJO", "El efectivo siempre está disponible: es con lo que se cuadra el cajón. Puedes cambiarle el nombre.")
		}
		_, err := tx.Exec(ctx, `UPDATE metodos_pago SET nombre = $2, tipo = $3, codigo_forma_pago_sri = $4, abre_cajon = $5, pide_referencia = $6, icono = $7,
			activo = coalesce($8, activo) WHERE id = $1`,
			id, in.Nombre, in.Tipo, CodigoSRIDe[in.Tipo], in.AbreCajon || in.Tipo == "EFECTIVO", in.PideReferencia, iconoDe[in.Tipo], in.Activo)
		return traducirMetodo(err)
	})
	if err != nil {
		return MetodoPago{}, err
	}
	return s.metodo(ctx, p, id)
}

func (s *Service) EliminarMetodo(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var tipo string
		if err := tx.QueryRow(ctx, `SELECT tipo FROM metodos_pago WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&tipo); err != nil {
			return db.NotFound(err)
		}
		if tipo == "EFECTIVO" {
			return apperr.New(apperr.Conflict, "EFECTIVO_FIJO", "El efectivo no se elimina: es con lo que se cuadra el cajón.")
		}
		_, err := tx.Exec(ctx, `UPDATE metodos_pago SET deleted_at = now() WHERE id = $1`, id)
		return err
	})
}

func (s *Service) metodo(ctx context.Context, p auth.Principal, id ids.ID) (MetodoPago, error) {
	var m MetodoPago
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		return db.NotFound(tx.QueryRow(ctx, `SELECT `+colsMetodo+` FROM metodos_pago WHERE id = $1 AND deleted_at IS NULL`, id).
			Scan(&m.ID, &m.Nombre, &m.Tipo, &m.CodigoSRI, &m.AbreCajon, &m.PideReferencia, &m.Icono, &m.Orden, &m.Activo))
	})
	return m, err
}

// ---------- Motivos de descuento ----------

type Motivo struct {
	ID     ids.ID `json:"id"`
	Nombre string `json:"nombre"`
	Tipo   string `json:"tipo"`
	Activo bool   `json:"activo"`
}

type MotivoInput struct {
	ID     *ids.ID `json:"id"`
	Nombre string  `json:"nombre"`
	Tipo   string  `json:"tipo"`
}

func (in *MotivoInput) validar() error {
	in.Nombre, in.Tipo = strings.TrimSpace(in.Nombre), strings.ToUpper(strings.TrimSpace(in.Tipo))
	if in.Tipo == "" {
		in.Tipo = "DESCUENTO"
	}
	var v apperr.Validation
	v.Check(len([]rune(in.Nombre)) >= 1 && len([]rune(in.Nombre)) <= 40, "nombre", "Escribe el motivo en 1 a 40 caracteres, como «Cliente frecuente».")
	v.Check(slices.Contains([]string{"DESCUENTO", "CORTESIA"}, in.Tipo), "tipo", "El motivo es de descuento o de cortesía.")
	return v.Err()
}

func (s *Service) Motivos(ctx context.Context, p auth.Principal) ([]Motivo, error) {
	out := []Motivo{}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, nombre, tipo, activo FROM motivos_descuento WHERE deleted_at IS NULL ORDER BY tipo, orden, nombre`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Motivo])
		return err
	})
	return out, err
}

func (s *Service) CrearMotivo(ctx context.Context, p auth.Principal, in MotivoInput) (Motivo, error) {
	if err := in.validar(); err != nil {
		return Motivo{}, err
	}
	m := Motivo{ID: db.IDOrNew(in.ID), Nombre: in.Nombre, Tipo: in.Tipo, Activo: true}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO motivos_descuento (id, tenant_id, nombre, tipo, orden)
			VALUES ($1, $2, $3, $4, (SELECT coalesce(max(orden), 0) + 1 FROM motivos_descuento WHERE deleted_at IS NULL)) ON CONFLICT (id) DO NOTHING`,
			m.ID, p.TenantID, m.Nombre, m.Tipo)
		return db.Translate(err, "motivos_descuento_nombre", "nombre", "Ya existe ese motivo.")
	})
	return m, err
}

func (s *Service) EliminarMotivo(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE motivos_descuento SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return err
	})
}
