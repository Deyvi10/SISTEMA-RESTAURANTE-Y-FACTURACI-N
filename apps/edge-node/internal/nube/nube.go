// Package nube es el cliente HTTP del Nodo Local hacia la API de la nube. Firma cada
// petición con la llave del nodo (packages/go/nodoauth) y traduce los errores RFC 9457.
package nube

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/nodoauth"
)

// Error es un problema devuelto por la nube.
type Error struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func (e *Error) Error() string { return fmt.Sprintf("nube %d %s: %s", e.Status, e.Code, e.Detail) }

// EsRevocado indica que la nube ya no reconoce al nodo (revocado o reemplazado).
func EsRevocado(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == http.StatusUnauthorized
}

// Client habla con la nube. Sin identidad (antes de activar) solo sirve para /activar.
type Client struct {
	BaseURL string
	NodoID  ids.ID
	Llave   ed25519.PrivateKey
	Now     func() time.Time
	HTTP    *http.Client
}

// httpClient no fija timeout global: cada llamada lo pone con su contexto (el long-poll
// del pull espera 25 s; un heartbeat, 15 s).
func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// Firmado devuelve un *http.Client que agrega el token del nodo a cada petición, con el
// timeout indicado (lo usa el pusher de edgesync, que arma sus propias peticiones).
func (c *Client) Firmado(timeout time.Duration) *http.Client {
	rt := c.httpClient().Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	return &http.Client{Timeout: timeout, Transport: firmador{c: c, next: rt}}
}

type firmador struct {
	c    *Client
	next http.RoundTripper
}

func (f firmador) RoundTrip(r *http.Request) (*http.Response, error) {
	tok, err := nodoauth.Sign(f.c.Llave, f.c.NodoID, f.c.now())
	if err != nil {
		return nil, err
	}
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+tok)
	return f.next.RoundTrip(r)
}

func (c *Client) now() time.Time {
	if c.Now == nil {
		return time.Now().UTC()
	}
	return c.Now()
}

// URL arma una ruta de la API.
func (c *Client) URL(path string) string { return strings.TrimRight(c.BaseURL, "/") + path }

// Do envía in como JSON y decodifica la respuesta en out. firmar=false solo para /activar.
func (c *Client) Do(ctx context.Context, method, path string, in, out any, firmar bool) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.URL(path), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	hc := c.httpClient()
	if firmar {
		hc = c.Firmado(0)
	}
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		e := &Error{Status: res.StatusCode}
		if json.Unmarshal(raw, e) != nil || e.Detail == "" {
			e.Detail = strings.TrimSpace(string(raw[:min(len(raw), 300)]))
		}
		e.Status = res.StatusCode
		return e
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}
