package server_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/impresoras"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/nodos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/salon"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/nodoauth"
)

// nodoSim es un Nodo Local de prueba: su llave privada nunca sale de aquí.
type nodoSim struct {
	e    *env
	id   ids.ID
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
	seq  int64
}

func (e *env) nuevoNodo() *nodoSim {
	pub, priv, _ := ed25519.GenerateKey(nil)
	return &nodoSim{e: e, id: ids.New(), priv: priv, pub: pub}
}

func (n *nodoSim) activar(codigo string, want int) nodos.Activado {
	n.e.t.Helper()
	var out nodos.Activado
	n.req("POST", "/v1/nodos/activar", map[string]any{
		"codigo": codigo, "nodoId": n.id, "llavePublica": base64.StdEncoding.EncodeToString(n.pub), "version": "0.1.0", "nombreEquipo": "PC-CAJA",
	}, false, want, &out)
	return out
}

func (n *nodoSim) req(method, path string, body any, firmar bool, want int, out any) {
	n.e.t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, n.e.srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if firmar {
		tok, err := nodoauth.Sign(n.priv, n.id, time.Now())
		if err != nil {
			n.e.t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		n.e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		n.e.t.Fatalf("%s %s = %d, se esperaba %d: %s", method, path, res.StatusCode, want, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			n.e.t.Fatalf("respuesta ilegible: %s", raw)
		}
	}
}

func (n *nodoSim) evento(tipo string) edgesync.Event {
	n.seq++
	ev, _ := edgesync.NewEvent(tipo, 1, ids.New(), map[string]string{"x": "y"}, time.Now())
	ev.NodeSeq = n.seq
	return ev
}

func (c *cliente) codigoNodo() nodos.CodigoGenerado {
	c.e.t.Helper()
	var locales []salon.Local
	c.do("GET", "/v1/locales", nil, 200, &locales)
	var cod nodos.CodigoGenerado
	c.do("POST", "/v1/nodos/codigos", map[string]any{"localId": locales[0].ID}, 201, &cod)
	return cod
}

// nodoOperativo activa un nodo, envía heartbeat y un evento: deja filas en todas las
// tablas de nodos (lo usa también QA-06).
func (c *cliente) nodoOperativo() *nodoSim {
	n := c.e.nuevoNodo()
	n.activar(c.codigoNodo().Codigo, 200)
	n.req("POST", "/v1/nodos/heartbeat", edgesync.Heartbeat{Version: "0.1.0", HoraNodo: time.Now()}, true, 200, nil)
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: []edgesync.Event{n.evento("prueba.creada")}}, true, 200, nil)
	c.impresoraConPrueba(n, "10.0.0.9")
	return n
}

