package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func abrir(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestPragmasYMigraciones(t *testing.T) {
	inicio := time.Now()
	s := abrir(t, filepath.Join(t.TempDir(), "nodo.db"))
	if d := time.Since(inicio); d > 10*time.Second {
		t.Fatalf("arranque en %v; RNF: ≤ 10 s", d)
	}
	ctx := context.Background()
	var modo string
	var sync, fk int
	if err := s.Read().QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&modo); err != nil || modo != "wal" {
		t.Fatalf("journal_mode = %q, %v", modo, err)
	}
	// El pragma es por conexión: se revisa en la de escritura, que es la que importa.
	if err := s.Write(ctx, func(tx *Tx) error {
		if err := tx.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&sync); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk)
	}); err != nil {
		t.Fatal(err)
	}
	if sync != 2 || fk != 1 {
		t.Fatalf("synchronous=%d (quiero 2=FULL) foreign_keys=%d", sync, fk)
	}
	for _, tabla := range []string{"nodo", "inbox_cursores", "outbox", "auditoria"} {
		var n int
		if err := s.Read().QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tabla).Scan(&n); err != nil || n != 1 {
			t.Errorf("falta la tabla %s", tabla)
		}
	}
	if err := s.Check(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestReabrirNoRepiteMigraciones(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodo.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	abrir(t, path)
}

func TestPoolDeLecturaEsSoloLectura(t *testing.T) {
	s := abrir(t, filepath.Join(t.TempDir(), "nodo.db"))
	if _, err := s.Read().Exec("INSERT INTO inbox_cursores (flujo, updated_at) VALUES ('x', 'y')"); err == nil {
		t.Fatal("el pool de lectura permitió escribir")
	}
}

func TestAuditoriaAppendOnly(t *testing.T) {
	s := abrir(t, filepath.Join(t.TempDir(), "nodo.db"))
	ctx := context.Background()
	if err := s.Write(ctx, func(tx *Tx) error {
		_, err := tx.Exec(`INSERT INTO auditoria (id, accion, entidad, created_at) VALUES ('a', 'PRUEBA', 'nodo', '2026-09-25T00:00:00Z')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"UPDATE auditoria SET accion = 'X'", "DELETE FROM auditoria"} {
		if err := s.Write(ctx, func(tx *Tx) error { _, err := tx.Exec(q); return err }); err == nil {
			t.Errorf("%q no debió permitirse", q)
		}
	}
}

// Un solo escritor: 40 goroutines hacen leer-modificar-escribir sobre el mismo contador.
// Si dos transacciones se intercalaran se perderían incrementos.
func TestEscritorSerializado(t *testing.T) {
	s := abrir(t, filepath.Join(t.TempDir(), "nodo.db"))
	ctx := context.Background()
	if err := s.Write(ctx, func(tx *Tx) error {
		_, err := tx.Exec("INSERT INTO inbox_cursores (flujo, cursor, updated_at) VALUES ('c', 0, '')")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 40 {
		wg.Go(func() {
			for range 25 {
				if err := s.Write(ctx, func(tx *Tx) error {
					var n int64
					if err := tx.QueryRow("SELECT cursor FROM inbox_cursores WHERE flujo='c'").Scan(&n); err != nil {
						return err
					}
					_, err := tx.Exec("UPDATE inbox_cursores SET cursor = ? WHERE flujo='c'", n+1)
					return err
				}); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	var n int
	_ = s.Read().QueryRow("SELECT cursor FROM inbox_cursores WHERE flujo='c'").Scan(&n)
	if n != 1000 {
		t.Fatalf("contador = %d, quiero 1000", n)
	}
}

// kill -9 a mitad de transacciones no corrompe la base ni deja transacciones a medias.
// El proceso hijo escribe pares de filas (a, b) en una misma transacción sin parar;
// se lo mata a la fuerza varias veces y después cada par debe estar completo.
func TestKill9NoCorrompe(t *testing.T) {
	if testing.Short() {
		t.Skip("prueba de procesos")
	}
	path := filepath.Join(t.TempDir(), "nodo.db")
	for ronda := range 6 {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAyudanteEscritor$") //nolint:gosec // re-ejecuta el binario de prueba
		cmd.Env = append(os.Environ(), "RESTPOS_ESCRITOR_DB="+path)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Duration(150+ronda*60) * time.Millisecond)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	s := abrir(t, path)
	if err := s.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	var total, incompletos int
	if err := s.Read().QueryRow(`SELECT count(*), count(*) FILTER (WHERE n <> 2) FROM (SELECT tx, count(*) n FROM pares GROUP BY tx)`).Scan(&total, &incompletos); err != nil {
		t.Fatal(err)
	}
	if total == 0 {
		t.Fatal("el hijo no alcanzó a escribir nada")
	}
	if incompletos != 0 {
		t.Fatalf("%d de %d transacciones quedaron a medias", incompletos, total)
	}
	t.Logf("%d transacciones completas tras 6 kill -9", total)
}

func TestAyudanteEscritor(t *testing.T) {
	path := os.Getenv("RESTPOS_ESCRITOR_DB")
	if path == "" {
		t.Skip("solo como proceso hijo de TestKill9NoCorrompe")
	}
	ctx := context.Background()
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Writer().Exec(`CREATE TABLE IF NOT EXISTS pares (tx TEXT NOT NULL, lado TEXT NOT NULL, relleno BLOB)`); err != nil {
		t.Fatal(err)
	}
	relleno := make([]byte, 4096)
	for i := 0; ; i++ {
		id := fmt.Sprintf("%d-%d", os.Getpid(), i)
		_ = s.Write(ctx, func(tx *sql.Tx) error {
			for _, lado := range []string{"a", "b"} {
				if _, err := tx.Exec("INSERT INTO pares VALUES (?, ?, ?)", id, lado, relleno); err != nil {
					return err
				}
			}
			return nil
		})
	}
}
