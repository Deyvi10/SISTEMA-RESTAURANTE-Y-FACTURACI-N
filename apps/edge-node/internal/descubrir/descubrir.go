// Package descubrir encuentra las impresoras de red del local sin configurar nada
// (F2-08, RF-02-02 «Zero-Setup»): anuncio mDNS y barrido de la subred en el puerto RAW 9100.
// La MAC se toma de la tabla ARP para reubicar una impresora cuando el DHCP le cambia la IP.
package descubrir

import (
	"bufio"
	"context"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/zeroconf/v2"
)

// Puerto RAW de las térmicas de red (JetDirect / AppSocket).
const PuertoRAW = 9100

// MaxHosts limita el barrido: una /22 como mucho (redes de restaurante son /24).
const MaxHosts = 1024

// Encontrada es una impresora vista en la red.
type Encontrada struct {
	Host   string
	Puerto int
	MAC    string // minúsculas con «:»; vacío si no se pudo saber
	Modelo string // del anuncio mDNS, si lo hubo
}

// Hosts devuelve las IPs a barrer: las de las subredes IPv4 privadas de este equipo.
func Hosts() []netip.Addr {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []netip.Addr
	vistos := map[netip.Addr]bool{}
	for _, it := range ifs {
		if it.Flags&net.FlagUp == 0 || it.Flags&net.FlagLoopback != 0 || Virtual(it.Name) {
			continue
		}
		addrs, _ := it.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			p, err := netip.ParsePrefix(ipn.String())
			if err != nil || !p.Addr().Is4() || !p.Addr().IsPrivate() {
				continue
			}
			for _, h := range Expandir(p) {
				if !vistos[h] && h != p.Addr() {
					vistos[h] = true
					out = append(out, h)
				}
			}
		}
	}
	return out
}

// Virtual indica interfaces de máquinas virtuales o contenedores (Docker, WSL, Hyper-V,
// VirtualBox, VMware): ahí no hay impresoras del local y barrerlas solo tarda.
func Virtual(nombre string) bool {
	n := strings.ToLower(nombre)
	for _, p := range []string{"docker", "br-", "veth", "virbr", "vmnet", "vboxnet", "vethernet", "wsl", "hyper-v", "virtualbox", "vmware", "tailscale", "zt", "utun"} {
		if strings.HasPrefix(n, p) || strings.Contains(n, "("+p) {
			return true
		}
	}
	return false
}

// Expandir lista los hosts de una subred IPv4 (sin red ni broadcast), como mucho MaxHosts.
// Una subred más grande se recorta a la /22 que contiene la IP del equipo.
func Expandir(p netip.Prefix) []netip.Addr {
	if !p.Addr().Is4() {
		return nil
	}
	if p.Bits() < 22 {
		p = netip.PrefixFrom(p.Addr(), 22)
	}
	p = p.Masked()
	var out []netip.Addr
	a := p.Addr().Next()
	for p.Contains(a) {
		n := a.Next()
		if !p.Contains(n) { // a es el broadcast
			break
		}
		out = append(out, a)
		a = n
	}
	if p.Bits() >= 31 { // /31 y /32: no hay red ni broadcast
		out = []netip.Addr{p.Addr()}
	}
	return out
}

// Barrer prueba el puerto en cada host con concurrencia limitada.
func Barrer(ctx context.Context, hosts []netip.Addr, puerto int, espera time.Duration) []string {
	var mu sync.Mutex
	var abiertos []string
	sem := make(chan struct{}, 64)
	var wg sync.WaitGroup
	for _, h := range hosts {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			d := net.Dialer{Timeout: espera}
			c, err := d.DialContext(ctx, "tcp", netip.AddrPortFrom(h, uint16(puerto)).String()) //nolint:gosec // puerto validado
			if err != nil {
				return
			}
			_ = c.Close()
			mu.Lock()
			abiertos = append(abiertos, h.String())
			mu.Unlock()
		})
	}
	wg.Wait()
	sort.Strings(abiertos)
	return abiertos
}

// MDNS busca impresoras que se anuncian (_pdl-datastream._tcp: impresión RAW).
func MDNS(ctx context.Context, espera time.Duration) map[string]Encontrada {
	out := map[string]Encontrada{}
	ctx, cancel := context.WithTimeout(ctx, espera)
	defer cancel()
	entradas := make(chan *zeroconf.ServiceEntry, 32)
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range entradas {
			for _, ip := range e.AddrIPv4 {
				modelo := e.Instance
				for _, t := range e.Text {
					if v, ok := strings.CutPrefix(t, "ty="); ok {
						modelo = v
					}
				}
				mu.Lock()
				out[ip.String()] = Encontrada{Host: ip.String(), Puerto: e.Port, Modelo: modelo}
				mu.Unlock()
			}
		}
	}()
	if err := zeroconf.Browse(ctx, "_pdl-datastream._tcp", "local.", entradas); err != nil {
		return out
	}
	<-ctx.Done()
	<-done
	return out
}

var macLinea = regexp.MustCompile(`(?i)(\d+\.\d+\.\d+\.\d+)\s+.*?(([0-9a-f]{2}[:-]){5}[0-9a-f]{2})`)

// TablaARP devuelve IP → MAC de los equipos vistos recientemente en la red.
func TablaARP() map[string]string {
	out := map[string]string{}
	if runtime.GOOS == "linux" {
		f, err := os.Open("/proc/net/arp")
		if err == nil {
			defer func() { _ = f.Close() }()
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				campos := strings.Fields(sc.Text())
				if len(campos) >= 4 && campos[3] != "00:00:00:00:00:00" && strings.Count(campos[3], ":") == 5 {
					out[campos[0]] = strings.ToLower(campos[3])
				}
			}
			return out
		}
	}
	// Windows y otros: «arp -a» (formato «192.168.1.50  00-11-62-aa-bb-cc  dinámico»).
	raw, err := exec.Command("arp", "-a").Output() //nolint:gosec // comando fijo
	if err != nil {
		return out
	}
	return ParseARP(string(raw))
}

// ParseARP extrae IP → MAC de la salida de «arp -a» (Windows, macOS, BSD).
func ParseARP(s string) map[string]string {
	out := map[string]string{}
	for _, l := range strings.Split(s, "\n") {
		m := macLinea.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		mac := strings.ToLower(strings.ReplaceAll(m[2], "-", ":"))
		if mac == "ff:ff:ff:ff:ff:ff" || mac == "00:00:00:00:00:00" {
			continue
		}
		out[m[1]] = mac
	}
	return out
}

// Buscar combina mDNS, barrido y ARP. hosts vacío = subredes de este equipo.
func Buscar(ctx context.Context, hosts []netip.Addr) []Encontrada {
	if hosts == nil {
		hosts = Hosts()
	}
	anunciadas := MDNS(ctx, 2*time.Second)
	abiertos := Barrer(ctx, hosts, PuertoRAW, 400*time.Millisecond)
	arp := TablaARP()
	res := map[string]Encontrada{}
	for _, h := range abiertos {
		e := Encontrada{Host: h, Puerto: PuertoRAW}
		if a, ok := anunciadas[h]; ok {
			e.Modelo = a.Modelo
		}
		res[h] = e
	}
	for h, a := range anunciadas {
		if _, ok := res[h]; !ok && a.Puerto == PuertoRAW {
			res[h] = a
		}
	}
	out := make([]Encontrada, 0, len(res))
	for h, e := range res {
		e.MAC = arp[h]
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}
