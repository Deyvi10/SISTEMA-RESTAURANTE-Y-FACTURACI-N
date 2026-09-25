// Package salon configura el local, sus zonas, mesas y estaciones de producción
// (RF-01-03, RF-03-01 cuadrícula, RF-02-03 estaciones).
package salon

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

type Service struct{ DB *db.DB }

// ---------- Local ----------

type Local struct {
	ID                    ids.ID `json:"id"`
	Nombre                string `json:"nombre"`
	Direccion             string `json:"direccion"`
	CodigoEstablecimiento string `json:"codigoEstablecimiento"`
	ZonaHoraria           string `json:"zonaHoraria"`
	PropinaLegalActiva    bool   `json:"propinaLegalActiva"`
	PropinaPorcentaje     string `json:"propinaPorcentaje"`
	PreciosIncluyenIVA    bool   `json:"preciosIncluyenIva"`
}

func (s *Service) Locales(ctx context.Context, p auth.Principal) ([]Local, error) {
	var out []Local
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, nombre, direccion, codigo_establecimiento, zona_horaria, propina_legal_activa, propina_porcentaje::text, precios_incluyen_iva
			FROM locales WHERE deleted_at IS NULL ORDER BY codigo_establecimiento`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Local, error) {
			var l Local
			err := r.Scan(&l.ID, &l.Nombre, &l.Direccion, &l.CodigoEstablecimiento, &l.ZonaHoraria, &l.PropinaLegalActiva, &l.PropinaPorcentaje, &l.PreciosIncluyenIVA)
			return l, err
		})
		return err
	})
	return out, err
}

type LocalCambios struct {
	Nombre             *string `json:"nombre"`
	Direccion          *string `json:"direccion"`
	PropinaLegalActiva *bool   `json:"propinaLegalActiva"`
	PropinaPorcentaje  *string `json:"propinaPorcentaje"`
	PreciosIncluyenIVA *bool   `json:"preciosIncluyenIva"`
}

func (s *Service) ActualizarLocal(ctx context.Context, p auth.Principal, id ids.ID, c LocalCambios) (Local, error) {
	var v apperr.Validation
	if c.Nombre != nil {
		*c.Nombre = strings.TrimSpace(*c.Nombre)
		v.Check(len(*c.Nombre) >= 1 && len(*c.Nombre) <= 120, "nombre", "El nombre debe tener de 1 a 120 caracteres.")
	}
	if c.PropinaPorcentaje != nil {
		d, err := decimal.NewFromString(*c.PropinaPorcentaje)
		v.Check(err == nil && !d.IsNegative() && d.LessThanOrEqual(decimal.NewFromInt(100)), "propinaPorcentaje", "Escribe un porcentaje entre 0 y 100.")
	}
	if err := v.Err(); err != nil {
		return Local{}, err
	}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE locales SET
			nombre = coalesce($2, nombre), direccion = coalesce($3, direccion),
			propina_legal_activa = coalesce($4, propina_legal_activa), propina_porcentaje = coalesce($5::numeric, propina_porcentaje),
			precios_incluyen_iva = coalesce($6, precios_incluyen_iva)
			WHERE id = $1 AND deleted_at IS NULL`, id, c.Nombre, c.Direccion, c.PropinaLegalActiva, c.PropinaPorcentaje, c.PreciosIncluyenIVA)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return err
	})
	if err != nil {
		return Local{}, err
	}
	ls, err := s.Locales(ctx, p)
	for _, l := range ls {
		if l.ID == id {
			return l, err
		}
	}
	return Local{}, apperr.ErrNotFound
}

// ---------- Estaciones ----------

type Estacion struct {
	ID        ids.ID `json:"id"`
	LocalID   ids.ID `json:"localId"`
	Nombre    string `json:"nombre"`
	Tipo      string `json:"tipo"`
	Icono     string `json:"icono"`
	Color     string `json:"color"`
	Orden     int    `json:"orden"`
	EsDefecto bool   `json:"esDefecto"`
}

