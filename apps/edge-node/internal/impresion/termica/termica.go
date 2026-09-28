// Package termica es una impresora térmica de red falsa para pruebas: responde las
// consultas DLE EOT con el estado que se le fije y guarda cada trabajo recibido. Puede
// apagarse (cierra el puerto) y comportarse como una genérica que no responde estado.
package termica

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
)

type Termica struct {
	t         testing.TB
	mu        sync.Mutex
	ln        net.Listener
	addr      string
	st        escpos.Status
	sinEstado bool
	recibido  [][]byte
	llegadas  []time.Time
}

// Nueva enciende una impresora en un puerto libre de 127.0.0.1.
func Nueva(t testing.TB) *Termica {
	x := &Termica{t: t}
	x.Encender()
	t.Cleanup(x.Apagar)
	return x
}

func (x *Termica) Addr() string { x.mu.Lock(); defer x.mu.Unlock(); return x.addr }

// Encender abre el puerto (el mismo de antes si ya se había encendido).
func (x *Termica) Encender() {
	addr := x.Addr()
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		x.t.Fatal(err)
	}
	x.mu.Lock()
	x.ln, x.addr = ln, ln.Addr().String()
	x.mu.Unlock()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go x.atender(c)
		}
	}()
}

// Apagar cierra el puerto: el nodo recibe «connection refused».
func (x *Termica) Apagar() {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.ln != nil {
		_ = x.ln.Close()
		x.ln = nil
	}
}

func (x *Termica) Fijar(st escpos.Status) { x.mu.Lock(); x.st = st; x.mu.Unlock() }

// SinEstado hace que no responda DLE EOT (impresora genérica).
func (x *Termica) SinEstado() { x.mu.Lock(); x.sinEstado = true; x.mu.Unlock() }

func (x *Termica) atender(c net.Conn) {
	defer func() { _ = c.Close() }()
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		_ = c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, err := c.Read(tmp)
		if n > 0 {
			chunk := tmp[:n]
			for len(chunk) >= 3 && chunk[0] == escpos.DLE && chunk[1] == escpos.EOT {
				x.mu.Lock()
				mudo, st := x.sinEstado, x.st
				x.mu.Unlock()
				if !mudo {
					_, _ = c.Write([]byte{escpos.StatusByte(chunk[2], st)})
				}
				chunk = chunk[3:]
			}
			buf = append(buf, chunk...)
		}
		if err != nil {
			break
		}
	}
	if len(buf) > 0 {
		x.mu.Lock()
		x.recibido = append(x.recibido, buf)
		x.llegadas = append(x.llegadas, time.Now())
		x.mu.Unlock()
	}
}

// Trabajos devuelve una copia de lo recibido, en orden.
func (x *Termica) Trabajos() [][]byte {
	x.mu.Lock()
	defer x.mu.Unlock()
	return append([][]byte(nil), x.recibido...)
}

// Textos decodifica cada trabajo recibido a texto plano (para buscar contenido).
func (x *Termica) Textos() []string {
	var out []string
	for _, b := range x.Trabajos() {
		out = append(out, escpos.Decode(b).Text())
	}
	return out
}
