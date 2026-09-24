package sri

import "fmt"

// TipoIdentificacion del comprador según el catálogo del SRI (docs/05 §7 🔎).
type TipoIdentificacion string

const (
	IdRUC             TipoIdentificacion = "04"
	IdCedula          TipoIdentificacion = "05"
	IdPasaporte       TipoIdentificacion = "06"
	IdConsumidorFinal TipoIdentificacion = "07"
	IdExterior        TipoIdentificacion = "08"
)

// ConsumidorFinal es la identificación fija del consumidor final.
const ConsumidorFinal = "9999999999999"

// TipoContribuyente se deduce del tercer dígito del RUC o la cédula.
type TipoContribuyente string

const (
	PersonaNatural  TipoContribuyente = "PERSONA_NATURAL"
	SociedadPrivada TipoContribuyente = "SOCIEDAD_PRIVADA"
	SectorPublico   TipoContribuyente = "SECTOR_PUBLICO"
)

// Validacion es el resultado de revisar una identificación.
// Valida=false bloquea; Advertencia (con Valida=true) solo informa al usuario.
type Validacion struct {
	Valida            bool
	Tipo              TipoIdentificacion
	TipoContribuyente TipoContribuyente
	Motivo            string // por qué no es válida, en lenguaje claro
	Advertencia       string
}

// ValidarCedula aplica la regla de la cédula ecuatoriana (docs/05 §12):
// provincia 01-24 o 30, tercer dígito < 6 y verificador módulo 10 con coeficientes 2,1,2,…
func ValidarCedula(ced string) Validacion {
	if !esDigitos(ced, 10) {
		return invalida(IdCedula, "La cédula debe tener 10 dígitos.")
	}
	if m := motivoProvincia(ced); m != "" {
		return invalida(IdCedula, m)
	}
	if ced[2] >= '6' {
		return invalida(IdCedula, "El tercer dígito de una cédula debe ser menor que 6.")
	}
	suma := 0
	for i := 0; i < 9; i++ {
		p := int(ced[i]-'0') * (2 - i%2)
		if p > 9 {
			p -= 9
		}
		suma += p
	}
	if (10-suma%10)%10 != int(ced[9]-'0') {
		return invalida(IdCedula, "El dígito verificador de la cédula no coincide. Revisa el número.")
	}
	return Validacion{Valida: true, Tipo: IdCedula, TipoContribuyente: PersonaNatural}
}

// ValidarRUC valida un RUC de 13 dígitos según el tipo de contribuyente.
//
// Existen RUC de sociedades emitidos que no cumplen el módulo 11 (docs/05 §12): para
// sociedades privadas y públicas un verificador incorrecto es solo una advertencia y el
// SRI es la validación final. Para personas naturales sí bloquea (es una cédula).
func ValidarRUC(ruc string) Validacion {
	if !esDigitos(ruc, 13) {
		return invalida(IdRUC, "El RUC debe tener 13 dígitos.")
	}
	if m := motivoProvincia(ruc); m != "" {
		return invalida(IdRUC, m)
	}
	switch tercero := ruc[2]; {
	case tercero < '6':
		v := ValidarCedula(ruc[:10])
		if !v.Valida {
			return invalida(IdRUC, "Los 10 primeros dígitos no forman una cédula válida.")
		}
		if ruc[10:] == "000" {
			return invalida(IdRUC, "El RUC debe terminar en un establecimiento distinto de 000 (normalmente 001).")
		}
		return Validacion{Valida: true, Tipo: IdRUC, TipoContribuyente: PersonaNatural}
	case tercero == '9':
		ok := mod11(ruc[:9], []int{4, 3, 2, 7, 6, 5, 4, 3, 2}, int(ruc[9]-'0'))
		if ruc[10:] == "000" {
			return invalida(IdRUC, "El RUC debe terminar en un establecimiento distinto de 000 (normalmente 001).")
		}
		return conAdvertencia(SociedadPrivada, ok)
	case tercero == '6':
		ok := mod11(ruc[:8], []int{3, 2, 7, 6, 5, 4, 3, 2}, int(ruc[8]-'0'))
		if ruc[9:] == "0000" {
			return invalida(IdRUC, "El RUC público debe terminar en un establecimiento distinto de 0000 (normalmente 0001).")
		}
		return conAdvertencia(SectorPublico, ok)
	default:
		return invalida(IdRUC, fmt.Sprintf("El tercer dígito %c no corresponde a ningún tipo de RUC.", tercero))
	}
}

