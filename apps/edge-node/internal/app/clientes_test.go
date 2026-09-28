package app

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

func (c *cajaF4) cobrarA(orden, clave string, comprador map[string]any) (int, CobroOut, map[string]any) {
	st, raw := c.pos.req("POST", "/v1/ordenes/"+orden+"/cobrar", map[string]any{"cajaId": c.caja1, "metodoId": c.efectivo, "comprador": comprador, "idempotencyKey": clave})
	var out CobroOut
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &out)
	return st, out, raw
}

// F4-07: comprador identificado, validación local y consentimiento para guardarlo.
func TestCompradorIdentificado(t *testing.T) {
	c := nuevaCaja(t, nocheDel25)
	tel := c.emparejar(t, "Tablet")
	tel.entrar(c.carlos, pinCarlos)
	c.pos.req("POST", "/v1/turnos", map[string]any{"cajaId": c.caja1, "fondoInicial": "0"})
	buscar := func(id string) map[string]any {
		_, out := c.pos.req("GET", "/v1/clientes/buscar?identificacion="+id, nil)
		return out
	}

	// Validación local antes de buscar (mismas reglas que la caja web).
	if b := buscar("1710034066"); b["valida"] != false || !strings.Contains(b["motivo"].(string), "verificador") {
		t.Fatalf("cédula inválida: %v", b)
	}
	if b := buscar("1790011675001"); b["valida"] != true || b["advertencia"] == nil || b["tipo"] != "04" {
		t.Fatalf("RUC de sociedad con módulo 11 inválido: advierte y no bloquea: %v", b)
	}
	if b := buscar("1710034065"); b["valida"] != true || b["tipo"] != "05" || b["cliente"] != nil {
		t.Fatalf("cédula nueva: %v", b)
	}

	persona := map[string]any{"identificacion": "1710034065", "razonSocial": "  María   José Pérez ", "email": "MARIA@example.com", "telefono": "0991234567"}
	// Datos incompletos o inválidos.
	for nombre, cambio := range map[string]map[string]any{
		"sin correo":       {"email": ""},
		"correo inválido":  {"email": "maria@"},
		"sin nombre":       {"razonSocial": " "},
		"cédula inválida":  {"identificacion": "1710034066"},
		"número muy corto": {"identificacion": "12345"},
	} {
		comp := map[string]any{}
		for k, v := range persona {
			comp[k] = v
		}
		for k, v := range cambio {
			comp[k] = v
		}
		orden, _ := c.ordenEnMesa(t, tel, "orden-"+strings.ReplaceAll(nombre, " ", "-"), c.mesa3, plato(c.cerveza, "1"))
		if st, _, raw := c.cobrarA(orden, "cobro-"+strings.ReplaceAll(nombre, " ", "-"), comp); st != 422 {
			t.Fatalf("%s: %d %v", nombre, st, raw)
		}
		c.cobrar(orden, c.efectivo.String(), "", "limpia-"+strings.ReplaceAll(nombre, " ", "-")) // deja la mesa libre
	}

	// Sin consentimiento: el documento lleva sus datos, pero no se guarda para después.
	orden, _ := c.ordenEnMesa(t, tel, "orden-sin-consentimiento", c.mesa1, plato(c.cerveza, "1"))
	st, out, raw := c.cobrarA(orden, "cobro-sin-consentimiento", persona)
	d := out.Documento
	if st != 200 || d.Comprador != "María José Pérez" || d.CompradorTipo != "05" || d.CompradorIdentificacion != "1710034065" || d.CompradorEmail != "maria@example.com" || d.ClienteGuardado {
		t.Fatalf("sin consentimiento: %d %v", st, raw)
	}
	if b := buscar("1710034065"); b["cliente"] != nil {
		t.Fatalf("se guardó sin consentimiento: %v", b)
	}
	esperar(t, func() bool {
		return slices.ContainsFunc(c.cocina.Textos(), func(s string) bool { return strings.Contains(s, "C.I./RUC: 1710034065") })
	})

	// Con consentimiento: queda guardado y la próxima vez se autocompleta desde el nodo.
	persona["consentimiento"] = true
	orden, _ = c.ordenEnMesa(t, tel, "orden-con-consentimiento", c.mesa1, plato(c.cerveza, "1"))
	if st, out, raw := c.cobrarA(orden, "cobro-con-consentimiento", persona); st != 200 || !out.Documento.ClienteGuardado {
		t.Fatalf("con consentimiento: %d %v", st, raw)
	}
	b := buscar("1710034065")
	cli, _ := b["cliente"].(map[string]any)
	if b["origen"] != "NODO" || cli == nil || cli["razonSocial"] != "María José Pérez" || cli["telefono"] != "0991234567" {
		t.Fatalf("autocompletado: %v", b)
	}
	var eventos int
	_ = c.a.Store.Read().QueryRow(`SELECT count(*) FROM outbox WHERE tipo = ?`, EventoClienteGuardado).Scan(&eventos)
	if eventos != 1 {
		t.Fatalf("eventos de cliente: %d", eventos)
	}

	// Una compra posterior solo marca como nuevo el campo que cambió (fusión por campo).
	c.reloj.Advance(time.Hour)
	persona["telefono"] = "0987654321"
	orden, _ = c.ordenEnMesa(t, tel, "orden-cambio-telefono", c.mesa1, plato(c.cerveza, "1"))
	c.cobrarA(orden, "cobro-cambio-telefono", persona)
	var campos string
	_ = c.a.Store.Read().QueryRow(`SELECT campos_at FROM clientes WHERE identificacion = '1710034065'`).Scan(&campos)
	var at map[string]time.Time
	_ = json.Unmarshal([]byte(campos), &at)
	if !at["telefono"].After(at["email"]) {
		t.Fatalf("campos_at: %s", campos)
	}

	// El límite de consumidor final no aplica a un comprador identificado.
	param := func(v string) {
		if err := c.a.Store.Write(context.Background(), func(tx *store.Tx) error {
			_, err := tx.Exec(`UPDATE parametros_globales SET valor = ? WHERE clave = 'consumidor_final_maximo'`, v)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	param("1.00")
	orden, _ = c.ordenEnMesa(t, tel, "orden-grande", c.mesa2, plato(c.ceviche, "2"))
	if st, _, raw := c.cobrarA(orden, "cobro-grande-cf", map[string]any{"identificacion": "9999999999999"}); st != 422 || raw["code"] != "CONSUMIDOR_FINAL_EXCEDIDO" {
		t.Fatalf("consumidor final sobre el límite: %d %v", st, raw)
	}
	if st, _, raw := c.cobrarA(orden, "cobro-grande-ruc", map[string]any{"identificacion": "1790011674001", "razonSocial": "Empresa S.A.", "email": "compras@empresa.ec"}); st != 200 {
		t.Fatalf("RUC sobre el límite: %d %v", st, raw)
	}
}

func TestFusionPorCampo(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	nodo := Cliente{RazonSocial: "Ana", Email: "ana@nodo.ec", Telefono: "099", CamposAt: map[string]time.Time{"razonSocial": t0, "email": t0.Add(2 * time.Hour), "telefono": t0}}
	nube := Cliente{RazonSocial: "Ana María", Email: "ana@nube.ec", Telefono: "098", Direccion: "Quito", CamposAt: map[string]time.Time{"razonSocial": t0.Add(time.Hour), "email": t0.Add(time.Hour), "direccion": t0}}
	f := fusionar(nodo, nube)
	if f.RazonSocial != "Ana María" || f.Email != "ana@nodo.ec" || f.Telefono != "099" || f.Direccion != "Quito" {
		t.Fatalf("fusión: %+v", f)
	}
	// Es simétrica en el resultado de los campos.
	g := fusionar(nube, nodo)
	if g.RazonSocial != f.RazonSocial || g.Email != f.Email || g.Direccion != f.Direccion {
		t.Fatalf("no simétrica: %+v", g)
	}
}

// Cascada: si el nodo no lo tiene, pregunta a la nube (y lo guarda en caché); si la nube
// tarda, sigue sin esperar para que la caja autocomplete en ≤ 300 ms o muestre el formulario.
func TestBusquedaDeClienteEnLaNube(t *testing.T) {
	f := newNubeFalsa()
	f.codigos["ABCDEFGH"] = true
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	f.clientes = map[string]Cliente{"1790011674001": {ID: ids.New(), TipoIdentificacion: "04", Identificacion: "1790011674001", RazonSocial: "Empresa S.A.",
		Email: "compras@empresa.ec", CamposAt: map[string]time.Time{"razonSocial": t0, "email": t0}}}
	a, lan := nodoDePrueba(t, f)
	if st, _ := postJSON(t, lan.URL+"/v1/activacion", map[string]string{"codigo": "ABCDEFGH"}); st != 200 {
		t.Fatal("activación")
	}
	esperar(t, func() bool { a.syncMu.Lock(); defer a.syncMu.Unlock(); return a.nube != nil })
	ctx := context.Background()

	b, err := a.BuscarCliente(ctx, "", "1790011674001")
	if err != nil || b.Origen != "NUBE" || b.Cliente == nil || b.Cliente.RazonSocial != "Empresa S.A." {
		t.Fatalf("desde la nube: %v %+v", err, b)
	}
	f.mu.Lock()
	delete(f.clientes, "1790011674001")
	f.mu.Unlock()
	if b, _ := a.BuscarCliente(ctx, "", "1790011674001"); b.Origen != "NODO" {
		t.Fatalf("la segunda vez debería salir de la caché del nodo: %+v", b)
	}

	// La nube lenta no retrasa la caja.
	f.mu.Lock()
	f.demora = 600 * time.Millisecond
	f.clientes["1710034065"] = Cliente{ID: ids.New(), TipoIdentificacion: "05", Identificacion: "1710034065", RazonSocial: "Lenta"}
	f.mu.Unlock()
	t1 := time.Now()
	b, _ = a.BuscarCliente(ctx, "", "1710034065")
	if d := time.Since(t1); d > 300*time.Millisecond || b.Cliente != nil || !b.Valida {
		t.Fatalf("nube lenta: %v %+v", d, b)
	}
}
