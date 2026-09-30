package certificados

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

var (
	una       sync.Once
	pub, priv []byte
)

func kek(t *testing.T) (*KEKPublica, *KEKPrivada) {
	t.Helper()
	una.Do(func() {
		var err error
		if pub, priv, err = GenerarKEK(); err != nil {
			panic(err)
		}
	})
	p, err := CargarKEKPublica(pub)
	if err != nil {
		t.Fatal(err)
	}
	k, err := CargarKEKPrivada(priv)
	if err != nil {
		t.Fatal(err)
	}
	return p, k
}

func TestSobreIdaYVuelta(t *testing.T) {
	p, k := kek(t)
	s, err := Sellar(p, "p12:a:b", []byte("contenido del p12"), []byte("clave secreta"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.DEK) != 384 || s.KEK != p.ID() || bytes.Contains(s.Datos[1], []byte("clave secreta")) {
		t.Fatalf("sobre: dek %d bytes, kek %q", len(s.DEK), s.KEK)
	}
	partes, err := Abrir(k, "p12:a:b", s.DEK, s.Datos...)
	if err != nil || string(partes[0]) != "contenido del p12" || string(partes[1]) != "clave secreta" {
		t.Fatalf("abrir: %q %v", partes, err)
	}
	Borrar(partes...)
	if !bytes.Equal(partes[1], make([]byte, len(partes[1]))) {
		t.Fatal("Borrar debe dejar ceros")
	}
	// Cada sellado usa una DEK y nonces nuevos.
	otro, _ := Sellar(p, "p12:a:b", []byte("contenido del p12"), []byte("clave secreta"))
	if bytes.Equal(otro.Datos[0], s.Datos[0]) || bytes.Equal(otro.DEK, s.DEK) {
		t.Fatal("dos sellados iguales")
	}
}

// El sobre no se puede usar fuera de su lugar: otra fila, partes cambiadas, bytes alterados u
// otra KEK.
func TestSobreNoSeAbreFueraDeSuLugar(t *testing.T) {
	p, k := kek(t)
	s, _ := Sellar(p, "p12:a:b", []byte("p12"), []byte("clave"))
	alterado := bytes.Clone(s.Datos[0])
	alterado[len(alterado)-1] ^= 1
	_, otraPriv, _ := GenerarKEK()
	otra, _ := CargarKEKPrivada(otraPriv)
	casos := map[string]func() error{
		"otro contexto":    func() error { _, err := Abrir(k, "p12:otro:b", s.DEK, s.Datos...); return err },
		"partes cambiadas": func() error { _, err := Abrir(k, "p12:a:b", s.DEK, s.Datos[1], s.Datos[0]); return err },
		"texto alterado":   func() error { _, err := Abrir(k, "p12:a:b", s.DEK, alterado); return err },
		"otra KEK":         func() error { _, err := Abrir(otra, "p12:a:b", s.DEK, s.Datos...); return err },
		"DEK recortada":    func() error { _, err := Abrir(k, "p12:a:b", s.DEK[:100], s.Datos...); return err },
		"dato sin nonce":   func() error { _, err := Abrir(k, "p12:a:b", s.DEK, []byte{1, 2}); return err },
	}
	for nombre, f := range casos {
		if err := f(); !errors.Is(err, ErrSobre) {
			t.Errorf("%s: %v", nombre, err)
		}
	}
}

// La API solo recibe un Envolvedor: su llave no sirve para abrir (F5-07, mínimo privilegio).
func TestLaLlavePublicaNoDescifra(t *testing.T) {
	p, _ := kek(t)
	if _, ok := any(p).(Desenvolvedor); ok {
		t.Fatal("la KEK pública no debe poder desenvolver")
	}
	if _, err := CargarKEKPublica(priv); err == nil {
		t.Fatal("la privada no se acepta como pública")
	}
	if _, err := CargarKEKPrivada(pub); err == nil {
		t.Fatal("la pública no se acepta como privada")
	}
}
