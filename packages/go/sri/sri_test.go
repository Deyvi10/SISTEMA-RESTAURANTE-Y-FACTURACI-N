package sri

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/testdata"
	"pgregory.net/rapid"
)

func cargar(t *testing.T, nombre string, v any) {
	t.Helper()
	b, err := testdata.FS.ReadFile(nombre)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

// QA-04: vectores conocidos.
func TestClaveAccesoVectores(t *testing.T) {
	var v struct {
		Validas   []struct{ Clave string }
		Invalidas []struct{ Clave, Nota string }
		Digito    []struct {
			Base48 string
			Digito int
			Nota   string
		}
	}
	cargar(t, "sri-claves-acceso.json", &v)
	for _, c := range v.Validas {
		if _, err := ParseClaveAcceso(c.Clave); err != nil {
			t.Errorf("%s debería ser válida: %v", c.Clave, err)
		}
	}
	for _, c := range v.Invalidas {
		if _, err := ParseClaveAcceso(c.Clave); !errors.Is(err, ErrClaveInvalida) {
			t.Errorf("%s (%s) debería ser inválida", c.Clave, c.Nota)
		}
	}
	for _, c := range v.Digito {
		if got := DigitoVerificadorMod11(c.Base48); got != c.Digito {
			t.Errorf("%s: dígito %d, se esperaba %d", c.Nota, got, c.Digito)
		}
	}
}

func TestNuevaClaveAccesoReproduceFicha(t *testing.T) {
	c, err := NuevaClaveAcceso(ClaveAccesoInput{
		FechaEmision:    time.Date(2011, 10, 21, 0, 0, 0, 0, clock.Guayaquil),
		TipoComprobante: TipoFactura,
		RUC:             "1792146739001",
		Ambiente:        AmbientePruebas,
		Establecimiento: "002",
		PuntoEmision:    "001",
		Secuencial:      1,
		CodigoNumerico:  "12345678",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c != "2110201101179214673900110020010000000011234567813" {
		t.Fatalf("clave = %s", c)
	}
	if c.RUC() != "1792146739001" || c.Serie() != "002001" || c.Secuencial() != "000000001" || c.Ambiente() != AmbientePruebas || c.TipoEmision() != "1" {
		t.Fatalf("campos mal extraídos de %s", c)
	}
}

func TestNuevaClaveAccesoRechazaDatosMalos(t *testing.T) {
	ok := ClaveAccesoInput{FechaEmision: time.Now(), TipoComprobante: "01", RUC: "1790011674001", Ambiente: 2, Establecimiento: "001", PuntoEmision: "001", Secuencial: 1}
	malos := map[string]func(*ClaveAccesoInput){
		"sin fecha":        func(i *ClaveAccesoInput) { i.FechaEmision = time.Time{} },
		"tipo 05":          func(i *ClaveAccesoInput) { i.TipoComprobante = "05" },
		"RUC corto":        func(i *ClaveAccesoInput) { i.RUC = "179001167400" },
		"ambiente 3":       func(i *ClaveAccesoInput) { i.Ambiente = 3 },
		"serie con letras": func(i *ClaveAccesoInput) { i.PuntoEmision = "0A1" },
		"secuencial 0":     func(i *ClaveAccesoInput) { i.Secuencial = 0 },
		"secuencial largo": func(i *ClaveAccesoInput) { i.Secuencial = 1_000_000_000 },
		"código 7 dígitos": func(i *ClaveAccesoInput) { i.CodigoNumerico = "1234567" },
	}
	for nombre, cambio := range malos {
		in := ok
		cambio(&in)
		if _, err := NuevaClaveAcceso(in); !errors.Is(err, ErrClaveInvalida) {
			t.Errorf("%s: se esperaba ErrClaveInvalida, obtuvo %v", nombre, err)
		}
	}
}

// QA-04: propiedad sobre claves aleatorias (la ejecución larga de 10⁶ va en el CI nocturno con -rapid.checks).
func TestPropertyClaveAcceso(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		dias := rapid.IntRange(0, 365*30).Draw(t, "dias")
		in := ClaveAccesoInput{
			FechaEmision:    time.Date(2010, 1, 1, 0, 0, 0, 0, clock.Guayaquil).AddDate(0, 0, dias),
			TipoComprobante: rapid.SampledFrom([]string{TipoFactura, TipoNotaCredito}).Draw(t, "tipo"),
			RUC:             rapid.StringMatching(`[0-9]{13}`).Draw(t, "ruc"),
			Ambiente:        Ambiente(rapid.IntRange(1, 2).Draw(t, "amb")),
			Establecimiento: rapid.StringMatching(`[0-9]{3}`).Draw(t, "estab"),
			PuntoEmision:    rapid.StringMatching(`[0-9]{3}`).Draw(t, "pto"),
			Secuencial:      rapid.Int64Range(1, 999_999_999).Draw(t, "sec"),
		}
		c, err := NuevaClaveAcceso(in)
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != ClaveAccesoLen {
			t.Fatalf("longitud %d", len(c))
		}
		if _, err := ParseClaveAcceso(string(c)); err != nil {
			t.Fatalf("clave generada no valida: %v", err)
		}
		// Alterar un dígito siempre cambia la suma módulo 11 (pesos 2..7 y deltas 1..9
		// no son múltiplos de 11). La única ceguera del esquema del SRI es que los
		// residuos 1 y 10 producen ambos el dígito 1, así que ese caso se excluye.
		if c[48] == '1' {
			return
		}
		pos := rapid.IntRange(0, 47).Draw(t, "pos")
		b := []byte(c)
		b[pos] = '0' + (b[pos]-'0'+byte(rapid.IntRange(1, 9).Draw(t, "delta")))%10
		if _, err := ParseClaveAcceso(string(b)); err == nil {
			t.Fatalf("alterar la posición %d no se detectó", pos)
		}
	})
}

func TestIdentificacionesVectores(t *testing.T) {
	var v struct {
		Casos []struct {
			ID            string `json:"id"`
			Valida        bool
			Tipo          string
			Contribuyente string
			Advertencia   bool
			Nota          string
		}
	}
	cargar(t, "identificaciones.json", &v)
	for _, c := range v.Casos {
		got := ValidarIdentificacion(c.ID)
		if got.Valida != c.Valida || string(got.Tipo) != c.Tipo {
			t.Errorf("%s (%s): válida=%v tipo=%q, se esperaba válida=%v tipo=%q (%s)", c.ID, c.Nota, got.Valida, got.Tipo, c.Valida, c.Tipo, got.Motivo)
		}
		if c.Contribuyente != "" && string(got.TipoContribuyente) != c.Contribuyente {
			t.Errorf("%s: contribuyente %s, se esperaba %s", c.ID, got.TipoContribuyente, c.Contribuyente)
		}
		if (got.Advertencia != "") != c.Advertencia {
			t.Errorf("%s: advertencia %q", c.ID, got.Advertencia)
		}
		if !got.Valida && got.Motivo == "" {
			t.Errorf("%s: inválida sin motivo para el usuario", c.ID)
		}
	}
}

func TestRUCEmisorExige001(t *testing.T) {
	if !ValidarRUCEmisor("1790011674001").Valida {
		t.Error("1790011674001 debería ser un emisor válido")
	}
	if ValidarRUCEmisor("1790011674002").Valida {
		t.Error("un emisor debe terminar en 001")
	}
}
