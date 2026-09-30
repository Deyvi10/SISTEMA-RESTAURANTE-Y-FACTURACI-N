// Package firma firma comprobantes del SRI con XAdES-BES (F5-09).
//
// Especificación: ficha técnica offline v2.34 §6 y la factura firmada de ejemplo del Anexo 4
// (documentacion-proyecto/docs/fuentes/sri/): XAdES-BES v1.3.2, firma ENVELOPED al final del
// comprobante, C14N 1.0 inclusiva, RSA-SHA1 con clave de 2048 bits, resúmenes SHA-1, y tres
// referencias: SignedProperties, KeyInfo (el certificado) y #comprobante.
//
// El .p12 solo se abre en el worker fiscal de la nube (CLAUDE.md, docs/06 §4): este paquete no
// guarda nada en disco ni en logs.
package firma

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // el SRI exige RSA-SHA1 y resúmenes SHA-1 (ficha §6.8)
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strings"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

const (
	nsDS   = "http://www.w3.org/2000/09/xmldsig#"
	nsETSI = "http://uri.etsi.org/01903/v1.3.2#"
	// Declaraciones que cada parte firmada hereda de <ds:Signature>: en la forma canónica
	// inclusiva aparecen en el elemento raíz de la parte (ds antes que etsi).
	nsHeredados = ` xmlns:ds="` + nsDS + `" xmlns:etsi="` + nsETSI + `"`
)

var ErrFirma = errors.New("firma: no se pudo firmar")

// Firmante tiene el certificado y la llave del emisor, ya abiertos.
type Firmante struct {
	cert  *x509.Certificate
	llave *rsa.PrivateKey
	Ahora func() time.Time
	// Zona de la hora de firma (la del local).
	Zona *time.Location
}

// CargarP12 abre el .p12 del emisor. Si trae varios certificados (cadena del proveedor), usa
// el que corresponde a la llave privada. Exige RSA de 2048 bits o más y un certificado vigente.
func CargarP12(p12 []byte, clave string, ahora time.Time) (*Firmante, error) {
	llave, cert, cadena, err := pkcs12.DecodeChain(p12, clave)
	if err != nil {
		return nil, fmt.Errorf("%w: el archivo .p12 o su contraseña no son válidos: %v", ErrFirma, err)
	}
	rsaKey, ok := llave.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: la llave del certificado no es RSA", ErrFirma)
	}
	// El certificado de la llave puede venir en la cadena en vez de como principal.
	for _, c := range append([]*x509.Certificate{cert}, cadena...) {
		if pub, ok := c.PublicKey.(*rsa.PublicKey); ok && pub.N.Cmp(rsaKey.N) == 0 {
			cert = c
			break
		}
	}
	if pub, ok := cert.PublicKey.(*rsa.PublicKey); !ok || pub.N.Cmp(rsaKey.N) != 0 {
		return nil, fmt.Errorf("%w: ningún certificado del .p12 corresponde a su llave", ErrFirma)
	}
	if rsaKey.N.BitLen() < 2048 {
		return nil, fmt.Errorf("%w: la llave tiene %d bits; el SRI exige 2048", ErrFirma, rsaKey.N.BitLen())
	}
	if ahora.Before(cert.NotBefore) || ahora.After(cert.NotAfter) {
		return nil, fmt.Errorf("%w: el certificado no está vigente (del %s al %s)", ErrFirma, cert.NotBefore.Format(time.DateOnly), cert.NotAfter.Format(time.DateOnly))
	}
	return &Firmante{cert: cert, llave: rsaKey, Ahora: time.Now, Zona: time.Local}, nil
}

// Certificado devuelve el certificado firmante (para mostrar vencimiento en el panel).
func (f *Firmante) Certificado() *x509.Certificate { return f.cert }

