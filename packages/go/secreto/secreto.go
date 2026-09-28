// Package secreto guarda y verifica contraseñas y PIN (Argon2id) igual en la nube y en el
// Nodo Local, que valida los PIN sin internet (F3-04).
//
// PIN: pin_hash = Argon2id(HMAC-SHA256(pepper_tenant, pin‖usuario)).
// pepper_tenant = HMAC-SHA256(pepper_global, "pin-tenant|"+tenant): cada nodo recibe solo
// el de su restaurante, así una PC robada no sirve para atacar los PIN de otros restaurantes.
package secreto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Parámetros Argon2id (OWASP 2024: m ≥ 19 MiB, t ≥ 2). Se guardan en el hash (formato PHC),
// así se pueden subir en el futuro sin invalidar lo existente.
const (
	argonTime    = 2
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	argonKeyLen  = 32
)

// Hash devuelve $argon2id$v=19$m=…,t=…,p=…$sal$hash.
func Hash(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify compara en tiempo constante.
func Verify(pw, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("secreto: hash con formato desconocido")
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, err
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want))) //nolint:gosec // G115: largo de un hash de 32 bytes
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// PepperTenant deriva el pepper de PIN de un restaurante.
func PepperTenant(global []byte, tenant ids.ID) []byte {
	m := hmac.New(sha256.New, global)
	m.Write([]byte("pin-tenant|" + tenant.String()))
	return m.Sum(nil)
}

// HashPIN protege un PIN ya validado.
func HashPIN(pepper []byte, usuario ids.ID, pin string) (string, error) {
	return Hash(mac(pepper, pin+"|"+usuario.String()))
}

// VerifyPIN comprueba un PIN contra su hash.
func VerifyPIN(pepper []byte, usuario ids.ID, pin, hash string) (bool, error) {
	return Verify(mac(pepper, pin+"|"+usuario.String()), hash)
}

// FingerprintPIN es determinístico: permite exigir PIN único por restaurante sin revelarlo.
func FingerprintPIN(pepper []byte, pin string) string { return mac(pepper, "fp|"+pin) }

func mac(pepper []byte, s string) string {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(s))
	return hex.EncodeToString(m.Sum(nil))
}
