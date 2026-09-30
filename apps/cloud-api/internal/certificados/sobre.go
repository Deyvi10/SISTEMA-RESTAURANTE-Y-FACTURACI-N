// Package certificados guarda la firma electrónica (.p12) del restaurante con cifrado de sobre
// (F5-07, docs/06 §4): cada certificado se cifra con su propia llave de datos (DEK, AES-256-GCM)
// y la DEK se cifra con la llave maestra (KEK). La KEK es asimétrica: la API tiene solo la
// pública (puede cifrar, no descifrar) y el worker fiscal la privada (descifra en memoria para
// firmar y la olvida). En Azure la KEK vive en Key Vault con permisos wrapKey / unwrapKey
// separados (ADR-0013); en local, en archivos PEM fuera del repositorio.
package certificados

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// Envolvedor cifra una DEK con la KEK (lo único que puede hacer la API).
type Envolvedor interface {
	Envolver(dek []byte) ([]byte, error)
	// ID identifica la KEK con que se envolvió (para rotarla sin perder lo anterior).
	ID() string
}

// Desenvolvedor descifra una DEK (solo el worker fiscal).
type Desenvolvedor interface {
	Desenvolver(envuelta []byte) ([]byte, error)
}

// KEKPublica envuelve con RSA-OAEP-SHA256.
type KEKPublica struct {
	pub *rsa.PublicKey
	id  string
}

// KEKPrivada desenvuelve con RSA-OAEP-SHA256.
type KEKPrivada struct {
	priv *rsa.PrivateKey
}

var ErrSobre = errors.New("certificados: no se pudo abrir el sobre")

// CargarKEKPublica lee la llave pública (PEM «PUBLIC KEY»). Exige RSA de 3072 bits o más.
func CargarKEKPublica(pemData []byte) (*KEKPublica, error) {
	b, _ := pem.Decode(pemData)
	if b == nil {
		return nil, errors.New("certificados: la KEK pública no es un PEM")
	}
	k, err := x509.ParsePKIXPublicKey(b.Bytes)
	if err != nil {
		return nil, fmt.Errorf("certificados: KEK pública: %w", err)
	}
	pub, ok := k.(*rsa.PublicKey)
	if !ok || pub.N.BitLen() < 3072 {
		return nil, errors.New("certificados: la KEK debe ser RSA de 3072 bits o más")
	}
	h := sha256.Sum256(b.Bytes)
	return &KEKPublica{pub: pub, id: fmt.Sprintf("rsa-%x", h[:6])}, nil
}

// CargarKEKPrivada lee la llave privada (PEM «PRIVATE KEY», PKCS#8).
func CargarKEKPrivada(pemData []byte) (*KEKPrivada, error) {
	b, _ := pem.Decode(pemData)
	if b == nil {
		return nil, errors.New("certificados: la KEK privada no es un PEM")
	}
	k, err := x509.ParsePKCS8PrivateKey(b.Bytes)
	if err != nil {
		return nil, fmt.Errorf("certificados: KEK privada: %w", err)
	}
	priv, ok := k.(*rsa.PrivateKey)
	if !ok || priv.N.BitLen() < 3072 {
		return nil, errors.New("certificados: la KEK debe ser RSA de 3072 bits o más")
	}
	return &KEKPrivada{priv: priv}, nil
}

func (k *KEKPublica) ID() string { return k.id }

func (k *KEKPublica) Envolver(dek []byte) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, k.pub, dek, []byte("restpos-dek"))
}

func (k *KEKPrivada) Desenvolver(envuelta []byte) ([]byte, error) {
	dek, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, k.priv, envuelta, []byte("restpos-dek"))
	if err != nil {
		return nil, ErrSobre
	}
	return dek, nil
}

// Sobre es lo que se guarda: los datos cifrados con la DEK y la DEK envuelta con la KEK.
type Sobre struct {
	Datos [][]byte // cada uno: nonce (12 bytes) || texto cifrado + etiqueta GCM
	DEK   []byte   // DEK envuelta con la KEK
	KEK   string   // ID de la KEK
}

// Sellar cifra cada parte (el .p12 y su contraseña) con una DEK nueva de 256 bits, que se
// envuelve con la KEK y se borra de la memoria. Cada parte lleva como dato asociado el
// contexto y su posición: no se puede mover a otra fila ni intercambiar con otra parte.
func Sellar(env Envolvedor, contexto string, partes ...[]byte) (Sobre, error) {
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return Sobre{}, err
	}
	defer borrar(dek)
	s := Sobre{KEK: env.ID()}
	for i, p := range partes {
		c, err := cifrar(dek, p, []byte(fmt.Sprintf("%s#%d", contexto, i)))
		if err != nil {
			return Sobre{}, err
		}
		s.Datos = append(s.Datos, c)
	}
	envuelta, err := env.Envolver(dek)
	if err != nil {
		return Sobre{}, err
	}
	s.DEK = envuelta
	return s, nil
}

// Abrir descifra las partes con la KEK privada. Quien llama debe borrar el resultado apenas
// termine de usarlo (Borrar).
func Abrir(des Desenvolvedor, contexto string, dekEnvuelta []byte, datos ...[]byte) ([][]byte, error) {
	dek, err := des.Desenvolver(dekEnvuelta)
	if err != nil {
		return nil, err
	}
	defer borrar(dek)
	out := make([][]byte, len(datos))
	for i, d := range datos {
		p, err := descifrar(dek, d, []byte(fmt.Sprintf("%s#%d", contexto, i)))
		if err != nil {
			return nil, ErrSobre
		}
		out[i] = p
	}
	return out, nil
}

// Borrar sobrescribe con ceros datos secretos ya usados.
func Borrar(bs ...[]byte) {
	for _, b := range bs {
		borrar(b)
	}
}

func borrar(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func cifrar(llave, plano, ad []byte) ([]byte, error) {
	bloque, err := aes.NewCipher(llave)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(bloque)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plano, ad), nil
}

func descifrar(llave, cifrado, ad []byte) ([]byte, error) {
	bloque, err := aes.NewCipher(llave)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(bloque)
	if err != nil {
		return nil, err
	}
	if len(cifrado) < gcm.NonceSize() {
		return nil, ErrSobre
	}
	return gcm.Open(nil, cifrado[:gcm.NonceSize()], cifrado[gcm.NonceSize():], ad)
}

// GenerarKEK crea un par RSA de 3072 bits en PEM (pública PKIX, privada PKCS#8). Solo para
// desarrollo local: en la nube la KEK se crea dentro de Key Vault y no sale de ahí.
func GenerarKEK() (publica, privada []byte, err error) {
	k, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return nil, nil, err
	}
	pub, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	priv, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv}), nil
}