// Firmar agrega <ds:Signature> al final del comprobante (elemento raíz con id="comprobante").
func (f *Firmante) Firmar(comprobante []byte) ([]byte, error) {
	canon, raiz, err := canonico(comprobante)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFirma, err)
	}
	ids, err := nuevosIDs()
	if err != nil {
		return nil, err
	}
	digestComprobante := b64sha1(canon)

	certB64 := base64.StdEncoding.EncodeToString(f.cert.Raw)
	pub := f.cert.PublicKey.(*rsa.PublicKey)
	keyInfo := func(ns string) string {
		return `<ds:KeyInfo` + ns + ` Id="` + ids.cert + `"><ds:X509Data><ds:X509Certificate>` + certB64 + `</ds:X509Certificate></ds:X509Data>` +
			`<ds:KeyValue><ds:RSAKeyValue><ds:Modulus>` + base64.StdEncoding.EncodeToString(pub.N.Bytes()) + `</ds:Modulus>` +
			`<ds:Exponent>` + base64.StdEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()) + `</ds:Exponent></ds:RSAKeyValue></ds:KeyValue></ds:KeyInfo>`
	}
	hora := f.Ahora().In(f.Zona).Format("2006-01-02T15:04:05-07:00")
	certDigest := sha1.Sum(f.cert.Raw) //nolint:gosec // ficha §6.8
	signedProps := func(ns string) string {
		return `<etsi:SignedProperties` + ns + ` Id="` + ids.firma + `-SignedProperties` + ids.props + `"><etsi:SignedSignatureProperties>` +
			`<etsi:SigningTime>` + hora + `</etsi:SigningTime><etsi:SigningCertificate><etsi:Cert><etsi:CertDigest>` +
			`<ds:DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"></ds:DigestMethod>` +
			`<ds:DigestValue>` + base64.StdEncoding.EncodeToString(certDigest[:]) + `</ds:DigestValue></etsi:CertDigest>` +
			`<etsi:IssuerSerial><ds:X509IssuerName>` + texto(f.cert.Issuer.String()) + `</ds:X509IssuerName>` +
			`<ds:X509SerialNumber>` + f.cert.SerialNumber.String() + `</ds:X509SerialNumber></etsi:IssuerSerial>` +
			`</etsi:Cert></etsi:SigningCertificate></etsi:SignedSignatureProperties><etsi:SignedDataObjectProperties>` +
			`<etsi:DataObjectFormat ObjectReference="#Reference-ID-` + ids.ref + `"><etsi:Description>contenido comprobante</etsi:Description>` +
			`<etsi:MimeType>text/xml</etsi:MimeType></etsi:DataObjectFormat></etsi:SignedDataObjectProperties></etsi:SignedProperties>`
	}
	signedInfo := func(ns string) string {
		return `<ds:SignedInfo` + ns + ` Id="Signature-SignedInfo` + ids.info + `">` +
			`<ds:CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"></ds:CanonicalizationMethod>` +
			`<ds:SignatureMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1"></ds:SignatureMethod>` +
			`<ds:Reference Id="SignedPropertiesID` + ids.propsRef + `" Type="http://uri.etsi.org/01903#SignedProperties" URI="#` + ids.firma + `-SignedProperties` + ids.props + `">` +
			`<ds:DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"></ds:DigestMethod><ds:DigestValue>` + b64sha1([]byte(signedProps(nsHeredados))) + `</ds:DigestValue></ds:Reference>` +
			`<ds:Reference URI="#` + ids.cert + `"><ds:DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"></ds:DigestMethod>` +
			`<ds:DigestValue>` + b64sha1([]byte(keyInfo(nsHeredados))) + `</ds:DigestValue></ds:Reference>` +
			`<ds:Reference Id="Reference-ID-` + ids.ref + `" URI="#comprobante"><ds:Transforms>` +
			`<ds:Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"></ds:Transform></ds:Transforms>` +
			`<ds:DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"></ds:DigestMethod><ds:DigestValue>` + digestComprobante + `</ds:DigestValue></ds:Reference>` +
			`</ds:SignedInfo>`
	}
	h := sha1.Sum([]byte(signedInfo(nsHeredados))) //nolint:gosec // RSA-SHA1 (ficha §6.8)
	valor, err := rsa.SignPKCS1v15(rand.Reader, f.llave, crypto.SHA1, h[:])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFirma, err)
	}
	firma := `<ds:Signature` + nsHeredados + ` Id="` + ids.firma + `">` + signedInfo("") +
		`<ds:SignatureValue Id="SignatureValue` + ids.valor + `">` + base64.StdEncoding.EncodeToString(valor) + `</ds:SignatureValue>` +
		keyInfo("") +
		`<ds:Object Id="` + ids.firma + `-Object` + ids.objeto + `"><etsi:QualifyingProperties Target="#` + ids.firma + `">` + signedProps("") +
		`</etsi:QualifyingProperties></ds:Object></ds:Signature>`

	cierre := []byte("</" + raiz + ">")
	i := bytes.LastIndex(comprobante, cierre)
	if i < 0 {
		return nil, fmt.Errorf("%w: no encuentro el cierre de <%s>", ErrFirma, raiz)
	}
	out := make([]byte, 0, len(comprobante)+len(firma))
	out = append(out, comprobante[:i]...)
	out = append(out, firma...)
	return append(out, comprobante[i:]...), nil
}

