package edgesync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// chaos es una red que falla de todas las formas que importan para la sincronización.
type chaos struct {
	base http.RoundTripper
	mu   sync.Mutex
	rng  *rand.Rand

	refused, lostResponse, unavailable, slow, ok atomic.Int64
}

func (c *chaos) roll() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rng.Float64()
}

func (c *chaos) RoundTrip(req *http.Request) (*http.Response, error) {
	switch r := c.roll(); {
	case r < 0.10: // la petición nunca llega (router caído, DNS, conexión rechazada)
		c.refused.Add(1)
		return nil, errors.New("caos: conexión rechazada")
	case r < 0.22: // la nube APLICÓ el lote pero el ACK se pierde: el caso peligroso
		res, err := c.base.RoundTrip(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
		}
		c.lostResponse.Add(1)
		return nil, errors.New("caos: respuesta perdida")
	case r < 0.27: // balanceador devuelve 503 sin llegar a la API
		c.unavailable.Add(1)
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("Service Unavailable")), Header: http.Header{}, Request: req}, nil
	case r < 0.35: // latencia alta
		c.slow.Add(1)
		time.Sleep(time.Duration(c.roll()*40) * time.Millisecond)
	}
	c.ok.Add(1)
	return c.base.RoundTrip(req)
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("define TEST_DATABASE_URL (con `make dev`: postgres://restpos_owner:restpos_dev@localhost:5442/restpos) para correr la prueba de caos")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("spike_sync_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	cfg, _ := pgxpool.ParseConfig(dsn)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ReceiverSchema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	return pool
}

// nodo simula un Nodo Local: una SQLite con una tabla de negocio y su outbox.
type nodo struct {
	id        ids.ID
	path      string
	db        *sql.DB
	outbox    *Outbox
	committed atomic.Int64
}

