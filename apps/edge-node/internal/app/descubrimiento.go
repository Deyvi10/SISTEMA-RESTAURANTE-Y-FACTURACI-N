package app

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/descubrir"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/store"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/edgesync"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

const (
	// BuscarCada: repaso periódico de la red (impresoras nuevas o IP cambiada por DHCP).
	BuscarCada = 30 * time.Minute
	// BuscarTrasCaida: si una impresora queda sin conexión, se busca por su MAC, pero no
	// más de una vez en este lapso.
	BuscarTrasCaida = 5 * time.Minute
)

// Busqueda es el resultado del último descubrimiento (para la página de estado).
type Busqueda struct {
	mu          sync.Mutex
	Ultima      time.Time
	Encontradas []descubrir.Encontrada
	enCurso     bool
}

func (b *Busqueda) copia() (time.Time, []descubrir.Encontrada, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Ultima, append([]descubrir.Encontrada(nil), b.Encontradas...), b.enCurso
}

// pedirBusqueda adelanta el próximo descubrimiento (no bloquea).
func (a *App) pedirBusqueda() {
	select {
	case a.buscarYa <- struct{}{}:
	default:
	}
}

func (a *App) busquedaLoop(ctx context.Context) {
	t := time.NewTicker(BuscarCada)
	defer t.Stop()
	for {
		a.BuscarImpresoras(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-a.buscarYa:
		}
	}
}

type impresoraConocida struct {
	id        ids.ID
	host, mac string
	puerto    int
}

// BuscarImpresoras recorre la red e informa a la nube lo nuevo o lo que cambió de IP.
// Devuelve cuántas impresoras encontró.
func (a *App) BuscarImpresoras(ctx context.Context) int {
	a.busqueda.mu.Lock()
	if a.busqueda.enCurso {
		a.busqueda.mu.Unlock()
		return 0
	}
	a.busqueda.enCurso = true
	a.busqueda.mu.Unlock()
	defer func() { a.busqueda.mu.Lock(); a.busqueda.enCurso = false; a.busqueda.mu.Unlock() }()

	encontradas := a.buscar(ctx)
	a.busqueda.mu.Lock()
	a.busqueda.Ultima, a.busqueda.Encontradas = a.Clock.Now(), encontradas
	a.busqueda.mu.Unlock()

	conocidas, err := a.impresorasConocidas(ctx)
	if err != nil {
		a.Log.Error("descubrimiento: réplica ilegible", "err", err)
		return len(encontradas)
	}
	now := a.Clock.Now()
	var eventos []edgesync.Event
	for _, e := range encontradas {
		id, informar := ids.New(), true
		for _, k := range conocidas {
			mismaMAC := e.MAC != "" && k.mac == e.MAC
			mismaIP := k.host == e.Host && k.puerto == e.Puerto
			if mismaMAC || mismaIP {
				id = k.id
				// Solo se informa si algo cambió: IP nueva (DHCP) o MAC que no conocíamos.
				informar = !mismaIP || (e.MAC != "" && k.mac != e.MAC)
				break
			}
		}
		if !informar {
			continue
		}
		payload := map[string]any{"id": id, "conexion": "TCP", "host": e.Host, "puerto": e.Puerto, "modelo": e.Modelo}
		if e.MAC != "" {
			payload["mac"] = e.MAC
		}
		ev, err := edgesync.NewEvent("impresora.detectada", 1, id, payload, now)
		if err == nil {
			eventos = append(eventos, ev)
		}
	}
	if len(eventos) > 0 {
		if err := a.Store.Write(ctx, func(tx *store.Tx) error {
			for _, ev := range eventos {
				if _, err := a.outbox.Append(ctx, tx, ev); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			a.Log.Error("descubrimiento: no se pudo informar", "err", err)
		} else {
			a.notificarPush()
		}
	}
	a.Log.Info("búsqueda de impresoras", "encontradas", len(encontradas), "informadas", len(eventos))
	return len(encontradas)
}

func (a *App) impresorasConocidas(ctx context.Context) ([]impresoraConocida, error) {
	rows, err := a.Store.Read().QueryContext(ctx, `SELECT id, coalesce(host, ''), coalesce(puerto, 0), coalesce(mac, '') FROM impresoras WHERE deleted_at IS NULL AND conexion = 'TCP'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []impresoraConocida
	for rows.Next() {
		var k impresoraConocida
		var id string
		if err := rows.Scan(&id, &k.host, &k.puerto, &k.mac); err != nil {
			return nil, err
		}
		k.id, _ = ids.Parse(id)
		k.mac = strings.ToLower(k.mac)
		out = append(out, k)
	}
	return out, rows.Err()
}

// direccion arma «host:puerto» para mostrar.
func direccion(e descubrir.Encontrada) string {
	return net.JoinHostPort(e.Host, strconv.Itoa(e.Puerto))
}

func resumenBusqueda(n int) string {
	switch n {
	case 0:
		return "No encontré impresoras de red. Revisa que estén encendidas y en la misma red que esta PC."
	case 1:
		return "Encontré 1 impresora en la red."
	}
	return fmt.Sprintf("Encontré %d impresoras en la red.", n)
}
