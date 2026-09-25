// Package db envuelve PostgreSQL con la regla de oro del multi-tenant (ADR-0008):
// toda consulta de negocio corre en una transacción con app.tenant_id fijado con SET LOCAL,
// así RLS filtra las filas aunque el código olvide un WHERE.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// DB es el pool del rol de la aplicación.
type DB struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("db: url inválida: %w", err)
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = "cloud-api"
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: sin conexión: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() { d.Pool.Close() }

// Tx es la transacción que reciben los servicios. Solo existe dentro de InTenant o Global.
type Tx = pgx.Tx

// InTenant ejecuta fn en una transacción con el tenant fijado. Commit si fn no falla.
func (d *DB) InTenant(ctx context.Context, tenant ids.ID, fn func(Tx) error) error {
	if tenant == ids.Nil {
		return errors.New("db: InTenant sin tenant")
	}
	return pgx.BeginFunc(ctx, d.Pool, func(tx pgx.Tx) error {
		// set_config(..., true) equivale a SET LOCAL: muere con la transacción,
		// así un pooler en modo transacción nunca filtra el tenant a otra petición.
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant.String()); err != nil {
			return err
		}
		return fn(tx)
	})
}

// InTenantSnapshot es InTenant de solo lectura con REPEATABLE READ: todas las consultas
// ven la misma foto de la base (volcados completos para el Nodo Local).
func (d *DB) InTenantSnapshot(ctx context.Context, tenant ids.ID, fn func(Tx) error) error {
	if tenant == ids.Nil {
		return errors.New("db: InTenantSnapshot sin tenant")
	}
	return pgx.BeginTxFunc(ctx, d.Pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant.String()); err != nil {
			return err
		}
		return fn(tx)
	})
}

// Global ejecuta fn sin tenant: RLS no devuelve filas de negocio. Solo para tablas globales
// (planes, tarifas_iva, intentos_login) y funciones SECURITY DEFINER de autenticación.
func (d *DB) Global(ctx context.Context, fn func(Tx) error) error {
	return pgx.BeginFunc(ctx, d.Pool, fn)
}

// Errores de PostgreSQL que el dominio traduce a mensajes claros.
func IsUniqueViolation(err error, constraint string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" && (constraint == "" || pg.ConstraintName == constraint)
}

func IsForeignKeyViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23503"
}

func IsCheckViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23514"
}

// NotFound indica que la fila no existe o pertenece a otro tenant (RLS la oculta).
var ErrNotFound = pgx.ErrNoRows
