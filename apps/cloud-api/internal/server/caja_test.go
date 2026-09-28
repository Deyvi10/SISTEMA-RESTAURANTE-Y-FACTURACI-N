package server_test

import (
	"context"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/salon"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

func TestConfiguracionDeCaja(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "a@a.ec")

	// El alta deja lista una caja, los métodos de pago con su código SRI y motivos.
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	if len(cajas) != 1 || cajas[0].Nombre != "Caja 1" || cajas[0].EstacionID == nil {
		t.Fatalf("cajas iniciales: %+v", cajas)
	}
	var metodos []caja.MetodoPago
	c.do("GET", "/v1/metodos-pago", nil, 200, &metodos)
	codigos := map[string]string{}
	for _, m := range metodos {
		codigos[m.Nombre] = m.CodigoSRI
		if m.ID.Version() != 7 {
			t.Fatalf("%s sin UUID v7: %s", m.Nombre, m.ID)
		}
	}
	if codigos["Efectivo"] != "01" || codigos["Tarjeta crédito"] != "19" || codigos["Tarjeta débito"] != "16" || codigos["Transferencia"] != "20" {
		t.Fatalf("códigos SRI: %v", codigos)
	}
	var motivos []caja.Motivo
	c.do("GET", "/v1/motivos-descuento", nil, 200, &motivos)
	if len(motivos) != 5 {
		t.Fatalf("motivos: %+v", motivos)
	}

	// Una caja imprime en una estación de tipo Caja, no en la cocina.
	var estaciones []salon.Estacion
	c.do("GET", "/v1/estaciones", nil, 200, &estaciones)
	var cocina ids.ID
	for _, s := range estaciones {
		if s.Tipo == "PRODUCCION" {
			cocina = s.ID
		}
	}
	c.do("POST", "/v1/cajas", map[string]any{"localId": r.LocalID, "nombre": "Barra", "estacionId": cocina}, 422, nil)
	var barra caja.Caja
	c.do("POST", "/v1/cajas", map[string]any{"localId": r.LocalID, "nombre": "Barra", "estacionId": cajas[0].EstacionID}, 201, &barra)
	c.do("POST", "/v1/cajas", map[string]any{"localId": r.LocalID, "nombre": "barra"}, 409, nil)
	c.do("DELETE", "/v1/cajas/"+barra.ID.String(), nil, 204, nil)
	// La última caja del local no se elimina.
	c.do("DELETE", "/v1/cajas/"+cajas[0].ID.String(), nil, 409, nil)

	// El código SRI sale del tipo; el efectivo es único, fijo y siempre abre el cajón.
	var deuna caja.MetodoPago
	c.do("POST", "/v1/metodos-pago", map[string]any{"nombre": "Payphone", "tipo": "BILLETERA", "pideReferencia": true}, 201, &deuna)
	if deuna.CodigoSRI != "20" || deuna.AbreCajon {
		t.Fatalf("billetera: %+v", deuna)
	}
	c.do("POST", "/v1/metodos-pago", map[string]any{"nombre": "Efectivo 2", "tipo": "EFECTIVO"}, 422, nil)
	var efectivo caja.MetodoPago
	for _, m := range metodos {
		if m.Tipo == "EFECTIVO" {
			efectivo = m
		}
	}
	c.do("DELETE", "/v1/metodos-pago/"+efectivo.ID.String(), nil, 409, nil)
	c.do("PUT", "/v1/metodos-pago/"+efectivo.ID.String(), map[string]any{"nombre": "Efectivo", "tipo": "TRANSFERENCIA"}, 409, nil)
	var renombrado caja.MetodoPago
	c.do("PUT", "/v1/metodos-pago/"+efectivo.ID.String(), map[string]any{"nombre": "Contado", "tipo": "EFECTIVO", "abreCajon": false}, 200, &renombrado)
	if renombrado.Nombre != "Contado" || !renombrado.AbreCajon || renombrado.CodigoSRI != "01" {
		t.Fatalf("efectivo renombrado: %+v", renombrado)
	}
	c.do("POST", "/v1/motivos-descuento", map[string]any{"nombre": "Cumpleaños", "tipo": "CORTESIA"}, 201, nil)
	c.do("POST", "/v1/motivos-descuento", map[string]any{"nombre": "x", "tipo": "REGALO"}, 422, nil)

	// Todo viaja al nodo por el feed de cambios.
	var n int
	if err := e.tdb.Admin.QueryRow(context.Background(), `SELECT count(*) FROM sync_cambios WHERE tenant_id = $1 AND tabla IN ('cajas','metodos_pago','motivos_descuento')`, r.TenantID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 12 {
		t.Fatalf("cambios para el nodo: %d", n)
	}
}
