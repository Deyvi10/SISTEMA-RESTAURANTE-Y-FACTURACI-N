// Package impresion es el motor de impresión del Nodo Local (F2-11, F2-12): colas
// persistentes por impresora, envío en paralelo y detección de fallos por DLE EOT.
package impresion

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
)

const (
	conectarMax  = 2 * time.Second
	escribirMax  = 10 * time.Second
	respuestaMax = 800 * time.Millisecond
)

// ErrSinConexion: no se pudo abrir el puerto de la impresora (apagada, IP cambiada, cable).
var ErrSinConexion = errors.New("impresora sin conexión")

// Transporte habla con impresoras de red en el puerto RAW (9100). Es una interfaz para que
// las pruebas y, más adelante, USB (spooler de Windows) usen el mismo motor.
type Transporte interface {
	// Consultar pregunta el estado con DLE EOT. soporta=false si la impresora no responde
	// consultas (algunas genéricas): entonces se imprime a ciegas si el puerto abre.
	Consultar(ctx context.Context, addr string) (st escpos.Status, soporta bool, err error)
	Enviar(ctx context.Context, addr string, data []byte) error
}

// TCP es el transporte RAW sobre TCP.
type TCP struct{}

func (TCP) dial(ctx context.Context, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: conectarMax}
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSinConexion, err)
	}
	return c, nil
}

func (t TCP) Consultar(ctx context.Context, addr string) (escpos.Status, bool, error) {
	c, err := t.dial(ctx, addr)
	if err != nil {
		return escpos.Status{}, false, err
	}
	defer func() { _ = c.Close() }()
	var st escpos.Status
	consultas := []byte{escpos.StatusPrinter, escpos.StatusOffline, escpos.StatusError, escpos.StatusPaper}
	_ = c.SetDeadline(time.Now().Add(respuestaMax))
	for _, n := range consultas {
		if _, err := c.Write(escpos.StatusRequest(n)); err != nil {
			return st, false, fmt.Errorf("%w: %w", ErrSinConexion, err)
		}
		b := make([]byte, 1)
		if _, err := c.Read(b); err != nil {
			// El puerto abrió pero no hay respuesta (timeout o cierre): impresora genérica
			// sin consultas de estado. Se imprime a ciegas.
			return escpos.Status{}, false, nil
		}
		if !st.ApplyStatus(n, b[0]) {
			return escpos.Status{}, false, nil
		}
	}
	return st, true, nil
}

func (t TCP) Enviar(ctx context.Context, addr string, data []byte) error {
	c, err := t.dial(ctx, addr)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	_ = c.SetWriteDeadline(time.Now().Add(escribirMax))
	if _, err := c.Write(data); err != nil {
		return fmt.Errorf("%w: %w", ErrSinConexion, err)
	}
	return nil
}
