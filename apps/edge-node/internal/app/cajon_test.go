package app

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/secreto"
)

// F4-13: abrir el cajón sin venta exige el permiso ABRIR_CAJON o el PIN de un supervisor,
// un motivo, imprime el comprobante con el pulso y queda auditado.
func TestAbrirCajonSinVenta(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	conceder := func(usuario, permiso string) {
		if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
			_, err := tx.Exec(`INSERT INTO permisos_usuario (tenant_id, usuario_id, permiso, concedido) VALUES ('t', ?, ?, 1)`, usuario, permiso)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	abrir := func(motivo, autorizacion string) (int, map[string]any) {
		return c.pos.req("POST", "/v1/caja/cajon", map[string]any{"cajaId": c.caja1, "motivo": motivo, "autorizacion": autorizacion})
	}
	autorizar := func(usuario, pin string) (int, map[string]any) {
		return c.pos.req("POST", "/v1/autorizaciones", map[string]any{"usuarioId": usuario, "pin": pin, "accion": "ABRIR_CAJON", "referencia": c.caja1})
	}

	// Un cajero sin el permiso necesita a un supervisor; el motivo es obligatorio.
	if st, out := abrir("Cambio de monedas", ""); st != 403 || out["code"] != "REQUIERE_SUPERVISOR" {
		t.Fatalf("sin permiso: %d %v", st, out)
	}
	if st, _ := abrir("x", ""); st != 422 {
		t.Fatalf("motivo corto: %d", st)
	}
	// Quien autoriza también debe tener el permiso: una mesera no puede, ni aunque se lo
	// concedan (en la matriz RBAC solo es configurable para cajeros).
	conceder(c.ana.String(), "ABRIR_CAJON")
	if st, out := autorizar(c.ana.String(), pinAna); st != 403 {
		t.Fatalf("mesera como supervisora: %d %v", st, out)
	}
	// La administradora sí (lo tiene siempre).
	sofia := ids.New()
	if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		var pepper []byte
		if err := tx.QueryRow(`SELECT pin_pepper FROM nodo WHERE id = 1`).Scan(&pepper); err != nil {
			return err
		}
		h, err := secreto.HashPIN(pepper, sofia, "5173")
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO usuarios (id, tenant_id, nombre_mostrar, rol, pin_hash, activo) VALUES (?, 't', 'Sofía A.', 'ADMIN', ?, 1)`, sofia.String(), h)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	st, au := autorizar(sofia.String(), "5173")
	if st != 200 {
		t.Fatalf("autorizar: %d %v", st, au)
	}
	token := au["token"].(string)
	st, out := abrir("  Cambio de   monedas para la barra ", token)
	if st != 200 || out["autorizadoPor"] != "Sofía A." || out["impresora"] == "" {
		t.Fatalf("abrir con autorización: %d %v", st, out)
	}
	// La autorización es de un solo uso.
	if st, out := abrir("Otra vez", token); st != 403 || out["code"] != "AUTORIZACION_INVALIDA" {
		t.Fatalf("reusar la autorización: %d %v", st, out)
	}
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool {
			return strings.HasPrefix(s, "[abrir cajón]") && strings.Contains(s, "APERTURA DE CAJÓN SIN VENTA") && strings.Contains(s, "Autorizó: Sofía A.") && strings.Contains(s, "Cambio de monedas para la barra")
		})
	})
	var detalle string
	if err := c.a.Store.Read().QueryRow(`SELECT detalle FROM auditoria WHERE accion = 'CAJON_ABIERTO_SIN_VENTA'`).Scan(&detalle); err != nil || !strings.Contains(detalle, sofia.String()) {
		t.Fatalf("auditoría: %v %s", err, detalle)
	}
	var eventos int
	_ = c.a.Store.Read().QueryRow(`SELECT count(*) FROM outbox WHERE tipo = ?`, EventoCajonAbierto).Scan(&eventos)
	if eventos != 1 {
		t.Fatalf("eventos: %d", eventos)
	}

	// Con el permiso propio no hace falta supervisor.
	conceder(c.luis.String(), "ABRIR_CAJON")
	c.pos.entrar(c.luis, pinLuis) // la sesión vuelve a leer sus permisos
	if st, out := abrir("Guardar un vale", ""); st != 200 || out["autorizadoPor"] != nil {
		t.Fatalf("con permiso: %d %v", st, out)
	}
}