type idsFirma struct{ firma, info, propsRef, props, cert, ref, valor, objeto string }

func nuevosIDs() (idsFirma, error) {
	n := func() (string, error) {
		v, err := rand.Int(rand.Reader, big.NewInt(900000))
		if err != nil {
			return "", fmt.Errorf("%w: sin fuente aleatoria: %v", ErrFirma, err)
		}
		return fmt.Sprint(v.Int64() + 100000), nil
	}
	var out idsFirma
	for _, p := range []*string{&out.firma, &out.info, &out.propsRef, &out.props, &out.cert, &out.ref, &out.valor, &out.objeto} {
		v, err := n()
		if err != nil {
			return out, err
		}
		*p = v
	}
	out.firma, out.cert = "Signature"+out.firma, "Certificate"+out.cert
	return out, nil
}

func b64sha1(b []byte) string {
	h := sha1.Sum(b) //nolint:gosec // ficha §6.8
	return base64.StdEncoding.EncodeToString(h[:])
}

// canonico da la forma C14N 1.0 inclusiva del elemento raíz (sin declaración XML ni
// comentarios): atributos ordenados, elementos vacíos abiertos y cerrados, y el escape de
// texto y atributos de la especificación. El comprobante no debe declarar namespaces ni traer
// ya una firma; su raíz debe tener id="comprobante".
func canonico(doc []byte) ([]byte, string, error) {
	d := xml.NewDecoder(bytes.NewReader(doc))
	var b bytes.Buffer
	var raiz string
	nivel := 0
	for {
		tok, err := d.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("comprobante ilegible: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space != "" {
				return nil, "", errors.New("el comprobante no debe usar prefijos de namespace")
			}
			if nivel == 0 {
				if raiz != "" {
					return nil, "", errors.New("el comprobante tiene más de un elemento raíz")
				}
				raiz = t.Name.Local
				id := ""
				for _, a := range t.Attr {
					if a.Name.Local == "id" && a.Name.Space == "" {
						id = a.Value
					}
				}
				if id != "comprobante" {
					return nil, "", errors.New(`la raíz debe tener id="comprobante"`)
				}
			}
			attrs := append([]xml.Attr(nil), t.Attr...)
			for _, a := range attrs {
				if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
					return nil, "", errors.New("el comprobante no debe declarar namespaces")
				}
			}
			sort.Slice(attrs, func(i, j int) bool { return attrs[i].Name.Local < attrs[j].Name.Local })
			b.WriteString("<" + t.Name.Local)
			for _, a := range attrs {
				b.WriteString(" " + a.Name.Local + `="` + atributo(a.Value) + `"`)
			}
			b.WriteString(">")
			nivel++
		case xml.EndElement:
			b.WriteString("</" + t.Name.Local + ">")
			nivel--
		case xml.CharData:
			if nivel > 0 { // fuera de la raíz solo hay espacios, que C14N omite
				b.WriteString(texto(string(t)))
			}
		case xml.Comment, xml.ProcInst, xml.Directive:
			// C14N sin comentarios; la declaración XML y las directivas no se incluyen.
		}
	}
	if raiz == "" {
		return nil, "", errors.New("comprobante vacío")
	}
	if bytes.Contains(doc, []byte("Signature")) && bytes.Contains(doc, []byte(nsDS)) {
		return nil, "", errors.New("el comprobante ya está firmado")
	}
	return b.Bytes(), raiz, nil
}

var (
	escTexto    = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", "&#xD;")
	escAtributo = strings.NewReplacer("&", "&amp;", "<", "&lt;", `"`, "&quot;", "\t", "&#x9;", "\n", "&#xA;", "\r", "&#xD;")
)

func texto(s string) string    { return escTexto.Replace(s) }
func atributo(s string) string { return escAtributo.Replace(s) }
