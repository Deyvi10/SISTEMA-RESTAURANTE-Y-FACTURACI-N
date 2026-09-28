package auth

import (
	"strings"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/secreto"
)

// El PIN nunca se guarda ni se valida en el teléfono (hallazgo X-04): se valida en la nube o
// en el Nodo Local con el pepper del restaurante (packages/go/secreto).

// HashPIN protege un PIN ya validado. pepper es el del restaurante (secreto.PepperTenant).
func HashPIN(pepper []byte, usuario ids.ID, pin string) (string, error) {
	return secreto.HashPIN(pepper, usuario, pin)
}

// VerifyPIN comprueba un PIN contra su hash.
func VerifyPIN(pepper []byte, usuario ids.ID, pin, hash string) (bool, error) {
	return secreto.VerifyPIN(pepper, usuario, pin, hash)
}

// FingerprintPIN es determinístico para detectar PIN repetidos.
func FingerprintPIN(pepper []byte, pin string) string { return secreto.FingerprintPIN(pepper, pin) }

// ValidarPIN aplica RF-01-04: 4 a 6 dígitos, sin PIN triviales. Devuelve un mensaje o "".
func ValidarPIN(pin string) string {
	if len(pin) < 4 || len(pin) > 6 {
		return "El PIN debe tener de 4 a 6 dígitos."
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return "El PIN solo lleva números."
		}
	}
	if strings.Count(pin, pin[:1]) == len(pin) {
		return "El PIN no puede repetir el mismo número (como 1111). Elige otro."
	}
	asc, desc := true, true
	for i := 1; i < len(pin); i++ {
		asc = asc && pin[i] == pin[i-1]+1
		desc = desc && pin[i] == pin[i-1]-1
	}
	if asc || desc {
		return "El PIN no puede ser una secuencia (como 1234 o 4321). Elige otro."
	}
	if len(pin)%2 == 0 && pin[:len(pin)/2] == pin[len(pin)/2:] {
		return "El PIN no puede ser un par repetido (como 1212). Elige otro."
	}
	if pin == "2580" || pin == "0852" || pin == "1379" {
		return "Ese PIN es una línea recta en el teclado y es fácil de adivinar. Elige otro."
	}
	return ""
}
