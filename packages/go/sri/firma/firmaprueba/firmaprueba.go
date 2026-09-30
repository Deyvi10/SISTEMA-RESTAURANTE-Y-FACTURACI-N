// Package firmaprueba genera certificados .p12 ficticios y autofirmados para pruebas y para la
// demo local. El SRI no los acepta: sirven para recorrer la cadena contra el stub (tools/sri-stub)
// mientras no haya un .p12 de pruebas de una entidad certificadora (DP-04).
package firmaprueba

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// oidRUC imita a las entidades certificadoras que ponen el RUC en una extensión propia.
// 🔎 El arco exacto de cada entidad no está verificado (falta un .p12 real, DP-04); por eso la
// nube busca el RUC en todo el certificado y no en un OID fijo.
var oidRUC = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 37746, 3, 11}

// Opciones del certificado ficticio.
type Opciones struct {
	Titular string
	RUC     string
	Desde   time.Time
	Hasta   time.Time
	Bits    int // 2048 si es 0
}

// P12 devuelve un .p12 (PKCS#12 moderno) protegido con la clave dada.
func P12(o Opciones, clave string) ([]byte, error) {
	if o.Bits == 0 {
		o.Bits = 2048
	}
	llave, err := rsa.GenerateKey(rand.Reader, o.Bits)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return nil, err
	}
	ruc, _ := asn1.Marshal(o.RUC)
	plantilla := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: o.Titular, SerialNumber: o.RUC, Country: []string{"EC"}, Organization: []string{"CERTIFICADO DE PRUEBA - SIN VALIDEZ"}},
		NotBefore:    o.Desde, NotAfter: o.Hasta,
		KeyUsage:        x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		ExtraExtensions: []pkix.Extension{{Id: oidRUC, Value: ruc}},
	}
	der, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &llave.PublicKey, llave)
	if err != nil {
		return nil, err
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return pkcs12.Modern.Encode(llave, c, nil, clave)
}
