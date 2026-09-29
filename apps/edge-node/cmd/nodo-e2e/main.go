// Command nodo-e2e arranca un Nodo Local efímero para las pruebas E2E de la caja (F4-16):
// base SQLite temporal ya activada, un restaurante sembrado (personal con PIN, salón, carta,
// caja y métodos de pago) e impresoras térmicas simuladas en memoria. No necesita la nube.
//
//	go run ./apps/edge-node/cmd/nodo-e2e
//
// Variables: RESTPOS_E2E_HTTP (caja y API del nodo, por defecto 127.0.0.1:7181) y
// RESTPOS_E2E_AUX (ayudas de la prueba, por defecto 127.0.0.1:7182):
//
//	GET  /impresiones  texto de cada trabajo recibido por impresora
//	GET  /datos        IDs sembrados (usuarios, mesas, productos)
//	POST /ordenes      {"mesa": "Mesa 1", "platos": [["Cerveza", 2]]}: el mesero la envía desde
//	                   un teléfono emparejado de verdad (la PC de caja tiene su propia sesión)
//
// Todos los datos son ficticios y se borran al salir.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/app"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/escpos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/secreto"
)

// Datos sembrados que la prueba necesita conocer.
type Datos struct {
	Cajera     Persona           `json:"cajera"`
	Supervisor Persona           `json:"supervisor"`
	Mesero     Persona           `json:"mesero"`
	Mesas      map[string]string `json:"mesas"`     // nombre → id
	Productos  map[string]string `json:"productos"` // nombre → id
}

type Persona struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
	PIN    string `json:"pin"`
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := correr(log); err != nil {
		log.Error("nodo-e2e", "err", err)
		os.Exit(1)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func correr(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dir, err := os.MkdirTemp("", "nodo-e2e-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	impresoras := map[string]*termica{"Cocina": {}, "Caja": {}}
	for _, t := range impresoras {
		if err := t.encender(); err != nil {
			return err
		}
		defer t.apagar()
	}
	// La nube no existe: la sincronización falla rápido y reintenta en segundo plano.
	cfg := app.Config{DataDir: dir, HTTPAddr: env("RESTPOS_E2E_HTTP", "127.0.0.1:7181"), NubeURL: "http://127.0.0.1:9", SinMDNS: true}
	a, err := app.New(ctx, cfg, log)
	if err != nil {
		return err
	}
	datos, err := sembrar(ctx, a, impresoras)
	if err != nil {
		return fmt.Errorf("siembra: %w", err)
	}

	aux := http.NewServeMux()
	responder := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	aux.HandleFunc("GET /datos", func(w http.ResponseWriter, _ *http.Request) { responder(w, datos) })
	aux.HandleFunc("GET /impresiones", func(w http.ResponseWriter, _ *http.Request) {
		out := map[string][]string{}
		for n, t := range impresoras {
			out[n] = t.textos()
		}
		responder(w, out)
	})
	tel := &telefono{base: "http://" + cfg.HTTPAddr, a: a, mesero: datos.Mesero}
	aux.HandleFunc("POST /ordenes", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Mesa   string  `json:"mesa"`
			Platos [][]any `json:"platos"`
		}
		dec := json.NewDecoder(r.Body)
		dec.UseNumber() // cantidades enteras, sin pasar por float
		if err := dec.Decode(&in); err != nil || datos.Mesas[in.Mesa] == "" {
			http.Error(w, "mesa o platos inválidos", http.StatusBadRequest)
			return
		}
		var lineas []map[string]any
		for _, p := range in.Platos {
			nombre, _ := p[0].(string)
			num, _ := p[1].(json.Number)
			n, err := num.Int64()
			if datos.Productos[nombre] == "" || err != nil || n <= 0 {
				http.Error(w, "plato desconocido: "+nombre, http.StatusBadRequest)
				return
			}
			lineas = append(lineas, map[string]any{"id": ids.New(), "productoId": datos.Productos[nombre], "cantidad": strconv.FormatInt(n, 10), "modificadores": []any{}})
		}
		st, out, err := tel.enviar(r.Context(), map[string]any{"idempotencyKey": ids.New(), "mesaId": datos.Mesas[in.Mesa], "ordenId": ids.New(), "lineas": lineas})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(st)
		_, _ = w.Write(out)
	})
	srv := &http.Server{Addr: env("RESTPOS_E2E_AUX", "127.0.0.1:7182"), Handler: aux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("servidor auxiliar", "err", err)
			stop()
		}
	}()
	defer func() { _ = srv.Close() }()
	fmt.Printf("nodo-e2e listo: caja en http://%s/pos/ · ayudas en http://%s\n", cfg.HTTPAddr, srv.Addr)
	return a.Run(ctx)
}

