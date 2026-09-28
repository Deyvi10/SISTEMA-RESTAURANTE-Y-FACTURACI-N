package app

import (
	"context"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/auditoria"
)

// F4-15: cada acción auditada queda encadenada con la anterior, con sus campos (RF-08-06.2)
// y en el outbox para que la nube verifique la cadena.
func TestAuditoriaEncadenada(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	tel := c.emparejar(t, "Tablet de Carlos")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "20"})
	c.pos.req("POST", "/v1/caja/movimientos", map[string]any{"cajaId": c.caja1, "tipo": "GASTO", "monto": "4.50", "motivo": "Hielo para el bar", "idempotencyKey": "gasto-aud-1"})
	orden, _ := c.ordenEnMesa(t, tel, "orden-aud", c.mesa1, plato(c.cerveza, "2"))
	// Anular un plato enviado con la autorización de Luis (supervisor) desde la tablet.
	_, o := tel.req("GET", "/v1/ordenes/"+orden, nil)
	linea := o["orden"].(map[string]any)["lineas"].([]any)[0].(map[string]any)["id"]
	_, au := tel.req("POST", "/v1/autorizaciones", map[string]any{"usuarioId": c.luis, "pin": pinLuis, "accion": "ANULAR_ITEM_ENVIADO", "referencia": orden})
	if st, out := tel.req("POST", "/v1/ordenes/"+orden+"/anular", map[string]any{"lineas": []any{linea}, "motivo": "El cliente cambió de opinión", "sePreparo": false, "autorizacion": au["token"]}); st != 200 {
		t.Fatalf("anular: %d %v", st, out)
	}

	cadena, err := c.a.CadenaAuditoria(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cadena) < 4 || auditoria.Verificar(cadena) != -1 {
		t.Fatalf("cadena: %d registros, rota en %d", len(cadena), auditoria.Verificar(cadena))
	}
	for i, r := range cadena {
		if r.Seq != int64(i+1) || r.Hash == "" {
			t.Fatalf("registro %d: seq %d hash %q", i, r.Seq, r.Hash)
		}
	}
	porAccion := map[string]auditoria.Registro{}
	for _, r := range cadena {
		porAccion[r.Accion] = r
	}
	if g := porAccion["MOVIMIENTO_CAJA"]; g.Monto != "4.50" || g.Motivo != "Hielo para el bar" || g.DispositivoID == nil {
		t.Fatalf("movimiento: %+v", g)
	}
	if a := porAccion["LINEA_ANULADA"]; a.AutorizadoPor == nil || *a.AutorizadoPor != c.luis || a.Motivo != "El cliente cambió de opinión" || a.DispositivoID == nil || *a.DispositivoID == *porAccion["MOVIMIENTO_CAJA"].DispositivoID {
		t.Fatalf("anulación: %+v", a)
	}

	// Cada registro viaja a la nube, en el mismo orden.
	var eventos int
	_ = c.a.Store.Read().QueryRow(`SELECT count(*) FROM outbox WHERE tipo = ?`, EventoAuditoria).Scan(&eventos)
	if eventos != len(cadena) {
		t.Fatalf("eventos de auditoría: %d, registros: %d", eventos, len(cadena))
	}

	// Una alteración (hecha fuera de la base, que no la permite) se detecta.
	cadena[1].Motivo = "otro"
	if i := auditoria.Verificar(cadena); i != 1 {
		t.Fatalf("alteración no detectada: %d", i)
	}
}
