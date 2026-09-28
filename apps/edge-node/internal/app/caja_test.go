package app

import (
	"context"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// cajaF4 es el salón de la fase 3 con la configuración de caja replicada y un reloj falso.
type cajaF4 struct {
	*salonF3
	reloj    *clock.Fake
	caja1    ids.ID
	efectivo ids.ID
	pos      *telefono // la caja en la PC del nodo (loopback, sin emparejar)
}

func nuevaCaja(t *testing.T, ahora time.Time) *cajaF4 {
	t.Helper()
	s := nuevoSalon(t)
	c := &cajaF4{salonF3: s, reloj: clock.NewFake(ahora), caja1: ids.New(), efectivo: ids.New()}
	s.a.Clock = c.reloj
	if err := s.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO cajas (id, tenant_id, local_id, nombre) VALUES (?, 't', 'l', 'Caja 1')`, []any{c.caja1.String()}},
			{`INSERT INTO metodos_pago (id, tenant_id, nombre, tipo, codigo_forma_pago_sri, abre_cajon, orden) VALUES (?, 't', 'Efectivo', 'EFECTIVO', '01', 1, 1), (?, 't', 'Tarjeta crédito', 'TARJETA_CREDITO', '19', 0, 2)`,
				[]any{c.efectivo.String(), ids.New().String()}},
			{`INSERT INTO motivos_descuento (id, tenant_id, nombre, tipo) VALUES (?, 't', 'Cliente frecuente', 'DESCUENTO')`, []any{ids.New().String()}},
			{`INSERT INTO parametros_globales (clave, valor, descripcion) VALUES ('consumidor_final_maximo', '50.00', 'x')`, nil},
		} {
			if _, err := tx.Exec(q.sql, q.args...); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	c.pos = &telefono{t: t, s: s}
	if st, out := c.pos.entrar(s.luis, pinLuis); st != 200 {
		t.Fatalf("cajero: %d %v", st, out)
	}
	return c
}

func (c *cajaF4) jornadas(t *testing.T) (abiertas, total int) {
	t.Helper()
	if err := c.a.Store.Read().QueryRow(`SELECT count(*) FILTER (WHERE cerrada_at IS NULL), count(*) FROM jornadas`).Scan(&abiertas, &total); err != nil {
		t.Fatal(err)
	}
	return abiertas, total
}

// Guayaquil está en UTC−5: 22:00 local del 25 de septiembre.
var nocheDel25 = time.Date(2026, 9, 26, 3, 0, 0, 0, time.UTC)

func TestConfigDeCaja(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	st, cfg := c.pos.req("GET", "/v1/caja/config", nil)
	if st != 200 || len(cfg["cajas"].([]any)) != 1 || len(cfg["metodos"].([]any)) != 2 || cfg["consumidorFinalMaximo"] != "50.00" || cfg["propinaActiva"] != true || len(cfg["denominaciones"].([]any)) != 12 {
		t.Fatalf("config: %d %v", st, cfg)
	}
	// Un teléfono de mesero no es una caja.
	tel := c.emparejar(t, "Teléfono")
	tel.entrar(c.carlos, pinCarlos)
	if st, out := tel.req("GET", "/v1/caja/config", nil); st != 403 || out["code"] != "SOLO_CAJA" {
		t.Fatalf("teléfono en la caja: %d %v", st, out)
	}
}

func TestJornadaCruzaLaMedianocheYSeCierraConCondiciones(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	tel := c.emparejar(t, "Teléfono")
	tel.entrar(c.carlos, pinCarlos)

	// El primer pedido del día abre la jornada sola: el restaurante no espera trámites.
	if st, out := tel.enviar("pedido-noche-1", c.mesa1, plato(c.hamburguesa, "1", c.terMedio)); st != 200 {
		t.Fatalf("pedido: %d %v", st, out)
	}
	if a, n := c.jornadas(t); a != 1 || n != 1 {
		t.Fatalf("jornadas abiertas %d de %d", a, n)
	}
	// Pasada la medianoche la orden nueva sigue en la jornada del 25 (X-15).
	c.reloj.Advance(3 * time.Hour) // 01:00 del 26
	if st, out := tel.enviar("pedido-madrugada", c.mesa2, plato(c.hamburguesa, "1", c.terMedio)); st != 200 {
		t.Fatalf("pedido de madrugada: %d %v", st, out)
	}
	var fechas []string
	rows, _ := c.a.Store.Read().Query(`SELECT DISTINCT fecha_negocio FROM ordenes`)
	for rows.Next() {
		var f string
		_ = rows.Scan(&f)
		fechas = append(fechas, f)
	}
	_ = rows.Close()
	if len(fechas) != 1 || fechas[0] != "2026-09-25" {
		t.Fatalf("fechas de negocio: %v", fechas)
	}

	// Con un turno abierto no se cierra.
	if st, out := c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "50.00"}); st != 200 {
		t.Fatalf("abrir turno: %d %v", st, out)
	}
	if st, out := c.pos.req("POST", "/v1/jornada/cerrar", map[string]any{}); st != 409 || out["code"] != "TURNOS_ABIERTOS" {
		t.Fatalf("cerrar con turno: %d %v", st, out)
	}
	if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE turnos_caja SET estado = 'CERRADO', cerrado_at = ?`, c.reloj.Now().Format(time.RFC3339Nano))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// Con órdenes abiertas tampoco, salvo que se transfieran explícitamente.
	if st, out := c.pos.req("POST", "/v1/jornada/cerrar", map[string]any{}); st != 409 || out["code"] != "ORDENES_ABIERTAS" {
		t.Fatalf("cerrar con órdenes: %d %v", st, out)
	}
	if st, out := c.pos.req("POST", "/v1/jornada/cerrar", map[string]any{"transferirOrdenes": true}); st != 200 {
		t.Fatalf("cerrar transfiriendo: %d %v", st, out)
	}
	// Las sesiones de los meseros terminaron con la jornada.
	if st, _ := tel.req("GET", "/v1/salon", nil); st != 401 {
		t.Fatalf("la sesión del mesero sigue viva: %d", st)
	}

	// Al día siguiente la jornada nueva adopta las órdenes transferidas.
	c.reloj.Advance(10 * time.Hour) // 11:00 del 26
	c.pos.entrar(c.luis, pinLuis)
	st, j := c.pos.req("POST", "/v1/jornada/abrir", nil)
	if st != 200 || j["fechaNegocio"] != "2026-09-26" {
		t.Fatalf("jornada del 26: %d %v", st, j)
	}
	var huerfanas, adoptadas int
	_ = c.a.Store.Read().QueryRow(`SELECT count(*) FILTER (WHERE jornada_id IS NULL), count(*) FILTER (WHERE jornada_id = ?) FROM ordenes`, j["id"]).Scan(&huerfanas, &adoptadas)
	if huerfanas != 0 || adoptadas != 2 {
		t.Fatalf("órdenes sin jornada %d, adoptadas %d", huerfanas, adoptadas)
	}
	// Una jornada por día: la del 26 ya existe.
	if st, out := c.pos.req("POST", "/v1/jornada/abrir", nil); st != 409 || out["code"] != "JORNADA_ABIERTA" {
		t.Fatalf("segunda jornada: %d %v", st, out)
	}
}