func sembrar(ctx context.Context, a *app.App, imp map[string]*termica) (Datos, error) {
	tenant, local := ids.New(), ids.New()
	t, l := tenant.String(), local.String()
	pepper := secreto.PepperTenant([]byte("pepper-e2e-ficticio-0123456789abcdef"), tenant)
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return Datos{}, err
	}
	d := Datos{
		Cajera:     Persona{ids.New().String(), "Eva Caja", "1357"},
		Supervisor: Persona{ids.New().String(), "Sol Admin", "2468"},
		Mesero:     Persona{ids.New().String(), "Carlos Mesa", "8899"},
		Mesas:      map[string]string{},
		Productos:  map[string]string{},
	}
	hash := func(p Persona) string {
		h, _ := secreto.HashPIN(pepper, ids.MustParse(p.ID), p.PIN)
		return h
	}
	zona, estCocina, estCaja, cat := ids.New(), ids.New(), ids.New(), ids.New()
	impCocina, impCaja := ids.New(), ids.New()
	hp := func(x *termica) (string, string) { h, p, _ := net.SplitHostPort(x.addr); return h, p }
	hc, pc := hp(imp["Cocina"])
	hk, pk := hp(imp["Caja"])
	q := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO nodo (id, nodo_id, tenant_id, local_id, nube_url, llave_privada, nombre_local, nombre_comercial, activado_at, pin_pepper) VALUES (1, ?, ?, ?, 'http://127.0.0.1:9', ?, 'Matriz', 'Restaurante de Prueba', ?, ?)`,
			[]any{ids.New().String(), t, l, []byte(priv), time.Now().UTC().Format(time.RFC3339Nano), pepper}},
		{`INSERT INTO locales (id, tenant_id, nombre, codigo_establecimiento, propina_legal_activa, propina_porcentaje, precios_incluyen_iva) VALUES (?, ?, 'Matriz', '001', 1, '10', 1)`, []any{l, t}},
		{`INSERT INTO usuarios (id, tenant_id, nombre_mostrar, rol, pin_hash, activo) VALUES (?, ?, ?, 'CAJERO', ?, 1), (?, ?, ?, 'ADMIN', ?, 1), (?, ?, ?, 'MESERO', ?, 1)`,
			[]any{d.Cajera.ID, t, d.Cajera.Nombre, hash(d.Cajera), d.Supervisor.ID, t, d.Supervisor.Nombre, hash(d.Supervisor), d.Mesero.ID, t, d.Mesero.Nombre, hash(d.Mesero)}},
		{`INSERT INTO zonas (id, tenant_id, local_id, nombre, orden) VALUES (?, ?, ?, 'Salón', 1)`, []any{zona.String(), t, l}},
		{`INSERT INTO estaciones (id, tenant_id, local_id, nombre, tipo, es_defecto, orden) VALUES (?, ?, ?, 'Cocina', 'PRODUCCION', 1, 1), (?, ?, ?, 'Caja', 'CAJA', 0, 9)`,
			[]any{estCocina.String(), t, l, estCaja.String(), t, l}},
		{`INSERT INTO impresoras (id, tenant_id, local_id, nombre, host, puerto, ancho_papel) VALUES (?, ?, ?, 'Cocina', ?, ?, 80), (?, ?, ?, 'Caja', ?, ?, 80)`,
			[]any{impCocina.String(), t, l, hc, pc, impCaja.String(), t, l, hk, pk}},
		{`INSERT INTO estacion_impresoras (tenant_id, estacion_id, impresora_id, local_id) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
			[]any{t, estCocina.String(), impCocina.String(), l, t, estCaja.String(), impCaja.String(), l}},
		{`INSERT INTO tarifas_iva (id, codigo_sri, porcentaje, descripcion, vigente_desde) VALUES ('iva15', '4', '15', 'IVA 15 %', '2024-04-01')`, nil},
		{`INSERT INTO categorias (id, tenant_id, nombre, estacion_id) VALUES (?, ?, 'Carta', ?)`, []any{cat.String(), t, estCocina.String()}},
		{`INSERT INTO cajas (id, tenant_id, local_id, nombre, estacion_id) VALUES (?, ?, ?, 'Caja 1', ?)`, []any{ids.New().String(), t, l, estCaja.String()}},
		{`INSERT INTO metodos_pago (id, tenant_id, nombre, tipo, codigo_forma_pago_sri, abre_cajon, pide_referencia, icono, orden) VALUES
			(?, ?, 'Efectivo', 'EFECTIVO', '01', 1, 0, 'efectivo', 1),
			(?, ?, 'Tarjeta crédito', 'TARJETA_CREDITO', '19', 0, 1, 'tarjeta', 2),
			(?, ?, 'Transferencia', 'TRANSFERENCIA', '20', 0, 0, 'transferencia', 3)`,
			[]any{ids.New().String(), t, ids.New().String(), t, ids.New().String(), t}},
		{`INSERT INTO motivos_descuento (id, tenant_id, nombre, tipo) VALUES (?, ?, 'Cliente frecuente', 'DESCUENTO')`, []any{ids.New().String(), t}},
		{`INSERT INTO parametros_globales (clave, valor, descripcion) VALUES ('consumidor_final_maximo', '50.00', 'Límite de consumidor final (DP-07)')`, nil},
	}
	for i := 1; i <= 6; i++ {
		id := ids.New().String()
		d.Mesas[fmt.Sprintf("Mesa %d", i)] = id
		q = append(q, struct {
			sql  string
			args []any
		}{`INSERT INTO mesas (id, tenant_id, local_id, zona_id, nombre, capacidad) VALUES (?, ?, ?, ?, ?, 4)`, []any{id, t, l, zona.String(), fmt.Sprintf("Mesa %d", i)}})
	}
	for _, p := range []struct{ nombre, precio string }{
		{"Ceviche de camarón", "12.50"}, {"Arroz marinero", "14.00"}, {"Pizza personal", "8.00"}, {"Cerveza", "3.00"}, {"Jugo natural", "2.50"},
	} {
		id := ids.New().String()
		d.Productos[p.nombre] = id
		q = append(q, struct {
			sql  string
			args []any
		}{`INSERT INTO productos (id, tenant_id, categoria_id, nombre, precio, tarifa_iva_id) VALUES (?, ?, ?, ?, ?, 'iva15')`, []any{id, t, cat.String(), p.nombre, p.precio}})
	}
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		for _, x := range q {
			if _, err := tx.Exec(x.sql, x.args...); err != nil {
				return fmt.Errorf("%.60s…: %w", x.sql, err)
			}
		}
		return nil
	})
	return d, err
}

