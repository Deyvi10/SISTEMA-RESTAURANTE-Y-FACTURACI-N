// Package impresoras administra desde la nube las impresoras del local, su asignación a
// estaciones y el ruteo de categorías y productos (F2-08, F2-10, RF-02-02, RF-02-03).
// El estado vivo de cada impresora lo informa el Nodo Local en su heartbeat.
package impresoras

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

type Service struct {
	DB    *db.DB
	Clock clock.Clock
}

type Impresora struct {
	ID         ids.ID   `json:"id"`
	LocalID    ids.ID   `json:"localId"`
	Nombre     string   `json:"nombre"`
	Conexion   string   `json:"conexion"`
	Host       *string  `json:"host"`
	Puerto     *int     `json:"puerto"`
	MAC        *string  `json:"mac"`
	Modelo     string   `json:"modelo"`
	AnchoPapel int      `json:"anchoPapel"`
	Origen     string   `json:"origen"`
	Activa     bool     `json:"activa"`
	Estaciones []ids.ID `json:"estaciones"`
	// Estado vivo según el último heartbeat del nodo (DESCONOCIDO si el nodo no la reporta).
	Estado    string     `json:"estado"`
	Cola      int        `json:"cola"`
	EstadoAt  *time.Time `json:"estadoAt"`
	NodoEnRed bool       `json:"nodoEnLinea"`
}

func (s *Service) Listar(ctx context.Context, p auth.Principal) ([]Impresora, error) {
	out := []Impresora{}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT i.id, i.local_id, i.nombre, i.conexion, i.host, i.puerto, i.mac, i.modelo, i.ancho_papel, i.origen, i.activa,
			coalesce((SELECT array_agg(ei.estacion_id ORDER BY ei.estacion_id) FROM estacion_impresoras ei WHERE ei.impresora_id = i.id), '{}')
			FROM impresoras i WHERE i.deleted_at IS NULL ORDER BY i.created_at`)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Impresora, error) {
			var x Impresora
			err := r.Scan(&x.ID, &x.LocalID, &x.Nombre, &x.Conexion, &x.Host, &x.Puerto, &x.MAC, &x.Modelo, &x.AnchoPapel, &x.Origen, &x.Activa, &x.Estaciones)
			x.Estado = "DESCONOCIDO"
			return x, err
		})
		if err != nil {
			return err
		}
		// Estado vivo desde el heartbeat del nodo activo de cada local.
		hbs, err := tx.Query(ctx, `SELECT local_id, ultimo_heartbeat_at, heartbeat->'impresoras' FROM nodos WHERE estado = 'ACTIVO' AND ultimo_heartbeat_at IS NOT NULL`)
		if err != nil {
			return err
		}
		type salud struct {
			ID     ids.ID `json:"id"`
			Estado string `json:"estado"`
			Cola   int    `json:"cola"`
		}
		estados := map[ids.ID]salud{}
		vistos := map[ids.ID]time.Time{}
		for hbs.Next() {
			var local ids.ID
			var at time.Time
			var raw []byte
			if err := hbs.Scan(&local, &at, &raw); err != nil {
				hbs.Close()
				return err
			}
			vistos[local] = at
			var lista []salud
			_ = json.Unmarshal(raw, &lista)
			for _, x := range lista {
				estados[x.ID] = x
			}
		}
		hbs.Close()
		now := s.Clock.Now()
		for i := range out {
			at, ok := vistos[out[i].LocalID]
			out[i].NodoEnRed = ok && now.Sub(at) < 3*time.Minute
			if x, ok := estados[out[i].ID]; ok && out[i].NodoEnRed {
				out[i].Estado, out[i].Cola, out[i].EstadoAt = x.Estado, x.Cola, &at
			}
		}
		return hbs.Err()
	})
	return out, err
}

// Input para agregar una impresora de red a mano (la detección automática llega del nodo).
type Input struct {
	ID         *ids.ID `json:"id"`
	LocalID    ids.ID  `json:"localId"`
	Nombre     string  `json:"nombre"`
	Host       string  `json:"host"`
	Puerto     int     `json:"puerto"`
	AnchoPapel int     `json:"anchoPapel"`
	Activa     *bool   `json:"activa"`
}

var hostnameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,62}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,62}[a-zA-Z0-9])?)*$`)

