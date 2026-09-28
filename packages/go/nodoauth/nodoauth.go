// Package nodoauth autentica al Nodo Local ante la nube (F2-02).
//
// En la activación el nodo genera su par ed25519 y entrega solo la llave pública; la
// privada nunca sale de la PC del restaurante. Cada petición lleva un JWT corto (5 min)
// firmado por el nodo. La nube lo verifica con la llave pública registrada y, en cada
// petición, que el nodo siga ACTIVO: revocarlo corta el acceso al instante.
package nodoauth

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

const (
	issuer   = "restpos-nodo"
	audience = "restpos-cloud"
	// TTL de cada token; MaxTTL es lo máximo que la nube acepta aunque el nodo pida más.
	TTL    = 5 * time.Minute
	MaxTTL = 10 * time.Minute
	// Leeway tolera relojes desfasados. Deriva mayor a 60 s ya dispara alerta (F2-04).
	Leeway = 2 * time.Minute
)

var ErrInvalido = errors.New("nodoauth: token de nodo inválido")

// Sign crea el token de una petición del nodo.
func Sign(priv ed25519.PrivateKey, nodoID ids.ID, now time.Time) (string, error) {
	c := jwt.RegisteredClaims{
		Issuer: issuer, Audience: jwt.ClaimStrings{audience}, Subject: nodoID.String(),
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(TTL)), ID: ids.New().String(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodEdDSA, c).SignedString(priv)
}

// Lookup devuelve la llave pública registrada de un nodo (o error si no existe).
type Lookup func(nodoID ids.ID) (ed25519.PublicKey, error)

// Verify valida firma, emisor, audiencia y vigencia, y devuelve el id del nodo.
func Verify(token string, lookup Lookup, now time.Time) (ids.ID, error) {
	var c jwt.RegisteredClaims
	var nodoID ids.ID
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		sub, err := c.GetSubject()
		if err != nil {
			return nil, err
		}
		if nodoID, err = ids.Parse(sub); err != nil {
			return nil, err
		}
		pub, err := lookup(nodoID)
		if err != nil {
			return nil, err
		}
		if len(pub) != ed25519.PublicKeySize {
			return nil, errors.New("llave pública inválida")
		}
		return pub, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithIssuer(issuer), jwt.WithAudience(audience),
		jwt.WithTimeFunc(func() time.Time { return now }), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithLeeway(Leeway))
	if err != nil {
		return ids.Nil, fmt.Errorf("%w: %w", ErrInvalido, err)
	}
	if c.IssuedAt == nil || c.ExpiresAt.Sub(c.IssuedAt.Time) > MaxTTL {
		return ids.Nil, fmt.Errorf("%w: vigencia mayor a %v", ErrInvalido, MaxTTL)
	}
	return nodoID, nil
}
