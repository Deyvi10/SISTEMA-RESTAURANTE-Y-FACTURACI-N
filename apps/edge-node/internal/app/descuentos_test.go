package app

import (
	"context"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/secreto"
)

// Propiedad: con cualquier combinación de descuentos por línea y de cuenta, Σ finales +
// Σ descuentos = Σ líneas al centavo, y ninguna línea queda negativa.
func TestDescuentosCuadranAlCentavo(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 11))
	for range 2000 {
		n := 1 + r.IntN(8)
		var lineas []LineaOrden
		var ds []Descuento
		suma := money.Money{}
		for i := range n {
			l := LineaOrden{ID: ids.New(), Estado: "ENVIADA", Total: money.FromCents(r.Int64N(5000)).String()}
			if i == 0 && r.IntN(4) == 0 {
				l.Estado = "ANULADA"
			} else {
				m, _ := money.Parse(l.Total)
				suma = suma.Add(m)
			}
			lineas = append(lineas, l)
			if r.IntN(3) == 0 {
				id := l.ID
				ds = append(ds, descuentoAzar(r, &id))
			}
		}
		if r.IntN(2) == 0 {
			ds = append(ds, descuentoAzar(r, nil))
		}
		finales, conMonto := aplicarDescuentos(lineas, ds)
		total := money.Money{}
		for _, f := range finales {
			if f.IsNegative() {
				t.Fatalf("línea negativa: %s", f)
			}
			total = total.Add(f)
		}
		for _, d := range conMonto {
			m, err := money.Parse(d.Monto)
			if err != nil || m.IsNegative() {
				t.Fatalf("monto de descuento inválido: %q", d.Monto)
			}
			total = total.Add(m)
		}
		if !total.Equal(suma) {
			t.Fatalf("no cuadra: %s ≠ %s (%+v)", total, suma, conMonto)
		}
	}
}

func descuentoAzar(r *rand.Rand, linea *ids.ID) Descuento {
	d := Descuento{ID: ids.New(), LineaID: linea, Tipo: "PORCENTAJE", Valor: money.FromCents(r.Int64N(10001)).String()}
	switch r.IntN(3) {
	case 0:
		d.Tipo, d.Valor = "MONTO", money.FromCents(r.Int64N(8000)).String()
	case 1:
		d.Cortesia = true
	}
	return d
}

