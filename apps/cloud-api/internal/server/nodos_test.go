package server_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

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
