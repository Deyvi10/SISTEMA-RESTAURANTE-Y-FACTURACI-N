package db

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	cloudmigrations "github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/db/cloud"
)

// Migrate aplica las migraciones pendientes con el rol dueño de las tablas y devuelve
// cuántas se aplicaron.
func Migrate(ctx context.Context, adminURL string) (int, error) {
	cfg, err := pgx.ParseConfig(adminURL)
	if err != nil {
		return 0, fmt.Errorf("migraciones: url inválida: %w", err)
	}
	sqlDB := stdlib.OpenDB(*cfg)
	defer func() { _ = sqlDB.Close() }()

	migrations, err := fs.Sub(cloudmigrations.Migrations, "migrations")
	if err != nil {
		return 0, err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations)
	if err != nil {
		return 0, fmt.Errorf("migraciones: %w", err)
	}
	res, err := provider.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("migraciones: %w", err)
	}
	return len(res), nil
}
