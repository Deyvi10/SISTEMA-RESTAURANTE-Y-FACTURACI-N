package catalogo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

type Modificador struct {
	ID              ids.ID `json:"id"`
	Nombre          string `json:"nombre"`
	PrecioAdicional string `json:"precioAdicional"`
	Activo          bool   `json:"activo"`
}

// Grupo de modificadores con reglas (RF-03-06.1): «Término de la carne» obligatorio 1 de 4,
// «Extras» opcional 0 a 3. Un grupo se reutiliza en varios productos.
type Grupo struct {
	ID            ids.ID        `json:"id"`
	Nombre        string        `json:"nombre"`
	Obligatorio   bool          `json:"obligatorio"`
	Min           int           `json:"min"`
	Max           int           `json:"max"`
	Modificadores []Modificador `json:"modificadores"`
	Productos     int           `json:"productos"`
}

type ModificadorInput struct {
	ID              *ids.ID `json:"id"`
	Nombre          string  `json:"nombre"`
	PrecioAdicional string  `json:"precioAdicional"`
	Activo          *bool   `json:"activo"`
}

type GrupoInput struct {
	ID            *ids.ID            `json:"id"`
	Nombre        string             `json:"nombre"`
	Obligatorio   bool               `json:"obligatorio"`
	Min           int                `json:"min"`
	Max           int                `json:"max"`
	Modificadores []ModificadorInput `json:"modificadores"`
}

func (in *GrupoInput) validar() error {
	in.Nombre = strings.TrimSpace(in.Nombre)
	if in.Obligatorio && in.Min == 0 {
		in.Min = 1
	}
	if in.Max == 0 {
		in.Max = max(1, in.Min)
	}
	var v apperr.Validation
	v.Check(len([]rune(in.Nombre)) >= 1 && len([]rune(in.Nombre)) <= 40, "nombre", "Ponle un nombre de 1 a 40 caracteres, como «Término de la carne».")
	v.Check(in.Min >= 0 && in.Max >= 1 && in.Max <= 20, "max", "Se puede elegir de 1 a 20 opciones.")
	v.Check(in.Max >= in.Min, "max", "El máximo no puede ser menor que el mínimo.")
	v.Check(!in.Obligatorio || in.Min >= 1, "min", "Si es obligatorio, hay que elegir al menos 1.")
	v.Check(len(in.Modificadores) >= 1 && len(in.Modificadores) <= 30, "modificadores", "Agrega de 1 a 30 opciones.")
	v.Check(in.Min <= len(in.Modificadores), "min", fmt.Sprintf("Pides elegir %d, pero solo hay %d opciones.", in.Min, len(in.Modificadores)))
	vistos := map[string]bool{}
	for i := range in.Modificadores {
		m := &in.Modificadores[i]
		m.Nombre = strings.TrimSpace(m.Nombre)
		campo := fmt.Sprintf("modificadores[%d]", i)
		v.Check(len([]rune(m.Nombre)) >= 1 && len([]rune(m.Nombre)) <= 40, campo+".nombre", "Cada opción necesita un nombre de 1 a 40 caracteres.")
		key := strings.ToLower(m.Nombre)
		v.Check(!vistos[key], campo+".nombre", "Hay dos opciones con el mismo nombre.")
		vistos[key] = true
		if m.PrecioAdicional == "" {
			m.PrecioAdicional = "0"
		}
		d, err := decimal.NewFromString(m.PrecioAdicional)
		v.Check(err == nil && !d.IsNegative() && d.LessThanOrEqual(maxPrecio), campo+".precioAdicional", "El precio adicional es un número desde 0, por ejemplo 1.00.")
	}
	return v.Err()
}

