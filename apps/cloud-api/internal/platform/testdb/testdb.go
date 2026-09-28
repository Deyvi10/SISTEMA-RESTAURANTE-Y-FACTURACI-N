// Package testdb crea una base PostgreSQL desechable por prueba, con todas las migraciones,
// y un rol de conexión que hereda los permisos de restpos_app (sujeto a RLS, sin BYPASSRLS).
package testdb

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
)

const (
	loginRole = "restpos_test_login"
	loginPass = "restpos_test_login_pw" //nolint:gosec // G101: rol de pruebas efímero, solo en bases de test
)

// DB agrupa la base de la aplicación (rol con RLS) y la administrativa (dueño).
type DB struct {
	App      *db.DB
	Admin    *pgxpool.Pool
	AppURL   string
	AdminURL string
}

// New crea la base o salta la prueba si no hay TEST_DATABASE_URL (ver `make test`).
func New(t *testing.T) *DB {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("define TEST_DATABASE_URL o levanta `make dev` y usa `make test`")
	}
	ctx := context.Background()
	root, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("testdb: %v", err)
	}
	defer func() { _ = root.Close(ctx) }()

	name := fmt.Sprintf("restpos_t_%d", time.Now().UnixNano())
	if _, err := root.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("testdb: crear base: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), base)
		if err == nil {
			_, _ = c.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
			_ = c.Close(context.Background())
		}
	})

	adminURL := withDB(t, base, name, "", "")
	if _, err := db.Migrate(ctx, adminURL); err != nil {
		t.Fatal(err)
	}
	// Rol de conexión de prueba: hereda restpos_app. Es global al servidor, así que se crea una vez.
	if _, err := root.Exec(ctx, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '`+loginRole+`') THEN
		  CREATE ROLE `+loginRole+` LOGIN PASSWORD '`+loginPass+`' NOSUPERUSER NOBYPASSRLS INHERIT;
		END IF; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Exec(ctx, "GRANT restpos_app TO "+loginRole); err != nil {
		t.Fatal(err)
	}
	appURL := withDB(t, base, name, loginRole, loginPass)
	app, err := db.Open(ctx, appURL)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Close(); admin.Close() })
	return &DB{App: app, Admin: admin, AppURL: appURL, AdminURL: adminURL}
}

func withDB(t *testing.T, base, name, user, pass string) string {
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	if user != "" {
		u.User = url.UserPassword(user, pass)
	}
	return u.String()
}
