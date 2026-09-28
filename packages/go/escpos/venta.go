package escpos

import (
	"fmt"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// DocumentoVenta es el documento interno que entrega la caja mientras no hay facturación
// electrónica (F4-16): se imprime claramente marcado como sin valor tributario.
type DocumentoVenta struct {
	Local      string
	Numero     int
	Hora       time.Time // en la zona del local
	Mesa       string
	Cajero     string
	Comprador  string // «CONSUMIDOR FINAL»
	Lineas     []LineaCuenta
	Subtotal   money.Money
	IVA        money.Money
	Propina    money.Money
	Total      money.Money
	Metodo     string
	Recibido   money.Money // efectivo entregado (cero si no aplica)
	Vuelto     money.Money
	AbrirCajon bool // pulso al cajón antes de imprimir: se abre al instante
}

// NumeroInterno formatea el número del documento interno: INT-000123.
func NumeroInterno(n int) string { return fmt.Sprintf("INT-%06d", n) }

func ImprimirDocumentoVenta(p Paper, d DocumentoVenta) []byte {
	b := New(p)
	if d.AbrirCajon {
		b.OpenDrawer()
	}
	b.Align(Center).Bold(true).Size(1, 2).Line(d.Local).Size(1, 1)
	b.Line("DOCUMENTO INTERNO DE VENTA").Bold(false)
	b.Invert(true).Line(" SIN VALOR TRIBUTARIO ").Invert(false)
	b.Align(Left).Separator('-')
	b.Columns(NumeroInterno(d.Numero), d.Hora.Format("02/01/2006 15:04"))
	b.Columns(d.Mesa, "Caja: "+d.Cajero)
	b.Line("Cliente: " + d.Comprador)
	b.Separator('-')
	for _, l := range d.Lineas {
		b.Columns(l.Cantidad+" "+l.Producto, "$"+l.Total.String())
	}
	b.Separator('-')
	b.Columns("Subtotal", "$"+d.Subtotal.String())
	b.Columns("IVA", "$"+d.IVA.String())
	if !d.Propina.IsZero() {
		b.Columns("Servicio", "$"+d.Propina.String())
	}
	b.Bold(true).Size(1, 2).Columns("TOTAL", "$"+d.Total.String()).Size(1, 1).Bold(false)
	b.Separator('-')
	b.Columns(d.Metodo, "$"+d.Total.String())
	if !d.Recibido.IsZero() {
		b.Columns("Recibido", "$"+d.Recibido.String())
		b.Bold(true).Columns("Vuelto", "$"+d.Vuelto.String()).Bold(false)
	}
	b.Separator('-').Align(Center)
	b.Wrapped("Este documento no reemplaza a la factura electrónica.", "")
	b.Line("¡Gracias por su visita!")
	return b.Feed(3).Cut(true).Bytes()
}