// Iconos y colores permitidos: los del sistema de diseño (packages/design).
var (
	IconosEstacion = []string{"cocina", "bar", "cocinaFria", "kds", "factura", "impresora", "caja"}
	Colores        = []string{"blue", "green", "orange", "red", "yellow", "indigo", "purple", "teal", "pink", "gray"}
)

type EstacionInput struct {
	ID      *ids.ID `json:"id"`
	LocalID ids.ID  `json:"localId"`
	Nombre  string  `json:"nombre"`
	Tipo    string  `json:"tipo"`
	Icono   string  `json:"icono"`
	Color   string  `json:"color"`
}

func (in *EstacionInput) validar() error {
	in.Nombre = strings.TrimSpace(in.Nombre)
	if in.Tipo == "" {
		in.Tipo = "PRODUCCION"
	}
	if in.Icono == "" {
		in.Icono = "cocina"
	}
	if in.Color == "" {
		in.Color = "orange"
	}
	var v apperr.Validation
	v.Check(len([]rune(in.Nombre)) >= 1 && len([]rune(in.Nombre)) <= 40, "nombre", "Ponle un nombre de 1 a 40 caracteres, como «Cocina caliente» o «Bar».")
	v.Check(in.Tipo == "PRODUCCION" || in.Tipo == "CAJA", "tipo", "El tipo debe ser PRODUCCION o CAJA.")
	v.Check(slices.Contains(IconosEstacion, in.Icono), "icono", "Elige uno de los iconos disponibles.")
	v.Check(slices.Contains(Colores, in.Color), "color", "Elige uno de los colores disponibles.")
	return v.Err()
}

func (s *Service) Estaciones(ctx context.Context, p auth.Principal) ([]Estacion, error) {
	var out []Estacion
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, local_id, nombre, tipo, icono, color, orden, es_defecto FROM estaciones WHERE deleted_at IS NULL ORDER BY orden, nombre`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Estacion])
		return err
	})
	return out, err
}

func (s *Service) CrearEstacion(ctx context.Context, p auth.Principal, in EstacionInput) (Estacion, error) {
	if err := in.validar(); err != nil {
		return Estacion{}, err
	}
	id := db.IDOrNew(in.ID)
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO estaciones (id, tenant_id, local_id, nombre, tipo, icono, color, orden, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, (SELECT coalesce(max(orden),0)+1 FROM estaciones WHERE local_id = $3), $8)
			ON CONFLICT (id) DO NOTHING`, id, p.TenantID, in.LocalID, in.Nombre, in.Tipo, in.Icono, in.Color, p.UserID)
		return db.Translate(err, "estaciones_nombre", "nombre", "Ya existe una estación con ese nombre.")
	})
	if err != nil {
		return Estacion{}, err
	}
	return s.estacion(ctx, p, id)
}

func (s *Service) ActualizarEstacion(ctx context.Context, p auth.Principal, id ids.ID, in EstacionInput) (Estacion, error) {
	if err := in.validar(); err != nil {
		return Estacion{}, err
	}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE estaciones SET nombre=$2, tipo=$3, icono=$4, color=$5 WHERE id=$1 AND deleted_at IS NULL`, id, in.Nombre, in.Tipo, in.Icono, in.Color)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return db.Translate(err, "estaciones_nombre", "nombre", "Ya existe una estación con ese nombre.")
	})
	if err != nil {
		return Estacion{}, err
	}
	return s.estacion(ctx, p, id)
}

// EliminarEstacion la oculta. Las categorías y productos que la usaban pasan a la estación
// por defecto (nunca se pierde una comanda, RF-02-03.4). La de por defecto no se elimina.
func (s *Service) EliminarEstacion(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var defecto bool
		if err := tx.QueryRow(ctx, `SELECT es_defecto FROM estaciones WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&defecto); err != nil {
			return db.NotFound(err)
		}
		if defecto {
			return apperr.New(apperr.Conflict, "ESTACION_DEFECTO", "Esta es la estación por defecto: recibe lo que no tiene otra estación. Elige otra como defecto antes de eliminarla.")
		}
		if _, err := tx.Exec(ctx, `UPDATE categorias SET estacion_id = NULL WHERE estacion_id = $1`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE productos SET estacion_id = NULL WHERE estacion_id = $1`, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE estaciones SET deleted_at = now() WHERE id = $1`, id)
		return err
	})
}

