package escpos

import (
	"fmt"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
)

// LineaComanda es un plato dentro de una comanda.
type LineaComanda struct {
	Cantidad      string   // "1", "2", "0.5" (texto ya formateado)
	Producto      string   // snapshot del nombre
	Modificadores []string // "Término medio", "Extra queso"
	Nota          string   // nota libre de ESTA línea
	Tiempo        string   // "ENTRADA", "FUERTE"… vacío si no aplica
}

// Comanda es lo que se imprime en una estación de producción (RF-02-04.4).
type Comanda struct {
	Estacion    string    // "COCINA CALIENTE", "BAR"
	Mesa        string    // "Mesa 4", "Llevar #12 · Ana"
	Mesero      string    // "Carlos M."
	Numero      int       // número de comanda de la jornada
	Hora        time.Time // hora original de envío (en la zona del local)
	Lineas      []LineaComanda
	Reimpresion bool      // imprime «REIMPRESIÓN» y la hora original
	Anulacion   bool      // ticket de ANULACIÓN (RF-02-04.5)
	Motivo      string    // motivo de la anulación
	Impreso     time.Time // hora de esta impresión (para reimpresiones)
}

// ImprimirComanda arma el ticket de cocina/bar. Está pensado para leerse de lejos:
// mesa y cantidades en tamaño doble, una línea por plato, modificadores y notas debajo.
func ImprimirComanda(p Paper, c Comanda) []byte {
	b := New(p)
	b.Align(Center)
	if c.Anulacion {
		b.Invert(true).Size(2, 2).Bold(true).Line(" ANULACIÓN ").Invert(false).Size(1, 1)
	}
	if c.Reimpresion {
		b.Bold(true).Line("*** REIMPRESIÓN ***").Bold(false)
	}
	b.Size(1, 1).Bold(true).Line(strings.ToUpper(c.Estacion)).Bold(false)
	b.Size(2, 2).Bold(true).Line(c.Mesa).Size(1, 1).Bold(false)
	b.Align(Left)
	b.Columns(fmt.Sprintf("Comanda #%d", c.Numero), c.Hora.Format("15:04"))
	b.Columns("Mesero: "+c.Mesero, c.Hora.Format("02/01/2006"))
	if c.Reimpresion && !c.Impreso.IsZero() {
		b.Line("Reimpresa: " + c.Impreso.Format("02/01 15:04"))
	}
	b.Separator('=')

	tiempo := ""
	for _, l := range c.Lineas {
		if l.Tiempo != "" && l.Tiempo != tiempo {
			tiempo = l.Tiempo
			b.Align(Center).Bold(true).Line("-- " + tiempo + " --").Bold(false).Align(Left)
		}
		b.Size(1, 2).Bold(true)
		b.Wrapped(l.Cantidad+" x "+l.Producto, "")
		b.Size(1, 1).Bold(false)
		for _, m := range l.Modificadores {
			b.Wrapped("> "+m, "    ")
		}
		if n := strings.TrimSpace(l.Nota); n != "" {
			b.Bold(true).Wrapped("* "+n, "    ").Bold(false)
		}
	}
	if c.Anulacion && c.Motivo != "" {
		b.Separator('-').Wrapped("Motivo: "+c.Motivo, "")
	}
	b.Separator('=')
	return b.Feed(2).Cut(true).Bytes()
}

// LineaCuenta es un renglón de la pre-cuenta.
type LineaCuenta struct {
	Cantidad string
	Producto string
	Total    money.Money // total de la línea, IVA incluido
}

// PreCuenta es el ticket informativo sin valor tributario (RF-03-09).
type PreCuenta struct {
	Local    string
	Mesa     string
	Mesero   string
	Hora     time.Time
	Lineas   []LineaCuenta
	Subtotal money.Money // base imponible (sin IVA)
	IVA      money.Money
	Propina  money.Money // cero si el local no cobra propina legal
	Total    money.Money
	Personas int // > 1 imprime el total dividido en partes iguales (informativo)
}

// ImprimirPreCuenta arma la pre-cuenta para la estación de caja.
func ImprimirPreCuenta(p Paper, c PreCuenta) []byte {
	b := New(p)
	b.Align(Center).Bold(true).Size(1, 2).Line(c.Local).Size(1, 1)
	b.Line("PRE-CUENTA").Bold(false)
	b.Line("DOCUMENTO SIN VALOR TRIBUTARIO")
	b.Align(Left).Separator('-')
	b.Columns(c.Mesa+" · "+c.Mesero, c.Hora.Format("02/01 15:04"))
	b.Separator('-')
	for _, l := range c.Lineas {
		b.Columns(l.Cantidad+" "+l.Producto, "$"+l.Total.String())
	}
	b.Separator('-')
	b.Columns("Subtotal", "$"+c.Subtotal.String())
	b.Columns("IVA", "$"+c.IVA.String())
	if !c.Propina.IsZero() {
		b.Columns("Servicio 10%", "$"+c.Propina.String())
	}
	b.Bold(true).Size(1, 2).Columns("TOTAL", "$"+c.Total.String()).Size(1, 1).Bold(false)
	if c.Personas > 1 {
		if partes, err := money.SplitEqual(c.Total, c.Personas); err == nil {
			b.Separator('-').Line(fmt.Sprintf("Dividido entre %d personas:", c.Personas))
			for i, parte := range partes {
				b.Columns(fmt.Sprintf("Persona %d", i+1), "$"+parte.String())
			}
		}
	}
	b.Separator('-').Align(Center).Line("DOCUMENTO SIN VALOR TRIBUTARIO")
	return b.Feed(3).Cut(true).Bytes()
}

// ImprimirPrueba es el ticket del botón «Imprimir prueba» (RF-02-02.2): confirma la
// conexión, el ancho, las tildes, los tamaños, el corte y los códigos.
func ImprimirPrueba(p Paper, nombre, conexion string, ahora time.Time) []byte {
	b := New(p)
	b.Align(Center).Size(2, 2).Bold(true).Line("¡Funciona!").Size(1, 1).Bold(false)
	b.Line(nombre).Line(conexion).Line(ahora.Format("02/01/2006 15:04:05"))
	b.Separator('-').Align(Left)
	b.Line(fmt.Sprintf("Papel: %d mm · %d columnas", p, p.Columns()))
	b.Line(strings.Repeat("1234567890", p.Columns()/10+1)[:p.Columns()])
	b.Line("Tildes: áéíóú ÁÉÍÓÚ ñÑ ü ¿? ¡!")
	b.Bold(true).Line("Negrita").Bold(false).Underline(true).Line("Subrayado").Underline(false)
	b.Size(2, 1).Line("Doble ancho").Size(1, 2).Line("Doble alto").Size(1, 1)
	b.Invert(true).Line(" Invertido ").Invert(false)
	b.Align(Center).Barcode128("PRUEBA-001").QR("https://example.com/prueba", 6)
	return b.Feed(2).Cut(true).Bytes()
}