// telefono es la app de meseros vista desde la API del nodo: se empareja con un código,
// abre su sesión firmando el desafío con su llave Ed25519 y el mesero entra con su PIN.
type telefono struct {
	mu      sync.Mutex
	base    string
	a       *app.App
	mesero  Persona
	disp    string
	usuario string
}

func (t *telefono) req(ctx context.Context, method, path string, body any) (int, []byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	r, err := http.NewRequestWithContext(ctx, method, t.base+path, bytes.NewReader(b))
	if err != nil {
		return 0, nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	var cred []string
	if t.disp != "" {
		cred = append(cred, "Dispositivo "+t.disp)
	}
	if t.usuario != "" {
		cred = append(cred, "Usuario "+t.usuario)
	}
	if len(cred) > 0 {
		r.Header.Set("Authorization", strings.Join(cred, ", "))
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = res.Body.Close() }()
	out, err := io.ReadAll(res.Body)
	return res.StatusCode, out, err
}

// conectar empareja el teléfono (una sola vez) y abre la sesión del mesero.
func (t *telefono) conectar(ctx context.Context) error {
	if t.usuario != "" {
		return nil
	}
	e, err := t.a.NuevoEmparejamiento(ctx)
	if err != nil {
		return err
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	id := ids.New()
	var sal struct {
		Token string `json:"token"`
		Nonce string `json:"nonce"`
	}
	paso := func(st int, b []byte, err error) error {
		if err != nil {
			return err
		}
		if st != http.StatusOK {
			return fmt.Errorf("el nodo respondió %d: %s", st, b)
		}
		return json.Unmarshal(b, &sal)
	}
	if err := paso(t.req(ctx, "POST", "/v1/dispositivos/emparejar", map[string]any{"codigo": e.Codigo, "dispositivoId": id, "llavePublica": base64.StdEncoding.EncodeToString(pub),
		"nombre": "Teléfono de prueba", "plataforma": "android 14", "versionApp": "0.1.0"})); err != nil {
		return fmt.Errorf("emparejar: %w", err)
	}
	if err := paso(t.req(ctx, "GET", "/v1/dispositivos/desafio?dispositivoId="+id.String(), nil)); err != nil {
		return fmt.Errorf("desafío: %w", err)
	}
	firma := ed25519.Sign(priv, app.MensajeDesafio(sal.Nonce))
	if err := paso(t.req(ctx, "POST", "/v1/dispositivos/sesion", map[string]any{"dispositivoId": id, "nonce": sal.Nonce, "firma": base64.StdEncoding.EncodeToString(firma)})); err != nil {
		return fmt.Errorf("sesión del dispositivo: %w", err)
	}
	t.disp = sal.Token
	if err := paso(t.req(ctx, "POST", "/v1/sesiones", map[string]any{"usuarioId": t.mesero.ID, "pin": t.mesero.PIN})); err != nil {
		return fmt.Errorf("PIN del mesero: %w", err)
	}
	t.usuario = sal.Token
	return nil
}

func (t *telefono) enviar(ctx context.Context, orden map[string]any) (int, []byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.conectar(ctx); err != nil {
		return 0, nil, err
	}
	return t.req(ctx, "POST", "/v1/ordenes/enviar", orden)
}

// termica es una impresora de red simulada: responde DLE EOT como «lista» y guarda cada trabajo.
type termica struct {
	mu       sync.Mutex
	ln       net.Listener
	addr     string
	recibido [][]byte
}

func (x *termica) encender() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	x.ln, x.addr = ln, ln.Addr().String()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go x.atender(c)
		}
	}()
	return nil
}

func (x *termica) apagar() { _ = x.ln.Close() }

func (x *termica) atender(c net.Conn) {
	defer func() { _ = c.Close() }()
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		_ = c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, err := c.Read(tmp)
		chunk := tmp[:n]
		for len(chunk) >= 3 && chunk[0] == escpos.DLE && chunk[1] == escpos.EOT {
			_, _ = c.Write([]byte{escpos.StatusByte(chunk[2], escpos.Status{})})
			chunk = chunk[3:]
		}
		buf = append(buf, chunk...)
		if err != nil {
			break
		}
	}
	if len(buf) > 0 {
		x.mu.Lock()
		x.recibido = append(x.recibido, buf)
		x.mu.Unlock()
	}
}

func (x *termica) textos() []string {
	x.mu.Lock()
	defer x.mu.Unlock()
	out := make([]string, 0, len(x.recibido))
	for _, b := range x.recibido {
		out = append(out, escpos.Decode(b).Text())
	}
	return out
}