// ValidarRUCEmisor es la validación estricta para el RUC de un restaurante (RF-01-01):
// además de ValidarRUC exige que termine en 001 (matriz).
func ValidarRUCEmisor(ruc string) Validacion {
	v := ValidarRUC(ruc)
	matriz := ruc[10:] == "001" || (v.TipoContribuyente == SectorPublico && ruc[9:] == "0001")
	if v.Valida && !matriz {
		return invalida(IdRUC, "El RUC del emisor debe terminar en 001.")
	}
	return v
}

// ValidarIdentificacion detecta el tipo por el contenido para el campo único
// «Cédula / RUC / Pasaporte» de la caja (RF-04-05): 10 dígitos → cédula,
// 13 dígitos → RUC (o consumidor final), con letras → pasaporte.
//
// Un número puro de otra longitud casi siempre es una cédula o un RUC mal digitado,
// así que se rechaza con un mensaje que lo dice. Un pasaporte solo numérico se
// registra eligiendo el tipo explícitamente (ValidarPasaporte).
func ValidarIdentificacion(id string) Validacion {
	switch {
	case id == ConsumidorFinal:
		return Validacion{Valida: true, Tipo: IdConsumidorFinal}
	case esDigitos(id, 10):
		return ValidarCedula(id)
	case esDigitos(id, 13):
		return ValidarRUC(id)
	case esDigitos(id, len(id)) && id != "":
		return invalida("", fmt.Sprintf("Tiene %d dígitos. Una cédula tiene 10 y un RUC 13. Si es un pasaporte, elige ese tipo.", len(id)))
	default:
		if v := ValidarPasaporte(id); v.Valida {
			return v
		}
		return invalida("", "Escribe una cédula (10 dígitos), un RUC (13 dígitos) o un pasaporte (solo letras y números).")
	}
}

// ValidarPasaporte acepta de 3 a 20 letras o dígitos.
func ValidarPasaporte(id string) Validacion {
	if len(id) >= 3 && len(id) <= 20 && esAlfanumerico(id) {
		return Validacion{Valida: true, Tipo: IdPasaporte}
	}
	return invalida(IdPasaporte, "El pasaporte debe tener de 3 a 20 letras o números, sin espacios ni guiones.")
}

func motivoProvincia(s string) string {
	p := int(s[0]-'0')*10 + int(s[1]-'0')
	if (p < 1 || p > 24) && p != 30 {
		return fmt.Sprintf("El código de provincia %02d no existe (debe ser 01-24 o 30).", p)
	}
	return ""
}

func mod11(digitos string, coef []int, verificador int) bool {
	suma := 0
	for i := range coef {
		suma += int(digitos[i]-'0') * coef[i]
	}
	d := 11 - suma%11
	if d == 11 {
		d = 0
	}
	return d == verificador // d = 10 nunca coincide: no existe ese verificador
}

func conAdvertencia(t TipoContribuyente, ok bool) Validacion {
	v := Validacion{Valida: true, Tipo: IdRUC, TipoContribuyente: t}
	if !ok {
		v.Advertencia = "El dígito verificador no cumple el módulo 11. Algunos RUC de sociedades son así; el SRI hará la validación final."
	}
	return v
}

func invalida(t TipoIdentificacion, motivo string) Validacion {
	return Validacion{Valida: false, Tipo: t, Motivo: motivo}
}

func esAlfanumerico(s string) bool {
	for _, r := range s {
		esDigito, esLetra := r >= '0' && r <= '9', r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
		if !esDigito && !esLetra {
			return false
		}
	}
	return true
}