func (in *Input) validar(v *apperr.Validation) {
	in.Nombre = strings.TrimSpace(in.Nombre)
	in.Host = strings.TrimSpace(in.Host)
	v.Check(len([]rune(in.Nombre)) >= 1 && len([]rune(in.Nombre)) <= 40, "nombre", "Ponle un nombre de 1 a 40 caracteres, como «Cocina caliente».")
	ip := net.ParseIP(in.Host)
	v.Check(in.Host != "" && ((ip != nil && ip.To4() != nil) || (ip == nil && hostnameRe.MatchString(in.Host))), "host", "Escribe la IP de la impresora, como 192.168.1.50. La encuentras imprimiendo su hoja de configuración.")
	if in.Puerto == 0 {
		in.Puerto = 9100
	}
	v.Check(in.Puerto >= 1 && in.Puerto <= 65535, "puerto", "El puerto va de 1 a 65535 (casi siempre es 9100).")
	if in.AnchoPapel == 0 {
		in.AnchoPapel = 80
	}
	v.Check(in.AnchoPapel == 58 || in.AnchoPapel == 80, "anchoPapel", "El papel térmico es de 58 mm o de 80 mm.")
}

var (
	errDuplicadaRed = "Ya tienes una impresora con esa IP y puerto."
	errDuplicadaNom = "Ya existe una impresora con ese nombre."
)

func traducir(err error) error {
	switch {
	case db.IsUniqueViolation(err, "impresoras_red"):
		return &apperr.Error{Kind: apperr.Conflict, Code: "DUPLICADO", Message: errDuplicadaRed, Fields: []apperr.FieldError{{Campo: "host", Mensaje: errDuplicadaRed}}}
	case db.IsUniqueViolation(err, "impresoras_nombre"):
		return &apperr.Error{Kind: apperr.Conflict, Code: "DUPLICADO", Message: errDuplicadaNom, Fields: []apperr.FieldError{{Campo: "nombre", Mensaje: errDuplicadaNom}}}
	}
	return db.Translate(err, "", "", "")
}

func (s *Service) Crear(ctx context.Context, p auth.Principal, in Input) (Impresora, error) {
	var v apperr.Validation
	in.validar(&v)
	if err := v.Err(); err != nil {
		return Impresora{}, err
	}
	id := db.IDOrNew(in.ID)
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO impresoras (id, tenant_id, local_id, nombre, conexion, host, puerto, ancho_papel, origen, created_by)
			VALUES ($1, $2, $3, $4, 'TCP', $5, $6, $7, 'MANUAL', $8) ON CONFLICT (id) DO NOTHING`,
			id, p.TenantID, in.LocalID, in.Nombre, in.Host, in.Puerto, in.AnchoPapel, p.UserID)
		return traducir(err)
	})
	if err != nil {
		return Impresora{}, err
	}
	return s.una(ctx, p, id)
}

func (s *Service) Actualizar(ctx context.Context, p auth.Principal, id ids.ID, in Input) (Impresora, error) {
	var v apperr.Validation
	in.validar(&v)
	if err := v.Err(); err != nil {
		return Impresora{}, err
	}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE impresoras SET nombre = $2, ancho_papel = $3, activa = coalesce($4, activa),
			host = CASE WHEN conexion = 'TCP' THEN $5 ELSE host END, puerto = CASE WHEN conexion = 'TCP' THEN $6 ELSE puerto END
			WHERE id = $1 AND deleted_at IS NULL`, id, in.Nombre, in.AnchoPapel, in.Activa, in.Host, in.Puerto)
		if err != nil {
			return traducir(err)
		}
		if tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return Impresora{}, err
	}
	return s.una(ctx, p, id)
}

func (s *Service) Eliminar(ctx context.Context, p auth.Principal, id ids.ID) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM estacion_impresoras WHERE impresora_id = $1`, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE impresoras SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return err
	})
}

func (s *Service) una(ctx context.Context, p auth.Principal, id ids.ID) (Impresora, error) {
	todas, err := s.Listar(ctx, p)
	if err != nil {
		return Impresora{}, err
	}
	for _, x := range todas {
		if x.ID == id {
			return x, nil
		}
	}
	return Impresora{}, apperr.ErrNotFound
}

// AsignarAEstacion reemplaza las impresoras de una estación (N:M). Solo impresoras del
// mismo local que la estación.
func (s *Service) AsignarAEstacion(ctx context.Context, p auth.Principal, estacionID ids.ID, in struct {
	Impresoras []ids.ID `json:"impresoras"`
},
) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var local ids.ID
		err := tx.QueryRow(ctx, `SELECT local_id FROM estaciones WHERE id = $1 AND deleted_at IS NULL`, estacionID).Scan(&local)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		var validas int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM impresoras WHERE id = ANY($1) AND local_id = $2 AND deleted_at IS NULL`, in.Impresoras, local).Scan(&validas); err != nil {
			return err
		}
		if validas != len(uniq(in.Impresoras)) {
			return apperr.New(apperr.Invalid, "IMPRESORA_INVALIDA", "Alguna impresora ya no existe o es de otro local. Recarga la página.")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM estacion_impresoras WHERE estacion_id = $1 AND NOT (impresora_id = ANY($2))`, estacionID, in.Impresoras); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO estacion_impresoras (tenant_id, estacion_id, impresora_id, local_id)
			SELECT $1, $2, x, $3 FROM unnest($4::uuid[]) x ON CONFLICT DO NOTHING`, p.TenantID, estacionID, local, in.Impresoras)
		return err
	})
}