func (n *nodo) open(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	db, err := OpenSQLite(ctx, n.path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS ordenes_demo (id TEXT PRIMARY KEY, agregado TEXT NOT NULL, n INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if n.outbox, err = NewOutbox(ctx, db); err != nil {
		t.Fatal(err)
	}
	n.db = db
}

// venta escribe una fila de negocio y su evento en UNA transacción. Con rollback=true
// simula un cobro que falla después de escribir: ni la venta ni el evento deben existir.
func (n *nodo) venta(ctx context.Context, agg ids.ID, i int, rollback bool) error {
	tx, err := n.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	id := ids.New()
	if _, err := tx.ExecContext(ctx, `INSERT INTO ordenes_demo (id, agregado, n) VALUES (?, ?, ?)`, id.String(), agg.String(), i); err != nil {
		return err
	}
	ev, err := NewEvent("order.submitted", 1, agg, map[string]any{"orden": id, "n": i, "total": "14.50"}, time.Now())
	if err != nil {
		return err
	}
	if _, err := n.outbox.Append(ctx, tx, ev); err != nil {
		return err
	}
	if rollback {
		return nil // defer hace rollback
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	n.committed.Add(1)
	return nil
}

// QA-07: eventos con cortes y fallas aleatorias → 0 pérdidas y 0 duplicados, en orden.
func TestChaosSync(t *testing.T) {
	pool := testPool(t)
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := httptest.NewServer((&Receiver{Pool: pool, Log: quiet}).Handler())
	defer api.Close()

	net := &chaos{base: http.DefaultTransport, rng: rand.New(rand.NewPCG(42, 7))} //nolint:gosec // prueba
	client := &http.Client{Transport: net, Timeout: 5 * time.Second}

	const porNodo, fases = 5000, 4
	nodos := []*nodo{
		{id: ids.New(), path: filepath.Join(t.TempDir(), "a.db")},
		{id: ids.New(), path: filepath.Join(t.TempDir(), "b.db")},
	}
	aggs := make([]ids.ID, 60) // 60 mesas/órdenes para mezclar agregados
	for i := range aggs {
		aggs[i] = ids.New()
	}

	var wg sync.WaitGroup
	start := time.Now()
	for _, n := range nodos {
		wg.Add(1)
		go func(n *nodo) {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(n.id[15]), 1)) //nolint:gosec // prueba
			i := 0
			// Cada fase: el pusher arranca, se venden eventos y el nodo «se reinicia»
			// (se corta el pusher en pleno envío y se cierra la base), como un corte de luz.
			for f := range fases {
				n.open(t)
				ctx, cancel := context.WithCancel(context.Background())
				p := &Pusher{Outbox: n.outbox, NodeID: n.id, URL: api.URL, Client: client, Log: quiet, Interval: 20 * time.Millisecond, MaxWait: 150 * time.Millisecond}
				done := make(chan struct{})
				go func() { p.Run(ctx); close(done) }()
				var ventaErr error
				for ; n.committed.Load() < porNodo*int64(f+1)/fases && ventaErr == nil; i++ {
					ventaErr = n.venta(context.Background(), aggs[rng.IntN(len(aggs))], i, rng.IntN(100) == 0)
					p.Notify()
				}
				cancel()
				<-done
				_ = n.db.Close()
				if ventaErr != nil {
					t.Error(ventaErr)
					return
				}
			}
			// Tras el último reinicio el nodo sigue hasta vaciar su outbox.
			n.open(t)
			defer func() { _ = n.db.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			p := &Pusher{Outbox: n.outbox, NodeID: n.id, URL: api.URL, Client: client, Log: quiet, Interval: 20 * time.Millisecond, MaxWait: 150 * time.Millisecond}
			go p.Run(ctx)
			for ctx.Err() == nil {
				if s, err := n.outbox.Stats(ctx); err == nil && s.Pending == 0 {
					// Atomicidad local: cada venta confirmada tiene su evento y viceversa;
					// las transacciones revertidas no dejaron ni venta ni evento.
					var ventas, eventos int64
					_ = n.db.QueryRowContext(ctx, `SELECT count(*) FROM ordenes_demo`).Scan(&ventas)
					_ = n.db.QueryRowContext(ctx, `SELECT count(*) FROM outbox`).Scan(&eventos)
					if ventas != n.committed.Load() || eventos != ventas {
						t.Errorf("nodo %s: %d ventas, %d eventos, %d confirmadas", n.id, ventas, eventos, n.committed.Load())
					}
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
			t.Errorf("el nodo %s no vació su outbox", n.id)
		}(n)
	}
	wg.Wait()
	if t.Failed() {
		return
	}
	elapsed := time.Since(start)

	ctx := context.Background()
	total := int64(0)
	for _, n := range nodos {
		var count, distinctSeq, maxSeq, cursor int64
		if err := pool.QueryRow(ctx, `SELECT count(*), count(DISTINCT node_seq), coalesce(max(node_seq),0) FROM sync_eventos WHERE nodo_id=$1`, n.id).Scan(&count, &distinctSeq, &maxSeq); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT ultimo_seq FROM sync_cursores WHERE nodo_id=$1`, n.id).Scan(&cursor); err != nil {
			t.Fatal(err)
		}
		want := n.committed.Load()
		if count != want || distinctSeq != want || maxSeq != want || cursor != want {
			t.Errorf("nodo %s: nube tiene %d eventos (%d seq distintos, máx %d, cursor %d); el nodo confirmó %d",
				n.id, count, distinctSeq, maxSeq, cursor, want)
		}
		// Orden: aplicados en la nube en exactamente el orden de node_seq, sin huecos.
		var desordenados int64
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM (
			SELECT node_seq - lag(node_seq) OVER (ORDER BY orden) AS d FROM sync_eventos WHERE nodo_id=$1) x WHERE d <> 1`, n.id).Scan(&desordenados); err != nil {
			t.Fatal(err)
		}
		if desordenados != 0 {
			t.Errorf("nodo %s: %d eventos aplicados fuera de orden", n.id, desordenados)
		}
		total += count
	}
	t.Logf("%d eventos de 2 nodos en %s · rechazadas %d · ACK perdido %d · 503 %d · lentas %d · OK %d",
		total, elapsed.Round(time.Millisecond), net.refused.Load(), net.lostResponse.Load(), net.unavailable.Load(), net.slow.Load(), net.ok.Load())
	if net.lostResponse.Load() == 0 || net.refused.Load() == 0 {
		t.Error("la prueba no ejercitó las fallas de red")
	}
}
