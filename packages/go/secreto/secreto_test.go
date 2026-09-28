package secreto

import (
	"bytes"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

func TestPINPorRestaurante(t *testing.T) {
	global := []byte("pepper-global-de-pruebas-0123456789")
	a, b := ids.New(), ids.New()
	pa, pb := PepperTenant(global, a), PepperTenant(global, b)
	if bytes.Equal(pa, pb) || bytes.Equal(pa, PepperTenant([]byte("otro-global-0123456789012345678901"), a)) {
		t.Fatal("el pepper debe depender del global y del restaurante")
	}
	u := ids.New()
	h, err := HashPIN(pa, u, "8899")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := VerifyPIN(pa, u, "8899", h); !ok {
		t.Fatal("PIN correcto rechazado")
	}
	// Con el pepper de otro restaurante (p. ej. un nodo robado de B) no se valida el PIN de A.
	if ok, _ := VerifyPIN(pb, u, "8899", h); ok {
		t.Fatal("el pepper de B validó un PIN de A")
	}
	if ok, _ := VerifyPIN(pa, u, "8898", h); ok {
		t.Fatal("PIN incorrecto aceptado")
	}
	if FingerprintPIN(pa, "8899") == FingerprintPIN(pb, "8899") {
		t.Fatal("la huella del PIN no debe coincidir entre restaurantes")
	}
}