func (s *Service) estacion(ctx context.Context, p auth.Principal, id ids.ID) (Estacion, error) {
	var e Estacion
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		return db.NotFound(tx.QueryRow(ctx, `SELECT id, local_id, nombre, tipo, icono, color, orden, es_defecto FROM estaciones WHERE id=$1 AND deleted_at IS NULL`, id).
			Scan(&e.ID, &e.LocalID, &e.Nombre, &e.Tipo, &e.Icono, &e.Color, &e.Orden, &e.EsDefecto))
	})
	return e, err
}

// ---------- Zonas ----------

type Zona struct {
	ID      ids.ID `json:"id"`
	LocalID ids.ID `json:"localId"`
	Nombre  string `json:"nombre"`
	Orden   int    `json:"orden"`
	Mesas   int    `json:"mesas"`
}

type ZonaInput struct {
	ID      *ids.ID `json:"id"`
	LocalID ids.ID  `json:"localId"`
	Nombre  string  `json:"nombre"`
}

func (s *Service) Zonas(ctx context.Context, p auth.Principal) ([]Zona, error) {
	var out []Zona
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT z.id, z.local_id, z.nombre, z.orden,
			(SELECT count(*) FROM mesas m WHERE m.zona_id = z.id AND m.deleted_at IS NULL)::int
			FROM zonas z WHERE z.deleted_at IS NULL ORDER BY z.orden, z.nombre`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Zona])
		return err
	})
	return out, err
}

func validarNombre(v *apperr.Validation, n string, max int, ejemplo string) string {
	n = strings.TrimSpace(n)
	v.Check(len([]rune(n)) >= 1 && len([]rune(n)) <= max, "nombre", fmt.Sprintf("Ponle un nombre de 1 a %d caracteres, como «%s».", max, ejemplo))
	return n
}

func (s *Service) CrearZona(ctx context.Context, p auth.Principal, in ZonaInput) (Zona, error) {
	var v apperr.Validation
	in.Nombre = validarNombre(&v, in.Nombre, 40, "Terraza")
	if err := v.Err(); err != nil {
		return Zona{}, err
	}
	id := db.IDOrNew(in.ID)
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO zonas (id, tenant_id, local_id, nombre, orden, created_by)
			VALUES ($1,$2,$3,$4,(SELECT coalesce(max(orden),0)+1 FROM zonas WHERE local_id=$3),$5) ON CONFLICT (id) DO NOTHING`,
			id, p.TenantID, in.LocalID, in.Nombre, p.UserID)
		return db.Translate(err, "zonas_nombre", "nombre", "Ya existe una zona con ese nombre.")
	})
	if err != nil {
		return Zona{}, err
	}
	return s.zona(ctx, p, id)
}

