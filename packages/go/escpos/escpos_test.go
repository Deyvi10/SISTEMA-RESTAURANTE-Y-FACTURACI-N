package escpos

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/cierrez"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"pgregory.net/rapid"
)

var update = flag.Bool("update", false, "reescribe los archivos golden")

var hora = time.Date(2026, 9, 24, 21, 15, 0, 0, time.FixedZone("ECT", -5*3600))

func golden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("falta %s: ejecuta go test ./packages/go/escpos -update", path)
	}
	if got != string(want) {
		t.Errorf("%s cambió.\n--- obtenido ---\n%s\n--- esperado ---\n%s", name, got, want)
	}
}

func TestRoundTripStylesAndAccents(t *testing.T) {
	raw := New(Paper80).
		Bold(true).Text("Ñandú ").Bold(false).Line("con ají").
		Align(Center).Size(2, 2).Line("Mesa 4").
		Size(1, 1).Align(Right).Underline(true).Line("¿Listo?").
		Cut(false).OpenDrawer().Bytes()

	doc := Decode(raw)
	if len(doc.Warnings) > 0 {
		t.Fatalf("advertencias: %v", doc.Warnings)
	}
	if doc.Consumed != len(raw) {
		t.Fatalf("consumió %d de %d bytes", doc.Consumed, len(raw))
	}
	e := doc.Elements
	if len(e) != 5 {
		t.Fatalf("elementos: %+v", e)
	}
	if e[0].PlainText() != "Ñandú con ají" || !e[0].Runs[0].Style.Bold || e[0].Runs[1].Style.Bold {
		t.Errorf("línea 1: %+v", e[0])
	}
	if e[1].Align != Center || e[1].Runs[0].Style.W != 2 || e[1].Runs[0].Style.H != 2 {
		t.Errorf("línea 2: %+v", e[1])
	}
	if e[2].Align != Right || !e[2].Runs[0].Style.Underline || e[2].PlainText() != "¿Listo?" {
		t.Errorf("línea 3: %+v", e[2])
	}
	if e[3].Kind != KindCut || e[4].Kind != KindDrawer {
		t.Errorf("eventos: %v %v", e[3].Kind, e[4].Kind)
	}
}

func TestCodes(t *testing.T) {
	clave := "2110201101179214673900110020010000000011234567813"
	doc := Decode(New(Paper80).Barcode128("001-002-000000123").QR(clave, 4).Bytes())
	if doc.Elements[0].Kind != KindBarcode || doc.Elements[0].Data != "001-002-000000123" {
		t.Errorf("barcode: %+v", doc.Elements[0])
	}
	if doc.Elements[1].Kind != KindQR || doc.Elements[1].Data != clave {
		t.Errorf("qr: %+v", doc.Elements[1])
	}
}

func TestIncompleteCommandIsNotConsumed(t *testing.T) {
	full := New(Paper80).Line("Hola").QR("abc", 4).Bytes()
	for cut := 1; cut < len(full); cut++ {
		doc := Decode(full[:cut])
		if doc.Consumed > cut {
			t.Fatalf("consumió más de lo recibido")
		}
		// Completar el flujo siempre da el mismo documento que recibirlo entero.
		if cut == len(full)-1 {
			if Decode(full).Text() != Decode(append(full[:cut:cut], full[cut:]...)).Text() {
				t.Fatal("reensamblado distinto")
			}
		}
	}
}

func TestStatusQueries(t *testing.T) {
	stream := append(StatusRequest(StatusPrinter), StatusRequest(StatusPaper)...)
	doc := Decode(stream)
	if !bytes.Equal(doc.StatusQueries, []byte{1, 4}) {
		t.Fatalf("consultas: %v", doc.StatusQueries)
	}

	cases := []Status{{}, {PaperOut: true}, {CoverOpen: true}, {PaperNearEnd: true}, {Offline: true}, {Error: true}}
	for _, want := range cases {
		var got Status
		for _, n := range []byte{StatusPrinter, StatusOffline, StatusError, StatusPaper} {
			b := StatusByte(n, want)
			if !got.ApplyStatus(n, b) {
				t.Fatalf("byte 0x%02x inválido", b)
			}
		}
		// «Fuera de línea» se deduce también de tapa abierta o sin papel.
		want.Offline = want.Offline || want.CoverOpen || want.PaperOut || want.Error
		if got != want {
			t.Errorf("estado %+v → %+v", want, got)
		}
		if got.OK() == (want.CoverOpen || want.PaperOut || want.Error || want.Offline) {
			t.Errorf("OK() incorrecto para %+v", want)
		}
		if !want.OK() && got.Motivo() == "" {
			t.Errorf("sin motivo para %+v", want)
		}
	}
	var s Status
	if s.ApplyStatus(StatusPaper, 0xFF) {
		t.Error("0xFF no es un byte de estado válido")
	}
}

