package edgesync

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

func TestValidate(t *testing.T) {
	ev := func(seq int64) Event {
		return Event{ID: ids.New(), NodeSeq: seq, Type: "order.submitted", Version: 1, AggregateID: ids.New(), Payload: []byte(`{}`)}
	}
	ok := PushRequest{NodeID: ids.New(), Events: []Event{ev(1), ev(2)}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := map[string]PushRequest{
		"sin nodo":        {Events: []Event{ev(1)}},
		"vacío":           {NodeID: ids.New()},
		"seq repetido":    {NodeID: ids.New(), Events: []Event{ev(2), ev(2)}},
		"seq decreciente": {NodeID: ids.New(), Events: []Event{ev(3), ev(1)}},
		"seq cero":        {NodeID: ids.New(), Events: []Event{ev(0)}},
		"payload roto":    {NodeID: ids.New(), Events: []Event{{ID: ids.New(), NodeSeq: 1, Type: "x", Version: 1, Payload: []byte(`{`)}}},
		"sin tipo":        {NodeID: ids.New(), Events: []Event{{ID: ids.New(), NodeSeq: 1, Version: 1, Payload: []byte(`{}`)}}},
	}
	for name, r := range bad {
		if err := r.Validate(); !errors.Is(err, ErrInvalidBatch) {
			t.Errorf("%s: se esperaba ErrInvalidBatch, obtuvo %v", name, err)
		}
	}
	big := PushRequest{NodeID: ids.New()}
	for i := range MaxBatchEvents + 1 {
		big.Events = append(big.Events, ev(int64(i+1)))
	}
	if err := big.Validate(); err == nil {
		t.Error("un lote de más de 500 eventos debe rechazarse")
	}
}

func TestBackoffBounds(t *testing.T) {
	for fails := 1; fails < 40; fails++ {
		d := backoff(fails, 30*time.Second)
		if d < 800*time.Millisecond || d > 36*time.Second {
			t.Fatalf("backoff(%d) = %s fuera de rango", fails, d)
		}
	}
}

func TestOutboxRollbackLeavesNoGapAndBatchLimits(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	o, err := NewOutbox(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	add := func(commit bool, payload string) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		e, _ := NewEvent("t", 1, ids.New(), payload, time.Now())
		if _, err := o.Append(ctx, tx, e); err != nil {
			t.Fatal(err)
		}
		if commit {
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		} else {
			_ = tx.Rollback()
		}
	}
	add(true, "a")
	add(false, "revertido") // no debe consumir el seq 2
	add(true, "b")
	evs, err := o.Pending(ctx, 10, MaxBatchBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 || evs[0].NodeSeq != 1 || evs[1].NodeSeq != 2 {
		t.Fatalf("seq con hueco tras rollback: %+v", evs)
	}
	// Un límite de bytes diminuto igual devuelve 1 evento para no atascarse.
	if evs, _ := o.Pending(ctx, 10, 1); len(evs) != 1 {
		t.Fatalf("límite de bytes: %d eventos", len(evs))
	}
	if err := o.MarkSent(ctx, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	if s, _ := o.Stats(ctx); s.Pending != 1 {
		t.Fatalf("pendientes = %d", s.Pending)
	}
}