func (s *Service) RenombrarZona(ctx context.Context, p auth.Principal, id ids.ID, nombre string) (Zona, error) {
	var v apperr.Validation
	nombre = validarNombre(&v, nombre, 40, "Terraza")
	if err := v.Err(); err != nil {
		return Zona{}, err
	}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE zonas SET nombre=$2 WHERE id=$1 AND deleted_at IS NULL`, id, nombre)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return db.Translate(err, "zonas_nombre", "nombre", "Ya existe una zona con ese nombre.")
	})
	if err != nil {
		return Zona{}, err
	}
	return s.zona(ctx, p, id)
}

func (s *Service) EliminarZona(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var mesas int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM mesas WHERE zona_id=$1 AND deleted_at IS NULL`, id).Scan(&mesas); err != nil {
			return err
		}
		if mesas > 0 {
			return apperr.New(apperr.Conflict, "ZONA_CON_MESAS", fmt.Sprintf("Esta zona tiene %d mesas. Muévelas o elimínalas antes de borrar la zona.", mesas))
		}
		tag, err := tx.Exec(ctx, `UPDATE zonas SET deleted_at = now() WHERE id=$1 AND deleted_at IS NULL`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return err
	})
}

func (s *Service) zona(ctx context.Context, p auth.Principal, id ids.ID) (Zona, error) {
	zs, err := s.Zonas(ctx, p)
	for _, z := range zs {
		if z.ID == id {
			return z, err
		}
	}
	if err == nil {
		err = apperr.ErrNotFound
	}
	return Zona{}, err
}

// ---------- Mesas ----------

type Mesa struct {
	ID        ids.ID `json:"id"`
	LocalID   ids.ID `json:"localId"`
	ZonaID    ids.ID `json:"zonaId"`
	Nombre    string `json:"nombre"`
	Capacidad int    `json:"capacidad"`
	Forma     string `json:"forma"`
	PosX      int    `json:"posX"`
	PosY      int    `json:"posY"`
	Activa    bool   `json:"activa"`
}

type MesaInput struct {
	ID        *ids.ID `json:"id"`
	ZonaID    ids.ID  `json:"zonaId"`
	Nombre    string  `json:"nombre"`
	Capacidad int     `json:"capacidad"`
	Forma     string  `json:"forma"`
	PosX      int     `json:"posX"`
	PosY      int     `json:"posY"`
	Activa    *bool   `json:"activa"`
}

func (in *MesaInput) validar() error {
	var v apperr.Validation
	in.Nombre = validarNombre(&v, in.Nombre, 20, "Mesa 4")
	if in.Capacidad == 0 {
		in.Capacidad = 4
	}
	if in.Forma == "" {
		in.Forma = "CUADRADA"
	}
	v.Check(in.Capacidad >= 1 && in.Capacidad <= 50, "capacidad", "La capacidad va de 1 a 50 personas.")
	v.Check(slices.Contains([]string{"CUADRADA", "REDONDA", "RECTANGULAR"}, in.Forma), "forma", "La forma debe ser CUADRADA, REDONDA o RECTANGULAR.")
	v.Check(in.PosX >= 0 && in.PosX <= 99 && in.PosY >= 0 && in.PosY <= 99, "posX", "La posición en la cuadrícula va de 0 a 99.")
	return v.Err()
}

const mesaCols = `id, local_id, zona_id, nombre, capacidad, forma, pos_x, pos_y, activa`

func (s *Service) Mesas(ctx context.Context, p auth.Principal) ([]Mesa, error) {
	var out []Mesa
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+mesaCols+` FROM mesas WHERE deleted_at IS NULL ORDER BY pos_y, pos_x, nombre`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Mesa])
		return err
	})
	return out, err
}

