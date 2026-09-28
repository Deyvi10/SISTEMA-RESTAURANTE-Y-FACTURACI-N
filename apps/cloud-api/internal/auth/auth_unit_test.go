package auth

import (
	"crypto/ed25519"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

func TestPasswordHashAndVerify(t *testing.T) {
	h, err := HashPassword("Ceviche2026!")
	if err != nil || !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=2,p=2$") {
		t.Fatalf("hash = %q, %v", h, err)
	}
	if ok, _ := VerifyPassword("Ceviche2026!", h); !ok {
		t.Fatal("la contraseña correcta no verifica")
	}
	if ok, _ := VerifyPassword("ceviche2026!", h); ok {
		t.Fatal("una contraseña distinta verificó")
	}
	h2, _ := HashPassword("Ceviche2026!")
	if h == h2 {
		t.Fatal("dos hashes iguales: la sal no es aleatoria")
	}
}

func TestValidarPassword(t *testing.T) {
	malas := map[string]string{"corta1": "", "sololetrasaqui": "", "12345678901": "", "restaurante": "", "carlos2026xx": "carlos@x.ec"}
	for pw, email := range malas {
		if ValidarPassword(pw, email) == "" {
			t.Errorf("%q debería rechazarse", pw)
		}
	}
	// Un correo corto («a@a.ec») no debe prohibir la letra «a» en la contraseña.
	if m := ValidarPassword("ClaveSegura2026", "a@a.ec"); m != "" {
		t.Errorf("correo corto bloqueó una contraseña válida: %s", m)
	}
	if m := ValidarPassword("MesaCuatro44", "ana@x.ec"); m != "" {
		t.Errorf("contraseña válida rechazada: %s", m)
	}
	for range 20 {
		if p := PasswordTemporal(); len(p) != 14 || ValidarPassword(p, "") != "" {
			t.Fatalf("temporal inválida: %q", p)
		}
	}
}

func TestValidarPIN(t *testing.T) {
	for _, bad := range []string{"123", "1234567", "12a4", "0000", "1111", "1234", "4321", "123456", "1212", "2580"} {
		if ValidarPIN(bad) == "" {
			t.Errorf("PIN %q debería rechazarse", bad)
		}
	}
	for _, ok := range []string{"8899", "1024", "7391", "58213"} {
		if m := ValidarPIN(ok); m != "" {
			t.Errorf("PIN %q rechazado: %s", ok, m)
		}
	}
}

func TestPINHashNeedsPepper(t *testing.T) {
	u := ids.New()
	h, _ := HashPIN([]byte("pepper-A-0123456789012345678901"), u, "8899")
	if ok, _ := VerifyPIN([]byte("pepper-A-0123456789012345678901"), u, "8899", h); !ok {
		t.Fatal("PIN correcto no verifica")
	}
	if ok, _ := VerifyPIN([]byte("pepper-B-0123456789012345678901"), u, "8899", h); ok {
		t.Fatal("con otro pepper no debe verificar")
	}
	if ok, _ := VerifyPIN([]byte("pepper-A-0123456789012345678901"), ids.New(), "8899", h); ok {
		t.Fatal("el hash está ligado al usuario")
	}
	a, b := FingerprintPIN([]byte("p"), "8899"), FingerprintPIN([]byte("p"), "8899")
	if a != b || a == FingerprintPIN([]byte("p"), "8898") {
		t.Fatal("la huella debe ser determinística")
	}
}

func TestTokens(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(nil)
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	s := NewSigner(key, func() time.Time { return now })
	p := Principal{UserID: ids.New(), TenantID: ids.New(), SessionID: ids.New(), Rol: "ADMIN", DebeCambiar: true}
	tok, err := s.Issue(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Parse(tok)
	if err != nil || got != p {
		t.Fatalf("parse = %+v, %v", got, err)
	}
	now = now.Add(AccessTTL + time.Second)
	if _, err := s.Parse(tok); err == nil {
		t.Fatal("un token vencido debe rechazarse")
	}
	_, otra, _ := ed25519.GenerateKey(nil)
	now = now.Add(-AccessTTL)
	if _, err := NewSigner(otra, func() time.Time { return now }).Parse(tok); err == nil {
		t.Fatal("un token firmado con otra llave debe rechazarse")
	}
	if _, err := s.Parse(tok[:len(tok)-2] + "xx"); err == nil {
		t.Fatal("un token alterado debe rechazarse")
	}
}
