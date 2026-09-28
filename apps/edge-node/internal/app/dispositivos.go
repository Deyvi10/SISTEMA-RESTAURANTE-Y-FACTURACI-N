package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image/png"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"rsc.io/qr"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/descubrir"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/hub"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

const (
	// CodigoEmparejarTTL: el QR vale 10 minutos y se usa una vez (F3-02).
	CodigoEmparejarTTL = 10 * time.Minute
	// SesionDispositivoTTL: el dispositivo renueva su sesión con un desafío firmado.
	SesionDispositivoTTL = 12 * time.Hour
	desafioTTL           = time.Minute
	alfabetoEmparejar    = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
)

func hashToken(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func tokenAleatorio() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func codigoEmparejar() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, 8)
	for i, x := range b {
		out[i] = alfabetoEmparejar[int(x)%len(alfabetoEmparejar)]
	}
	return string(out)
}

// ipsLAN son las direcciones por las que los teléfonos pueden llegar a este nodo.
func ipsLAN() []string {
	var out []string
	ifs, _ := net.Interfaces()
	for _, it := range ifs {
		if it.Flags&net.FlagUp == 0 || it.Flags&net.FlagLoopback != 0 || descubrir.Virtual(it.Name) {
			continue
		}
		addrs, _ := it.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && ipn.IP.IsPrivate() {
				out = append(out, ipn.IP.String())
			}
		}
	}
	return out
}

// Emparejamiento es lo que muestra la caja para que un teléfono se empareje.
type Emparejamiento struct {
	Codigo   string    `json:"codigo"`
	ExpiraAt time.Time `json:"expiraAt"`
	URLs     []string  `json:"urls"`
	// Enlace que va dentro del QR: restpos://emparejar?c=…&u=http://ip:puerto
	Enlace string `json:"enlace"`
	QR     string `json:"qr"` // PNG en data URL
}

