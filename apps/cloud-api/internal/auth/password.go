package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Parámetros Argon2id (OWASP 2024: m ≥ 19 MiB, t ≥ 2). Se guardan en el hash (formato PHC),
// así se pueden subir en el futuro sin invalidar contraseñas existentes.
const (
	argonTime    = 2
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	argonKeyLen  = 32
)

// HashPassword devuelve $argon2id$v=19$m=…,t=…,p=…$sal$hash.
func HashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword compara en tiempo constante.
func VerifyPassword(pw, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("auth: hash con formato desconocido")
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

// dummyHash se verifica cuando el usuario no existe, para que el tiempo de respuesta no
// revele qué correos están registrados.
var dummyHash, _ = HashPassword("usuario-inexistente")

var comunes = map[string]bool{
	"1234567890": true, "0987654321": true, "contraseña": true, "contrasena": true, "password12": true,
	"qwertyuiop": true, "restaurante": true, "ecuador123": true, "1234567891": true, "aaaaaaaaaa": true,
}

// ValidarPassword aplica la política: 10 a 128 caracteres, con letras y números,
// sin repetir el correo ni ser una clave común. Devuelve un mensaje para el usuario.
func ValidarPassword(pw, email string) string {
	n := utf8.RuneCountInString(pw)
	switch {
	case n < 10:
		return "Usa al menos 10 caracteres."
	case n > 128:
		return "Usa como máximo 128 caracteres."
	case comunes[strings.ToLower(pw)]:
		return "Esa contraseña es muy común. Elige otra."
	case len(localPart(email)) >= 4 && strings.Contains(strings.ToLower(pw), localPart(email)):
		return "La contraseña no debe contener tu correo."
	}
	var letra, digito bool
	for _, r := range pw {
		letra = letra || unicode.IsLetter(r)
		digito = digito || unicode.IsDigit(r)
	}
	if !letra || !digito {
		return "Combina letras y números."
	}
	return ""
}

// localPart devuelve la parte del correo antes de la @, en minúsculas.
func localPart(email string) string {
	return strings.ToLower(strings.SplitN(email, "@", 2)[0])
}

// PasswordTemporal genera una contraseña aleatoria de 14 caracteres legibles (RF-01-01.4).
func PasswordTemporal() string {
	const alfabeto = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for {
		b := make([]byte, 14)
		_, _ = rand.Read(b)
		for i := range b {
			b[i] = alfabeto[int(b[i])%len(alfabeto)]
		}
		if s := string(b); ValidarPassword(s, "") == "" {
			return s
		}
	}
}