// F4-09: descuentos por línea o cuenta con motivo, límite por persona o local, supervisor
// para pasarse o para cortesías, y todo auditado.
func TestDescuentosYCortesias(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	ctx := context.Background()
	tel := c.emparejar(t, "Tablet")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "0"})
	var frecuente, invitacion string
	sofia := ids.New()
	exec := func(q string, args ...any) {
		t.Helper()
		if err := c.a.Store.Write(ctx, func(tx *store.Tx) error { _, err := tx.Exec(q, args...); return err }); err != nil {
			t.Fatal(err)
		}
	}
	_ = c.a.Store.Read().QueryRow(`SELECT id FROM motivos_descuento WHERE tipo = 'DESCUENTO'`).Scan(&frecuente)
	invitacion = ids.New().String()
	exec(`INSERT INTO motivos_descuento (id, tenant_id, nombre, tipo) VALUES (?, 't', 'Invitación de la casa', 'CORTESIA')`, invitacion)
	var pepper []byte
	_ = c.a.Store.Read().QueryRow(`SELECT pin_pepper FROM nodo WHERE id = 1`).Scan(&pepper)
	h, _ := secreto.HashPIN(pepper, sofia, "5173")
	exec(`INSERT INTO usuarios (id, tenant_id, nombre_mostrar, rol, pin_hash, activo) VALUES (?, 't', 'Sofía A.', 'ADMIN', ?, 1)`, sofia.String(), h)

	orden, _ := c.ordenEnMesa(t, tel, "orden-desc", c.mesa1, plato(c.cerveza, "2"), plato(c.ceviche, "1")) // 6.00 + 12.50
	_, o := c.pos.req("GET", "/v1/ordenes/"+orden, nil)
	lineas := o["orden"].(map[string]any)["lineas"].([]any)
	cerveza, ceviche := lineas[0].(map[string]any)["id"].(string), lineas[1].(map[string]any)["id"].(string)
	descontar := func(body map[string]any) (int, map[string]any) {
		return c.pos.req("POST", "/v1/ordenes/"+orden+"/descuentos", body)
	}
	autorizar := func() string {
		_, au := c.pos.req("POST", "/v1/autorizaciones", map[string]any{"usuarioId": sofia, "pin": "5173", "accion": "DAR_DESCUENTO", "referencia": orden})
		return au["token"].(string)
	}

	// Sin el permiso, pide supervisor; el motivo es obligatorio y de la lista.
	if st, out := descontar(map[string]any{"tipo": "PORCENTAJE", "valor": "10", "motivoId": frecuente}); st != 403 || out["code"] != "REQUIERE_SUPERVISOR" {
		t.Fatalf("sin permiso: %d %v", st, out)
	}
	exec(`INSERT INTO permisos_usuario (tenant_id, usuario_id, permiso, concedido) VALUES ('t', ?, 'DAR_DESCUENTO', 1)`, c.luis.String())
	c.pos.entrar(c.luis, pinLuis)
	if st, _ := descontar(map[string]any{"tipo": "PORCENTAJE", "valor": "10", "motivoId": ids.New()}); st != 422 {
		t.Fatalf("motivo inexistente: %d", st)
	}
	if st, _ := descontar(map[string]any{"tipo": "PORCENTAJE", "valor": "120", "motivoId": frecuente}); st != 422 {
		t.Fatalf("más de 100 %%: %d", st)
	}

	// 10 % de la cuenta (dentro del límite del local): 18.50 − 1.85 = 16.65 con IVA incluido
	// → base 14.48, IVA 2.17, servicio 10 % de la base 1.448 → 1.44 (hacia abajo) → 18.09.
	st, tot := descontar(map[string]any{"tipo": "PORCENTAJE", "valor": "10", "motivoId": frecuente})
	if st != 200 || tot["descuento"] != "1.85" || tot["subtotal"] != "14.48" || tot["iva"] != "2.17" || tot["propina"] != "1.44" || tot["total"] != "18.09" {
		t.Fatalf("10 %%: %d %v", st, tot)
	}
	// 15 % pasa el límite: supervisor. Con su PIN, reemplaza al 10 % (uno por cuenta).
	if st, out := descontar(map[string]any{"tipo": "PORCENTAJE", "valor": "15", "motivoId": frecuente}); st != 403 {
		t.Fatalf("sobre el límite: %d %v", st, out)
	}
	st, tot = descontar(map[string]any{"tipo": "PORCENTAJE", "valor": "15", "motivoId": frecuente, "autorizacion": autorizar()})
	if st != 200 || tot["descuento"] != "2.78" || len(tot["descuentos"].([]any)) != 1 {
		t.Fatalf("15 %% autorizado: %d %v", st, tot)
	}
	// Un límite propio más alto le permite 15 % sin supervisor (un monto se compara en %).
	exec(`UPDATE usuarios SET descuento_maximo_pct = '20' WHERE id = ?`, c.luis.String())
	if st, out := descontar(map[string]any{"lineaId": ceviche, "tipo": "MONTO", "valor": "2.50", "motivoId": frecuente}); st != 200 {
		t.Fatalf("20 %% de la línea con límite propio: %d %v", st, out)
	}

	// Cortesía: motivo de cortesía y autorización (no es administrador).
	if st, _ := descontar(map[string]any{"lineaId": cerveza, "cortesia": true, "motivoId": frecuente}); st != 422 {
		t.Fatalf("cortesía con motivo de descuento: %d", st)
	}
	if st, out := descontar(map[string]any{"lineaId": cerveza, "cortesia": true, "motivoId": invitacion}); st != 403 || out["code"] != "REQUIERE_SUPERVISOR" {
		t.Fatalf("cortesía sin autorización: %d %v", st, out)
	}
	st, tot = descontar(map[string]any{"lineaId": cerveza, "cortesia": true, "motivoId": invitacion, "autorizacion": autorizar()})
	// Cerveza gratis (6.00) y ceviche 12.50 − 2.50 = 10.00; 15 % de cuenta sobre 10.00 = 1.50.
	if st != 200 || tot["descuento"] != "10.00" || tot["lineas"].(map[string]any)[cerveza] != "0.00" || tot["lineas"].(map[string]any)[ceviche] != "8.50" {
		t.Fatalf("cortesía: %d %v", st, tot)
	}

	// Quitar el de la cuenta devuelve su parte; queda auditado.
	var cuentaID string
	for _, d := range tot["descuentos"].([]any) {
		if dd := d.(map[string]any); dd["lineaId"] == nil {
			cuentaID = dd["id"].(string)
		}
	}
	if st, tot := c.pos.req("DELETE", "/v1/ordenes/"+orden+"/descuentos/"+cuentaID, nil); st != 200 || tot["descuento"] != "8.50" {
		t.Fatalf("quitar: %d %v", st, tot)
	}
	var acciones []string
	rows, _ := c.a.Store.Read().Query(`SELECT accion || ':' || coalesce(monto, '') || ':' || coalesce(autorizado_por, '') FROM auditoria WHERE accion LIKE '%DESCUENTO%' OR accion LIKE 'CORTESIA%' ORDER BY seq`)
	for rows.Next() {
		var a string
		_ = rows.Scan(&a)
		acciones = append(acciones, a)
	}
	_ = rows.Close()
	want := []string{"DESCUENTO_APLICADO:1.85:", "DESCUENTO_APLICADO:2.78:" + sofia.String(), "DESCUENTO_APLICADO:2.50:", "CORTESIA_APLICADA:6.00:" + sofia.String(), "DESCUENTO_QUITADO:1.50:"}
	if !slices.Equal(acciones, want) {
		t.Fatalf("auditoría:\n%v\nquiero\n%v", acciones, want)
	}

	// Se cobra con los descuentos: el documento y el ticket los muestran.
	st, out, raw := c.cobrar(orden, c.efectivo.String(), "", "cobro-con-descuentos")
	if st != 200 || out.Documento.Totales.Descuento != "8.50" {
		t.Fatalf("cobro: %d %v", st, raw)
	}
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool { return strings.Contains(s, "Descuento") && strings.Contains(s, "-$8.50") })
	})

	// Toda la mesa invitada: total $0, se cierra con un documento sin pagos ni cajón.
	orden2, _ := c.ordenEnMesa(t, tel, "orden-invitada", c.mesa2, plato(c.ceviche, "1"))
	if st, out := c.pos.req("POST", "/v1/ordenes/"+orden2+"/descuentos", map[string]any{"cortesia": true, "motivoId": invitacion, "autorizacion": func() string {
		_, au := c.pos.req("POST", "/v1/autorizaciones", map[string]any{"usuarioId": sofia, "pin": "5173", "accion": "DAR_DESCUENTO", "referencia": orden2})
		return au["token"].(string)
	}()}); st != 200 || out["total"] != "0.00" {
		t.Fatalf("cortesía de la cuenta: %d %v", st, out)
	}
	st, out, raw = c.cobrar(orden2, c.efectivo.String(), "", "cobro-invitada")
	if st != 200 || out.Documento.Totales.Total != "0.00" || out.Documento.Metodo != "Cortesía" || len(out.Documento.Pagos) != 0 || out.Documento.AbreCajon {
		t.Fatalf("cerrar invitada: %d %v", st, raw)
	}
	var pagos int
	_ = c.a.Store.Read().QueryRow(`SELECT count(*) FROM pagos WHERE orden_id = ?`, orden2).Scan(&pagos)
	if pagos != 0 {
		t.Fatalf("pagos de una cortesía: %d", pagos)
	}
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool { return strings.Contains(s, "Cortesía de la casa: sin cobro") })
	})
}