func TestTurnosYMovimientosDeCaja(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	caja := c.caja1.String()

	// Sin turno no se mueve efectivo.
	mov := func(clave, tipo, monto string) (int, map[string]any) {
		return c.pos.req("POST", "/v1/caja/movimientos", map[string]any{"cajaId": caja, "tipo": tipo, "monto": monto, "motivo": "Retiro a caja fuerte", "idempotencyKey": clave})
	}
	if st, out := mov("mov-sin-turno", "RETIRO", "20.00"); st != 409 || out["code"] != "SIN_TURNO" {
		t.Fatalf("movimiento sin turno: %d %v", st, out)
	}
	// Fondo inválido: negativo o con más de 2 decimales.
	for _, f := range []string{"-5", "10.005", "abc"} {
		if st, _ := c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": caja, "fondoInicial": f}); st != 422 {
			t.Fatalf("fondo %q: %d", f, st)
		}
	}
	st, turno := c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": caja, "fondoInicial": "50"})
	if st != 200 || turno["fondoInicial"] != "50.00" || turno["cajeroNombre"] != "Luis P." {
		t.Fatalf("abrir turno: %d %v", st, turno)
	}
	// Una caja, un turno abierto.
	if st, out := c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": caja, "fondoInicial": "10"}); st != 409 || out["code"] != "TURNO_ABIERTO" {
		t.Fatalf("segundo turno: %d %v", st, out)
	}
	// El estado de la caja nunca trae el esperado (cierre ciego).
	st, est := c.pos.req("GET", "/v1/cajas/"+caja+"/estado", nil)
	if st != 200 || est["turno"] == nil || est["jornada"] == nil || est["esperado"] != nil {
		t.Fatalf("estado: %d %v", st, est)
	}

	// Movimiento idempotente: el reintento devuelve el mismo.
	st, m1 := mov("retiro-0001", "RETIRO", "20.00")
	if st != 200 {
		t.Fatalf("retiro: %d %v", st, m1)
	}
	if st, m2 := mov("retiro-0001", "RETIRO", "20.00"); st != 200 || m2["id"] != m1["id"] {
		t.Fatalf("reintento: %d %v", st, m2)
	}
	for _, x := range []struct{ tipo, monto string }{{"GASTO", "0"}, {"PROPINA", "5"}, {"GASTO", "3.999"}} {
		if st, _ := mov("malo-"+x.tipo+x.monto, x.tipo, x.monto); st != 422 {
			t.Fatalf("movimiento %v: %d", x, st)
		}
	}
	mov("gasto-0001", "GASTO", "4.50")
	movs, err := c.a.MovimientosDeTurno(context.Background(), ids.MustParse(turno["id"].(string)))
	if err != nil || len(movs) != 2 || movs[0].Monto != "20.00" || movs[1].Tipo != "GASTO" {
		t.Fatalf("movimientos del turno: %v %+v", err, movs)
	}

	// Append-only: la base de datos no deja cambiar ni borrar un movimiento (QA-10).
	for _, q := range []string{`UPDATE movimientos_caja SET monto = '0.01'`, `DELETE FROM movimientos_caja`} {
		err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error { _, err := tx.Exec(q); return err })
		if err == nil {
			t.Fatalf("%s no falló", q)
		}
	}

	// Un mesero no abre turnos aunque use la caja.
	c.pos.entrar(c.carlos, pinCarlos)
	if st, out := c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": caja, "fondoInicial": "10"}); st != 403 {
		t.Fatalf("mesero abre turno: %d %v", st, out)
	}
}