func TestColumnsNeverOverflow(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		p := rapid.SampledFrom([]Paper{Paper58, Paper80}).Draw(t, "papel")
		left := rapid.StringMatching(`[A-Za-zñáé ]{0,120}`).Draw(t, "izq")
		right := "$" + rapid.StringMatching(`[0-9]{1,6}\.[0-9]{2}`).Draw(t, "der")
		doc := Decode(New(p).Columns(left, right).Bytes())
		for _, e := range doc.Elements {
			if n := utf8.RuneCountInString(e.PlainText()); n > p.Columns() {
				t.Fatalf("línea de %d columnas en papel de %d: %q", n, p.Columns(), e.PlainText())
			}
		}
		last := doc.Elements[len(doc.Elements)-1].PlainText()
		if !strings.HasSuffix(last, right) {
			t.Fatalf("el importe no quedó a la derecha: %q", last)
		}
	})
}

func TestWrap(t *testing.T) {
	got := Wrap("Hamburguesa doble con queso cheddar", 12)
	want := []string{"Hamburguesa", "doble con", "queso", "cheddar"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Wrap = %q", got)
	}
	if got := Wrap("Supercalifragilístico", 8); got[0] != "Supercal" {
		t.Errorf("palabra larga: %q", got)
	}
}

func comandaEjemplo() Comanda {
	return Comanda{
		Estacion: "Cocina caliente", Mesa: "Mesa 4", Mesero: "Carlos M.", Numero: 27, Hora: hora,
		Lineas: []LineaComanda{
			{Cantidad: "1", Producto: "Ceviche mixto", Tiempo: "ENTRADA", Nota: "Sin cebolla"},
			{Cantidad: "1", Producto: "Hamburguesa doble con queso cheddar y tocino", Tiempo: "FUERTE", Modificadores: []string{"Término medio", "Extra queso"}},
			{Cantidad: "1", Producto: "Hamburguesa doble con queso cheddar y tocino", Tiempo: "FUERTE", Modificadores: []string{"Bien cocida"}, Nota: "Alérgico al maní"},
		},
	}
}

func TestGoldenComanda(t *testing.T) {
	for _, p := range []Paper{Paper80, Paper58} {
		doc := Decode(ImprimirComanda(p, comandaEjemplo()))
		if len(doc.Warnings) > 0 {
			t.Fatal(doc.Warnings)
		}
		golden(t, "comanda-"+string(rune('0'+p/10))+string(rune('0'+p%10)), doc.Text())
	}
}

func TestGoldenAnulacionYReimpresion(t *testing.T) {
	c := comandaEjemplo()
	c.Lineas = c.Lineas[:1]
	c.Anulacion, c.Motivo = true, "Cliente cambió de plato"
	golden(t, "anulacion-80", Decode(ImprimirComanda(Paper80, c)).Text())

	c = comandaEjemplo()
	c.Reimpresion, c.Impreso = true, hora.Add(12*time.Minute)
	golden(t, "reimpresion-80", Decode(ImprimirComanda(Paper80, c)).Text())
}

func TestGoldenPreCuenta(t *testing.T) {
	pc := PreCuenta{
		Local: "Cevichería Don Pepe", Mesa: "Mesa 4", Mesero: "Carlos M.", Hora: hora,
		Lineas: []LineaCuenta{
			{Cantidad: "1", Producto: "Ceviche mixto", Total: money.MustParse("15.00")},
			{Cantidad: "2", Producto: "Cerveza", Total: money.MustParse("5.00")},
		},
		Subtotal: money.MustParse("17.39"), IVA: money.MustParse("2.61"), Propina: money.MustParse("1.74"),
		Total: money.MustParse("21.74"), Personas: 3,
	}
	golden(t, "precuenta-80", Decode(ImprimirPreCuenta(Paper80, pc)).Text())
}

func TestGoldenPrueba(t *testing.T) {
	doc := Decode(ImprimirPrueba(Paper80, "Cocina caliente", "TCP 192.168.1.101:9100", hora))
	if len(doc.Warnings) > 0 {
		t.Fatal(doc.Warnings)
	}
	golden(t, "prueba-80", doc.Text())
}

func TestBarcodeTooLongIsTruncatedNotCorrupted(t *testing.T) {
	long := strings.Repeat("9", 400)
	doc := Decode(New(Paper80).Barcode128(long).Line("después").Bytes())
	if len(doc.Warnings) > 0 || doc.Elements[0].Kind != KindBarcode {
		t.Fatalf("documento corrupto: %+v", doc)
	}
	if got := len(doc.Elements[0].Data); got != MaxBarcodeLen {
		t.Fatalf("largo %d, se esperaba %d", got, MaxBarcodeLen)
	}
	if doc.Elements[1].PlainText() != "después" {
		t.Fatal("el texto posterior al código se perdió")
	}
}

