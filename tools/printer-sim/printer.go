package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
)

// Estado simulable de una impresora.
type Estado string

const (
	Normal       Estado = "normal"
	PocoPapel    Estado = "poco_papel"
	SinPapel     Estado = "sin_papel"
	TapaAbierta  Estado = "tapa_abierta"
	Desconectada Estado = "desconectada"
)

func (e Estado) valido() bool {
	switch e {
	case Normal, PocoPapel, SinPapel, TapaAbierta, Desconectada:
		return true
	}
	return false
}

func (e Estado) status() escpos.Status {
	return escpos.Status{PaperNearEnd: e == PocoPapel, PaperOut: e == SinPapel, CoverOpen: e == TapaAbierta}
}

// Printer es una térmica simulada escuchando en un puerto TCP.
type Printer struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
	Addr   string `json:"addr"`
	Papel  int    `json:"papel"`

	mu        sync.Mutex
	estado    Estado
	latencia  time.Duration
	ln        net.Listener
	retenidos []int // IDs de tickets recibidos sin papel o con la tapa abierta

	store *Store
	log   *slog.Logger
}

// Snapshot es la vista pública de la impresora.
type Snapshot struct {
	ID         string `json:"id"`
	Nombre     string `json:"nombre"`
	Addr       string `json:"addr"`
	Papel      int    `json:"papel"`
	Estado     Estado `json:"estado"`
	LatenciaMs int    `json:"latenciaMs"`
	Retenidos  int    `json:"retenidos"`
}

func (p *Printer) Snapshot() Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	return Snapshot{p.ID, p.Nombre, p.Addr, p.Papel, p.estado, int(p.latencia / time.Millisecond), len(p.retenidos)}
}

// Start abre el puerto. Una impresora «desconectada» cierra el puerto, igual que una
// térmica apagada: el nodo recibe «connection refused».
func (p *Printer) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ln != nil {
		return nil
	}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", p.Addr)
	if err != nil {
		return fmt.Errorf("%s: %w", p.Nombre, err)
	}
	p.ln = ln
	go p.accept(ln)
	p.log.Info("impresora escuchando", "impresora", p.Nombre, "addr", p.Addr)
	return nil
}

func (p *Printer) stop() {
	if p.ln != nil {
		_ = p.ln.Close()
		p.ln = nil
	}
}

// SetEstado cambia el estado simulado. Al volver a un estado imprimible se «imprime»
// lo retenido, como hace una térmica real al reponer el papel.
func (p *Printer) SetEstado(ctx context.Context, e Estado, latencia time.Duration) error {
	if !e.valido() {
		return fmt.Errorf("estado desconocido %q", e)
	}
	if latencia < 0 || latencia > 10*time.Second {
		return errors.New("la latencia debe estar entre 0 y 10 000 ms")
	}
	p.mu.Lock()
	p.estado, p.latencia = e, latencia
	var liberar []int
	if e.status().OK() {
		liberar, p.retenidos = p.retenidos, nil
	}
	if e == Desconectada {
		p.stop()
	}
	p.mu.Unlock()

	if e != Desconectada {
		if err := p.Start(ctx); err != nil {
			return err
		}
	}
	for _, id := range liberar {
		p.store.MarkPrinted(id)
	}
	p.store.Changed()
	return nil
}

func (p *Printer) accept(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return // puerto cerrado
		}
		go p.serve(c)
	}
}

// idleTimeout cierra un trabajo si el cliente deja la conexión abierta sin enviar nada.
const idleTimeout = 2 * time.Second

func (p *Printer) serve(c net.Conn) {
	defer func() { _ = c.Close() }()
	start := time.Now()
	var buf []byte
	answered := 0
	chunk := make([]byte, 4096)
	for {
		_ = c.SetReadDeadline(time.Now().Add(idleTimeout))
		n, err := c.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if len(buf) > 1<<20 {
				p.log.Warn("trabajo de más de 1 MB descartado", "impresora", p.Nombre)
				return
			}
			// Responder las consultas DLE EOT nuevas en el momento, como el firmware.
			doc := escpos.Decode(buf)
			p.mu.Lock()
			st := p.estado.status()
			p.mu.Unlock()
			for _, q := range doc.StatusQueries[answered:] {
				if _, werr := c.Write([]byte{escpos.StatusByte(q, st)}); werr != nil {
					break
				}
			}
			answered = len(doc.StatusQueries)
		}
		if err != nil {
			var ne net.Error
			timeout := errors.As(err, &ne) && ne.Timeout()
			if !errors.Is(err, io.EOF) && !timeout {
				p.log.Warn("error de lectura", "impresora", p.Nombre, "err", err)
			}
			break
		}
	}
	p.finish(buf, time.Since(start))
}

func (p *Printer) finish(buf []byte, dur time.Duration) {
	doc := escpos.Decode(buf)
	if !hasContent(doc) {
		return // solo consultas de estado
	}
	p.mu.Lock()
	lat := p.latencia
	p.mu.Unlock()
	time.Sleep(lat)

	// Decidir si se imprime y registrar la retención bajo el mismo candado: así un
	// cambio de estado concurrente nunca deja un ticket retenido para siempre.
	p.mu.Lock()
	ok := p.estado.status().OK()
	id := p.store.Add(p, buf, doc, dur+lat, ok)
	if !ok {
		p.retenidos = append(p.retenidos, id)
	}
	p.mu.Unlock()
	p.store.Changed()
	p.log.Info("trabajo recibido", "impresora", p.Nombre, "bytes", len(buf), "impreso", ok)
}

func hasContent(d escpos.Document) bool {
	for _, e := range d.Elements {
		if e.Kind != escpos.KindText || len(e.Runs) > 0 {
			return true
		}
	}
	return false
}
