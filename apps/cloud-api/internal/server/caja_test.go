package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/salon"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/cierrez"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
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

// cierreZ simula que el nodo cerró un turno de la caja y envía su Cierre Z con la diferencia
// indicada en efectivo.
func (n *nodoSim) cierreZ(cajaID ids.ID, numero int, hashAnterior, diferencia string) cierrez.Cierre {
	n.e.t.Helper()
	efectivo := ids.New()
	c := cierrez.Cierre{ID: ids.New(), TurnoID: ids.New(), CajaID: cajaID, JornadaID: ids.New(), Caja: "Caja 1", Local: "Local", Numero: numero,
		Cajero: "Luis P.", CerradoPor: "Luis P.", FechaNegocio: "2026-09-25", AbiertoAt: time.Date(2026, 9, 25, 23, 0, 0, 0, time.UTC),
		CerradoAt: time.Date(2026, 9, 26, 4, 0, 0, 0, time.UTC), HashAnterior: hashAnterior,
		Movimientos: cierrez.Movimientos{FondoInicial: money.MustParse("50")}}
	dif := money.MustParse(diferencia)
	c.Conteo = []cierrez.Conteo{{Clave: "B50", Cantidad: 1}}
	c.EfectivoTotal = money.MustParse("50")
	c.Lineas, c.Resultado = cierrez.Calcular([]cierrez.Metodo{{ID: efectivo, Nombre: "Efectivo", Tipo: "EFECTIVO"}}, c.Movimientos,
		map[ids.ID]money.Money{efectivo: dif.Neg()}, c.EfectivoTotal, nil)
	c.Hash = c.CalcularHash()
	ev := n.evento(caja.EventoCierreZ)
	ev.Payload, _ = json.Marshal(c)
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: []edgesync.Event{ev}}, true, 200, nil)
	return c
}

func (e *env) notificarCierres() int {
	e.t.Helper()
	enviados, err := (&caja.Notificador{DB: e.tdb.App, Mail: e.mail, Log: slog.Default()}).EnviarPendientes(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	return enviados
}

func TestCierreZLlegaYSeEnviaAlDueno(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "a@a.ec")
	n := c.e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	ctx := context.Background()
	e.mail.Sent = nil

	// Cuadrado: correo normal con el PDF adjunto.
	z1 := n.cierreZ(cajas[0].ID, 1, "", "0")
	if got := e.notificarCierres(); got != 1 {
		t.Fatalf("enviados: %d", got)
	}
	m, _ := e.mail.Last()
	if m.To != "a@a.ec" || !strings.Contains(m.Subject, "Cierre Z 0001") || strings.Contains(m.Subject, "Alerta") || len(m.Attachments) != 1 {
		t.Fatalf("correo: %s → %q (%d adjuntos)", m.To, m.Subject, len(m.Attachments))
	}
	if a := m.Attachments[0]; a.ContentType != "application/pdf" || !bytes.HasPrefix(a.Data, []byte("%PDF-")) || !bytes.Contains(a.Data, []byte("CIERRE Z 0001")) {
		t.Fatalf("adjunto: %s %d bytes", a.Name, len(a.Data))
	}
	// Un segundo barrido no reenvía.
	if got := e.notificarCierres(); got != 0 {
		t.Fatalf("reenvío: %d", got)
	}

	// El umbral se configura en Ajustes; una diferencia mayor es una alerta crítica.
	c.do("PATCH", "/v1/locales/"+r.LocalID.String(), map[string]any{"umbralAlertaCierre": "2.005"}, 422, nil)
	var local salon.Local
	c.do("PATCH", "/v1/locales/"+r.LocalID.String(), map[string]any{"umbralAlertaCierre": "2"}, 200, &local)
	if local.UmbralAlertaCierre != "2.00" {
		t.Fatalf("umbral: %+v", local)
	}
	n.cierreZ(cajas[0].ID, 2, z1.Hash, "-1.50") // faltante bajo el umbral: aviso normal
	z3 := n.cierreZ(cajas[0].ID, 3, "", "-7.25")
	e.notificarCierres()
	var asuntos []string
	for _, s := range e.mail.Sent {
		asuntos = append(asuntos, s.Subject)
	}
	if len(asuntos) != 3 || strings.Contains(asuntos[1], "Alerta") || !strings.Contains(asuntos[2], "Alerta: faltante de $7.25") {
		t.Fatalf("asuntos: %q", asuntos)
	}

	// Guardado inmutable, con el hash verificado y la alerta registrada.
	var valido, alerta bool
	var numero int
	if err := e.tdb.Admin.QueryRow(ctx, `SELECT c.numero, c.hash_valido, e.alerta FROM cierres_z c JOIN cierres_z_envios e ON e.cierre_id = c.id WHERE c.id = $1`, z3.ID).Scan(&numero, &valido, &alerta); err != nil {
		t.Fatal(err)
	}
	if numero != 3 || !valido || !alerta {
		t.Fatalf("guardado: %d %v %v", numero, valido, alerta)
	}
	if err := e.tdb.App.InTenant(ctx, r.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE cierres_z SET resultado = 'CUADRADO'`)
		return err
	}); err == nil {
		t.Fatal("la aplicación pudo modificar un Cierre Z")
	}

	// El mismo evento repetido no duplica nada; uno alterado queda marcado.
	ev := n.evento(caja.EventoCierreZ)
	falso := z1
	falso.ID, falso.TurnoID, falso.Numero, falso.Cajero = ids.New(), ids.New(), 4, "Otro"
	ev.Payload, _ = json.Marshal(falso)
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: []edgesync.Event{ev}}, true, 200, nil)
	if err := e.tdb.Admin.QueryRow(ctx, `SELECT hash_valido FROM cierres_z WHERE id = $1`, falso.ID).Scan(&valido); err != nil || valido {
		t.Fatalf("hash alterado: %v %v", err, valido)
	}
	e.notificarCierres()
	if m, _ := e.mail.Last(); !strings.Contains(m.Text, "no coincide") {
		t.Fatalf("aviso de integridad: %q", m.Text)
	}
}
