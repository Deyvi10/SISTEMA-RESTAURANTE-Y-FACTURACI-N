package escpos

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/cierrez"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

func signo(m money.Money) string {
	if m.IsNegative() {
		return "-$" + m.Neg().String()
	}
	return "$" + m.String()
}

// ImprimirCierreZ arma el Cierre Z de un turno (RF-04-10.5): efectivo con su conteo, cada
// método de pago con esperado, declarado y diferencia, el resultado y el hash que lo encadena.
func ImprimirCierreZ(p Paper, c cierrez.Cierre, loc *time.Location) []byte {
	b := New(p)
	b.Align(Center).Bold(true).Line(c.Local)
	b.Size(2, 2).Line(fmt.Sprintf("CIERRE Z %04d", c.Numero)).Size(1, 1).Bold(false)
	b.Line(c.Caja)
	b.Align(Left).Separator('-')
	abierto, cerrado := c.AbiertoAt.In(loc), c.CerradoAt.In(loc)
	b.Columns("Jornada", c.FechaNegocio)
	b.Columns("Cajero", c.Cajero)
	if c.CerradoPor != "" && c.CerradoPor != c.Cajero {
		b.Columns("Cerró", c.CerradoPor)
	}
	b.Columns("Apertura", abierto.Format("02/01 15:04"))
	b.Columns("Cierre", cerrado.Format("02/01 15:04"))
	for _, l := range c.Lineas {
		b.Separator('-')
		b.Bold(true).Line(l.Metodo).Bold(false)
		if l.Tipo == "EFECTIVO" {
			b.Columns("  Fondo inicial", signo(c.FondoInicial))
			b.Columns("  Ventas en efectivo", signo(l.Cobrado))
			b.Columns("  Ingresos", signo(c.Ingresos))
			b.Columns("  Retiros", signo(c.Retiros.Neg()))
			b.Columns("  Gastos", signo(c.Gastos.Neg()))
		}
		b.Columns("  Esperado", signo(l.Esperado))
		b.Columns("  Declarado", signo(l.Declarado))
		b.Bold(true).Columns("  "+l.Resultado, signo(l.Diferencia)).Bold(false)
		if l.Tipo == "EFECTIVO" {
			b.Line("Conteo:")
			for _, cn := range c.Conteo {
				if cn.Cantidad == 0 {
					continue
				}
				for _, d := range cierrez.Denominaciones {
					if d.Clave == cn.Clave {
						tipo := "billete"
						if d.Moneda {
							tipo = "moneda"
						}
						b.Columns(fmt.Sprintf("    %d x %s (%s)", cn.Cantidad, d.Etiqueta, tipo), "$"+d.Valor.Mul(decimal.NewFromInt(int64(cn.Cantidad))).Round2().String())
					}
				}
			}
		}
	}
	b.Separator('=').Align(Center)
	b.Size(2, 2).Bold(true).Line(c.Resultado).Size(1, 1).Bold(false)
	b.Separator('=').Align(Left)
	b.Line("Hash: " + c.Hash[:min(16, len(c.Hash))])
	if c.HashAnterior != "" {
		b.Line("Anterior: " + c.HashAnterior[:min(16, len(c.HashAnterior))])
	}
	b.Feed(2).Line("Firma: " + strings.Repeat("_", b.Width()-len("Firma: ")))
	b.Align(Center).Wrapped("DOCUMENTO INTERNO · SIN VALOR TRIBUTARIO", "")
	return b.Feed(3).Cut(true).Bytes()
}