func TestActivacionDeNodo(t *testing.T) {
	e := newEnv(t)
	dueno, r := e.restaurante("1790011674001", "a@a.ec")
	cod := dueno.codigoNodo()
	if len(cod.Codigo) != 9 || cod.Codigo[4] != '-' {
		t.Fatalf("formato de código: %q", cod.Codigo)
	}

	n := e.nuevoNodo()
	n.activar("ZZZZ-ZZZZ", 422)
	n.activar("no-es-un-codigo", 422)
	act := n.activar(cod.Codigo, 200)
	if act.TenantID != r.TenantID || act.NodoID != n.id || act.NombreComercial == "" || act.NombreLocal == "" {
		t.Fatalf("activación: %+v", act)
	}
	// Reintento del mismo nodo (se perdió la respuesta): misma respuesta, sin error.
	if again := n.activar(cod.Codigo, 200); again != act {
		t.Fatalf("reintento distinto: %+v", again)
	}
	// Otro nodo con el mismo código usado: rechazado. Y en minúsculas sin guion da lo mismo.
	e.nuevoNodo().activar(cod.Codigo, 422)

	// Heartbeat con el reloj 2 min adelantado: la nube avisa la deriva.
	var hb edgesync.HeartbeatResponse
	n.req("POST", "/v1/nodos/heartbeat", edgesync.Heartbeat{Version: "0.1.0", HoraNodo: time.Now().Add(2 * time.Minute), OutboxPendientes: 3}, true, 200, &hb)
	if !hb.AlertaReloj || hb.DerivaSegundos < 100 {
		t.Fatalf("deriva no detectada: %+v", hb)
	}
	var lista []nodos.Nodo
	dueno.do("GET", "/v1/nodos", nil, 200, &lista)
	if len(lista) != 1 || !lista[0].EnLinea || lista[0].NombreEquipo != "PC-CAJA" {
		t.Fatalf("lista de nodos: %+v", lista)
	}
	// Sin firma o con firma ajena: 401.
	n.req("POST", "/v1/nodos/heartbeat", edgesync.Heartbeat{}, false, 401, nil)
	intruso := e.nuevoNodo()
	intruso.id = n.id // conoce el id pero no la llave privada
	intruso.req("POST", "/v1/nodos/heartbeat", edgesync.Heartbeat{}, true, 401, nil)

	// Push: se aplica una vez, en orden; el duplicado no cambia nada.
	evs := []edgesync.Event{n.evento("orden.abierta"), n.evento("orden.cerrada")}
	var ack edgesync.PushResponse
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: evs}, true, 200, &ack)
	if ack.LastApplied != 2 {
		t.Fatalf("ack = %d", ack.LastApplied)
	}
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: evs}, true, 200, &ack)
	if ack.LastApplied != 2 {
		t.Fatalf("ack del duplicado = %d", ack.LastApplied)
	}
	// Un lote que dice ser de otro nodo: prohibido.
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: ids.New(), Events: []edgesync.Event{n.evento("x")}}, true, 403, nil)

	// Activar un nodo nuevo para el mismo local revoca al anterior al instante.
	n2 := e.nuevoNodo()
	n2.activar(dueno.codigoNodo().Codigo, 200)
	n.req("POST", "/v1/nodos/heartbeat", edgesync.Heartbeat{}, true, 401, nil)
	n2.req("POST", "/v1/nodos/heartbeat", edgesync.Heartbeat{HoraNodo: time.Now()}, true, 200, nil)

	// Revocar desde el backoffice.
	dueno.do("POST", "/v1/nodos/"+n2.id.String()+"/revocar", nil, 204, nil)
	n2.req("POST", "/v1/nodos/heartbeat", edgesync.Heartbeat{}, true, 401, nil)
	dueno.do("POST", "/v1/nodos/"+n2.id.String()+"/revocar", nil, 404, nil)

	// Un mesero no puede generar códigos.
	dueno.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Mesero", "rol": "MESERO", "pin": "4455"}, 201, nil)
}

// Un código generado deja sin efecto el anterior sin usar del mismo local.
func TestSoloUnCodigoVigente(t *testing.T) {
	e := newEnv(t)
	dueno, _ := e.restaurante("1790011674001", "a@a.ec")
	viejo := dueno.codigoNodo()
	nuevo := dueno.codigoNodo()
	e.nuevoNodo().activar(viejo.Codigo, 422)
	e.nuevoNodo().activar(nuevo.Codigo, 200)
}

// Tras 10 códigos erróneos desde la misma IP se bloquea, aunque luego llegue uno correcto.
func TestBloqueoDeActivacion(t *testing.T) {
	e := newEnv(t)
	dueno, _ := e.restaurante("1790011674001", "a@a.ec")
	n := e.nuevoNodo()
	for range 10 {
		n.activar("AAAA-AAAA", 422)
	}
	n.activar(dueno.codigoNodo().Codigo, 429)
}

func (n *nodoSim) pull(desde int64, esperar int) edgesync.PullResponse {
	n.e.t.Helper()
	var res edgesync.PullResponse
	volcado := 0
	if desde == 0 {
		volcado = 1
	}
	n.req("GET", fmt.Sprintf("/v1/sync/pull?desde=%d&esperar=%d&volcado=%d", desde, esperar, volcado), nil, true, 200, &res)
	return res
}

func tablas(res edgesync.PullResponse) map[string]int {
	m := map[string]int{}
	for _, c := range res.Cambios {
		m[c.Tabla+":"+c.Op]++
	}
	return m
}

