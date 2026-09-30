package firma

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/sriws"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/tools/sri-stub/stub"
)

// La cadena fiscal completa sin el .p12 real: venta → desglose (F5-04) → XML 1.1.0 (F5-08)
// → firma XAdES-BES (F5-09) → recepción y autorización contra el stub del SRI (F5-10).
func TestCadenaFiscalHastaAutorizada(t *testing.T) {
	f, c := firmante(t)
	firmado, err := f.Firmar(facturaSinFirmar(t, "Ceviche de camarón"))
	if err != nil {
		t.Fatal(err)
	}
	if err := verificarConSignxml(t, firmado, c); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(stub.NewStub(time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes())
	defer srv.Close()
	cli := &sriws.Cliente{Host: srv.URL, HTTP: srv.Client()}
	clave, err := sri.ParseClaveAcceso(entre(string(firmado), "<claveAcceso>", "</claveAcceso>"))
	if err != nil {
		t.Fatal(err)
	}
	e := sriws.Envio{Clave: clave}
	ahora := ahoraPrueba
	for range 5 {
		if e.Fase.Final() {
			break
		}
		e = cli.Avanzar(context.Background(), e, firmado, ahora, sriws.PoliticaPorDefecto)
		ahora = e.Proximo
	}
	if e.Fase != sriws.Autorizada || e.NumeroAutorizacion != clave.String() || !strings.Contains(e.XMLAutorizado, "<ds:SignatureValue") {
		t.Fatalf("envío: %+v", e)
	}
}

func entre(s, a, b string) string {
	i := strings.Index(s, a)
	j := strings.Index(s, b)
	if i < 0 || j < i {
		return ""
	}
	return s[i+len(a) : j]
}
