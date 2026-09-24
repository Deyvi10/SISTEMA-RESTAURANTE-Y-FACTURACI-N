// Package sri contiene la lógica fiscal pura del Servicio de Rentas Internas de Ecuador:
// clave de acceso, validación de identificaciones y catálogos. No hace I/O.
//
// Fuente: docs/05-facturacion-electronica-sri.md. Todo punto marcado 🔎 allí debe
// confirmarse contra la ficha técnica vigente guardada en docs/fuentes/sri/.
package sri

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"
)

// Longitud de la clave de acceso del esquema offline.
const ClaveAccesoLen = 49

// Ambiente del SRI.
type Ambiente int

const (
	AmbientePruebas    Ambiente = 1
	AmbienteProduccion Ambiente = 2
)

// Tipos de comprobante soportados (docs/05 §1).
const (
	TipoFactura       = "01"
	TipoNotaCredito   = "04"
	TipoEmisionNormal = "1"
)

var ErrClaveInvalida = errors.New("sri: clave de acceso inválida")

// ClaveAccesoInput son los datos con los que el Nodo Local arma la clave al cobrar.
type ClaveAccesoInput struct {
	// FechaEmision en la zona horaria del local; solo se usa la fecha.
	FechaEmision    time.Time
	TipoComprobante string   // "01", "04"
	RUC             string   // 13 dígitos del emisor
	Ambiente        Ambiente // 1 o 2
	Establecimiento string   // "001"
	PuntoEmision    string   // "002"
	Secuencial      int64    // 1 … 999999999
	// CodigoNumerico de 8 dígitos. Si está vacío se genera con crypto/rand (docs/05 §4).
	CodigoNumerico string
}

// ClaveAcceso es la clave de 49 dígitos ya validada.
type ClaveAcceso string

// NuevaClaveAcceso construye la clave: fecha(8) tipo(2) ruc(13) ambiente(1) serie(6)
// secuencial(9) código numérico(8) tipo de emisión(1) dígito verificador(1).
func NuevaClaveAcceso(in ClaveAccesoInput) (ClaveAcceso, error) {
	switch {
	case in.FechaEmision.IsZero():
		return "", fmt.Errorf("%w: falta la fecha de emisión", ErrClaveInvalida)
	case in.TipoComprobante != TipoFactura && in.TipoComprobante != TipoNotaCredito:
		return "", fmt.Errorf("%w: tipo de comprobante %q no soportado", ErrClaveInvalida, in.TipoComprobante)
	case !esDigitos(in.RUC, 13):
		return "", fmt.Errorf("%w: el RUC debe tener 13 dígitos", ErrClaveInvalida)
	case in.Ambiente != AmbientePruebas && in.Ambiente != AmbienteProduccion:
		return "", fmt.Errorf("%w: ambiente %d", ErrClaveInvalida, in.Ambiente)
	case !esDigitos(in.Establecimiento, 3) || !esDigitos(in.PuntoEmision, 3):
		return "", fmt.Errorf("%w: establecimiento y punto de emisión deben tener 3 dígitos", ErrClaveInvalida)
	case in.Secuencial < 1 || in.Secuencial > 999_999_999:
		return "", fmt.Errorf("%w: secuencial %d fuera de rango", ErrClaveInvalida, in.Secuencial)
	}

	codigo := in.CodigoNumerico
	if codigo == "" {
		var err error
		if codigo, err = CodigoNumericoAleatorio(); err != nil {
			return "", err
		}
	}
	if !esDigitos(codigo, 8) {
		return "", fmt.Errorf("%w: el código numérico debe tener 8 dígitos", ErrClaveInvalida)
	}

	base := fmt.Sprintf("%s%s%s%d%s%s%09d%s%s",
		in.FechaEmision.Format("02012006"),
		in.TipoComprobante,
		in.RUC,
		in.Ambiente,
		in.Establecimiento, in.PuntoEmision,
		in.Secuencial,
		codigo,
		TipoEmisionNormal,
	)
	return ClaveAcceso(base + strconv.Itoa(DigitoVerificadorMod11(base))), nil
}

// DigitoVerificadorMod11 calcula el dígito del SRI sobre los primeros 48 dígitos:
// de derecha a izquierda se multiplica por 2,3,4,5,6,7 en ciclo, d = 11 − (suma mod 11),
// y si d = 11 → 0, si d = 10 → 1.
func DigitoVerificadorMod11(digitos string) int {
	suma, factor := 0, 2
	for i := len(digitos) - 1; i >= 0; i-- {
		suma += int(digitos[i]-'0') * factor
		if factor++; factor > 7 {
			factor = 2
		}
	}
	switch d := 11 - suma%11; d {
	case 11:
		return 0
	case 10:
		return 1
	default:
		return d
	}
}

// ParseClaveAcceso valida longitud, dígitos y dígito verificador.
func ParseClaveAcceso(s string) (ClaveAcceso, error) {
	if !esDigitos(s, ClaveAccesoLen) {
		return "", fmt.Errorf("%w: debe tener %d dígitos", ErrClaveInvalida, ClaveAccesoLen)
	}
	if int(s[48]-'0') != DigitoVerificadorMod11(s[:48]) {
		return "", fmt.Errorf("%w: dígito verificador incorrecto", ErrClaveInvalida)
	}
	return ClaveAcceso(s), nil
}

// Campos de una clave válida.
func (c ClaveAcceso) FechaEmision() string    { return string(c[0:8]) }
func (c ClaveAcceso) TipoComprobante() string { return string(c[8:10]) }
func (c ClaveAcceso) RUC() string             { return string(c[10:23]) }
func (c ClaveAcceso) Ambiente() Ambiente      { return Ambiente(c[23] - '0') }
func (c ClaveAcceso) Serie() string           { return string(c[24:30]) }
func (c ClaveAcceso) Secuencial() string      { return string(c[30:39]) }
func (c ClaveAcceso) CodigoNumerico() string  { return string(c[39:47]) }
func (c ClaveAcceso) TipoEmision() string     { return string(c[47:48]) }
func (c ClaveAcceso) String() string          { return string(c) }

// CodigoNumericoAleatorio genera 8 dígitos criptográficamente seguros.
func CodigoNumericoAleatorio() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
	if err != nil {
		return "", fmt.Errorf("sri: sin fuente aleatoria: %w", err)
	}
	return fmt.Sprintf("%08d", n.Int64()), nil
}

func esDigitos(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
