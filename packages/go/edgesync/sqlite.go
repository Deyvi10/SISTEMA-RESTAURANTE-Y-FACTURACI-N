package edgesync

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // driver SQLite sin CGO (ADR-0002)
)

// OpenSQLite abre la base del nodo con los pragmas de docs/03 §4:
// WAL, synchronous=FULL (lo fiscal no puede perderse en un corte de luz),
// claves foráneas y espera ante bloqueo. Usa una sola conexión: el nodo tiene
// un único escritor serializado, lo que hace atómicos secuenciales y bloqueos.
func OpenSQLite(ctx context.Context, path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)&_txlock=immediate", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("edgesync: abrir sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("edgesync: abrir sqlite: %w", err)
	}
	return db, nil
}