func (s *Service) Grupos(ctx context.Context, p auth.Principal) ([]Grupo, error) {
	var out []Grupo
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT g.id, g.nombre, g.obligatorio, g.min, g.max,
			(SELECT count(*) FROM producto_grupos_modificadores pg JOIN productos pr ON pr.id = pg.producto_id AND pr.deleted_at IS NULL WHERE pg.grupo_id = g.id)::int
			FROM grupos_modificadores g WHERE g.deleted_at IS NULL ORDER BY g.nombre`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Grupo, error) {
			var g Grupo
			err := r.Scan(&g.ID, &g.Nombre, &g.Obligatorio, &g.Min, &g.Max, &g.Productos)
			return g, err
		})
		if err != nil {
			return err
		}
		mrows, err := tx.Query(ctx, `SELECT grupo_id, id, nombre, precio_adicional::text, activo FROM modificadores WHERE activo ORDER BY orden, nombre`)
		if err != nil {
			return err
		}
		defer mrows.Close()
		idx := map[ids.ID]int{}
		for i := range out {
			idx[out[i].ID] = i
			out[i].Modificadores = []Modificador{}
		}
		for mrows.Next() {
			var gid ids.ID
			var m Modificador
			if err := mrows.Scan(&gid, &m.ID, &m.Nombre, &m.PrecioAdicional, &m.Activo); err != nil {
				return err
			}
			m.PrecioAdicional = decimal.RequireFromString(m.PrecioAdicional).String()
			if i, ok := idx[gid]; ok {
				out[i].Modificadores = append(out[i].Modificadores, m)
			}
		}
		return mrows.Err()
	})
	return out, err
}

func (s *Service) CrearGrupo(ctx context.Context, p auth.Principal, in GrupoInput) (Grupo, error) {
	if err := in.validar(); err != nil {
		return Grupo{}, err
	}
	id := db.IDOrNew(in.ID)
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO grupos_modificadores (id, tenant_id, nombre, obligatorio, min, max, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (id) DO NOTHING`,
			id, p.TenantID, in.Nombre, in.Obligatorio, in.Min, in.Max, p.UserID)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		return guardarModificadores(ctx, tx, p.TenantID, id, in.Modificadores)
	})
	if err != nil {
		return Grupo{}, err
	}
	return s.grupo(ctx, p, id)
}

func (s *Service) ActualizarGrupo(ctx context.Context, p auth.Principal, id ids.ID, in GrupoInput) (Grupo, error) {
	if err := in.validar(); err != nil {
		return Grupo{}, err
	}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE grupos_modificadores SET nombre=$2, obligatorio=$3, min=$4, max=$5 WHERE id=$1 AND deleted_at IS NULL`,
			id, in.Nombre, in.Obligatorio, in.Min, in.Max)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return guardarModificadores(ctx, tx, p.TenantID, id, in.Modificadores)
	})
	if err != nil {
		return Grupo{}, err
	}
	return s.grupo(ctx, p, id)
}

// guardarModificadores sincroniza las opciones: actualiza las que traen id, crea las nuevas
// y desactiva (no borra) las que ya no vienen, porque órdenes y recetas pueden apuntarlas.
func guardarModificadores(ctx context.Context, tx db.Tx, tenant, grupo ids.ID, mods []ModificadorInput) error {
	vigentes := make([]ids.ID, 0, len(mods))
	for i, m := range mods {
		id := db.IDOrNew(m.ID)
		vigentes = append(vigentes, id)
		activo := true
		if m.Activo != nil {
			activo = *m.Activo
		}
		if _, err := tx.Exec(ctx, `INSERT INTO modificadores (id, tenant_id, grupo_id, nombre, precio_adicional, orden, activo) VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (id) DO UPDATE SET nombre = EXCLUDED.nombre, precio_adicional = EXCLUDED.precio_adicional, orden = EXCLUDED.orden, activo = EXCLUDED.activo
			WHERE modificadores.grupo_id = EXCLUDED.grupo_id`, id, tenant, grupo, m.Nombre, m.PrecioAdicional, i, activo); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `UPDATE modificadores SET activo = false WHERE grupo_id = $1 AND NOT (id = ANY($2))`, grupo, vigentes)
	return err
}

func (s *Service) EliminarGrupo(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM producto_grupos_modificadores WHERE grupo_id=$1`, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE grupos_modificadores SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return err
	})
}

func (s *Service) grupo(ctx context.Context, p auth.Principal, id ids.ID) (Grupo, error) {
	gs, err := s.Grupos(ctx, p)
	for _, g := range gs {
		if g.ID == id {
			return g, err
		}
	}
	if err == nil {
		err = apperr.ErrNotFound
	}
	return Grupo{}, err
}
