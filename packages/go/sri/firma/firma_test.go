package firma

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // se verifica el resumen SHA-1 que exige el SRI
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"software.sslmate.com/src/go-pkcs12"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/money"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
)

var ahoraPrueba = time.Date(2026, 9, 29, 13, 0, 0, 0, clock.Guayaquil)

// certificado de prueba (ficticio): RSA de los bits pedidos, vigente del from al to.
func certificado(t *testing.T, bits int, desde, hasta time.Time) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	llave, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	plantilla := &x509.Certificate{
		SerialNumber: big.NewInt(1312833444),
		// Autofirmado: el emisor es el mismo sujeto; el & prueba el escape en X509IssuerName.
		Subject:   pkix.Name{CommonName: "RESTAURANTE DE PRUEBA & CIA", SerialNumber: "1710034065001", Country: []string{"EC"}},
		NotBefore: desde, NotAfter: hasta,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
	}
	der, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &llave.PublicKey, llave)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return llave, c
}

func p12(t *testing.T, llave *rsa.PrivateKey, c *x509.Certificate, cadena []*x509.Certificate, clave string) []byte {
	t.Helper()
	b, err := pkcs12.Modern.Encode(llave, c, cadena, clave)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func facturaSinFirmar(t *testing.T, descripcion string) []byte {
	t.Helper()
	iva := sri.Tarifa{Porcentaje: "15", Codigo: "4"}
	f, err := sri.Desglosar(sri.Venta{
		Lineas:     []sri.LineaVenta{{Codigo: "CEV", Descripcion: descripcion, Cantidad: decimal.NewFromInt(1), Bruto: money.MustParse("15.00"), Final: money.MustParse("15.00"), Tarifa: iva}},
		PorTarifa:  map[string]sri.TotalTarifa{"15": {Base: money.MustParse("13.04"), IVA: money.MustParse("1.96")}},
		IncluyeIVA: true, Total: money.MustParse("15.00"),
	})
	if err != nil {
		t.Fatal(err)
	}
	clave, err := sri.NuevaClaveAcceso(sri.ClaveAccesoInput{FechaEmision: ahoraPrueba, TipoComprobante: sri.TipoFactura, RUC: "1710034065001",
		Ambiente: sri.AmbientePruebas, Establecimiento: "001", PuntoEmision: "002", Secuencial: 67, CodigoNumerico: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := sri.FacturaXML(sri.DatosFactura{Ambiente: sri.AmbientePruebas, ClaveAcceso: clave, Secuencial: 67, Fecha: ahoraPrueba,
		Emisor:    sri.Emisor{RUC: "1710034065001", RazonSocial: "PÉREZ JOSÉ", DirMatriz: "Guayaquil", Establecimiento: "001", PuntoEmision: "002"},
		Comprador: sri.Comprador{TipoIdentificacion: "07", Identificacion: "9999999999999", RazonSocial: "CONSUMIDOR FINAL"},
		Pagos:     []sri.Pago{{FormaPago: "01", Total: f.ImporteTotal}}}, f)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func firmante(t *testing.T) (*Firmante, *x509.Certificate) {
	t.Helper()
	llave, c := certificado(t, 2048, ahoraPrueba.AddDate(-1, 0, 0), ahoraPrueba.AddDate(1, 0, 0))
	f, err := CargarP12(p12(t, llave, c, nil, "clave de prueba"), "clave de prueba", ahoraPrueba)
	if err != nil {
		t.Fatal(err)
	}
	f.Ahora, f.Zona = func() time.Time { return ahoraPrueba }, clock.Guayaquil
	return f, c
}

// verificarConSignxml verifica con una implementación independiente (signxml, sobre libxml2):
// las tres referencias y la firma RSA-SHA1. Sin signxml la prueba se salta; CI lo instala.
func verificarConSignxml(t *testing.T, doc []byte, c *x509.Certificate) error {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil || exec.Command(py, "-c", "import signxml").Run() != nil {
		t.Skip("sin signxml para verificar la firma de forma independiente")
	}
	dir := t.TempDir()
	xmlF, certF := filepath.Join(dir, "firmado.xml"), filepath.Join(dir, "cert.pem")
	_ = os.WriteFile(xmlF, doc, 0o600)
	_ = os.WriteFile(certF, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw}), 0o600)
	const script = `import sys
from signxml import XMLVerifier, SignatureConfiguration, SignatureMethod, DigestAlgorithm
cfg = SignatureConfiguration(signature_methods=frozenset({SignatureMethod.RSA_SHA1}),
                             digest_algorithms=frozenset({DigestAlgorithm.SHA1}), expect_references=3)
res = XMLVerifier().verify(open(sys.argv[1], "rb").read(), x509_cert=open(sys.argv[2]).read(), expect_config=cfg)
uris = sorted(r.signed_xml.get("Id") or r.signed_xml.tag for r in res)
print(len(res), uris)`
	out, err := exec.Command(py, "-c", script, xmlF, certF).CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	if !strings.HasPrefix(string(out), "3 ") {
		return errors.New("signxml no verificó las 3 referencias: " + string(out))
	}
	return nil
}

func TestFirmaVerificadaPorSignxml(t *testing.T) {
	f, c := firmante(t)
	// Texto con los caracteres que la forma canónica escapa distinto que encoding/xml.
	doc := facturaSinFirmar(t, `Ceviche "de la casa" & ají <picante> 'mixto'`)
	firmado, err := f.Firmar(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := verificarConSignxml(t, firmado, c); err != nil {
		t.Fatalf("la firma no verifica:\n%s\n%s", err, firmado)
	}
}

func TestFirmaSigueElAnexo4(t *testing.T) {
	f, c := firmante(t)
	firmado, err := f.Firmar(facturaSinFirmar(t, "Ceviche"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(firmado)
	resumen := sha1.Sum(c.Raw) //nolint:gosec // ficha §6.8
	for _, quiero := range []string{
		`<ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#" xmlns:etsi="http://uri.etsi.org/01903/v1.3.2#" Id="Signature`,
		`<ds:CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315">`,
		`<ds:SignatureMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1">`,
		`Type="http://uri.etsi.org/01903#SignedProperties"`,
		`URI="#comprobante"><ds:Transforms><ds:Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature">`,
		`<ds:Exponent>AQAB</ds:Exponent>`,
		`<etsi:SigningTime>2026-09-29T13:00:00-05:00</etsi:SigningTime>`,
		`<ds:DigestValue>` + base64.StdEncoding.EncodeToString(resumen[:]) + `</ds:DigestValue>`,
		`<ds:X509IssuerName>SERIALNUMBER=1710034065001,CN=RESTAURANTE DE PRUEBA &amp; CIA,C=EC</ds:X509IssuerName>`,
		`<ds:X509SerialNumber>1312833444</ds:X509SerialNumber>`,
		`<etsi:Description>contenido comprobante</etsi:Description><etsi:MimeType>text/xml</etsi:MimeType>`,
		`</ds:Signature></factura>`,
	} {
		if !strings.Contains(s, quiero) {
			t.Errorf("falta %s", quiero)
		}
	}
}

// El XML firmado sigue validando contra el XSD oficial (que incluye el esquema de firma W3C).
func TestFirmadoValidaContraElXSD(t *testing.T) {
	f, _ := firmante(t)
	firmado, err := f.Firmar(facturaSinFirmar(t, "Ceviche"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, n := range []string{"factura_V1.1.0.xsd", "xmldsig-core-schema.xsd"} {
		b, _ := sri.XSD.ReadFile("xsd/" + n)
		_ = os.WriteFile(filepath.Join(dir, n), b, 0o600)
	}
	archivo := filepath.Join(dir, "firmado.xml")
	_ = os.WriteFile(archivo, firmado, 0o600)
	var cmd *exec.Cmd
	if x, err := exec.LookPath("xmllint"); err == nil {
		cmd = exec.Command(x, "--noout", "--nonet", "--schema", filepath.Join(dir, "factura_V1.1.0.xsd"), archivo)
	} else if py, err := exec.LookPath("python3"); err == nil && exec.Command(py, "-c", "import lxml").Run() == nil {
		cmd = exec.Command(py, "-c", `import sys
from lxml import etree
p = etree.XMLParser(no_network=True, load_dtd=False)
s = etree.XMLSchema(etree.parse(sys.argv[1], p))
d = etree.parse(sys.argv[2], p)
sys.exit(0 if s.validate(d) else print(s.error_log) or 1)`, filepath.Join(dir, "factura_V1.1.0.xsd"), archivo)
	} else {
		t.Skip("sin validador XSD")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("el XSD rechaza el comprobante firmado: %s", out)
	}
}

// Cambiar un centavo del comprobante firmado rompe la firma.
func TestAlterarElComprobanteRompeLaFirma(t *testing.T) {
	f, c := firmante(t)
	firmado, err := f.Firmar(facturaSinFirmar(t, "Ceviche"))
	if err != nil {
		t.Fatal(err)
	}
	alterado := bytes.Replace(firmado, []byte("<importeTotal>15.00</importeTotal>"), []byte("<importeTotal>15.01</importeTotal>"), 1)
	if bytes.Equal(alterado, firmado) {
		t.Fatal("no encontré el importe para alterarlo")
	}
	if err := verificarConSignxml(t, alterado, c); err == nil {
		t.Fatal("un comprobante alterado no debe verificar")
	}
}

func TestCargarP12(t *testing.T) {
	vigente := func() (time.Time, time.Time) { return ahoraPrueba.AddDate(-1, 0, 0), ahoraPrueba.AddDate(1, 0, 0) }
	d, h := vigente()
	llave, c := certificado(t, 2048, d, h)
	if _, err := CargarP12(p12(t, llave, c, nil, "buena"), "mala", ahoraPrueba); !errors.Is(err, ErrFirma) {
		t.Errorf("contraseña equivocada: %v", err)
	}
	vencida, cv := certificado(t, 2048, ahoraPrueba.AddDate(-3, 0, 0), ahoraPrueba.AddDate(-1, 0, 0))
	if _, err := CargarP12(p12(t, vencida, cv, nil, "x"), "x", ahoraPrueba); err == nil || !strings.Contains(err.Error(), "vigente") {
		t.Errorf("certificado vencido: %v", err)
	}
	corta, cc := certificado(t, 1024, d, h)
	if _, err := CargarP12(p12(t, corta, cc, nil, "x"), "x", ahoraPrueba); err == nil || !strings.Contains(err.Error(), "2048") {
		t.Errorf("llave de 1024 bits: %v", err)
	}
	// Con la cadena del proveedor: elige el certificado que corresponde a la llave.
	_, raizCA := certificado(t, 2048, d, h)
	f, err := CargarP12(p12(t, llave, c, []*x509.Certificate{raizCA}, "x"), "x", ahoraPrueba)
	if err != nil || f.Certificado().SerialNumber.Cmp(c.SerialNumber) != 0 || !bytes.Equal(f.Certificado().Raw, c.Raw) {
		t.Errorf("con cadena: %v", err)
	}
}

func TestNoFirmaDosVecesNiComprobantesRaros(t *testing.T) {
	f, _ := firmante(t)
	firmado, err := f.Firmar(facturaSinFirmar(t, "Ceviche"))
	if err != nil {
		t.Fatal(err)
	}
	for nombre, doc := range map[string][]byte{
		"ya firmado":     firmado,
		"sin id":         []byte(`<factura version="1.1.0"></factura>`),
		"con namespaces": []byte(`<a:factura xmlns:a="urn:x" id="comprobante"></a:factura>`),
		"no es XML":      []byte(`hola`),
	} {
		if _, err := f.Firmar(doc); !errors.Is(err, ErrFirma) {
			t.Errorf("%s: se esperaba ErrFirma, llegó %v", nombre, err)
		}
	}
}
