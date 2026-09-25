package edgesync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Pusher envía el outbox a la nube hasta vaciarlo.
type Pusher struct {
	Outbox   *Outbox
	NodeID   ids.ID
	URL      string       // https://api…/v1/sync/push
	Client   *http.Client // con timeout; por defecto 15 s
	Clock    clock.Clock
	Log      *slog.Logger
	Interval time.Duration // espera cuando no hay nada pendiente; por defecto 2 s
	MaxWait  time.Duration // tope de la espera exponencial; por defecto 30 s

	wake chan struct{}
	mu   sync.Mutex // un solo envío a la vez (Run y los vaciados previos al pull)
}

// Notify despierta al pusher en cuanto hay un evento nuevo (sin esperar el intervalo).
func (p *Pusher) Notify() {
	p.init()
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *Pusher) init() {
	if p.wake == nil {
		p.wake = make(chan struct{}, 1)
	}
}

// Run envía hasta que ctx se cancela. Los errores de red nunca lo detienen.
func (p *Pusher) Run(ctx context.Context) {
	p.init()
	interval := orDefault(p.Interval, 2*time.Second)
	fails := 0
	for {
		sent, err := p.PushOnce(ctx)
		var wait time.Duration
		switch {
		case err != nil:
			fails++
			wait = backoff(fails, orDefault(p.MaxWait, 30*time.Second))
			p.log().Warn("sync: envío fallido, se reintentará", "err", err, "intento", fails, "espera", wait)
		case sent > 0:
			fails = 0
			continue // puede haber más pendiente: seguir sin esperar
		default:
			fails = 0
			wait = interval
		}
		select {
		case <-ctx.Done():
			return
		case <-p.wake:
		case <-time.After(wait):
		}
	}
}

// PushOnce envía un lote y devuelve cuántos eventos quedaron confirmados.
func (p *Pusher) PushOnce(ctx context.Context) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	batch, err := p.Outbox.Pending(ctx, MaxBatchEvents, MaxBatchBytes)
	if err != nil || len(batch) == 0 {
		return 0, err
	}
	body, err := json.Marshal(PushRequest{NodeID: p.NodeID, Events: batch})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return 0, fmt.Errorf("sync: la nube respondió %d: %s", res.StatusCode, bytes.TrimSpace(msg))
	}
	var ack PushResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&ack); err != nil {
		return 0, fmt.Errorf("sync: ACK ilegible: %w", err)
	}
	last := batch[len(batch)-1].NodeSeq
	if ack.LastApplied > last {
		// La nube dice tener más de lo que enviamos: solo puede ocurrir si otro nodo usa
		// nuestro ID o si se restauró un respaldo viejo. Nunca se marca lo no enviado.
		return 0, errors.New("sync: la nube confirmó un seq mayor al enviado; revisar identidad del nodo")
	}
	if err := p.Outbox.MarkSent(ctx, ack.LastApplied, p.now()); err != nil {
		return 0, err
	}
	n := 0
	for _, e := range batch {
		if e.NodeSeq <= ack.LastApplied {
			n++
		}
	}
	return n, nil
}

func (p *Pusher) now() time.Time {
	if p.Clock == nil {
		return time.Now().UTC()
	}
	return p.Clock.Now()
}

func (p *Pusher) log() *slog.Logger {
	if p.Log == nil {
		return slog.Default()
	}
	return p.Log
}

// backoff: 1 s, 2 s, 4 s… hasta max, con jitter del ±20 % para no sincronizar reintentos
// de cientos de nodos cuando vuelve internet.
func backoff(fails int, maxWait time.Duration) time.Duration {
	d := time.Second << min(fails-1, 10)
	d = min(d, maxWait)
	permil := 800 + rand.N(401) //nolint:gosec // jitter, no criptografía
	return d * time.Duration(permil) / 1000
}

func orDefault(d, def time.Duration) time.Duration {
	if d <= 0 {
		return def
	}
	return d
}
