package pdf

import (
	"errors"
	"strings"
)

// patrones128 son los 106 símbolos de Code 128 (ISO/IEC 15417) como anchos de barra y
// espacio alternados, más la parada (índice 106, 7 elementos). Cada símbolo suma 11 módulos.
var patrones128 = [...]string{
	"212222", "222122", "222221", "121223", "121322", "131222", "122213", "122312", "132212", "221213",
	"221312", "231212", "112232", "122132", "122231", "113222", "123122", "123221", "223211", "221132",
	"221231", "213212", "223112", "312131", "311222", "321122", "321221", "312212", "322112", "322211",
	"212123", "212321", "232121", "111323", "131123", "131321", "112313", "132113", "132311", "211313",
	"231113", "231311", "112133", "112331", "132131", "113123", "113321", "133121", "313121", "211331",
	"231131", "213113", "213311", "213131", "311123", "311321", "331121", "312113", "312311", "332111",
	"314111", "221411", "431111", "111224", "111422", "121124", "121421", "141122", "141221", "112214",
	"112412", "122114", "122411", "142112", "142211", "241211", "221114", "413111", "241112", "134111",
	"111242", "121142", "121241", "114212", "124112", "124211", "411212", "421112", "421211", "212141",
	"214121", "412121", "111143", "111341", "131141", "114113", "114311", "411113", "411311", "113141",
	"114131", "311141", "411131", "211412", "211214", "211232", "2331112",
}

const (
	codigoB   = 100
	codigoC   = 99
	inicioB   = 104
	inicioC   = 105
	parada128 = 106
)

// Code128Digitos codifica una cadena de dígitos (la clave de acceso) en Code 128: pares en
// el juego C y, si la cantidad es impar, el último dígito en el juego B. Devuelve los anchos
// de barras y espacios con los márgenes en blanco de 10 módulos a cada lado.
func Code128Digitos(s string) ([]int, error) {
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return nil, errors.New("pdf: Code128Digitos solo acepta dígitos")
	}
	valores := []int{inicioC}
	i := 0
	for ; i+1 < len(s); i += 2 {
		valores = append(valores, int(s[i]-'0')*10+int(s[i+1]-'0'))
	}
	if i < len(s) {
		valores = append(valores, codigoB, int(s[i])-32)
	}
	suma := valores[0]
	for k, v := range valores[1:] {
		suma += (k + 1) * v
	}
	valores = append(valores, suma%103, parada128)
	// Barras y espacios alternan empezando por barra: una barra de ancho 0 deja el margen.
	anchos := []int{0, 10}
	for _, v := range valores {
		for _, c := range patrones128[v] {
			anchos = append(anchos, int(c-'0'))
		}
	}
	return append(anchos, 10), nil
}