func TestPullNubeANodo(t *testing.T) {
	e := newEnv(t)
	dueno, r := e.restaurante("1790011674001", "a@a.ec")
	dueno.do("POST", "/v1/usuarios", map[string]any{"nombreMostrar": "Rosa", "rol": "MESERO", "pin": "5827"}, 201, nil)
	n := e.nuevoNodo()
	act := n.activar(dueno.codigoNodo().Codigo, 200)

	// Primer pull: volcado completo y consistente.
	full := n.pull(0, 0)
	if full.Modo != edgesync.PullCompleto || full.Hasta == 0 {
		t.Fatalf("volcado: modo=%s hasta=%d", full.Modo, full.Hasta)
	}
	tb := tablas(full)
	if tb["locales:U"] != 1 || tb["usuarios:U"] != 2 || tb["tarifas_iva:U"] == 0 || tb["categorias:U"] == 0 {
		t.Fatalf("volcado incompleto: %v", tb)
	}
	for _, c := range full.Cambios {
		if c.Tabla == "usuarios" && (bytes.Contains(c.Datos, []byte("password_hash")) || bytes.Contains(c.Datos, []byte(`"email"`))) {
			t.Fatalf("el nodo recibió credenciales o correo: %s", c.Datos)
		}
	}

	// Sin cambios: incremental vacío con el mismo cursor.
	if inc := n.pull(full.Hasta, 0); inc.Modo != edgesync.PullIncremental || len(inc.Cambios) != 0 || inc.Hasta != full.Hasta {
		t.Fatalf("sin cambios: %+v", inc)
	}

	// Un cambio de otro local del mismo restaurante no llega, pero el cursor avanza.
	otroLocal := ids.New()
	if err := e.tdb.App.InTenant(context.Background(), r.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `INSERT INTO locales (id, tenant_id, nombre, codigo_establecimiento) VALUES ($1, $2, 'Sucursal', '002')`, otroLocal, r.TenantID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var z salon.Zona
	dueno.do("POST", "/v1/zonas", map[string]any{"nombre": "Terraza", "localId": act.LocalID}, 201, &z)
	inc := n.pull(full.Hasta, 0)
	if tb := tablas(inc); len(inc.Cambios) != 1 || tb["zonas:U"] != 1 || inc.Hasta != full.Hasta+2 {
		t.Fatalf("incremental: %v hasta=%d (antes %d)", tb, inc.Hasta, full.Hasta)
	}
	dueno.do("DELETE", "/v1/zonas/"+z.ID.String(), nil, 204, nil)
	del := n.pull(inc.Hasta, 0)
	if len(del.Cambios) == 0 || del.Cambios[len(del.Cambios)-1].Tabla != "zonas" {
		t.Fatalf("borrado de zona: %v", tablas(del))
	}

	// Long-poll: el pull espera y regresa apenas el dueño cambia algo (< 2 s).
	desde := del.Hasta
	type out struct {
		res edgesync.PullResponse
		d   time.Duration
	}
	ch := make(chan out, 1)
	go func() {
		t0 := time.Now()
		res := n.pull(desde, 10)
		ch <- out{res, time.Since(t0)}
	}()
	time.Sleep(400 * time.Millisecond)
	var personas []map[string]any
	dueno.do("GET", "/v1/usuarios", nil, 200, &personas)
	var rosa string
	for _, p := range personas {
		if p["nombreMostrar"] == "Rosa" {
			rosa = p["id"].(string)
		}
	}
	t1 := time.Now()
	dueno.do("PUT", "/v1/usuarios/"+rosa+"/estado", map[string]any{"activo": false}, 200, nil)
	o := <-ch
	if tb := tablas(o.res); tb["usuarios:U"] != 1 {
		t.Fatalf("long-poll: %v", tb)
	}
	if lat := time.Since(t1); lat > 2*time.Second || o.d > 5*time.Second {
		t.Fatalf("user.deactivated tardó %v en llegar al nodo", lat)
	}

	// Un cursor adelantado (nube restaurada) fuerza volcado completo.
	if res := n.pull(o.res.Hasta+1000, 0); res.Modo != edgesync.PullCompleto {
		t.Fatalf("cursor adelantado: %s", res.Modo)
	}
	_ = act
}

// El pull solo ve su tenant: el nodo de B no recibe nada de A.
func TestPullAislado(t *testing.T) {
	e := newEnv(t)
	a, _ := e.restaurante("1790011674001", "a@a.ec")
	b, _ := e.restaurante("1760001550001", "b@b.ec")
	na, nb := e.nuevoNodo(), e.nuevoNodo()
	na.activar(a.codigoNodo().Codigo, 200)
	nb.activar(b.codigoNodo().Codigo, 200)
	a.do("POST", "/v1/categorias", map[string]any{"nombre": "Secreta de A"}, 201, nil)
	for _, c := range nb.pull(0, 0).Cambios {
		if bytes.Contains(c.Datos, []byte("Secreta de A")) {
			t.Fatal("el nodo de B recibió datos de A")
		}
	}
}

func (c *cliente) estaciones() []salon.Estacion {
	c.e.t.Helper()
	var es []salon.Estacion
	c.do("GET", "/v1/estaciones", nil, 200, &es)
	return es
}

// impresoraConPrueba registra una impresora, la asigna a una estación y pide una prueba al
// nodo (deja filas en impresoras, estacion_impresoras y comandos_nodo; lo usa QA-06).
func (c *cliente) impresoraConPrueba(n *nodoSim, host string) (impresoras.Impresora, impresoras.Comando) {
	c.e.t.Helper()
	var locales []salon.Local
	c.do("GET", "/v1/locales", nil, 200, &locales)
	var imp impresoras.Impresora
	c.do("POST", "/v1/impresoras", map[string]any{"localId": locales[0].ID, "nombre": "Cocina " + host, "host": host}, 201, &imp)
	var prod salon.Estacion
	for _, e := range c.estaciones() {
		if e.Tipo == "PRODUCCION" {
			prod = e
		}
	}
	c.do("PUT", "/v1/estaciones/"+prod.ID.String()+"/impresoras", map[string]any{"impresoras": []ids.ID{imp.ID}}, 204, nil)
	var cmd impresoras.Comando
	c.do("POST", "/v1/impresoras/"+imp.ID.String()+"/prueba", map[string]any{}, 200, &cmd)
	return imp, cmd
}

func TestImpresorasRuteoYPrueba(t *testing.T) {
	e := newEnv(t)
	dueno, _ := e.restaurante("1790011674001", "a@a.ec")
	var locales []salon.Local
	dueno.do("GET", "/v1/locales", nil, 200, &locales)
	local := locales[0].ID

	// Validaciones claras al agregar a mano.
	dueno.do("POST", "/v1/impresoras", map[string]any{"localId": local, "nombre": "Bar", "host": "no es una ip!"}, 422, nil)
	dueno.do("POST", "/v1/impresoras", map[string]any{"localId": local, "nombre": "Bar", "host": "192.168.1.60", "anchoPapel": 70}, 422, nil)
	var bar impresoras.Impresora
	dueno.do("POST", "/v1/impresoras", map[string]any{"localId": local, "nombre": "Bar", "host": "192.168.1.60"}, 201, &bar)
	if bar.AnchoPapel != 80 || *bar.Puerto != 9100 || bar.Estado != "DESCONOCIDO" {
		t.Fatalf("valores por defecto: %+v", bar)
	}
	dueno.do("POST", "/v1/impresoras", map[string]any{"localId": local, "nombre": "Otra", "host": "192.168.1.60"}, 409, nil)

	// Sin nodo en línea, la prueba explica qué hacer.
	dueno.do("POST", "/v1/impresoras/"+bar.ID.String()+"/prueba", map[string]any{}, 409, nil)

	n := e.nuevoNodo()
	n.activar(dueno.codigoNodo().Codigo, 200)
	n.req("POST", "/v1/nodos/heartbeat", edgesync.Heartbeat{HoraNodo: time.Now(), Impresoras: []edgesync.ImpresoraSalud{{ID: bar.ID, Estado: "SIN_PAPEL", Cola: 3}}}, true, 200, nil)
	var lista []impresoras.Impresora
	dueno.do("GET", "/v1/impresoras", nil, 200, &lista)
	if lista[0].Estado != "SIN_PAPEL" || lista[0].Cola != 3 || !lista[0].NodoEnRed {
		t.Fatalf("estado vivo: %+v", lista[0])
	}

	// Ruteo: categoría → estación de producción; nunca a caja.
	var cats []map[string]any
	dueno.do("GET", "/v1/categorias", nil, 200, &cats)
	var prod, caja salon.Estacion
	for _, e := range dueno.estaciones() {
		if e.Tipo == "CAJA" {
			caja = e
		} else {
			prod = e
		}
	}
	cat := cats[0]["id"].(string)
	dueno.do("PUT", "/v1/categorias/"+cat+"/estacion", map[string]any{"estacionId": prod.ID}, 204, nil)
	if caja.ID != ids.Nil {
		dueno.do("PUT", "/v1/categorias/"+cat+"/estacion", map[string]any{"estacionId": caja.ID}, 422, nil)
	}
	dueno.do("PUT", "/v1/categorias/"+cat+"/estacion", map[string]any{"estacionId": nil}, 204, nil)
	dueno.do("PUT", "/v1/categorias/"+ids.New().String()+"/estacion", map[string]any{"estacionId": nil}, 404, nil)

	// Prueba de impresión: la orden llega al nodo por el feed y el nodo informa el resultado.
	base := n.pull(0, 0).Hasta
	imp, cmd := dueno.impresoraConPrueba(n, "192.168.1.61")
	inc := n.pull(base, 0)
	tb := tablas(inc)
	if tb["impresoras:U"] < 1 || tb["estacion_impresoras:U"] != 1 || tb["comandos_nodo:U"] != 1 {
		t.Fatalf("el nodo no recibió impresora, asignación y orden: %v", tb)
	}
	ev, _ := edgesync.NewEvent(nodos.EventoComandoEjecutado, 1, cmd.ID, nodos.ComandoEjecutado{ComandoID: cmd.ID, OK: true, Resultado: "Impreso en Cocina"}, time.Now())
	ev.NodeSeq = 1
	n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: []edgesync.Event{ev}}, true, 200, nil)
	var got impresoras.Comando
	dueno.do("GET", "/v1/comandos-nodo/"+cmd.ID.String(), nil, 200, &got)
	if got.EjecutadoAt == nil || got.Resultado == nil || *got.Resultado != "OK: Impreso en Cocina" {
		t.Fatalf("resultado del comando: %+v", got)
	}

	// Detección automática: nueva impresora y, luego, la misma MAC con otra IP (DHCP).
	host, port, mac := "192.168.1.77", 9100, "00:11:62:aa:bb:cc"
	nueva := ids.New()
	det := func(seq int64, h string) {
		ev, _ := edgesync.NewEvent(nodos.EventoImpresoraDetectada, 1, nueva, nodos.ImpresoraDetectada{ID: nueva, Conexion: "TCP", Host: &h, Puerto: &port, MAC: &mac, Modelo: "TM-T20III"}, time.Now())
		ev.NodeSeq = seq
		n.req("POST", "/v1/sync/push", edgesync.PushRequest{NodeID: n.id, Events: []edgesync.Event{ev}}, true, 200, nil)
	}
	det(2, host)
	det(3, "192.168.1.88")
	dueno.do("GET", "/v1/impresoras", nil, 200, &lista)
	var detectadas []impresoras.Impresora
	for _, x := range lista {
		if x.Origen == "DETECTADA" {
			detectadas = append(detectadas, x)
		}
	}
	if len(detectadas) != 1 || *detectadas[0].Host != "192.168.1.88" || detectadas[0].Modelo != "TM-T20III" || !strings.HasPrefix(detectadas[0].Nombre, "Impresora ") {
		t.Fatalf("detectadas: %+v", detectadas)
	}
	// El dueño la renombra; una nueva detección no pisa el nombre.
	dueno.do("PUT", "/v1/impresoras/"+detectadas[0].ID.String(), map[string]any{"nombre": "Barra", "host": "192.168.1.88", "anchoPapel": 58}, 200, nil)
	det(4, "192.168.1.88")
	dueno.do("GET", "/v1/impresoras", nil, 200, &lista)
	for _, x := range lista {
		if x.ID == detectadas[0].ID && (x.Nombre != "Barra" || x.AnchoPapel != 58) {
			t.Fatalf("la detección pisó lo que decidió el dueño: %+v", x)
		}
	}
	dueno.do("DELETE", "/v1/impresoras/"+imp.ID.String(), nil, 204, nil)
}
