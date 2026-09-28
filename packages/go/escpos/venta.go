package escpos

import (
	"fmt"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// DocumentoVenta es el documento interno que entrega la caja mientras no hay facturación
// electrónica (F4-16): se imprime claramente marcado como sin valor tributario.
type DocumentoVenta struct {
	Local       string
	Numero      int
	Hora        time.Time // en la zona del local
	Mesa        string
	Cajero      string
	Comprador   string // «CONSUMIDOR FINAL» o el nombre del comprador
	CompradorID string // cédula, RUC o pasaporte (vacío para consumidor final)
	Lineas      []LineaCuenta
	Descuento   money.Money // descuentos y cortesías (F4-09)
	Subtotal    money.Money
	IVA         money.Money
	Propina     money.Money
	Total       money.Money
	Pagos       []PagoTicket
	Recibido    money.Money // efectivo entregado (cero si no aplica)
	Vuelto      money.Money
	AbrirCajon  bool // pulso al cajón antes de imprimir: se abre al instante
}

// PagoTicket es una línea de la forma de pago (un cobro puede combinar métodos).
type PagoTicket struct {
	Metodo   string
	Monto    money.Money
	Ultimos4 string
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
	b.Wrapped("Cliente: "+d.Comprador, "")
	if d.CompradorID != "" {
		b.Line("C.I./RUC: " + d.CompradorID)
	}
	b.Separator('-')
	for _, l := range d.Lineas {
		b.Columns(l.Cantidad+" "+l.Producto, "$"+l.Total.String())
	}
	b.Separator('-')
	if !d.Descuento.IsZero() {
		b.Columns("Descuento", "-$"+d.Descuento.String())
	}
	b.Columns("Subtotal", "$"+d.Subtotal.String())
	b.Columns("IVA", "$"+d.IVA.String())
	if !d.Propina.IsZero() {
		b.Columns("Servicio", "$"+d.Propina.String())
	}
	b.Bold(true).Size(1, 2).Columns("TOTAL", "$"+d.Total.String()).Size(1, 1).Bold(false)
	b.Separator('-')
	if len(d.Pagos) == 0 {
		b.Line("Cortesía de la casa: sin cobro")
	}
	for _, p := range d.Pagos {
		nombre := p.Metodo
		if p.Ultimos4 != "" {
			nombre += " ****" + p.Ultimos4
		}
		b.Columns(nombre, "$"+p.Monto.String())
	}
	if !d.Recibido.IsZero() {
		b.Columns("Recibido", "$"+d.Recibido.String())
		b.Bold(true).Columns("Vuelto", "$"+d.Vuelto.String()).Bold(false)
	}
	b.Separator('-').Align(Center)
	b.Wrapped("Este documento no reemplaza a la factura electrónica.", "")
	b.Line("¡Gracias por su visita!")
	return b.Feed(3).Cut(true).Bytes()
}

// AperturaCajon es el comprobante de una apertura del cajón sin venta (RF-02-07): el pulso
// va primero y el papel deja constancia de quién, cuándo y por qué.
type AperturaCajon struct {
	Local         string
	Caja          string
	Usuario       string
	AutorizadoPor string // supervisor que autorizó con su PIN (vacío si fue el mismo usuario)
	Motivo        string
	Hora          time.Time
}

func ImprimirAperturaCajon(p Paper, a AperturaCajon) []byte {
	b := New(p).OpenDrawer()
	b.Align(Center).Bold(true).Line(a.Local)
	b.Line("APERTURA DE CAJÓN SIN VENTA").Bold(false)
	b.Align(Left).Separator('-')
	b.Columns(a.Caja, a.Hora.Format("02/01/2006 15:04"))
	b.Line("Usuario: " + a.Usuario)
	if a.AutorizadoPor != "" && a.AutorizadoPor != a.Usuario {
		b.Line("Autorizó: " + a.AutorizadoPor)
	}
	b.Wrapped("Motivo: "+a.Motivo, "")
	b.Separator('-')
	return b.Feed(3).Cut(true).Bytes()
}