func TestQRTooLongFallsBackToText(t *testing.T) {
	doc := Decode(New(Paper80).QR(strings.Repeat("A", MaxQRLen+1), 4).Bytes())
	for _, e := range doc.Elements {
		if e.Kind == KindQR {
			t.Fatal("no debe enviarse un QR que no cabe")
		}
	}
	if !strings.Contains(doc.Text(), "AAAA") || len(doc.Warnings) > 0 {
		t.Fatalf("se esperaba el texto sin advertencias: %v", doc.Warnings)
	}
}

func TestGoldenCierreZ(t *testing.T) {
	efectivo, tarjeta := ids.New(), ids.New()
	c := cierrez.Cierre{
		Local: "Cevichería Don Pepe", Caja: "Caja 1", Numero: 7, Cajero: "Luis P.", CerradoPor: "Pepe Andrade", FechaNegocio: "2026-09-24",
		AbiertoAt: hora.Add(-5 * time.Hour), CerradoAt: hora,
		Movimientos: cierrez.Movimientos{FondoInicial: money.MustParse("50"), Ingresos: money.MustParse("10"), Retiros: money.MustParse("100"), Gastos: money.MustParse("3.50")},
		Conteo:      []cierrez.Conteo{{Clave: "B20", Cantidad: 3}, {Clave: "B10", Cantidad: 1}, {Clave: "B5", Cantidad: 1}, {Clave: "M1", Cantidad: 1}, {Clave: "M0.25", Cantidad: 3}, {Clave: "B100", Cantidad: 0}},
		Hash:        "9f2c4e1a7b3d5f60aa11bb22cc33dd44ee55ff6677889900aabbccddeeff0011", HashAnterior: "0011aabbccddeeff00112233445566778899aabbccddeeff0011223344556677",
	}
	c.EfectivoTotal, _ = cierrez.TotalConteo(c.Conteo)
	c.Lineas, c.Resultado = cierrez.Calcular([]cierrez.Metodo{{ID: efectivo, Nombre: "Efectivo", Tipo: "EFECTIVO"}, {ID: tarjeta, Nombre: "Tarjeta crédito", Tipo: "TARJETA_CREDITO"}},
		c.Movimientos, map[ids.ID]money.Money{efectivo: money.MustParse("120.40"), tarjeta: money.MustParse("35.40")}, c.EfectivoTotal, map[ids.ID]money.Money{tarjeta: money.MustParse("30.40")})
	doc := Decode(ImprimirCierreZ(Paper80, c, time.UTC))
	if len(doc.Warnings) > 0 {
		t.Fatal(doc.Warnings)
	}
	golden(t, "cierre-z-80", doc.Text())
	golden(t, "cierre-z-58", Decode(ImprimirCierreZ(Paper58, c, time.UTC)).Text())
}

func TestGoldenDocumentoVenta(t *testing.T) {
	d := DocumentoVenta{
		Local: "Cevichería Don Pepe", Numero: 123, Hora: hora, Mesa: "Mesa 4", Cajero: "Luis P.", Comprador: "CONSUMIDOR FINAL",
		Lineas:   []LineaCuenta{{Cantidad: "1", Producto: "Ceviche mixto", Total: money.MustParse("15.00")}, {Cantidad: "2", Producto: "Cerveza", Total: money.MustParse("5.00")}},
		Subtotal: money.MustParse("17.39"), IVA: money.MustParse("2.61"), Propina: money.MustParse("1.74"), Total: money.MustParse("21.74"),
		Pagos:    []PagoTicket{{Metodo: "Efectivo", Monto: money.MustParse("21.74")}},
		Recibido: money.MustParse("50"), Vuelto: money.MustParse("28.26"), AbrirCajon: true,
	}
	raw := ImprimirDocumentoVenta(Paper80, d)
	if !bytes.HasPrefix(raw, New(Paper80).OpenDrawer().Bytes()) {
		t.Fatal("el pulso del cajón debe ir antes que el texto")
	}
	golden(t, "venta-80", Decode(raw).Text())
	d.AbrirCajon, d.Recibido, d.Vuelto = false, money.Money{}, money.Money{}
	d.Pagos = []PagoTicket{{Metodo: "Tarjeta crédito", Monto: money.MustParse("21.74")}}
	golden(t, "venta-tarjeta-58", Decode(ImprimirDocumentoVenta(Paper58, d)).Text())
	// Pago mixto: $10 en efectivo (entregó $20) y el resto con tarjeta.
	d.Pagos = []PagoTicket{{Metodo: "Efectivo", Monto: money.MustParse("10")}, {Metodo: "Tarjeta débito", Monto: money.MustParse("11.74"), Ultimos4: "4821"}}
	d.Recibido, d.Vuelto, d.AbrirCajon = money.MustParse("20"), money.MustParse("10"), true
	golden(t, "venta-mixta-80", Decode(ImprimirDocumentoVenta(Paper80, d)).Text())
}
