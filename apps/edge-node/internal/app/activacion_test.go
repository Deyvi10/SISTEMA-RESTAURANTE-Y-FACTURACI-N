package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/nodoauth"
)

// nubeFalsa imita la API de la nube lo justo para probar al nodo (el lado real se prueba en
// apps/cloud-api/internal/server/nodos_test.go con los mismos tipos de edgesync).
type nubeFalsa struct {
	mu          sync.Mutex
	codigos     map[string]bool // código → vigente
	nodos       map[ids.ID]ed25519.PublicKey
	revocados   map[ids.ID]bool
	perderResp  int // cuántas activaciones responder con error tras registrar (respuesta perdida)
	activ       []map[string]any
	heartbeats  int
	eventos     []edgesync.Event
	tenant, loc ids.ID
}

func newNubeFalsa() *nubeFalsa {
	return &nubeFalsa{codigos: map[string]bool{}, nodos: map[ids.ID]ed25519.PublicKey{}, revocados: map[ids.ID]bool{}, tenant: ids.New(), loc: ids.New()}
}

func (f *nubeFalsa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	problema := func(st int, code string) {
		w.WriteHeader(st)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": st, "code": code, "detail": "falla " + code})
	}
	autenticado := func() (ids.ID, bool) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		id, err := nodoauth.Verify(tok, func(id ids.ID) (ed25519.PublicKey, error) { return f.nodos[id], nil }, time.Now())
		return id, err == nil && !f.revocados[id]
	}
	switch r.URL.Path {
	case "/v1/nodos/activar":
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		f.activ = append(f.activ, in)
		id, _ := ids.Parse(in["nodoId"].(string))
		pub, _ := base64.StdEncoding.DecodeString(in["llavePublica"].(string))
		cod := in["codigo"].(string)
		vigente, existe := f.codigos[cod]
		if !existe || (!vigente && !bytes.Equal(f.nodos[id], pub)) {
			problema(422, "CODIGO_INVALIDO")
			return
		}
		f.codigos[cod] = false
		f.nodos[id] = pub
		if f.perderResp > 0 {
			f.perderResp--
			problema(502, "GATEWAY") // la nube registró pero la respuesta no llegó
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"nodoId": id, "tenantId": f.tenant, "localId": f.loc, "nombreLocal": "Matriz", "nombreComercial": "Don Pepe"})
	case "/v1/nodos/heartbeat":
		if _, ok := autenticado(); !ok {
			problema(401, "NODO_NO_AUTORIZADO")
			return
		}
		f.heartbeats++
		_ = json.NewEncoder(w).Encode(edgesync.HeartbeatResponse{HoraNube: time.Now()})
	case "/v1/sync/push":
		if _, ok := autenticado(); !ok {
			problema(401, "NODO_NO_AUTORIZADO")
			return
		}
		var req edgesync.PushRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.eventos = append(f.eventos, req.Events...)
		_ = json.NewEncoder(w).Encode(edgesync.PushResponse{LastApplied: req.Events[len(req.Events)-1].NodeSeq})
	default:
		problema(404, "NO_ENCONTRADO")
	}
}

