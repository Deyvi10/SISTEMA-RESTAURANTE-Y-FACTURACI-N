package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

const (
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 7 * 24 * time.Hour
	issuer     = "restpos-cloud"
	audience   = "backoffice"
)

// Claims del access token. Es corto (15 min) porque un JWT no se puede revocar; la
// revocación inmediata la da la sesión, que el middleware revisa en cada petición.
type Claims struct {
	jwt.RegisteredClaims
	TenantID    string `json:"tid"`
	Rol         string `json:"rol"`
	SessionID   string `json:"sid"`
	DebeCambiar bool   `json:"chg,omitempty"`
}

// Principal es el usuario autenticado de una petición.
type Principal struct {
	UserID, TenantID, SessionID ids.ID
	Rol                         string
	DebeCambiar                 bool
}

type Signer struct {
	key ed25519.PrivateKey
	now func() time.Time
}

func NewSigner(key ed25519.PrivateKey, now func() time.Time) *Signer {
	return &Signer{key: key, now: now}
}

func (s *Signer) Issue(p Principal) (string, error) {
	now := s.now()
	c := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: issuer, Audience: jwt.ClaimStrings{audience}, Subject: p.UserID.String(),
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)), ID: ids.New().String(),
		},
		TenantID: p.TenantID.String(), Rol: p.Rol, SessionID: p.SessionID.String(), DebeCambiar: p.DebeCambiar,
	}
	return jwt.NewWithClaims(jwt.SigningMethodEdDSA, c).SignedString(s.key)
}

var ErrTokenInvalido = errors.New("auth: token inválido")

func (s *Signer) Parse(token string) (Principal, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		return s.key.Public(), nil
	}, jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithIssuer(issuer), jwt.WithAudience(audience),
		jwt.WithTimeFunc(s.now), jwt.WithExpirationRequired())
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %w", ErrTokenInvalido, err)
	}
	var p Principal
	var e1, e2, e3 error
	p.UserID, e1 = ids.Parse(c.Subject)
	p.TenantID, e2 = ids.Parse(c.TenantID)
	p.SessionID, e3 = ids.Parse(c.SessionID)
	if err := errors.Join(e1, e2, e3); err != nil {
		return Principal{}, fmt.Errorf("%w: %w", ErrTokenInvalido, err)
	}
	p.Rol, p.DebeCambiar = c.Rol, c.DebeCambiar
	return p, nil
}

// NewOpaqueToken genera un token aleatorio de 256 bits y su hash para guardar.
func NewOpaqueToken() (token string, hash []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token)
}

func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