func uniq(xs []ids.ID) map[ids.ID]bool {
	m := map[ids.ID]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// Ruta indica a qué estación va una categoría o un producto. estacionId nulo = la estación
// por defecto del local (o la de su categoría, si es un producto). Nunca se pierde una
// comanda: el nodo siempre cae en la estación por defecto (RF-02-03.4).
type Ruta struct {
	EstacionID *ids.ID `json:"estacionId"`
}

func (s *Service) RutearCategoria(ctx context.Context, p auth.Principal, id ids.ID, in Ruta) error {
	return s.rutear(ctx, p, "categorias", id, in)
}

func (s *Service) RutearProducto(ctx context.Context, p auth.Principal, id ids.ID, in Ruta) error {
	return s.rutear(ctx, p, "productos", id, in)
}

func (s *Service) rutear(ctx context.Context, p auth.Principal, tabla string, id ids.ID, in Ruta) error {
	return s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		if in.EstacionID != nil {
			var tipo string
			err := tx.QueryRow(ctx, `SELECT tipo FROM estaciones WHERE id = $1 AND deleted_at IS NULL`, *in.EstacionID).Scan(&tipo)
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.New(apperr.Invalid, "ESTACION_INVALIDA", "Esa estación ya no existe. Recarga la página.")
			}
			if err != nil {
				return err
			}
			if tipo == "CAJA" {
				return apperr.New(apperr.Invalid, "ESTACION_CAJA", "La estación de caja imprime pre-cuentas y comprobantes, no comandas. Elige una de producción.")
			}
		}
		// Solo cambia si es distinto: no dispara una sincronización por nada.
		tag, err := tx.Exec(ctx, `UPDATE `+tabla+` SET estacion_id = $2 WHERE id = $1 AND deleted_at IS NULL AND estacion_id IS DISTINCT FROM $2`, id, in.EstacionID) //nolint:gosec // tabla fija
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var existe bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+tabla+` WHERE id = $1 AND deleted_at IS NULL)`, id).Scan(&existe); err != nil { //nolint:gosec // tabla fija
				return err
			}
			if !existe {
				return apperr.ErrNotFound
			}
		}
		return nil
	})
}

// Comando es una orden de la nube para el nodo y su resultado.
type Comando struct {
	ID          ids.ID     `json:"id"`
	Tipo        string     `json:"tipo"`
	CreatedAt   time.Time  `json:"createdAt"`
	EjecutadoAt *time.Time `json:"ejecutadoAt"`
	Resultado   *string    `json:"resultado"`
}

var errSinNodo = apperr.New(apperr.Conflict, "NODO_SIN_CONEXION", "El Nodo Local de este local no está en línea. Revisa que la PC de caja esté encendida y con internet.")

// ImprimirPrueba pide al nodo imprimir una hoja de prueba en la impresora.
func (s *Service) ImprimirPrueba(ctx context.Context, p auth.Principal, id ids.ID) (Comando, error) {
	c := Comando{ID: ids.New(), Tipo: "IMPRIMIR_PRUEBA", CreatedAt: s.Clock.Now()}
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		var local ids.ID
		err := tx.QueryRow(ctx, `SELECT local_id FROM impresoras WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&local)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		var enLinea bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM nodos WHERE local_id = $1 AND estado = 'ACTIVO' AND ultimo_heartbeat_at > $2)`,
			local, c.CreatedAt.Add(-3*time.Minute)).Scan(&enLinea); err != nil {
			return err
		}
		if !enLinea {
			return errSinNodo
		}
		datos, _ := json.Marshal(map[string]ids.ID{"impresoraId": id})
		_, err = tx.Exec(ctx, `INSERT INTO comandos_nodo (id, tenant_id, local_id, tipo, datos, creado_por, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			c.ID, p.TenantID, local, c.Tipo, datos, p.UserID, c.CreatedAt)
		return err
	})
	return c, err
}

func (s *Service) Comando(ctx context.Context, p auth.Principal, id ids.ID) (Comando, error) {
	var c Comando
	err := s.DB.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		return db.NotFound(tx.QueryRow(ctx, `SELECT id, tipo, created_at, ejecutado_at, resultado FROM comandos_nodo WHERE id = $1`, id).
			Scan(&c.ID, &c.Tipo, &c.CreatedAt, &c.EjecutadoAt, &c.Resultado))
	})
	return c, err
}
