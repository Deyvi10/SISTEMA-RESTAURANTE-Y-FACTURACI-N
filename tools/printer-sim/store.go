package main

import (
	"encoding/base64"
	"sync"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"rsc.io/qr"
)

const maxTickets = 200

// Ticket es un trabajo recibido por una impresora simulada.
type Ticket struct {
	ID          int            `json:"id"`
	ImpresoraID string         `json:"impresoraId"`
	Impresora   string         `json:"impresora"`
	Papel       int            `json:"papel"`
	Recibido    time.Time      `json:"recibido"`
	DuracionMs  int64          `json:"duracionMs"`
	Bytes       int            `json:"bytes"`
	Impreso     bool           `json:"impreso"` // false = retenido (sin papel o tapa abierta)
	Texto       string         `json:"texto"`
	Elementos   []elementoJSON `json:"elementos"`
	Avisos      []string       `json:"avisos,omitempty"`
}

type runJSON struct {
	T string `json:"t"`
	B bool   `json:"b,omitempty"`
	U bool   `json:"u,omitempty"`
	I bool   `json:"i,omitempty"`
	W int    `json:"w"`
	H int    `json:"h"`
}

type elementoJSON struct {
	Tipo  string    `json:"tipo"`
	Alin  int       `json:"alin"`
	Runs  []runJSON `json:"runs,omitempty"`
	Dato  string    `json:"dato,omitempty"`
	QRPNG string    `json:"qrPng,omitempty"` // data URI para el panel
}

// Store guarda los últimos tickets en memoria y avisa a los paneles conectados.
type Store struct {
	mu      sync.Mutex
	next    int
	tickets []*Ticket
	subs    map[chan struct{}]struct{}
}

func NewStore() *Store { return &Store{subs: map[chan struct{}]struct{}{}} }

// Add guarda un ticket y devuelve su ID. No notifica: el llamador llama a Changed.
func (s *Store) Add(p *Printer, raw []byte, doc escpos.Document, dur time.Duration, impreso bool) int {
	t := &Ticket{
		ImpresoraID: p.ID, Impresora: p.Nombre, Papel: p.Papel,
		Recibido: time.Now(), DuracionMs: dur.Milliseconds(), Bytes: len(raw),
		Impreso: impreso, Texto: doc.Text(), Elementos: convert(doc), Avisos: doc.Warnings,
	}
	s.mu.Lock()
	s.next++
	t.ID = s.next
	s.tickets = append(s.tickets, t)
	if len(s.tickets) > maxTickets {
		s.tickets = s.tickets[len(s.tickets)-maxTickets:]
	}
	s.mu.Unlock()
	return t.ID
}

func (s *Store) MarkPrinted(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tickets {
		if t.ID == id {
			t.Impreso = true
		}
	}
}

// List devuelve los tickets del más reciente al más antiguo.
func (s *Store) List() []Ticket {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Ticket, 0, len(s.tickets))
	for i := len(s.tickets) - 1; i >= 0; i-- {
		out = append(out, *s.tickets[i])
	}
	return out
}

func (s *Store) Clear() {
	s.mu.Lock()
	s.tickets = nil
	s.mu.Unlock()
	s.Changed()
}

// Subscribe devuelve un canal que recibe una señal en cada cambio.
func (s *Store) Subscribe() (chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}
}

func (s *Store) Changed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default: // ya hay una señal pendiente
		}
	}
}

func convert(doc escpos.Document) []elementoJSON {
	out := make([]elementoJSON, 0, len(doc.Elements))
	for _, e := range doc.Elements {
		ej := elementoJSON{Tipo: string(e.Kind), Alin: int(e.Align), Dato: e.Data}
		for _, r := range e.Runs {
			ej.Runs = append(ej.Runs, runJSON{T: r.Text, B: r.Style.Bold, U: r.Style.Underline, I: r.Style.Invert, W: r.Style.W, H: r.Style.H})
		}
		if e.Kind == escpos.KindQR && e.Data != "" {
			if c, err := qr.Encode(e.Data, qr.M); err == nil {
				ej.QRPNG = "data:image/png;base64," + base64.StdEncoding.EncodeToString(c.PNG())
			}
		}
		out = append(out, ej)
	}
	return out
}
