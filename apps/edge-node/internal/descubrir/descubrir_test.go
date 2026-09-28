package descubrir

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestExpandir(t *testing.T) {
	h := Expandir(netip.MustParsePrefix("192.168.1.37/24"))
	if len(h) != 254 || h[0].String() != "192.168.1.1" || h[253].String() != "192.168.1.254" {
		t.Fatalf("/24: %d hosts, %v … %v", len(h), h[0], h[len(h)-1])
	}
	if n := len(Expandir(netip.MustParsePrefix("10.0.0.5/8"))); n != MaxHosts-2 {
		t.Fatalf("/8 recortada a /22: %d hosts", n)
	}
	if n := len(Expandir(netip.MustParsePrefix("192.168.1.5/32"))); n != 1 {
		t.Fatalf("/32: %d", n)
	}
}

func TestParseARPWindows(t *testing.T) {
	salida := `
Interfaz: 192.168.1.10 --- 0xb
  Dirección de Internet          Dirección física      Tipo
  192.168.1.1           a4-2b-b0-11-22-33     dinámico
  192.168.1.50          00-11-62-AA-BB-CC     dinámico
  192.168.1.255         ff-ff-ff-ff-ff-ff     estático
`
	m := ParseARP(salida)
	if m["192.168.1.50"] != "00:11:62:aa:bb:cc" || m["192.168.1.1"] != "a4:2b:b0:11:22:33" {
		t.Fatalf("ARP: %v", m)
	}
	if _, ok := m["192.168.1.255"]; ok {
		t.Fatal("incluyó el broadcast")
	}
}

func TestBarrerEncuentraPuertoAbierto(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	hosts := []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("127.0.0.2"), netip.MustParseAddr("127.0.0.3")}
	got := Barrer(context.Background(), hosts, port, 300*time.Millisecond)
	if len(got) != 1 || got[0] != "127.0.0.1" {
		t.Fatalf("abiertos = %v", got)
	}
}

func TestVirtual(t *testing.T) {
	for n, want := range map[string]bool{"docker0": true, "br-30046f5cc307": true, "vEthernet (WSL)": true, "VirtualBox Host-Only Network": true,
		"wlo1": false, "eth0": false, "Ethernet": false, "Wi-Fi": false, "enp3s0": false} {
		if Virtual(n) != want {
			t.Errorf("Virtual(%q) = %v", n, !want)
		}
	}
}
