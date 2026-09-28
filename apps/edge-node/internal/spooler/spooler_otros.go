//go:build !windows

package spooler

import (
	"context"
	"fmt"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
)

// Listar solo funciona en Windows.
func Listar() ([]Instalada, error) { return nil, ErrNoSoportado }

// Transporte fuera de Windows no puede imprimir en colas del spooler.
type Transporte struct{}

func (Transporte) Consultar(context.Context, string) (escpos.Status, bool, error) {
	return escpos.Status{}, false, fmt.Errorf("%w: %w", impresion.ErrSinConexion, ErrNoSoportado)
}

func (Transporte) Enviar(context.Context, string, []byte) error {
	return fmt.Errorf("%w: %w", impresion.ErrSinConexion, ErrNoSoportado)
}
