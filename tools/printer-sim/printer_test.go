package main

import (
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
)

func newTestPrinter(t *testing.T) (*Printer, *Store) {
	t.Helper()
	store := NewStore()
	p := &Printer{ID: "cocina", Nombre: "Cocina", Addr: "127.0.0.1:0", Papel: 80, estado: Normal, store: store, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := p.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	p.Addr = p.ln.Addr().String() // puerto real asignado, para reabrir el mismo tras «desconectada»
	t.Cleanup(func() { p.mu.Lock(); p.stop(); p.mu.Unlock() })
	return p, store
}

func queryStatus(t *testing.T, addr string, n byte) byte {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, err := c.Write(escpos.StatusRequest(n)); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	b := make([]byte, 1)
	if _, err := io.ReadFull(c, b); err != nil {
		t.Fatalf("sin respuesta de estado: %v", err)
	}
	return b[0]
}

func waitTickets(t *testing.T, s *Store, n int) []Ticket {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if l := s.List(); len(l) >= n {
			return l
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no llegaron %d tickets", n)
	return nil
}

func TestPrinterReceivesAndAnswersStatus(t *testing.T) {
	p, store := newTestPrinter(t)

	var st escpos.Status
	st.ApplyStatus(escpos.StatusPaper, queryStatus(t, p.Addr, escpos.StatusPaper))
	if !st.OK() {
		t.Fatalf("una impresora normal debería estar OK: %+v", st)
	}

	if err := send(t.Context(), p.Addr, escpos.ImprimirPrueba(escpos.Paper80, "Cocina", "TCP", time.Now())); err != nil {
		t.Fatal(err)
	}
	tk := waitTickets(t, store, 1)[0]
	if !tk.Impreso || !strings.Contains(tk.Texto, "¡Funciona!") {
		t.Fatalf("ticket inesperado: impreso=%v\n%s", tk.Impreso, tk.Texto)
	}
	if len(tk.Avisos) > 0 {
		t.Fatalf("avisos del decodificador: %v", tk.Avisos)
	}
}

func TestPaperOutHoldsAndReleases(t *testing.T) {
	p, store := newTestPrinter(t)
	if err := p.SetEstado(t.Context(), SinPapel, 0); err != nil {
		t.Fatal(err)
	}
	var st escpos.Status
	st.ApplyStatus(escpos.StatusPaper, queryStatus(t, p.Addr, escpos.StatusPaper))
	if !st.PaperOut {
		t.Fatal("debería reportar sin papel")
	}

	if err := send(t.Context(), p.Addr, escpos.New(escpos.Paper80).Line("Ceviche").Cut(true).Bytes()); err != nil {
		t.Fatal(err)
	}
	if tk := waitTickets(t, store, 1)[0]; tk.Impreso {
		t.Fatal("sin papel el ticket debe quedar retenido")
	}
	if p.Snapshot().Retenidos != 1 {
		t.Fatal("debería haber 1 retenido")
	}

	if err := p.SetEstado(t.Context(), Normal, 0); err != nil {
		t.Fatal(err)
	}
	if tk := store.List()[0]; !tk.Impreso {
		t.Fatal("al reponer el papel lo retenido se imprime")
	}
}

func TestDisconnectedRefusesConnections(t *testing.T) {
	p, _ := newTestPrinter(t)
	if err := p.SetEstado(t.Context(), Desconectada, 0); err != nil {
		t.Fatal(err)
	}
	if c, err := net.DialTimeout("tcp", p.Addr, 300*time.Millisecond); err == nil {
		_ = c.Close()
		t.Fatal("una impresora apagada no debe aceptar conexiones")
	}
	if err := p.SetEstado(t.Context(), Normal, 0); err != nil {
		t.Fatal(err)
	}
	if b := queryStatus(t, p.Addr, escpos.StatusPrinter); b&0x08 != 0 {
		t.Fatal("al encenderla vuelve a estar en línea")
	}
}

func TestSetEstadoValidates(t *testing.T) {
	p, _ := newTestPrinter(t)
	if err := p.SetEstado(t.Context(), "rota", 0); err == nil {
		t.Error("estado desconocido debería fallar")
	}
	if err := p.SetEstado(t.Context(), Normal, 11*time.Second); err == nil {
		t.Error("latencia excesiva debería fallar")
	}
}

func TestParsePrinters(t *testing.T) {
	ps, err := parsePrinters("Cocina caliente=:9100, Bar=:9101", 80, NewStore(), slog.Default())
	if err != nil || len(ps) != 2 || ps[0].ID != "cocina-caliente" || ps[1].ID != "bar" {
		t.Fatalf("parse: %v %v", ps, err)
	}
	for _, bad := range []string{"Cocina", "=:9100", "A=:1,A=:2"} {
		if _, err := parsePrinters(bad, 80, NewStore(), slog.Default()); err == nil {
			t.Errorf("%q debería fallar", bad)
		}
	}
}
