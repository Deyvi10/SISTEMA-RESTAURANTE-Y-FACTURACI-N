package app

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// F4-04: venta directa en mostrador desde la caja (Para llevar y Barra; sin delivery por ahora).
func TestVentaEnMostrador(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "0"})
	orden := ids.New()
	enviar := func(clave string, body map[string]any) (int, map[string]any) {
		b := map[string]any{"idempotencyKey": clave, "ordenId": orden, "tipo": "LLEVAR", "etiqueta": "  Ana   María ", "lineas": []any{plato(c.ceviche, "1")}}
		for k, v := range body {
			b[k] = v
		}
		return c.pos.req("POST", "/v1/ordenes/enviar", b)
	}

	// Validaciones: sin mesa, con su id, un tipo real y un nombre corto.
	for nombre, body := range map[string]map[string]any{
		"con mesa":         {"mesaId": c.mesa1},
		"sin id de orden":  {"ordenId": nil},
		"tipo desconocido": {"tipo": "DRIVE"},
		"delivery":         {"tipo": "DELIVERY"}, // pospuesto: no se acepta
		"nombre muy largo": {"etiqueta": "Una persona con un nombre larguísimo"},
		"mesa sin mesaId":  {"tipo": "MESA"},
	} {
		if st, out := enviar("malo-"+strings.ReplaceAll(nombre, " ", "-"), body); st != 422 {
			t.Fatalf("%s: %d %v", nombre, st, out)
		}
	}

	st, out := enviar("llevar-0001", nil)
	if st != 200 {
		t.Fatalf("enviar: %d %v", st, out)
	}
	o := out["orden"].(map[string]any)
	if o["tipo"] != "LLEVAR" || o["mesaId"] != nil || o["etiqueta"] != "Ana María" || o["mesa"] != "Llevar #1 · Ana María" {
		t.Fatalf("orden: %v", o)
	}
	// La comanda sale en cocina con el nombre corto.
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool { return strings.Contains(s, "Llevar #1 · Ana María") })
	})
	// Más platos a la misma orden; otro tipo sobre la misma orden no.
	if st, out := enviar("llevar-0002", map[string]any{"lineas": []any{plato(c.cerveza, "2")}}); st != 200 || len(out["orden"].(map[string]any)["lineas"].([]any)) != 2 {
		t.Fatalf("agregar: %d %v", st, out)
	}
	if st, _ := enviar("llevar-0003", map[string]any{"tipo": "BARRA"}); st != 422 {
		t.Fatalf("cambiar el tipo: %d", st)
	}

	// La caja ve las órdenes sin mesa; las de mesa siguen en el salón.
	barra := ids.New()
	c.pos.req("POST", "/v1/ordenes/enviar", map[string]any{"idempotencyKey": "barra-0001", "ordenId": barra, "tipo": "BARRA", "lineas": []any{plato(c.cerveza, "1")}})
	var lista []map[string]any
	st, _ = c.pos.reqLista("GET", "/v1/ordenes/sin-mesa", &lista)
	if st != 200 || len(lista) != 2 || lista[0]["nombre"] != "Llevar #1 · Ana María" || lista[0]["platos"].(float64) != 2 || lista[1]["nombre"] != "Barra #2" {
		t.Fatalf("sin mesa: %d %v", st, lista)
	}

	// Se cobra como cualquier orden; el documento lleva su nombre y deja de estar abierta.
	st, cob, raw := c.cobrar(orden.String(), c.efectivo.String(), "", "cobro-llevar")
	if st != 200 || cob.Documento.Mesa != "Llevar #1 · Ana María" {
		t.Fatalf("cobro: %d %v", st, raw)
	}
	if st, out := enviar("llevar-0004", nil); st != 409 || out["code"] != "ORDEN_CERRADA" {
		t.Fatalf("enviar a una orden cobrada: %d %v", st, out)
	}
	c.pos.reqLista("GET", "/v1/ordenes/sin-mesa", &lista)
	if len(lista) != 1 || lista[0]["tipo"] != "BARRA" {
		t.Fatalf("tras cobrar: %v", lista)
	}
}

// reqLista es req para respuestas que son una lista JSON.
func (p *telefono) reqLista(method, path string, out any) (int, error) {
	p.t.Helper()
	st, raw := p.req(method, path, nil)
	s, _ := raw["_raw"].(string)
	return st, json.Unmarshal([]byte(s), out)
}