func (s *Service) CrearMesa(ctx context.Context, p auth.Principal, in MesaInput) (Mesa, error) {
	if err := in.validar(); err != nil {
		return Mesa{}, err
	}
	id := db.IDOrNew(in.ID)
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO mesas (id, tenant_id, local_id, zona_id, nombre, capacidad, forma, pos_x, pos_y, created_by)
			SELECT $1, $2, z.local_id, z.id, $4, $5, $6, $7, $8, $9 FROM zonas z WHERE z.id = $3 AND z.deleted_at IS NULL
			ON CONFLICT (id) DO NOTHING`, id, p.TenantID, in.ZonaID, in.Nombre, in.Capacidad, in.Forma, in.PosX, in.PosY, p.UserID)
		return db.Translate(err, "mesas_nombre", "nombre", "Ya existe una mesa con ese nombre.")
	})
	if err != nil {
		return Mesa{}, err
	}
	return s.mesa(ctx, p, id)
}

// CrearLote crea «Mesa N» … en fila, acomodadas en una cuadrícula de 6 columnas, saltando
// los nombres que ya existen. Es el camino rápido del asistente de configuración.
func (s *Service) CrearLote(ctx context.Context, p auth.Principal, zonaID ids.ID, cantidad, capacidad int) ([]Mesa, error) {
	var v apperr.Validation
	v.Check(cantidad >= 1 && cantidad <= 60, "cantidad", "Crea de 1 a 60 mesas a la vez.")
	if capacidad == 0 {
		capacidad = 4
	}
	v.Check(capacidad >= 1 && capacidad <= 50, "capacidad", "La capacidad va de 1 a 50 personas.")
	if err := v.Err(); err != nil {
		return nil, err
	}
	var creadas []ids.ID
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var localID ids.ID
		if err := tx.QueryRow(ctx, `SELECT local_id FROM zonas WHERE id=$1 AND deleted_at IS NULL`, zonaID).Scan(&localID); err != nil {
			return db.NotFound(err)
		}
		rows, err := tx.Query(ctx, `SELECT lower(nombre) FROM mesas WHERE local_id=$1 AND deleted_at IS NULL`, localID)
		if err != nil {
			return err
		}
		existentes, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		var enZona int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM mesas WHERE zona_id=$1 AND deleted_at IS NULL`, zonaID).Scan(&enZona); err != nil {
			return err
		}
		n := 1
		for len(creadas) < cantidad {
			nombre := fmt.Sprintf("Mesa %d", n)
			n++
			if slices.Contains(existentes, strings.ToLower(nombre)) {
				continue
			}
			pos := enZona + len(creadas)
			id := ids.New()
			if _, err := tx.Exec(ctx, `INSERT INTO mesas (id, tenant_id, local_id, zona_id, nombre, capacidad, pos_x, pos_y, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
				id, p.TenantID, localID, zonaID, nombre, capacidad, pos%6, min(pos/6, 99), p.UserID); err != nil {
				return err
			}
			creadas = append(creadas, id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	todas, err := s.Mesas(ctx, p)
	out := make([]Mesa, 0, len(creadas))
	for _, m := range todas {
		if slices.Contains(creadas, m.ID) {
			out = append(out, m)
		}
	}
	return out, err
}

func (s *Service) ActualizarMesa(ctx context.Context, p auth.Principal, id ids.ID, in MesaInput) (Mesa, error) {
	if err := in.validar(); err != nil {
		return Mesa{}, err
	}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE mesas m SET nombre=$3, capacidad=$4, forma=$5, pos_x=$6, pos_y=$7, activa=coalesce($8, m.activa),
			zona_id = z.id, local_id = z.local_id
			FROM zonas z WHERE m.id=$1 AND m.deleted_at IS NULL AND z.id=$2 AND z.deleted_at IS NULL`,
			id, in.ZonaID, in.Nombre, in.Capacidad, in.Forma, in.PosX, in.PosY, in.Activa)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return db.Translate(err, "mesas_nombre", "nombre", "Ya existe una mesa con ese nombre.")
	})
	if err != nil {
		return Mesa{}, err
	}
	return s.mesa(ctx, p, id)
}

func (s *Service) EliminarMesa(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE mesas SET deleted_at = now() WHERE id=$1 AND deleted_at IS NULL`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return err
	})
}

func (s *Service) mesa(ctx context.Context, p auth.Principal, id ids.ID) (Mesa, error) {
	var m Mesa
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+mesaCols+` FROM mesas WHERE id=$1 AND deleted_at IS NULL`, id)
		if err != nil {
			return err
		}
		m, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[Mesa])
		return db.NotFound(err)
	})
	return m, err
}
