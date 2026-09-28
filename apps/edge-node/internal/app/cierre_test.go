package app

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/cierrez"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// pago simula un cobro de F4-05 en el turno abierto.
func (c *cajaF4) pago(t *testing.T, turno, metodo, monto string) {
	t.Helper()
	if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT INTO pagos (id, orden_id, turno_id, metodo_pago_id, monto, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			ids.New().String(), ids.New().String(), turno, metodo, monto, time.Now().UTC().Format(time.RFC3339Nano))
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func (c *cajaF4) cerrar(clave string, conteo []cierrez.Conteo, declarado ...map[string]any) (int, CierreOut, map[string]any) {
	st, raw := c.pos.req("POST", "/v1/turnos/cerrar", map[string]any{"cajaId": c.caja1.String(), "conteo": conteo, "declarado": declarado, "idempotencyKey": clave})
	var out CierreOut
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &out)
	return st, out, raw
}

func TestCierreZCiego(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	caja := c.caja1.String()
	var tarjeta string
	if err := c.a.Store.Read().QueryRow(`SELECT id FROM metodos_pago WHERE tipo = 'TARJETA_CREDITO'`).Scan(&tarjeta); err != nil {
		t.Fatal(err)
	}

	// Sin turno no hay cierre.
	if st, _, raw := c.cerrar("cierre-sin-turno", nil); st != 409 || raw["code"] != "SIN_TURNO" {
		t.Fatalf("cierre sin turno: %d %v", st, raw)
	}
	_, turno := c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": caja, "fondoInicial": "50"})
	tid := turno["id"].(string)
	for _, m := range []struct{ clave, tipo, monto string }{{"r-000001", "RETIRO", "20"}, {"i-000001", "INGRESO", "10"}, {"g-000001", "GASTO", "4.50"}} {
		if st, out := c.pos.req("POST", "/v1/caja/movimientos", map[string]any{"cajaId": caja, "tipo": m.tipo, "monto": m.monto, "motivo": "Movimiento de prueba", "idempotencyKey": m.clave}); st != 200 {
			t.Fatalf("movimiento: %d %v", st, out)
		}
	}
	c.pago(t, tid, c.efectivo.String(), "30.00")
	c.pago(t, tid, tarjeta, "10.40")
	c.pago(t, tid, tarjeta, "5.00")

	// Lo declarado se valida: denominaciones reales, sin efectivo en los vouchers.
	if st, _, _ := c.cerrar("cierre-malo-1", []cierrez.Conteo{{Clave: "B3", Cantidad: 1}}); st != 422 {
		t.Fatalf("billete de $3: %d", st)
	}
	if st, _, _ := c.cerrar("cierre-malo-2", nil, map[string]any{"metodoId": c.efectivo, "monto": "5"}); st != 422 {
		t.Fatalf("efectivo como voucher: %d", st)
	}

	// Esperado en efectivo: 50 + 30 + 10 − 20 − 4.50 = 65.50. Se cuentan 3×$20 + $5 + 2×25¢.
	conteo := []cierrez.Conteo{{Clave: "B20", Cantidad: 3}, {Clave: "B5", Cantidad: 1}, {Clave: "M0.25", Cantidad: 2}, {Clave: "B100", Cantidad: 0}}
	st, out, raw := c.cerrar("cierre-000001", conteo, map[string]any{"metodoId": tarjeta, "monto": "15.00"})
	if st != 200 {
		t.Fatalf("cierre: %d %v", st, raw)
	}
	z := out.Cierre
	if z.Numero != 1 || z.Resultado != cierrez.Faltante || z.HashAnterior != "" || !z.Verificar() || z.Cajero != "Luis P." || z.FechaNegocio != "2026-09-25" {
		t.Fatalf("cierre: %+v", z)
	}
	res := map[string]string{}
	for _, l := range z.Lineas {
		res[l.Tipo] = l.Esperado.String() + "/" + l.Declarado.String() + "/" + l.Resultado
	}
	if res["EFECTIVO"] != "65.50/65.50/CUADRADO" || res["TARJETA_CREDITO"] != "15.40/15.00/FALTANTE" {
		t.Fatalf("resultado por método: %v", res)
	}
	// El reintento (se cortó la red) devuelve el mismo cierre.
	if st, again, _ := c.cerrar("cierre-000001", conteo); st != 200 || again.Cierre.ID != z.ID {
		t.Fatalf("reintento: %d %+v", st, again.Cierre)
	}

	// La caja queda bloqueada hasta abrir un turno nuevo.
	if _, est := c.pos.req("GET", "/v1/cajas/"+caja+"/estado", nil); est["turno"] != nil {
		t.Fatalf("la caja sigue con turno: %v", est)
	}
	if st, raw := c.pos.req("POST", "/v1/caja/movimientos", map[string]any{"cajaId": caja, "tipo": "RETIRO", "monto": "1", "motivo": "Después del cierre", "idempotencyKey": "r-despues"}); st != 409 || raw["code"] != "SIN_TURNO" {
		t.Fatalf("movimiento tras el cierre: %d %v", st, raw)
	}

	// Se imprime (la estación de caja no tiene impresora: sale en la primera activa) y va a la nube.
	if len(out.Impresoras) != 1 || !strings.Contains(out.Aviso, "no tiene impresora") {
		t.Fatalf("impresión: %+v", out)
	}
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool { return strings.Contains(s, "CIERRE Z 0001") && strings.Contains(s, "FALTANTE") })
	})
	var enviado string
	if err := c.a.Store.Read().QueryRow(`SELECT payload FROM outbox WHERE tipo = ?`, EventoCierreZ).Scan(&enviado); err != nil {
		t.Fatal(err)
	}
	var viaje cierrez.Cierre
	if err := json.Unmarshal([]byte(enviado), &viaje); err != nil || !viaje.Verificar() || viaje.ID != z.ID {
		t.Fatalf("evento a la nube: %v %+v", err, viaje)
	}

	// Segundo turno del mismo día: numeración por caja y hash encadenado.
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": caja, "fondoInicial": "20"})
	st, out2, raw := c.cerrar("cierre-000002", []cierrez.Conteo{{Clave: "B20", Cantidad: 1}})
	if st != 200 || out2.Cierre.Numero != 2 || out2.Cierre.HashAnterior != z.Hash || out2.Cierre.Resultado != cierrez.Cuadrado {
		t.Fatalf("segundo cierre: %d %v", st, raw)
	}

	// Inmutable: la base de datos no deja cambiar ni borrar un Cierre Z (QA-10).
	for _, q := range []string{`UPDATE cierres_z SET resultado = 'CUADRADO'`, `DELETE FROM cierres_z`, `UPDATE pagos SET monto = '0'`} {
		if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error { _, err := tx.Exec(q); return err }); err == nil {
			t.Fatalf("%s no falló", q)
		}
	}

	// Sin turnos abiertos la jornada ya se puede cerrar.
	if st, raw := c.pos.req("POST", "/v1/jornada/cerrar", map[string]any{"transferirOrdenes": true}); st != 200 {
		t.Fatalf("cerrar jornada: %d %v", st, raw)
	}
}