// NuevoEmparejamiento crea un código de un solo uso para emparejar un dispositivo.
func (a *App) NuevoEmparejamiento(ctx context.Context) (Emparejamiento, error) {
	id, err := a.Identidad(ctx)
	if err != nil {
		return Emparejamiento{}, err
	}
	if !id.Activo() {
		return Emparejamiento{}, problema(http.StatusConflict, "SIN_ACTIVAR", "Activa el Nodo Local antes de emparejar teléfonos.")
	}
	now := a.Clock.Now()
	e := Emparejamiento{Codigo: codigoEmparejar(), ExpiraAt: now.Add(CodigoEmparejarTTL)}
	_, puerto, _ := net.SplitHostPort(a.Cfg.HTTPAddr)
	if puerto == "" {
		puerto = "7080"
	}
	for _, ip := range ipsLAN() {
		e.URLs = append(e.URLs, "http://"+net.JoinHostPort(ip, puerto))
	}
	q := url.Values{"c": {e.Codigo}}
	for _, u := range e.URLs {
		q.Add("u", u)
	}
	e.Enlace = "restpos://emparejar?" + q.Encode()
	if err := a.Store.Write(ctx, func(tx *store.Tx) error {
		// Un solo código vigente: el anterior deja de valer.
		if _, err := tx.ExecContext(ctx, `UPDATE codigos_emparejamiento SET expira_at = ? WHERE usado_at IS NULL AND expira_at > ?`, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO codigos_emparejamiento (codigo_hash, expira_at) VALUES (?, ?)`, hashToken(e.Codigo), e.ExpiraAt.Format(time.RFC3339Nano))
		return err
	}); err != nil {
		return e, err
	}
	code, err := qr.Encode(e.Enlace, qr.M)
	if err != nil {
		return e, err
	}
	code.Scale = 8
	var buf bytes.Buffer
	if err := png.Encode(&buf, code.Image()); err != nil {
		return e, err
	}
	e.QR = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	return e, nil
}

// EmparejarIn es lo que envía la app al escanear el QR.
type EmparejarIn struct {
	Codigo        string `json:"codigo"`
	DispositivoID ids.ID `json:"dispositivoId"`
	LlavePublica  string `json:"llavePublica"`
	Nombre        string `json:"nombre"`
	Tipo          string `json:"tipo"`
	Plataforma    string `json:"plataforma"`
	VersionApp    string `json:"versionApp"`
}

type EmparejadoOut struct {
	DispositivoID ids.ID `json:"dispositivoId"`
	NodoID        ids.ID `json:"nodoId"`
	Restaurante   string `json:"restaurante"`
	Local         string `json:"local"`
}

// limitador sencillo en memoria para códigos erróneos por IP (10 → 15 min).
type limitador struct {
	mu     sync.Mutex
	fallos map[string][]time.Time
}

func (l *limitador) bloqueado(clave string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	var vig []time.Time
	for _, t := range l.fallos[clave] {
		if now.Sub(t) < 15*time.Minute {
			vig = append(vig, t)
		}
	}
	l.fallos[clave] = vig
	return len(vig) >= 10
}

func (l *limitador) fallo(clave string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fallos[clave] = append(l.fallos[clave], now)
}

var errCodigoEmparejar = problema(http.StatusUnprocessableEntity, "CODIGO_INVALIDO", "El código no es válido o ya venció. Pide uno nuevo en la caja (Estado del nodo › Emparejar teléfono).")

// Emparejar registra un dispositivo con su llave pública (la privada queda en el teléfono).
func (a *App) Emparejar(ctx context.Context, in EmparejarIn, ip string) (EmparejadoOut, error) {
	now := a.Clock.Now()
	if a.limite.bloqueado(ip, now) {
		return EmparejadoOut{}, problema(http.StatusTooManyRequests, "DEMASIADOS_INTENTOS", "Demasiados códigos incorrectos. Espera 15 minutos.")
	}
	pub, err := base64.StdEncoding.DecodeString(in.LlavePublica)
	if err != nil || len(pub) != ed25519.PublicKeySize || in.DispositivoID.Version() != 7 {
		return EmparejadoOut{}, problema(http.StatusUnprocessableEntity, "IDENTIDAD_INVALIDA", "La app envió una identidad inválida. Reinstálala.")
	}
	tipo := strings.ToUpper(strings.TrimSpace(in.Tipo))
	if tipo != "TABLET" && tipo != "KDS" && tipo != "POS" {
		tipo = "MOVIL"
	}
	nombre := strings.TrimSpace(in.Nombre)
	if nombre == "" {
		nombre = "Teléfono"
	}
	if r := []rune(nombre); len(r) > 60 {
		nombre = string(r[:60])
	}
	id, err := a.Identidad(ctx)
	if err != nil {
		return EmparejadoOut{}, err
	}
	if !id.Activo() {
		return EmparejadoOut{}, problema(http.StatusConflict, "SIN_ACTIVAR", "El Nodo Local no está activado.")
	}
	codigo := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(in.Codigo), "-", ""))
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		var usado sql.NullString
		var expira string
		err := tx.QueryRowContext(ctx, `SELECT expira_at, dispositivo_id FROM codigos_emparejamiento WHERE codigo_hash = ?`, hashToken(codigo)).Scan(&expira, &usado)
		if errors.Is(err, sql.ErrNoRows) {
			return errCodigoEmparejar
		}
		if err != nil {
			return err
		}
		// Reintento del mismo dispositivo (respuesta perdida): idempotente.
		if usado.Valid {
			if usado.String == in.DispositivoID.String() {
				return nil
			}
			return errCodigoEmparejar
		}
		if t, _ := time.Parse(time.RFC3339Nano, expira); !now.Before(t) {
			return errCodigoEmparejar
		}
		if _, err := tx.ExecContext(ctx, `UPDATE codigos_emparejamiento SET usado_at = ?, dispositivo_id = ? WHERE codigo_hash = ?`,
			now.Format(time.RFC3339Nano), in.DispositivoID.String(), hashToken(codigo)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO dispositivos_nodo (id, nombre, tipo, llave_publica, plataforma, version_app, emparejado_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			in.DispositivoID.String(), nombre, tipo, pub, truncarR(in.Plataforma, 40), truncarR(in.VersionApp, 40), now.Format(time.RFC3339Nano)); err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return problema(http.StatusConflict, "YA_EMPAREJADO", "Este dispositivo ya estaba emparejado.")
			}
			return err
		}
		ev, err := edgesync.NewEvent("dispositivo.emparejado", 1, in.DispositivoID, map[string]any{
			"id": in.DispositivoID, "nombre": nombre, "tipo": tipo, "llavePublica": pub, "plataforma": in.Plataforma, "versionApp": in.VersionApp, "emparejadoAt": now,
		}, now)
		if err != nil {
			return err
		}
		if _, err := a.outbox.Append(ctx, tx, ev); err != nil {
			return err
		}
		return auditar(ctx, tx, "DISPOSITIVO_EMPAREJADO", "dispositivo", in.DispositivoID, nil, map[string]any{"nombre": nombre, "tipo": tipo}, now)
	})
	if errors.Is(err, errCodigoEmparejar) {
		a.limite.fallo(ip, now)
	}
	if err != nil {
		return EmparejadoOut{}, err
	}
	a.notificarPush()
	return EmparejadoOut{DispositivoID: in.DispositivoID, NodoID: id.NodoID, Restaurante: id.NombreComercial, Local: id.NombreLocal}, nil
}

func truncarR(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// ---------- Desafío-respuesta ----------

type desafios struct {
	mu sync.Mutex
	m  map[string]desafio
}

type desafio struct {
	dispositivo ids.ID
	expira      time.Time
}

// MensajeDesafio es lo que firma el dispositivo con su llave privada.
func MensajeDesafio(nonce string) []byte { return []byte("restpos|sesion-dispositivo|" + nonce) }

// NuevoDesafio entrega un número de un solo uso para que el dispositivo lo firme.
func (a *App) NuevoDesafio(dispositivo ids.ID) (string, time.Time) {
	nonce := tokenAleatorio()
	exp := a.Clock.Now().Add(desafioTTL)
	a.desafios.mu.Lock()
	defer a.desafios.mu.Unlock()
	for k, d := range a.desafios.m { // limpieza de vencidos
		if a.Clock.Now().After(d.expira) {
			delete(a.desafios.m, k)
		}
	}
	a.desafios.m[nonce] = desafio{dispositivo: dispositivo, expira: exp}
	return nonce, exp
}

type SesionDispositivoIn struct {
	DispositivoID ids.ID `json:"dispositivoId"`
	Nonce         string `json:"nonce"`
	Firma         string `json:"firma"`
}

type SesionDispositivo struct {
	Token    string    `json:"token"`
	ExpiraAt time.Time `json:"expiraAt"`
}

var errDispositivo = problema(http.StatusUnauthorized, "DISPOSITIVO_NO_AUTORIZADO", "Este dispositivo no está emparejado o fue revocado. Escanea de nuevo el código de tu restaurante.")

// AbrirSesionDispositivo verifica la firma del desafío y entrega un token de 12 h.
func (a *App) AbrirSesionDispositivo(ctx context.Context, in SesionDispositivoIn) (SesionDispositivo, error) {
	a.desafios.mu.Lock()
	d, ok := a.desafios.m[in.Nonce]
	delete(a.desafios.m, in.Nonce)
	a.desafios.mu.Unlock()
	now := a.Clock.Now()
	if !ok || d.dispositivo != in.DispositivoID || now.After(d.expira) {
		return SesionDispositivo{}, problema(http.StatusUnauthorized, "DESAFIO_INVALIDO", "El desafío venció; vuelve a intentarlo.")
	}
	pub, err := a.llaveDispositivo(ctx, in.DispositivoID)
	if err != nil {
		return SesionDispositivo{}, err
	}
	firma, err := base64.StdEncoding.DecodeString(in.Firma)
	if err != nil || !ed25519.Verify(pub, MensajeDesafio(in.Nonce), firma) {
		return SesionDispositivo{}, errDispositivo
	}
	s := SesionDispositivo{Token: tokenAleatorio(), ExpiraAt: now.Add(SesionDispositivoTTL)}
	err = a.Store.Write(ctx, func(tx *store.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sesiones_dispositivo WHERE expira_at < ?`, now.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO sesiones_dispositivo (token_hash, dispositivo_id, creada_at, expira_at) VALUES (?, ?, ?, ?)`,
			hashToken(s.Token), in.DispositivoID.String(), now.Format(time.RFC3339Nano), s.ExpiraAt.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		ev, err := edgesync.NewEvent("dispositivo.uso", 1, in.DispositivoID, map[string]any{"id": in.DispositivoID}, now)
		if err != nil {
			return err
		}
		_, err = a.outbox.Append(ctx, tx, ev)
		return err
	})
	return s, err
}

// llaveDispositivo devuelve la llave pública si el dispositivo está vigente (ni revocado
// en el nodo ni en la nube).
func (a *App) llaveDispositivo(ctx context.Context, id ids.ID) (ed25519.PublicKey, error) {
	var pub []byte
	var revocado sql.NullString
	err := a.Store.Read().QueryRowContext(ctx, `SELECT d.llave_publica, coalesce(d.revocado_at, (SELECT r.revocado_at FROM dispositivos r WHERE r.id = d.id AND r.estado = 'REVOCADO'))
		FROM dispositivos_nodo d WHERE d.id = ?`, id.String()).Scan(&pub, &revocado)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && revocado.Valid) {
		return nil, errDispositivo
	}
	return pub, err
}

// Dispositivo autenticado de una petición.
type Dispositivo struct {
	ID     ids.ID
	Nombre string
	Tipo   string
	Local  bool // la propia PC del nodo (caja web), sin emparejar
}

// PCNodo es el «dispositivo» de las peticiones que llegan desde esta misma PC.
var PCNodo = ids.MustParse("00000000-0000-7000-8000-000000000001")

// dispositivoDe resuelve el dispositivo de una petición: token de sesión o esta misma PC.
func (a *App) dispositivoDe(r *http.Request) (Dispositivo, error) {
	tok := tokenDe(r, "Dispositivo")
	if tok == "" {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
				return Dispositivo{ID: PCNodo, Nombre: "PC del nodo", Tipo: "POS", Local: true}, nil
			}
		}
		return Dispositivo{}, errDispositivo
	}
	var d Dispositivo
	var id, expira string
	var revocado sql.NullString
	err := a.Store.Read().QueryRowContext(r.Context(), `SELECT s.dispositivo_id, s.expira_at, d.nombre, d.tipo,
			coalesce(d.revocado_at, (SELECT x.revocado_at FROM dispositivos x WHERE x.id = d.id AND x.estado = 'REVOCADO'))
		FROM sesiones_dispositivo s JOIN dispositivos_nodo d ON d.id = s.dispositivo_id WHERE s.token_hash = ?`, hashToken(tok)).
		Scan(&id, &expira, &d.Nombre, &d.Tipo, &revocado)
	if err != nil || revocado.Valid {
		return Dispositivo{}, errDispositivo
	}
	if t, _ := time.Parse(time.RFC3339Nano, expira); a.Clock.Now().After(t) {
		return Dispositivo{}, problema(http.StatusUnauthorized, "SESION_DISPOSITIVO_VENCIDA", "La sesión del dispositivo venció.")
	}
	d.ID, err = ids.Parse(id)
	return d, err
}

// tokenDe lee «Authorization: <esquema> <token>» (se admite más de una credencial en la
// misma cabecera separadas por coma, como envían los clientes HTTP móviles) o, para
// WebSocket, ?<esquema>=token.
func tokenDe(r *http.Request, esquema string) string {
	for _, h := range r.Header.Values("Authorization") {
		for _, parte := range strings.Split(h, ",") {
			if t, ok := strings.CutPrefix(strings.TrimSpace(parte), esquema+" "); ok {
				return strings.TrimSpace(t)
			}
		}
	}
	return r.URL.Query().Get(strings.ToLower(esquema))
}

// revocarDispositivos aplica las revocaciones que llegan de la nube: cierra sus sesiones y
// los desconecta del hub al instante (F3-02: < 2 s).
func (a *App) revocarDispositivos(ctx context.Context, cambios []edgesync.Cambio) {
	for _, c := range cambios {
		if c.Tabla != "dispositivos" {
			continue
		}
		var d struct {
			ID     ids.ID `json:"id"`
			Estado string `json:"estado"`
		}
		if decodeJSON(c.Datos, &d) != nil || d.Estado != "REVOCADO" {
			continue
		}
		if err := a.Store.Write(ctx, func(tx *store.Tx) error {
			if _, err := tx.ExecContext(ctx, `DELETE FROM sesiones_dispositivo WHERE dispositivo_id = ?`, d.ID.String()); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `UPDATE sesiones_usuario SET cerrada_at = ? WHERE dispositivo_id = ? AND cerrada_at IS NULL`, a.now(), d.ID.String())
			return err
		}); err != nil {
			a.Log.Error("no se pudo revocar el dispositivo", "err", err)
			continue
		}
		dev := d.ID
		_ = a.hub.DifundirA(eventos.DeviceRevoked{DeviceID: dev}, func(c *hub.Cliente) bool { return c.DispositivoID != nil && *c.DispositivoID == dev })
		a.hub.Desconectar(func(c *hub.Cliente) bool { return c.DispositivoID != nil && *c.DispositivoID == dev })
		a.Log.Info("dispositivo revocado", "dispositivo", dev)
	}
}

// autenticarWS: la caja en esta PC o un dispositivo con sesión.
func (a *App) autenticarWS(r *http.Request) (hub.Cliente, error) {
	d, err := a.dispositivoDe(r)
	if err != nil {
		return hub.Cliente{}, hub.ErrNoAutorizado
	}
	c := hub.Cliente{Tipo: d.Tipo}
	if !d.Local {
		id := d.ID
		c.DispositivoID = &id
	} else if t := r.URL.Query().Get("tipo"); t == "KDS" || t == "ESTADO" {
		c.Tipo = t
	}
	if u, err := a.usuarioDe(r, d); err == nil {
		uid := u.ID
		c.UsuarioID = &uid
	}
	return c, nil
}

func ipDe(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func itoa(n int) string { return strconv.Itoa(n) }