func nodoDePrueba(t *testing.T, nube http.Handler) (*App, *httptest.Server) {
	t.Helper()
	cloud := httptest.NewServer(nube)
	t.Cleanup(cloud.Close)
	cfg := Config{DataDir: t.TempDir(), HTTPAddr: "127.0.0.1:0", NubeURL: cloud.URL}
	a, err := New(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.bg = ctx
	t.Cleanup(func() { cancel(); a.wg.Wait(); _ = a.Store.Close() })
	lan := httptest.NewServer(a.Handler())
	t.Cleanup(lan.Close)
	return a, lan
}

func postJSON(t *testing.T, url string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	res, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func TestActivacionDelNodo(t *testing.T) {
	f := newNubeFalsa()
	f.codigos["ABCDEFGH"] = true
	f.perderResp = 1
	a, lan := nodoDePrueba(t, f)

	// Sin activar, la raíz lleva a /activar y la página carga.
	cli := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, _ := cli.Get(lan.URL + "/")
	if loc := res.Header.Get("Location"); loc != "/activar" {
		t.Fatalf("raíz sin activar → %q", loc)
	}
	res, _ = http.Get(lan.URL + "/activar")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(body), "Activa este Nodo Local") {
		t.Fatalf("/activar = %d", res.StatusCode)
	}

	if st, out := postJSON(t, lan.URL+"/v1/activacion", map[string]string{"codigo": "abc"}); st != 422 || out["code"] != "CODIGO_INVALIDO" {
		t.Fatalf("código corto: %d %v", st, out)
	}
	// Primer intento: la nube registra pero la respuesta se pierde (502).
	if st, _ := postJSON(t, lan.URL+"/v1/activacion", map[string]string{"codigo": "abcd-efgh"}); st != 502 {
		t.Fatalf("respuesta perdida: %d", st)
	}
	// Reintento con el mismo código: misma identidad → la nube lo reconoce.
	st, out := postJSON(t, lan.URL+"/v1/activacion", map[string]string{"codigo": "ABCD-EFGH"})
	if st != 200 || out["nombreComercial"] != "Don Pepe" {
		t.Fatalf("reintento: %d %v", st, out)
	}
	if f.activ[0]["nodoId"] != f.activ[1]["nodoId"] || f.activ[0]["llavePublica"] != f.activ[1]["llavePublica"] {
		t.Fatal("el reintento usó otra identidad: el código habría quedado gastado")
	}
	id, _ := a.Identidad(context.Background())
	if !id.Activo() || id.TenantID != f.tenant || id.LocalID != f.loc {
		t.Fatalf("identidad guardada: %+v", id)
	}
	// Ya activo: no se puede re-activar sin revocar.
	if st, out := postJSON(t, lan.URL+"/v1/activacion", map[string]string{"codigo": "ABCDEFGH"}); st != 409 || out["code"] != "YA_ACTIVADO" {
		t.Fatalf("doble activación: %d %v", st, out)
	}

	// La sincronización arrancó sola: heartbeat firmado y outbox enviado.
	esperar(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.heartbeats > 0 })
	ev, _ := edgesync.NewEvent("prueba.creada", 1, ids.New(), map[string]int{"n": 1}, time.Now())
	if err := a.Store.Write(context.Background(), func(tx *store.Tx) error { _, err := a.outbox.Append(context.Background(), tx, ev); return err }); err != nil {
		t.Fatal(err)
	}
	a.pusher.Notify()
	esperar(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.eventos) == 1 })
	if s := a.salud.Copia(); s.UltimaNube.IsZero() || !s.SyncActivo {
		t.Fatalf("salud: %+v", s)
	}

	// La nube revoca el nodo: al siguiente heartbeat se marca revocado y se puede reactivar.
	f.mu.Lock()
	f.revocados[id.NodoID] = true
	f.codigos["JKMNPQRS"] = true
	f.mu.Unlock()
	a.EnviarHeartbeat(context.Background())
	id, _ = a.Identidad(context.Background())
	if id.Activo() || !a.salud.Copia().Revocado {
		t.Fatal("no se marcó revocado")
	}
	esperar(t, func() bool { return !a.salud.Copia().SyncActivo })
	res, _ = cli.Get(lan.URL + "/")
	if loc := res.Header.Get("Location"); loc != "/activar" {
		t.Fatalf("revocado → %q", loc)
	}
	if st, _ := postJSON(t, lan.URL+"/v1/activacion", map[string]string{"codigo": "JKMN-PQRS"}); st != 200 {
		t.Fatalf("reactivación: %d", st)
	}
	nuevo, _ := a.Identidad(context.Background())
	if nuevo.NodoID == id.NodoID {
		t.Fatal("la reactivación debe usar una identidad nueva")
	}
}

func esperar(t *testing.T, cond func() bool) {
	t.Helper()
	for range 250 {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("la condición no se cumplió en 5 s")
}
