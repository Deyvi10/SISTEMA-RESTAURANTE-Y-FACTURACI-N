// Package store es la base SQLite del Nodo Local (F2-01, docs/03 §4).
//
// Un solo escritor: la conexión de escritura es única, así que toda transacción de
// escritura se serializa y los secuenciales, bloqueos y reservas son atómicos sin locks
// distribuidos. Las lecturas van por un pool aparte en modo solo lectura y, gracias a WAL,
// nunca esperan al escritor.
//
// Confirmación agrupada: con synchronous=FULL cada commit espera al disco (fsync). Con el
// restaurante lleno, las escrituras que llegan a la vez se ejecutan una tras otra en la misma
// transacción, cada una en su SAVEPOINT (si falla, se deshace solo la suya), y se confirman
// con un único fsync. Nadie recibe respuesta antes de que el disco confirme; con poca carga el
// grupo es de una sola escritura y todo sigue igual.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"runtime"
	"sync"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // driver SQLite sin CGO (ADR-0002)

	edgemigrations "github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/db/edge"
)

// Store agrupa la conexión de escritura y el pool de lectura.
type Store struct {
	w *sql.DB
	r *sql.DB

	cola      chan *escritura
	fin       chan struct{}
	cerrar    sync.Once
	terminado chan struct{}
}

// escritura es un pedido de Write esperando su turno en el escritor.
type escritura struct {
	ctx   context.Context
	fn    func(*Tx) error
	err   error
	panic any
	listo chan struct{}
}

// maxGrupo limita cuántas escrituras comparten un commit (acota la latencia del grupo).
const maxGrupo = 64

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
	s := &Store{w: w, r: r, cola: make(chan *escritura, maxGrupo), fin: make(chan struct{}), terminado: make(chan struct{})}
	go s.escritor()
	return s, nil
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

// Write ejecuta fn en una transacción de escritura; sus cambios quedan en disco al volver sin
// error. Si fn falla, no queda nada de lo suyo. fn no debe llamar a Write (el escritor es uno).
func (s *Store) Write(ctx context.Context, fn func(*Tx) error) error {
	e := &escritura{ctx: ctx, fn: fn, listo: make(chan struct{})}
	select {
	case s.cola <- e:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.fin:
		return errCerrado
	}
	// Una vez en la cola se espera el resultado real: devolver antes dejaría la duda de si
	// la escritura quedó o no. Si la base se cerró sin atenderla, no se hizo.
	select {
	case <-e.listo:
	case <-s.terminado:
		select {
		case <-e.listo:
		default:
			return errCerrado
		}
	}
	if e.panic != nil {
		panic(e.panic)
	}
	return e.err
}

var errCerrado = errors.New("store: la base está cerrada")

// escritor es el único que escribe: toma lo que haya en la cola y lo confirma en grupo.
func (s *Store) escritor() {
	defer close(s.terminado)
	grupo := make([]*escritura, 0, maxGrupo)
	for {
		var e *escritura
		select {
		case e = <-s.cola:
		case <-s.fin:
			return
		}
		grupo = append(grupo[:0], e)
	juntar:
		for len(grupo) < maxGrupo {
			select {
			case e := <-s.cola:
				grupo = append(grupo, e)
			default:
				break juntar
			}
		}
		s.confirmar(grupo)
		for _, e := range grupo {
			close(e.listo)
		}
	}
}

// confirmar corre el grupo en una transacción, cada escritura en su SAVEPOINT, y hace un commit.
func (s *Store) confirmar(grupo []*escritura) {
	tx, err := s.w.BeginTx(context.Background(), nil)
	if err != nil {
		for _, e := range grupo {
			e.err = err
		}
		return
	}
	var hechas []*escritura
	for _, e := range grupo {
		if err := e.ctx.Err(); err != nil { // quien pidió ya no espera: ni se empieza
			e.err = err
			continue
		}
		if _, err := tx.Exec(`SAVEPOINT escritura`); err != nil {
			e.err = err
			continue
		}
		e.panic, e.err = correr(e, tx)
		if e.err != nil || e.panic != nil {
			if _, err := tx.Exec(`ROLLBACK TO escritura; RELEASE escritura`); err != nil {
				// La transacción quedó inservible: nada del grupo se confirma.
				_ = tx.Rollback()
				for _, h := range hechas {
					h.err = fmt.Errorf("store: se deshizo el grupo: %w", err)
				}
				for _, x := range grupo {
					if x.err == nil && x.panic == nil {
						x.err = fmt.Errorf("store: se deshizo el grupo: %w", err)
					}
				}
				return
			}
			continue
		}
		if _, err := tx.Exec(`RELEASE escritura`); err != nil {
			e.err = err
			continue
		}
		hechas = append(hechas, e)
	}
	if err := tx.Commit(); err != nil {
		for _, h := range hechas {
			h.err = err
		}
	}
}

func correr(e *escritura, tx *Tx) (p any, err error) {
	defer func() {
		if r := recover(); r != nil {
			p = r
		}
	}()
	return nil, e.fn(tx)
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
	s.cerrar.Do(func() {
		close(s.fin)
		<-s.terminado
	})
	return errors.Join(s.r.Close(), s.w.Close())
}
