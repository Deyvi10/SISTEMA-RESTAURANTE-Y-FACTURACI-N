package sri

import (
	"strings"
	"testing"
)

// Lo que se escribe en el XML se lee igual (el RIDE se arma desde el comprobante).
func TestLeerFacturaIdaYVuelta(t *testing.T) {
	f, err := Desglosar(totalesComoElNodo([]LineaVenta{
		linea("Parrillada para dos", "1", "24.00", "21.60", iva15),
		linea("Agua sin gas", "2", "2.00", "2.00", iva0),
	}, true, true))
	if err != nil {
		t.Fatal(err)
	}
	comprador := Comprador{TipoIdentificacion: "04", Identificacion: "1790011674001", RazonSocial: "Distribuidora del Pacífico S.A.", Direccion: "Quito"}
	datos := datosPrueba(t, f, 67, comprador)
	doc, err := FacturaXML(datos, f)
	if err != nil {
		t.Fatal(err)
	}
	l, err := LeerFactura(doc)
	if err != nil {
		t.Fatal(err)
	}
	if l.Numero() != "001-002-000000067" || l.ClaveAcceso != datos.ClaveAcceso.String() || l.RazonSocialComprador != comprador.RazonSocial ||
		l.ImporteTotal != f.ImporteTotal.String() || l.Propina != f.Propina.String() || l.FechaEmision != "29/09/2026" {
		t.Fatalf("leída: %+v", l)
	}
	if len(l.Detalles) != 2 || l.Detalles[0].Descripcion != "Parrillada para dos" || len(l.Detalles[0].Impuestos) != 1 || l.Detalles[0].Impuestos[0].Tarifa != "15.00" {
		t.Fatalf("detalles: %+v", l.Detalles)
	}
	if len(l.Impuestos) != 2 || len(l.Pagos) != 1 || l.Adicional("mesa") != "Mesa 4" {
		t.Fatalf("totales, pagos o adicionales: %+v %+v %+v", l.Impuestos, l.Pagos, l.Adicionales)
	}
	if _, err := LeerFactura([]byte(strings.Replace(string(doc), "<codDoc>01</codDoc>", "<codDoc>04</codDoc>", 1))); err == nil {
		t.Fatal("una nota de crédito no es una factura")
	}
	if _, err := LeerFactura([]byte("<x")); err == nil {
		t.Fatal("XML roto")
	}
}
