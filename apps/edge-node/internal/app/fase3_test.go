package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/rbac"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/secreto"
)

// salonF3 es un restaurante con nodo activado, personal con PIN, mesas y modificadores.
type salonF3 struct {
	*restaurante
	carlos, ana, luis      ids.ID // mesero, mesera, cajero (Luis autoriza anulaciones)
	mesa1, mesa2, mesa3    ids.ID
	grupoTermino, terMedio ids.ID
	terBien, extraQueso    ids.ID
	hamburguesa            ids.ID
}

const pinCarlos, pinAna, pinLuis = "8899", "1024", "7391"

func nuevoSalon(t *testing.T) *salonF3 {
	t.Helper()
	r := nuevoRestaurante(t)
	s := &salonF3{restaurante: r, carlos: ids.New(), ana: ids.New(), luis: ids.New(), mesa1: ids.New(), mesa2: ids.New(), mesa3: ids.New(),
		grupoTermino: ids.New(), terMedio: ids.New(), terBien: ids.New(), extraQueso: ids.New(), hamburguesa: ids.New()}
	pepper := secreto.PepperTenant([]byte("pepper-global-de-pruebas-0123456789"), ids.New())
	hash := func(u ids.ID, pin string) string { h, _ := secreto.HashPIN(pepper, u, pin); return h }
	_, priv, _ := ed25519.GenerateKey(nil)
	zona, grupoExtras, catRapidas := ids.New(), ids.New(), ids.New()
	seed := []struct {
		q    string
		args []any
	}{
		{`INSERT INTO nodo (id, nodo_id, tenant_id, local_id, nube_url, llave_privada, nombre_local, nombre_comercial, activado_at, pin_pepper) VALUES (1, ?, ?, ?, 'http://nube.invalida', ?, 'Matriz', 'Don Pepe', ?, ?)`,
			[]any{ids.New().String(), ids.New().String(), ids.New().String(), []byte(priv), time.Now().UTC().Format(time.RFC3339Nano), pepper}},
		{`INSERT INTO usuarios (id, tenant_id, nombre_mostrar, rol, pin_hash, activo) VALUES (?, 't', 'Carlos M.', 'MESERO', ?, 1), (?, 't', 'Ana R.', 'MESERO', ?, 1), (?, 't', 'Luis P.', 'CAJERO', ?, 1), (?, 't', 'Sin PIN', 'MESERO', NULL, 1)`,
			[]any{s.carlos.String(), hash(s.carlos, pinCarlos), s.ana.String(), hash(s.ana, pinAna), s.luis.String(), hash(s.luis, pinLuis), ids.New().String()}},
		{`INSERT INTO permisos_usuario (tenant_id, usuario_id, permiso, concedido) VALUES ('t', ?, 'ANULAR_ITEM_ENVIADO', 1)`, []any{s.luis.String()}},
		{`INSERT INTO zonas (id, tenant_id, local_id, nombre, orden) VALUES (?, 't', 'l', 'Salón', 1)`, []any{zona.String()}},
		{`INSERT INTO mesas (id, tenant_id, local_id, zona_id, nombre, capacidad) VALUES (?, 't', 'l', ?, 'Mesa 1', 4), (?, 't', 'l', ?, 'Mesa 2', 2), (?, 't', 'l', ?, 'Mesa 3', 6)`,
			[]any{s.mesa1.String(), zona.String(), s.mesa2.String(), zona.String(), s.mesa3.String(), zona.String()}},
		{`INSERT INTO estaciones (id, tenant_id, local_id, nombre, tipo, orden) VALUES (?, 't', 'l', 'Caja', 'CAJA', 9)`, []any{ids.New().String()}},
		{`INSERT INTO tarifas_iva (id, codigo_sri, porcentaje, descripcion, vigente_desde) VALUES ('iva', '4', '15', 'IVA 15 %', '2024-04-01')`, nil},
		{`INSERT INTO locales (id, tenant_id, nombre, codigo_establecimiento, propina_legal_activa, propina_porcentaje, precios_incluyen_iva) VALUES ('l2', 't', 'x', '002', 0, '10', 1)`, nil},
		{`DELETE FROM locales WHERE id = 'l2'`, nil},
		{`UPDATE locales SET propina_legal_activa = 1, propina_porcentaje = '10', precios_incluyen_iva = 1`, nil},
		{`INSERT INTO categorias (id, tenant_id, nombre, estacion_id) VALUES (?, 't', 'Rápidas', ?)`, []any{catRapidas.String(), s.estCocina.String()}},
		{`INSERT INTO productos (id, tenant_id, categoria_id, nombre, alias, precio, tarifa_iva_id) VALUES (?, 't', ?, 'Hamburguesa Don Pepe', 'HB', '9.50', 'iva')`, []any{s.hamburguesa.String(), catRapidas.String()}},
		{`INSERT INTO grupos_modificadores (id, tenant_id, nombre, obligatorio, min, max) VALUES (?, 't', 'Término de la carne', 1, 1, 1), (?, 't', 'Extras', 0, 0, 3)`, []any{s.grupoTermino.String(), grupoExtras.String()}},
		{`INSERT INTO modificadores (id, tenant_id, grupo_id, nombre, precio_adicional) VALUES (?, 't', ?, 'Término medio', '0'), (?, 't', ?, 'Bien cocido', '0'), (?, 't', ?, 'Extra queso', '1.00')`,
			[]any{s.terMedio.String(), s.grupoTermino.String(), s.terBien.String(), s.grupoTermino.String(), s.extraQueso.String(), grupoExtras.String()}},
		{`INSERT INTO producto_grupos_modificadores (tenant_id, producto_id, grupo_id) VALUES ('t', ?, ?), ('t', ?, ?)`, []any{s.hamburguesa.String(), s.grupoTermino.String(), s.hamburguesa.String(), grupoExtras.String()}},
	}
	if err := r.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		for _, x := range seed {
			if _, err := tx.Exec(x.q, x.args...); err != nil {
				return fmt.Errorf("%s: %w", x.q, err)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return s
}

// telefono es la app de meseros vista desde la API del nodo.
type telefono struct {
	t       *testing.T
	s       *salonF3
	id      ids.ID
	priv    ed25519.PrivateKey
	disp    string // token del dispositivo
	usuario string // token del usuario (tras el PIN)
}

func (s *salonF3) emparejar(t *testing.T, nombre string) *telefono {
	t.Helper()
	e, err := s.a.NuevoEmparejamiento(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(e.Enlace, "restpos://emparejar?c="+e.Codigo) || !strings.HasPrefix(e.QR, "data:image/png;base64,") {
		t.Fatalf("emparejamiento: %+v", e)
	}
	pub, priv, _ := ed25519.GenerateKey(nil)
	ph := &telefono{t: t, s: s, id: ids.New(), priv: priv}
	st, out := ph.req("POST", "/v1/dispositivos/emparejar", map[string]any{"codigo": e.Codigo, "dispositivoId": ph.id, "llavePublica": base64.StdEncoding.EncodeToString(pub), "nombre": nombre, "plataforma": "android 14", "versionApp": "0.1.0"})
	if st != 200 || out["restaurante"] != "Don Pepe" {
		t.Fatalf("emparejar: %d %v", st, out)
	}
	ph.abrirSesion()
	return ph
}

func (p *telefono) abrirSesion() {
	p.t.Helper()
	st, d := p.req("GET", "/v1/dispositivos/desafio?dispositivoId="+p.id.String(), nil)
	if st != 200 {
		p.t.Fatalf("desafío: %d %v", st, d)
	}
	nonce := d["nonce"].(string)
	firma := ed25519.Sign(p.priv, MensajeDesafio(nonce))
	st, s := p.req("POST", "/v1/dispositivos/sesion", map[string]any{"dispositivoId": p.id, "nonce": nonce, "firma": base64.StdEncoding.EncodeToString(firma)})
	if st != 200 {
		p.t.Fatalf("sesión del dispositivo: %d %v", st, s)
	}
	p.disp = s["token"].(string)
}

func (p *telefono) entrar(usuario ids.ID, pin string) (int, map[string]any) {
	st, out := p.req("POST", "/v1/sesiones", map[string]any{"usuarioId": usuario, "pin": pin})
	if st == 200 {
		p.usuario = out["token"].(string)
	}
	return st, out
}

func (p *telefono) req(method, path string, body any) (int, map[string]any) {
	p.t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	r, _ := http.NewRequest(method, p.s.lan.URL+path, rdr)
	r.Header.Set("Content-Type", "application/json")
	// Como la app: ambas credenciales en una sola cabecera.
	var cred []string
	if p.disp != "" {
		cred = append(cred, "Dispositivo "+p.disp)
	}
	if p.usuario != "" {
		cred = append(cred, "Usuario "+p.usuario)
	}
	if len(cred) > 0 {
		r.Header.Set("Authorization", strings.Join(cred, ", "))
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		p.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]any{"_raw": string(raw)}
	}
	return res.StatusCode, out
}

func (p *telefono) enviar(clave string, mesa ids.ID, lineas ...map[string]any) (int, map[string]any) {
	return p.req("POST", "/v1/ordenes/enviar", map[string]any{"idempotencyKey": clave, "mesaId": mesa, "ordenId": ids.New(), "lineas": lineas})
}

func plato(p ids.ID, cant string, mods ...ids.ID) map[string]any {
	if mods == nil {
		mods = []ids.ID{}
	}
	return map[string]any{"id": ids.New(), "productoId": p, "cantidad": cant, "modificadores": mods}
}

func TestEmparejarYEntrarConPIN(t *testing.T) {
	s := nuevoSalon(t)
	p := s.emparejar(t, "Tablet de Carlos")

	// La cuadrícula de personal: meseros y cajero con PIN (el que no tiene PIN no aparece).
	st, raw := p.req("GET", "/v1/personal", nil)
	if st != 200 {
		t.Fatalf("personal: %d %v", st, raw)
	}
	var gente []Persona
	b, _ := json.Marshal(raw["_raw"])
	_ = b
	res, _ := http.NewRequest("GET", s.lan.URL+"/v1/personal", nil)
	res.Header.Set("Authorization", "Dispositivo "+p.disp)
	rr, _ := http.DefaultClient.Do(res)
	_ = json.NewDecoder(rr.Body).Decode(&gente)
	_ = rr.Body.Close()
	if len(gente) != 3 || gente[0].Rol != "MESERO" {
		t.Fatalf("cuadrícula: %+v", gente)
	}
	// Cada persona trae sus permisos: el mesero toma pedidos pero no anula sin supervisor.
	if !slices.Contains(gente[0].Permisos, rbac.TomarPedido) || slices.Contains(gente[0].Permisos, rbac.AnularItemEnviado) {
		t.Fatalf("permisos del mesero: %v", gente[0].Permisos)
	}

	// Sin sesión de usuario no se ve el salón.
	if st, _ := p.req("GET", "/v1/salon", nil); st != 401 {
		t.Fatalf("salón sin PIN: %d", st)
	}
	if st, out := p.entrar(s.carlos, "1111"); st != 401 || out["code"] != "PIN_INCORRECTO" {
		t.Fatalf("PIN erróneo: %d %v", st, out)
	}
	if st, out := p.entrar(s.carlos, pinCarlos); st != 200 {
		t.Fatalf("PIN correcto: %d %v", st, out)
	}
	if st, out := p.req("GET", "/v1/yo", nil); st != 200 || out["nombre"] != "Carlos M." {
		t.Fatalf("yo: %d %v", st, out)
	}
	if st, _ := p.req("GET", "/v1/salon", nil); st != 200 {
		t.Fatalf("salón: %d", st)
	}

	// 5 PIN erróneos seguidos bloquean el DISPOSITIVO 5 minutos (aunque luego acierte).
	for range 5 {
		p.entrar(s.ana, "0000")
	}
	if st, out := p.entrar(s.ana, pinAna); st != 429 || out["code"] != "PIN_BLOQUEADO" {
		t.Fatalf("bloqueo: %d %v", st, out)
	}
	// Otro teléfono no queda bloqueado.
	otro := s.emparejar(t, "Teléfono de Ana")
	if st, _ := otro.entrar(s.ana, pinAna); st != 200 {
		t.Fatal("el bloqueo alcanzó a otro dispositivo")
	}
	var auditoria int
	_ = s.a.Store.Read().QueryRow(`SELECT count(*) FROM auditoria WHERE accion = 'PIN_BLOQUEO_DISPOSITIVO'`).Scan(&auditoria)
	if auditoria != 1 {
		t.Fatalf("auditoría de bloqueo: %d", auditoria)
	}

	// El código de emparejamiento es de un solo uso.
	e, _ := s.a.NuevoEmparejamiento(context.Background())
	pub, _, _ := ed25519.GenerateKey(nil)
	uno := &telefono{t: t, s: s}
	body := map[string]any{"codigo": e.Codigo, "dispositivoId": ids.New(), "llavePublica": base64.StdEncoding.EncodeToString(pub)}
	if st, _ := uno.req("POST", "/v1/dispositivos/emparejar", body); st != 200 {
		t.Fatal("primer uso")
	}
	body["dispositivoId"] = ids.New()
	if st, _ := uno.req("POST", "/v1/dispositivos/emparejar", body); st != 422 {
		t.Fatal("el código se usó dos veces")
	}
	// Una firma falsa no abre sesión.
	st, d := p.req("GET", "/v1/dispositivos/desafio?dispositivoId="+p.id.String(), nil)
	_, falsa, _ := ed25519.GenerateKey(nil)
	if st, _ = p.req("POST", "/v1/dispositivos/sesion", map[string]any{"dispositivoId": p.id, "nonce": d["nonce"], "firma": base64.StdEncoding.EncodeToString(ed25519.Sign(falsa, MensajeDesafio(d["nonce"].(string))))}); st != 401 {
		t.Fatalf("firma falsa aceptada: %d", st)
	}
}

// QA-01: 100 pedidos simultáneos de la misma mesa → exactamente 1 gana.
func TestBloqueoDeMesaExclusivo(t *testing.T) {
	s := nuevoSalon(t)
	a, b := s.emparejar(t, "A"), s.emparejar(t, "B")
	a.entrar(s.carlos, pinCarlos)
	b.entrar(s.ana, pinAna)
	var ganan, pierden atomic.Int32
	var wg sync.WaitGroup
	for i := range 100 {
		p := a
		if i%2 == 1 {
			p = b
		}
		wg.Go(func() {
			st, out := p.req("POST", "/v1/mesas/"+s.mesa1.String()+"/bloqueo", nil)
			switch {
			case st == 200:
				ganan.Add(1)
			case st == 409 && out["code"] == "LOCKED_BY":
				pierden.Add(1)
			default:
				t.Errorf("respuesta inesperada: %d %v", st, out)
			}
		})
	}
	wg.Wait()
	// El ganador puede repetir su propio pedido (renueva), así que se cuenta por usuario.
	st, sal := a.req("GET", "/v1/salon", nil)
	if st != 200 {
		t.Fatal(st)
	}
	var dueno string
	for _, m := range sal["mesas"].([]any) {
		mm := m.(map[string]any)
		if mm["id"] == s.mesa1.String() {
			dueno = mm["bloqueo"].(map[string]any)["usuarioNombre"].(string)
		}
	}
	perdedor := b
	if dueno == "Ana R." {
		perdedor = a
	}
	if st, out := perdedor.req("POST", "/v1/mesas/"+s.mesa1.String()+"/bloqueo", nil); st != 409 || !strings.Contains(out["detail"].(string), dueno) {
		t.Fatalf("el otro mesero no ve «Editando: %s»: %d %v", dueno, st, out)
	}
	if ganan.Load()+pierden.Load() != 100 || pierden.Load() == 0 {
		t.Fatalf("ganan %d, pierden %d", ganan.Load(), pierden.Load())
	}
	// Sin latido la mesa se libera sola a los 45 s.
	if err := s.a.Store.Write(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE bloqueos_mesa SET ultimo_heartbeat_at = ?`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	s.a.BarrerBloqueosVencidos(context.Background())
	if st, _ := perdedor.req("POST", "/v1/mesas/"+s.mesa1.String()+"/bloqueo", nil); st != 200 {
		t.Fatal("la mesa no se liberó al vencer el latido")
	}
}

func TestPedidoCompleto(t *testing.T) {
	s := nuevoSalon(t)
	p := s.emparejar(t, "Tablet")
	p.entrar(s.carlos, pinCarlos)

	// La hamburguesa exige término: sin él, el nodo lo explica.
	if st, out := p.enviar("envio-sin-termino", s.mesa1, plato(s.hamburguesa, "1")); st != 422 || !strings.Contains(out["detail"].(string), "término") {
		t.Fatalf("modificador obligatorio: %d %v", st, out)
	}
	if st, out := p.enviar("envio-dos-terminos", s.mesa1, plato(s.hamburguesa, "1", s.terMedio, s.terBien)); st != 422 {
		t.Fatalf("máximo del grupo: %d %v", st, out)
	}
	p.req("POST", "/v1/mesas/"+s.mesa1.String()+"/bloqueo", nil)
	// Dos hamburguesas distintas + cervezas; el postre queda en espera (se marcha después).
	postre := plato(s.postre, "2")
	postre["tiempo"], postre["enEspera"] = "POSTRE", true
	lineas := []map[string]any{plato(s.hamburguesa, "1", s.terMedio, s.extraQueso), plato(s.hamburguesa, "1", s.terBien), plato(s.cerveza, "3"), postre}
	lineas[1]["nota"] = "sin cebolla"
	var primera map[string]any
	for range 5 { // QA-08: reenviar 5 veces → 1 sola comanda
		st, out := p.enviar("envio-mesa-1", s.mesa1, lineas...)
		if st != 200 {
			t.Fatalf("enviar: %d %v", st, out)
		}
		if primera == nil {
			primera = out
		}
	}
	var comandas, lineasN int
	_ = s.a.Store.Read().QueryRow(`SELECT count(*) FROM comandas`).Scan(&comandas)
	_ = s.a.Store.Read().QueryRow(`SELECT count(*) FROM orden_lineas`).Scan(&lineasN)
	if comandas != 1 || lineasN != 4 {
		t.Fatalf("QA-08: %d comandas, %d líneas", comandas, lineasN)
	}
	orden := primera["orden"].(map[string]any)
	if orden["total"] != "36.00" { // 10.50 + 9.50 + 3×3.00 + 2×3.50 (el postre en espera también cuenta)
		t.Fatalf("total = %v", orden["total"])
	}
	esperar(t, func() bool { return len(s.cocina.Trabajos()) == 1 && len(s.bar.Trabajos()) == 1 })
	cocina := s.cocina.Textos()[0]
	for _, want := range []string{"Hamburguesa Don Pepe", "Término medio", "Extra queso", "Bien cocido", "sin cebolla", "Mesa 1"} {
		if !strings.Contains(cocina, want) {
			t.Errorf("la comanda de cocina no trae %q:\n%s", want, cocina)
		}
	}
	if strings.Contains(cocina, "Tres leches") {
		t.Fatal("el postre en espera se imprimió")
	}
	// El bloqueo se liberó al enviar y la mesa figura ocupada.
	_, sal := p.req("GET", "/v1/salon", nil)
	for _, m := range sal["mesas"].([]any) {
		mm := m.(map[string]any)
		if mm["id"] == s.mesa1.String() && (mm["estado"] != "OCUPADA" || mm["bloqueo"] != nil || mm["total"] != "36.00") {
			t.Fatalf("mesa 1 en el mapa: %v", mm)
		}
	}
	ordenID := orden["id"].(string)
	// «Marchar postres».
	if st, out := p.req("POST", "/v1/ordenes/"+ordenID+"/marchar", map[string]any{"idempotencyKey": "marchar-postre-1", "tiempo": "POSTRE"}); st != 200 {
		t.Fatalf("marchar: %d %v", st, out)
	}
	esperar(t, func() bool {
		return len(s.cocina.Trabajos()) == 2 && strings.Contains(s.cocina.Textos()[1], "Tres leches")
	})

	// Pre-cuenta: base, IVA 15 % incluido y propina 10 % sobre la base.
	st, pc := p.req("POST", "/v1/ordenes/"+ordenID+"/precuenta", map[string]any{"personas": 2})
	if st != 200 {
		t.Fatalf("pre-cuenta: %d %v", st, pc)
	}
	tot := pc["totales"].(map[string]any)
	// 36.00 con IVA → base 31.30, IVA 4.70, propina 3.13, total 39.13
	if tot["subtotal"] != "31.30" || tot["iva"] != "4.70" || tot["propina"] != "3.13" || tot["total"] != "39.13" {
		t.Fatalf("totales: %v", tot)
	}
	esperar(t, func() bool {
		tx := append(s.cocina.Textos(), s.bar.Textos()...)
		for _, x := range tx {
			if strings.Contains(x, "SIN VALOR TRIBUTARIO") {
				return true
			}
		}
		return false
	})
	_, sal = p.req("GET", "/v1/salon", nil)
	for _, m := range sal["mesas"].([]any) {
		if mm := m.(map[string]any); mm["id"] == s.mesa1.String() && mm["estado"] != "POR_PAGAR" {
			t.Fatalf("tras la pre-cuenta: %v", mm["estado"])
		}
	}
}

func TestMoverUnirYTransferir(t *testing.T) {
	s := nuevoSalon(t)
	p := s.emparejar(t, "Tablet")
	p.entrar(s.carlos, pinCarlos)
	st0, o1 := p.enviar("envio-m1", s.mesa1, plato(s.cerveza, "2"), plato(s.ceviche, "1"))
	if st0 != 200 {
		t.Fatalf("enviar: %d %v", st0, o1)
	}
	ord1 := o1["orden"].(map[string]any)
	ordenID := ord1["id"].(string)
	lineas := ord1["lineas"].([]any)
	cevicheLinea := lineas[1].(map[string]any)["id"]

	// Un mesero sin permiso de transferir no puede (MESERO: configurable, apagado por defecto).
	if st, out := p.req("POST", "/v1/ordenes/"+ordenID+"/mover", map[string]any{"mesaId": s.mesa2}); st != 403 {
		t.Fatalf("mesero sin permiso: %d %v", st, out)
	}
	caja := s.emparejar(t, "Caja")
	caja.entrar(s.luis, pinLuis)
	// Mover el ceviche a la mesa 2 (se crea su orden).
	st, out := caja.req("POST", "/v1/ordenes/"+ordenID+"/mover", map[string]any{"mesaId": s.mesa2, "lineas": []any{cevicheLinea}})
	if st != 200 || len(out["lineas"].([]any)) != 1 {
		t.Fatalf("mover líneas: %d %v", st, out)
	}
	orden2 := out["id"].(string)
	// Mover toda la orden 1 a la mesa 3 (libre).
	if st, out := caja.req("POST", "/v1/ordenes/"+ordenID+"/mover", map[string]any{"mesaId": s.mesa3}); st != 200 {
		t.Fatalf("mover orden: %d %v", st, out)
	}
	// Unir la mesa 2 con la 3.
	st, out = caja.req("POST", "/v1/ordenes/"+orden2+"/unir", map[string]any{"ordenDestino": ordenID})
	if st != 200 || len(out["lineas"].([]any)) != 2 {
		t.Fatalf("unir: %d %v", st, out)
	}
	// Pasar la mesa a Ana.
	if st, out := caja.req("POST", "/v1/ordenes/"+ordenID+"/transferir", map[string]any{"meseroId": s.ana}); st != 200 || out["meseroNombre"] != "Ana R." {
		t.Fatalf("transferir: %d %v", st, out)
	}
	_, sal := caja.req("GET", "/v1/salon", nil)
	estados := map[string]string{}
	for _, m := range sal["mesas"].([]any) {
		mm := m.(map[string]any)
		estados[mm["nombre"].(string)] = mm["estado"].(string)
	}
	if estados["Mesa 1"] != "LIBRE" || estados["Mesa 2"] != "LIBRE" || estados["Mesa 3"] != "OCUPADA" {
		t.Fatalf("estados: %v", estados)
	}
	var audit int
	_ = s.a.Store.Read().QueryRow(`SELECT count(*) FROM auditoria WHERE accion IN ('LINEAS_MOVIDAS','ORDEN_MOVIDA','MESAS_UNIDAS','MESA_TRANSFERIDA')`).Scan(&audit)
	if audit != 4 {
		t.Fatalf("auditoría: %d", audit)
	}
}

func TestAnularConPINDeSupervisor(t *testing.T) {
	s := nuevoSalon(t)
	p := s.emparejar(t, "Tablet")
	p.entrar(s.carlos, pinCarlos)
	_, out := p.enviar("anular-1", s.mesa1, plato(s.cerveza, "2"), plato(s.ceviche, "1"))
	o := out["orden"].(map[string]any)
	ordenID := o["id"].(string)
	cerveza := o["lineas"].([]any)[0].(map[string]any)["id"]
	esperar(t, func() bool { return len(s.bar.Trabajos()) == 1 })

	// Carlos no tiene el permiso: se le pide el PIN de un supervisor.
	if st, out := p.req("POST", "/v1/ordenes/"+ordenID+"/anular", map[string]any{"lineas": []any{cerveza}, "motivo": "El cliente cambió de idea"}); st != 403 || out["code"] != "REQUIERE_SUPERVISOR" {
		t.Fatalf("sin permiso: %d %v", st, out)
	}
	// Luis (con permiso) escribe su PIN en el mismo teléfono: autoriza ESTA orden.
	st, au := p.req("POST", "/v1/autorizaciones", map[string]any{"usuarioId": s.luis, "pin": pinLuis, "accion": "ANULAR_ITEM_ENVIADO", "referencia": ordenID})
	if st != 200 {
		t.Fatalf("autorizar: %d %v", st, au)
	}
	// Ana (sin permiso) no puede autorizar.
	if st, _ := p.req("POST", "/v1/autorizaciones", map[string]any{"usuarioId": s.ana, "pin": pinAna, "accion": "ANULAR_ITEM_ENVIADO", "referencia": ordenID}); st != 403 {
		t.Fatalf("Ana autorizó: %d", st)
	}
	body := map[string]any{"lineas": []any{cerveza}, "motivo": "El cliente cambió de idea", "sePreparo": false, "autorizacion": au["token"]}
	if st, out := p.req("POST", "/v1/ordenes/"+ordenID+"/anular", body); st != 200 || out["total"] != "12.50" {
		t.Fatalf("anular: %d %v", st, out)
	}
	esperar(t, func() bool { return len(s.bar.Trabajos()) == 2 && strings.Contains(s.bar.Textos()[1], "ANULACIÓN") })
	// La autorización era de un solo uso.
	if st, _ := p.req("POST", "/v1/ordenes/"+ordenID+"/anular", body); st != 403 {
		t.Fatalf("la autorización se usó dos veces: %d", st)
	}
	var autorizo string
	_ = s.a.Store.Read().QueryRow(`SELECT anulada_autoriza FROM orden_lineas WHERE id = ?`, cerveza).Scan(&autorizo)
	if autorizo != s.luis.String() {
		t.Fatalf("auditoría sin el supervisor: %q", autorizo)
	}
}

// Revocar un teléfono desde la nube lo desconecta al instante (F3-02: < 2 s).
func TestRevocarDispositivo(t *testing.T) {
	s := nuevoSalon(t)
	p := s.emparejar(t, "Robado")
	p.entrar(s.carlos, pinCarlos)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(s.lan.URL, "http")+"/v1/ws?dispositivo="+p.disp, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.CloseNow() }()
	esperar(t, func() bool { return s.a.hub.Conectados() == 1 })
	t0 := time.Now()
	s.a.revocarDispositivos(ctx, []edgesync.Cambio{{Tabla: "dispositivos", Op: "U", Datos: []byte(`{"id":"` + p.id.String() + `","estado":"REVOCADO"}`)}})
	_, raw, err := ws.Read(ctx)
	if err == nil {
		var sob eventos.Sobre
		_ = json.Unmarshal(raw, &sob)
		if sob.Type != eventos.TipoDeviceRevoked {
			t.Fatalf("primer mensaje: %s", sob.Type)
		}
	}
	esperar(t, func() bool { return s.a.hub.Conectados() == 0 })
	if d := time.Since(t0); d > 2*time.Second {
		t.Fatalf("desconexión en %v", d)
	}
	if st, _ := p.req("GET", "/v1/salon", nil); st != 401 {
		t.Fatalf("el teléfono revocado sigue entrando: %d", st)
	}
}
