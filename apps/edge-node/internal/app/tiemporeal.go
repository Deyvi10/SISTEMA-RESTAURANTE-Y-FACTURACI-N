package app

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/libp2p/zeroconf/v2"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/hub"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/eventos"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// ServicioMDNS es el tipo que anuncia el nodo en la LAN para que la app lo encuentre sola
// (RF-02-08.2). Si el router bloquea multicast, la app permite escribir la IP a mano.
const ServicioMDNS = "_restpos._tcp"

// autenticarLAN decide quién puede abrir el WebSocket. Hasta el emparejamiento de
// dispositivos (F3-02) solo se aceptan conexiones desde esta misma PC (caja y página de
// estado servidas por el nodo); un teléfono de la red todavía no puede conectarse.
func autenticarLAN(r *http.Request) (hub.Cliente, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return hub.Cliente{}, hub.ErrNoAutorizado
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return hub.Cliente{}, hub.ErrNoAutorizado
	}
	tipo := r.URL.Query().Get("tipo")
	switch tipo {
	case "POS", "KDS", "ESTADO":
	default:
		tipo = "POS"
	}
	return hub.Cliente{Tipo: tipo}, nil
}

// difundirCambios avisa a los dispositivos qué cambió en la réplica y cierra la sesión de
// los usuarios desactivados (F2-03: se difunde en menos de 2 s).
func (a *App) difundirCambios(c CambiosAplicados) {
	tablas := make([]string, 0, len(c.Tablas))
	for t := range c.Tablas {
		tablas = append(tablas, t)
	}
	sort.Strings(tablas)
	if err := a.hub.Difundir(eventos.CatalogUpdated{Tables: tablas, Full: c.Completo}); err != nil {
		a.Log.Error("hub: difundir catálogo", "err", err)
	}
	if c.Completo {
		return
	}
	for _, cam := range c.Cambios {
		if cam.Tabla != "usuarios" {
			continue
		}
		var u struct {
			ID     ids.ID `json:"id"`
			Activo *bool  `json:"activo"`
		}
		if json.Unmarshal(cam.Datos, &u) == nil && (cam.Op == "D" || (u.Activo != nil && !*u.Activo)) {
			_ = a.hub.Difundir(eventos.UserDeactivated{UserID: u.ID})
		}
	}
}

// Conectividad es el semáforo de la caja y la app (RF-02-09.3). El rojo («sin Nodo
// Local») lo decide el cliente cuando no puede llegar a este endpoint.
type Conectividad struct {
	Indicador    string     `json:"indicador"` // VERDE, AMARILLO
	Nube         string     `json:"nube"`      // EN_LINEA, SIN_INTERNET, SIN_ACTIVAR, REVOCADO
	UltimaNube   *time.Time `json:"ultimaNube"`
	Dispositivos int        `json:"dispositivos"`
	Version      string     `json:"version"`
}

func (a *App) conectividad(ctx context.Context) Conectividad {
	c := Conectividad{Indicador: "AMARILLO", Nube: "SIN_INTERNET", Dispositivos: a.hub.Conectados(), Version: Version}
	s := a.salud.Copia()
	if !s.UltimaNube.IsZero() {
		t := s.UltimaNube
		c.UltimaNube = &t
	}
	id, err := a.Identidad(ctx)
	switch {
	case err != nil || id == nil:
		c.Nube = "SIN_ACTIVAR"
	case !id.Activo() || s.Revocado:
		c.Nube = "REVOCADO"
	case !s.UltimaNube.IsZero() && a.Clock.Now().Sub(s.UltimaNube) < 2*HeartbeatCada+EsperaPull:
		c.Nube, c.Indicador = "EN_LINEA", "VERDE"
	}
	return c
}

func (a *App) handleConectividad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, a.conectividad(r.Context()))
}

// anunciarMDNS publica el nodo en la LAN mientras ctx viva.
func (a *App) anunciarMDNS(ctx context.Context, port int) {
	txt := []string{"v=" + Version, "api=/v1"}
	if id, err := a.Identidad(ctx); err == nil && id.Activo() {
		txt = append(txt, "nodo="+id.NodoID.String())
	}
	srv, err := zeroconf.Register("RestPOS-Nodo", ServicioMDNS, "local.", port, txt, nil)
	if err != nil {
		a.Log.Warn("mDNS no disponible: la app deberá usar la IP del nodo", "err", err)
		return
	}
	a.Log.Info("nodo anunciado en la red", "servicio", ServicioMDNS, "puerto", strconv.Itoa(port))
	<-ctx.Done()
	srv.Shutdown()
}
