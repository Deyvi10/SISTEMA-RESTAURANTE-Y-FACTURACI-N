// Package store es la base SQLite del Nodo Local (F2-01, docs/03 §4).
//
// Un solo escritor: la conexión de escritura es única, así que toda transacción de
// escritura se serializa y los secuenciales, bloqueos y reservas son atómicos sin locks
// distribuidos. Las lecturas van por un pool aparte en modo solo lectura y, gracias a WAL,
// nunca esperan al escritor.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"runtime"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // driver SQLite sin CGO (ADR-0002)

	edgemigrations "github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/db/edge"
)

// Store agrupa la conexión de escritura y el pool de lectura.
type Store struct {
	w *sql.DB
	r *sql.DB
}

// Tx es la transacción de escritura que reciben los servicios del nodo.
type Tx = sql.Tx

// Pragmas de docs/03 §4. synchronous=FULL: una venta confirmada sobrevive a un corte de luz.
func dsn(path string, readOnly bool) string {
	q := url.Values{}
	for _, p := range []string{"journal_mode(WAL)", "synchronous(FULL)", "foreign_keys(ON)", "busy_timeout(5000)"} {
		q.Add("_pragma", p)
	}
	if readOnly {
		q.Add("_pragma", "query_only(ON)")
	} else {
		// BEGIN IMMEDIATE: la transacción toma el candado de escritura al empezar, no a mitad.
		q.Set("_txlock", "immediate")
	}
	return "file:" + path + "?" + q.Encode()
}

// Open abre (o crea) la base y aplica las migraciones pendientes.
func Open(ctx context.Context, path string) (*Store, error) {
	w, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		return nil, fmt.Errorf("store: abrir: %w", err)
	}
	w.SetMaxOpenConns(1)
	w.SetMaxIdleConns(1)
	w.SetConnMaxLifetime(0)
	if err := w.PingContext(ctx); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("store: abrir %s: %w", path, err)
	}
	if err := migrate(ctx, w); err != nil {
		_ = w.Close()
		return nil, err
	}
	r, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		_ = w.Close()
		return nil, err
	}
	r.SetMaxOpenConns(max(4, runtime.NumCPU()))
	return &Store{w: w, r: r}, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	sub, err := fs.Sub(edgemigrations.Migrations, "migrations")
	if err != nil {
		return err
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, db, sub)
	if err != nil {
		return fmt.Errorf("store: migraciones: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("store: migraciones: %w", err)
	}
	return nil
}

// Write ejecuta fn en una transacción de escritura; commit si fn no falla.
func (s *Store) Write(ctx context.Context, fn func(*Tx) error) (err error) {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Read es el pool de lectura (solo lectura: una escritura por aquí falla).
func (s *Store) Read() *sql.DB { return s.r }

// Writer expone la conexión de escritura para componentes que gestionan su propia
// transacción (outbox de edgesync). Úsalo solo para eso.
func (s *Store) Writer() *sql.DB { return s.w }

// Check verifica la integridad física de la base (se usa al arrancar y en /estado).
func (s *Store) Check(ctx context.Context) error {
	var res string
	if err := s.r.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&res); err != nil {
		return err
	}
	if res != "ok" {
		return fmt.Errorf("store: base dañada: %s", res)
	}
	return nil
}

func (s *Store) Close() error {
	return errors.Join(s.r.Close(), s.w.Close())
}
