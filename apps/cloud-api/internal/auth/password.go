package auth

import (
	"crypto/rand"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/secreto"

	"strings"
	"unicode"
	"unicode/utf8"
)

// HashPassword devuelve $argon2id$v=19$m=…,t=…,p=…$sal$hash (packages/go/secreto).
func HashPassword(pw string) (string, error) { return secreto.Hash(pw) }

// VerifyPassword compara en tiempo constante.
func VerifyPassword(pw, encoded string) (bool, error) { return secreto.Verify(pw, encoded) }

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
