package archivo

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// muestra imita un XML autorizado de restaurante: misma estructura, datos que cambian.
func muestra(r *rand.Rand, i int) []byte {
	platos := []string{"Ceviche de camarón", "Encebollado", "Bolón de verde", "Cerveza Club", "Arroz con menestra y carne", "Jugo de naranjilla"}
	s := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<autorizacion><estado>AUTORIZADO</estado><numeroAutorizacion>2909202601179001167400110010010%09d12345678%d</numeroAutorizacion>
<fechaAutorizacion>2026-09-29T13:%02d:%02d-05:00</fechaAutorizacion><ambiente>PRODUCCIÓN</ambiente><comprobante><![CDATA[<factura id="comprobante" version="1.1.0">
<infoTributaria><ambiente>2</ambiente><tipoEmision>1</tipoEmision><razonSocial>DISTRIBUIDORA DEL PACIFICO S.A.</razonSocial><nombreComercial>Cevichería Don Pepe</nombreComercial><ruc>1790011674001</ruc><claveAcceso>2909202601179001167400110010010%09d123456781</claveAcceso><codDoc>01</codDoc><estab>001</estab><ptoEmi>001</ptoEmi><secuencial>%09d</secuencial><dirMatriz>Malecón 2000 y Av. 9 de Octubre, Guayaquil</dirMatriz></infoTributaria>
<infoFactura><fechaEmision>29/09/2026</fechaEmision><obligadoContabilidad>SI</obligadoContabilidad><tipoIdentificacionComprador>07</tipoIdentificacionComprador><razonSocialComprador>CONSUMIDOR FINAL</razonSocialComprador><identificacionComprador>9999999999999</identificacionComprador><totalSinImpuestos>%d.%02d</totalSinImpuestos><totalDescuento>0.00</totalDescuento>
<totalConImpuestos><totalImpuesto><codigo>2</codigo><codigoPorcentaje>4</codigoPorcentaje><baseImponible>%d.%02d</baseImponible><tarifa>15.00</tarifa><valor>%d.%02d</valor></totalImpuesto></totalConImpuestos><propina>0.00</propina><importeTotal>%d.%02d</importeTotal><moneda>DOLAR</moneda><pagos><pago><formaPago>01</formaPago><total>%d.00</total></pago></pagos></infoFactura><detalles>`,
		i, i%10, r.IntN(60), r.IntN(60), i, i, r.IntN(90)+5, r.IntN(100), r.IntN(90)+5, r.IntN(100), r.IntN(20), r.IntN(100), r.IntN(90)+5, r.IntN(100), r.IntN(90)+5)
	for k := range r.IntN(5) + 1 {
		p := platos[r.IntN(len(platos))]
		s += fmt.Sprintf(`<detalle><codigoPrincipal>P%03d</codigoPrincipal><descripcion>%s</descripcion><cantidad>%d.000000</cantidad><precioUnitario>%d.%06d</precioUnitario><descuento>0.00</descuento><precioTotalSinImpuesto>%d.%02d</precioTotalSinImpuesto><impuestos><impuesto><codigo>2</codigo><codigoPorcentaje>4</codigoPorcentaje><tarifa>15.00</tarifa><baseImponible>%d.%02d</baseImponible><valor>%d.%02d</valor></impuesto></impuestos></detalle>`,
			k, p, r.IntN(3)+1, r.IntN(20), r.IntN(999999), r.IntN(40), r.IntN(100), r.IntN(40), r.IntN(100), r.IntN(6), r.IntN(100))
	}
	s += `</detalles><infoAdicional><campoAdicional nombre="Mesa">Mesa ` + fmt.Sprint(r.IntN(30)+1) + `</campoAdicional><campoAdicional nombre="RUC Proveedor">1790011674001</campoAdicional></infoAdicional>`
	s += `<ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#" Id="Signature` + fmt.Sprint(r.IntN(1e6)) + `"><ds:SignedInfo><ds:CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/><ds:SignatureMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1"/></ds:SignedInfo><ds:SignatureValue>`
	for range 4 {
		s += fmt.Sprintf("%016x", r.Uint64())
	}
	return []byte(s + `</ds:SignatureValue></ds:Signature></factura>]]></comprobante></autorizacion>`)
}

// El diccionario del emisor funciona y comprime mejor que zstd solo (docs/13 §1).
func TestDiccionarioDelEmisor(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 11))
	var entreno, prueba [][]byte
	for i := range MuestrasDiccionario {
		entreno = append(entreno, muestra(r, i))
	}
	for i := range 300 {
		prueba = append(prueba, muestra(r, 5000+i))
	}
	dic, err := Entrenar(entreno)
	if err != nil {
		t.Fatal(err)
	}
	if len(dic) > TamanoDiccionario+8<<10 || idDiccionario(dic) < 32768 {
		t.Fatalf("diccionario de %d bytes, id %d", len(dic), idDiccionario(dic))
	}
	sin, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	con, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression), zstd.WithEncoderDict(dic))
	dec, _ := zstd.NewReader(nil, zstd.WithDecoderDicts(dic))
	var original, tamSin, tamCon int
	for _, m := range prueba {
		original += len(m)
		tamSin += len(sin.EncodeAll(m, nil))
		f := con.EncodeAll(m, nil)
		tamCon += len(f)
		back, err := dec.DecodeAll(f, nil)
		if err != nil || string(back) != string(m) {
			t.Fatal("ida y vuelta con diccionario")
		}
	}
	t.Logf("300 facturas: %d bytes; zstd %d (%.1fx); con diccionario %d (%.1fx)", original, tamSin, float64(original)/float64(tamSin), tamCon, float64(original)/float64(tamCon))
	if tamCon*10 > tamSin*7 {
		t.Fatalf("el diccionario debía ahorrar al menos 30 %%: %d vs %d", tamCon, tamSin)
	}
}
