package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/descubrir"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion/termica"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// restaurante arma un nodo con la réplica de un local real en pequeño: Cocina (por
// defecto) y Bar, cada una con su impresora falsa.
type restaurante struct {
	a                        *App
	lan                      *httptest.Server
	cocina, bar              *termica.Termica
	impCocina, impBar        ids.ID
	estCocina, estBar        ids.ID
	ceviche, cerveza, postre ids.ID
}

func nuevoRestaurante(t *testing.T) *restaurante {
	t.Helper()
	cfg := Config{DataDir: t.TempDir(), HTTPAddr: "127.0.0.1:0", NubeURL: "http://nube.invalida"}
	a, err := New(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	r := &restaurante{a: a, cocina: termica.Nueva(t), bar: termica.Nueva(t),
		impCocina: ids.New(), impBar: ids.New(), estCocina: ids.New(), estBar: ids.New(),
		ceviche: ids.New(), cerveza: ids.New(), postre: ids.New()}
	hp := func(x *termica.Termica) (string, int) {
		h, p, _ := net.SplitHostPort(x.Addr())
		n, _ := strconv.Atoi(p)
		return h, n
	}
	hc, pc := hp(r.cocina)
	hb, pb := hp(r.bar)
	catPlatos, catBebidas, catPostres := ids.New(), ids.New(), ids.New()
	seed := []string{
		fmt.Sprintf(`INSERT INTO locales (id, tenant_id, nombre, codigo_establecimiento) VALUES ('%s', 't', 'Matriz', '001')`, ids.New()),
		fmt.Sprintf(`INSERT INTO estaciones (id, tenant_id, local_id, nombre, es_defecto, orden) VALUES ('%s', 't', 'l', 'Cocina', 1, 1), ('%s', 't', 'l', 'Bar', 0, 2)`, r.estCocina, r.estBar),
		fmt.Sprintf(`INSERT INTO categorias (id, tenant_id, nombre, estacion_id) VALUES ('%s', 't', 'Platos', '%s'), ('%s', 't', 'Bebidas', '%s'), ('%s', 't', 'Postres', NULL)`, catPlatos, r.estCocina, catBebidas, r.estBar, catPostres),
		fmt.Sprintf(`INSERT INTO productos (id, tenant_id, categoria_id, nombre, precio, tarifa_iva_id) VALUES ('%s', 't', '%s', 'Ceviche de camarón', '12.50', 'iva'), ('%s', 't', '%s', 'Cerveza', '3.00', 'iva'), ('%s', 't', '%s', 'Tres leches', '3.50', 'iva')`, r.ceviche, catPlatos, r.cerveza, catBebidas, r.postre, catPostres),
		fmt.Sprintf(`INSERT INTO impresoras (id, tenant_id, local_id, nombre, host, puerto, ancho_papel) VALUES ('%s', 't', 'l', 'Cocina caliente', '%s', %d, 80), ('%s', 't', 'l', 'Barra', '%s', %d, 58)`, r.impCocina, hc, pc, r.impBar, hb, pb),
		fmt.Sprintf(`INSERT INTO estacion_impresoras (tenant_id, estacion_id, impresora_id, local_id) VALUES ('t', '%s', '%s', 'l'), ('t', '%s', '%s', 'l')`, r.estCocina, r.impCocina, r.estBar, r.impBar),
	}
	if err := a.Store.Write(context.Background(), func(tx *store.Tx) error {
		for _, q := range seed {
			if _, err := tx.Exec(q); err != nil {
				return fmt.Errorf("%s: %w", q, err)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.bg = ctx
	a.motor.Sondeo, a.motor.Reintento = 100*time.Millisecond, 50*time.Millisecond
	a.motor.Iniciar(ctx)
	a.sincronizarImpresoras(ctx)
	r.lan = httptest.NewServer(a.Handler())
	t.Cleanup(func() { r.lan.Close(); cancel(); a.motor.Esperar(); a.wg.Wait(); _ = a.Store.Close() })
	return r
}

func (r *restaurante) enviar(t *testing.T, clave string, lineas ...map[string]any) (int, ComandaOut, map[string]any) {
	t.Helper()
	st, raw := postJSON(t, r.lan.URL+"/v1/comandas", map[string]any{"idempotencyKey": clave, "mesa": "Mesa 4", "mesero": "Carlos M.", "lineas": lineas})
	var out ComandaOut
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &out)
	return st, out, raw
}

func linea(p ids.ID, cant string, extra ...string) map[string]any {
	l := map[string]any{"productoId": p, "cantidad": cant}
	if len(extra) > 0 {
		l["nota"] = extra[0]
	}
	return l
}

func contiene(textos []string, sub string) bool {
	return slices.ContainsFunc(textos, func(s string) bool { return strings.Contains(s, sub) })
}

// DoD F2: una comanda mixta imprime en 2 impresoras en paralelo con p95 ≤ 1,5 s.
func TestComandaMixtaEnParaleloP95(t *testing.T) {
	r := nuevoRestaurante(t)
	const n = 25
	var lat []time.Duration
	for i := range n {
		t0 := time.Now()
		st, out, raw := r.enviar(t, "comanda-"+strconv.Itoa(i), linea(r.ceviche, "2", "sin cebolla"), linea(r.cerveza, "3"), linea(r.postre, "1"))
		if st != 201 || len(out.Envios) != 2 {
			t.Fatalf("envío: %d %v", st, raw)
		}
		esperar(t, func() bool { return len(r.cocina.Trabajos()) == i+1 && len(r.bar.Trabajos()) == i+1 })
		lat = append(lat, time.Since(t0))
	}
	slices.Sort(lat)
	p95 := lat[n*95/100]
	t.Logf("p50 %v · p95 %v", lat[n/2], p95)
	if p95 > 1500*time.Millisecond {
		t.Fatalf("p95 = %v > 1,5 s", p95)
	}
	cocina, bar := r.cocina.Textos(), r.bar.Textos()
	if !contiene(cocina, "Ceviche de camarón") || !contiene(cocina, "Tres leches") || contiene(cocina, "Cerveza") {
		t.Fatalf("cocina recibió: %q", cocina[0])
	}
	if !contiene(bar, "Cerveza") || contiene(bar, "Ceviche") {
		t.Fatalf("bar recibió: %q", bar[0])
	}
	if !contiene(cocina, "sin cebolla") || !contiene(cocina, "Mesa 4") || !contiene(cocina, "Comanda #1") {
		t.Fatal("faltan mesa, número o nota en el ticket de cocina")
	}
}

func TestComandaIdempotenteYValidada(t *testing.T) {
	r := nuevoRestaurante(t)
	st, a1, _ := r.enviar(t, "misma-clave-1", linea(r.cerveza, "1"))
	st2, a2, _ := r.enviar(t, "misma-clave-1", linea(r.cerveza, "1"))
	if st != 201 || st2 != 201 || a1.ID != a2.ID || !a2.Repetida || a1.Numero != a2.Numero {
		t.Fatalf("idempotencia: %+v vs %+v", a1, a2)
	}
	esperar(t, func() bool { return len(r.bar.Trabajos()) == 1 })
	time.Sleep(200 * time.Millisecond)
	if len(r.bar.Trabajos()) != 1 {
		t.Fatal("el reintento imprimió dos veces")
	}
	for nombre, l := range map[string]map[string]any{
		"cantidad cero":      linea(r.cerveza, "0"),
		"cantidad con coma":  linea(r.cerveza, "1,5"),
		"producto inventado": linea(ids.New(), "1"),
		"nota larga":         linea(r.cerveza, "1", strings.Repeat("x", 141)),
	} {
		if st, _, raw := r.enviar(t, "clave-"+nombre, l); st != 422 {
			t.Errorf("%s: %d %v", nombre, st, raw)
		}
	}
	// Fracciones (cuentas divididas): se aceptan con hasta 3 decimales.
	if st, _, raw := r.enviar(t, "clave-fraccion", linea(r.ceviche, "0.5")); st != 201 {
		t.Fatalf("media porción: %d %v", st, raw)
	}
}

func TestAnularYReimprimir(t *testing.T) {
	r := nuevoRestaurante(t)
	_, out, _ := r.enviar(t, "comanda-anular", linea(r.ceviche, "1"), linea(r.cerveza, "2"))
	esperar(t, func() bool { return len(r.cocina.Trabajos()) == 1 && len(r.bar.Trabajos()) == 1 })
	var lineas []Linea
	var raw string
	_ = r.a.Store.Read().QueryRow(`SELECT lineas FROM comandas WHERE id = ?`, out.ID.String()).Scan(&raw)
	_ = json.Unmarshal([]byte(raw), &lineas)
	var cerveza ids.ID
	for _, l := range lineas {
		if l.ProductoID == r.cerveza {
			cerveza = l.ID
			if l.EstacionID != r.estBar {
				t.Fatal("la línea no guardó su estación")
			}
		}
	}
	url := r.lan.URL + "/v1/comandas/" + out.ID.String()
	if st, raw := postJSON(t, url+"/anular", map[string]any{"lineas": []ids.ID{cerveza}, "motivo": "Cliente cambió de opinión"}); st != 200 {
		t.Fatalf("anular: %d %v", st, raw)
	}
	esperar(t, func() bool { return len(r.bar.Trabajos()) == 2 })
	if tx := r.bar.Textos()[1]; !strings.Contains(tx, "ANULACIÓN") || !strings.Contains(tx, "Cerveza") || !strings.Contains(strings.Join(strings.Fields(tx), " "), "Cliente cambió de opinión") {
		t.Fatalf("ticket de anulación: %q", tx)
	}
	time.Sleep(150 * time.Millisecond)
	if len(r.cocina.Trabajos()) != 1 {
		t.Fatal("la anulación del bar se imprimió también en cocina")
	}
	if st, _ := postJSON(t, url+"/anular", map[string]any{"lineas": []ids.ID{cerveza}, "motivo": "otra vez"}); st != 409 {
		t.Fatalf("doble anulación: %d", st)
	}
	// Reimpresión: sin la cerveza anulada, con marca y hora original.
	if st, raw := postJSON(t, url+"/reimprimir", map[string]any{}); st != 200 {
		t.Fatalf("reimprimir: %d %v", st, raw)
	}
	esperar(t, func() bool { return len(r.cocina.Trabajos()) == 2 })
	if tx := r.cocina.Textos()[1]; !strings.Contains(tx, "REIMPRESIÓN") || !strings.Contains(tx, "Ceviche") {
		t.Fatalf("reimpresión: %q", tx)
	}
	time.Sleep(150 * time.Millisecond)
	if len(r.bar.Trabajos()) != 2 {
		t.Fatal("se reimprimió un plato anulado")
	}
	var audit int
	_ = r.a.Store.Read().QueryRow(`SELECT count(*) FROM auditoria WHERE accion IN ('LINEA_ANULADA','COMANDA_REIMPRESA')`).Scan(&audit)
	if audit != 2 {
		t.Fatalf("auditoría: %d filas", audit)
	}
	var eventosN int
	_ = r.a.Store.Read().QueryRow(`SELECT count(*) FROM outbox WHERE tipo IN ('comanda.enviada','comanda.lineas_anuladas')`).Scan(&eventosN)
	if eventosN != 2 {
		t.Fatalf("outbox: %d eventos", eventosN)
	}
}

// RF-02-05: la barra se queda sin papel; el cajero redirige su cola a cocina con un toque.
func TestSinPapelYRedireccion(t *testing.T) {
	r := nuevoRestaurante(t)
	r.bar.Fijar(escpos.Status{PaperOut: true})
	r.enviar(t, "c-sin-papel", linea(r.cerveza, "1"), linea(r.ceviche, "1"))
	esperar(t, func() bool { return len(r.cocina.Trabajos()) == 1 })
	esperar(t, func() bool { return r.a.motor.Estado(r.impBar).Estado == "SIN_PAPEL" })
	res, err := http.Get(r.lan.URL + "/v1/impresoras")
	if err != nil {
		t.Fatal(err)
	}
	var vista struct {
		Impresoras []ImpresoraVista `json:"impresoras"`
	}
	_ = json.NewDecoder(res.Body).Decode(&vista)
	_ = res.Body.Close()
	for _, x := range vista.Impresoras {
		if x.ID == r.impBar && (x.Estado.Estado != "SIN_PAPEL" || x.Cola != 1 || x.Motivo == "") {
			t.Fatalf("vista de la barra: %+v", x)
		}
	}
	st, raw := postJSON(t, r.lan.URL+"/v1/estaciones/"+r.estBar.String()+"/redirigir", map[string]any{"impresoraId": r.impCocina})
	if st != 200 || raw["trabajosMovidos"].(float64) != 1 {
		t.Fatalf("redirigir: %d %v", st, raw)
	}
	esperar(t, func() bool { return len(r.cocina.Trabajos()) == 2 })
	if !strings.Contains(r.cocina.Textos()[1], "Cerveza") {
		t.Fatal("la cerveza no salió por cocina")
	}
	// Las siguientes comandas del bar también van a cocina hasta quitar la redirección.
	r.enviar(t, "c-redirigida", linea(r.cerveza, "1"))
	esperar(t, func() bool { return len(r.cocina.Trabajos()) == 3 })
	req, _ := http.NewRequest("DELETE", r.lan.URL+"/v1/estaciones/"+r.estBar.String()+"/redirigir", nil)
	res, _ = http.DefaultClient.Do(req)
	if res.StatusCode != 204 {
		t.Fatalf("quitar redirección: %d", res.StatusCode)
	}
	r.bar.Fijar(escpos.Status{})
	r.enviar(t, "c-vuelta", linea(r.cerveza, "1"))
	esperar(t, func() bool { return len(r.bar.Trabajos()) == 1 })
}

func TestPruebaDesdeLaNube(t *testing.T) {
	r := nuevoRestaurante(t)
	cmd := ids.New()
	if err := r.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT INTO comandos_nodo (id, tenant_id, local_id, tipo, datos, created_at) VALUES (?, 't', 'l', 'IMPRIMIR_PRUEBA', ?, ?)`,
			cmd.String(), `{"impresoraId":"`+r.impCocina.String()+`"}`, time.Now().UTC().Format(time.RFC3339Nano))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	r.a.ejecutarComandos(context.Background())
	r.a.ejecutarComandos(context.Background()) // repetido: no imprime dos veces
	esperar(t, func() bool {
		return len(r.cocina.Trabajos()) == 1 && strings.Contains(r.cocina.Textos()[0], "¡Funciona!")
	})
	esperar(t, func() bool {
		var n int
		_ = r.a.Store.Read().QueryRow(`SELECT count(*) FROM outbox WHERE tipo = 'comando.ejecutado' AND json_extract(payload, '$.ok')`).Scan(&n)
		return n == 1
	})
}

func TestSoloDesdeEstaPC(t *testing.T) {
	h := soloLocal(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	for addr, want := range map[string]int{"127.0.0.1:5000": 204, "[::1]:5000": 204, "192.168.1.40:5000": 403} {
		req := httptest.NewRequest("POST", "/v1/comandas", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%s → %d, quiero %d", addr, rec.Code, want)
		}
	}
}

// F2-08: lo nuevo se informa a la nube; lo conocido no; una IP cambiada se reubica por MAC.
func TestBusquedaDeImpresoras(t *testing.T) {
	r := nuevoRestaurante(t)
	host, port, _ := net.SplitHostPort(r.bar.Addr())
	p, _ := strconv.Atoi(port)
	if err := r.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE impresoras SET mac = '00:11:62:aa:bb:cc' WHERE id = ?`, r.impBar.String())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	r.a.buscar = func(context.Context) []descubrir.Encontrada {
		// Ya conocida y sin cambios: no se informa.
		return []descubrir.Encontrada{{Host: host, Puerto: p, MAC: "00:11:62:aa:bb:cc"}}
	}
	if n := r.a.BuscarImpresoras(context.Background()); n != 1 {
		t.Fatalf("encontradas = %d", n)
	}
	contar := func() int {
		var n int
		_ = r.a.Store.Read().QueryRow(`SELECT count(*) FROM outbox WHERE tipo = 'impresora.detectada'`).Scan(&n)
		return n
	}
	if contar() != 0 {
		t.Fatal("informó una impresora que no cambió")
	}
	// La barra cambió de IP (DHCP) y apareció una nueva.
	r.a.buscar = func(context.Context) []descubrir.Encontrada {
		return []descubrir.Encontrada{
			{Host: "192.168.1.99", Puerto: 9100, MAC: "00:11:62:aa:bb:cc"},
			{Host: "192.168.1.120", Puerto: 9100, MAC: "b8:27:eb:00:00:01", Modelo: "TM-T20III"},
		}
	}
	r.a.BuscarImpresoras(context.Background())
	if contar() != 2 {
		t.Fatalf("eventos = %d", contar())
	}
	var reubicada string
	_ = r.a.Store.Read().QueryRow(`SELECT json_extract(payload, '$.id') FROM outbox WHERE tipo = 'impresora.detectada' AND json_extract(payload, '$.host') = '192.168.1.99'`).Scan(&reubicada)
	if reubicada != r.impBar.String() {
		t.Fatalf("la reubicación no usó el id de la barra: %s", reubicada)
	}
	if !strings.Contains(resumenBusqueda(2), "2 impresoras") {
		t.Fatal(resumenBusqueda(2))
	}
}

// F2-05: la página de estado resume el nodo sin datos personales.
func TestPaginaDeEstado(t *testing.T) {
	r := nuevoRestaurante(t)
	r.bar.Fijar(escpos.Status{PaperOut: true})
	esperar(t, func() bool { return r.a.motor.Estado(r.impBar).Estado == "SIN_PAPEL" })
	res, err := http.Get(r.lan.URL + "/v1/estado")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	var e EstadoNodo
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if e.Activado || e.Conectividad.Nube != "SIN_ACTIVAR" || len(e.Impresoras) != 2 || e.Version == "" {
		t.Fatalf("estado: %+v", e)
	}
	for _, x := range e.Impresoras {
		if x.ID == r.impBar && x.Estado != "SIN_PAPEL" {
			t.Fatalf("barra: %+v", x)
		}
	}
	for _, prohibido := range []string{"Carlos", "pin", "email", "llave"} {
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(prohibido)) {
			t.Errorf("la página de estado expone %q", prohibido)
		}
	}
	res, _ = http.Get(r.lan.URL + "/estado")
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(body), "Estado del Nodo") {
		t.Fatalf("/estado = %d", res.StatusCode)
	}
}
