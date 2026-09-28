package nodoauth

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

func TestFirmaYVerificacion(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	otroPub, otroPriv, _ := ed25519.GenerateKey(nil)
	nodo := ids.New()
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	lookup := func(id ids.ID) (ed25519.PublicKey, error) {
		if id != nodo {
			return nil, errors.New("no existe")
		}
		return pub, nil
	}
	tok, err := Sign(priv, nodo, now)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Verify(tok, lookup, now.Add(time.Minute)); err != nil || got != nodo {
		t.Fatalf("Verify = %v, %v", got, err)
	}
	// Reloj del nodo 90 s adelantado: se tolera.
	if _, err := Verify(tok, lookup, now.Add(-90*time.Second)); err != nil {
		t.Fatalf("deriva tolerada rechazada: %v", err)
	}
	casos := map[string]struct {
		tok string
		at  time.Time
	}{
		"vencido":           {tok, now.Add(TTL + Leeway + time.Second)},
		"otra llave":        {must(Sign(otroPriv, nodo, now)), now},
		"nodo desconocido":  {must(Sign(priv, ids.New(), now)), now},
		"basura":            {"x.y.z", now},
		"vigencia excesiva": {must(jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.RegisteredClaims{Issuer: issuer, Audience: jwt.ClaimStrings{audience}, Subject: nodo.String(), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour))}).SignedString(priv)), now},
		"token de usuario":  {must(jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.RegisteredClaims{Issuer: "restpos-cloud", Audience: jwt.ClaimStrings{"backoffice"}, Subject: nodo.String(), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))}).SignedString(priv)), now},
	}
	_ = otroPub
	for nombre, c := range casos {
		if _, err := Verify(c.tok, lookup, c.at); !errors.Is(err, ErrInvalido) {
			t.Errorf("%s: se aceptó (err=%v)", nombre, err)
		}
	}
}

func must(s string, err error) string {
	if err != nil {
		panic(err)
	}
	return s
}
