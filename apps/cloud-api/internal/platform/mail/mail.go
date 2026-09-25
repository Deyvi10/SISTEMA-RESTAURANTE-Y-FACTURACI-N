// Package mail envía correos transaccionales. En local van a Mailpit (http://localhost:8025).
package mail

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"mime"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Sender interface {
	Send(ctx context.Context, m Message) error
}

// SMTP envía por un servidor SMTP (Mailpit en local; SES/SMTP con STARTTLS en la nube).
type SMTP struct {
	Addr, From string
	Auth       smtp.Auth
}

func (s SMTP) Send(_ context.Context, m Message) error {
	if strings.ContainsAny(m.To, "\r\n") || strings.ContainsAny(m.Subject, "\r\n") {
		return fmt.Errorf("mail: cabecera con salto de línea")
	}
	boundary := fmt.Sprintf("restpos-%d", time.Now().UnixNano())
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n", s.From, m.To, mime.QEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", boundary, m.Text)
	if m.HTML != "" {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n", boundary, m.HTML)
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	from := s.From
	if i := strings.LastIndex(from, "<"); i >= 0 {
		from = strings.Trim(from[i:], "<>")
	}
	return smtp.SendMail(s.Addr, s.Auth, from, []string{m.To}, b.Bytes())
}

// Memory guarda los correos para las pruebas.
type Memory struct {
	mu   sync.Mutex
	Sent []Message
}

func (m *Memory) Send(_ context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Sent = append(m.Sent, msg)
	return nil
}

func (m *Memory) Last() (Message, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Sent) == 0 {
		return Message{}, false
	}
	return m.Sent[len(m.Sent)-1], true
}

// Plantilla HTML simple y legible en cualquier cliente de correo, con la identidad del producto.
var layout = template.Must(template.New("mail").Parse(`<!doctype html><html lang="es"><body style="margin:0;background:#F2F2F7;font-family:-apple-system,Segoe UI,Roboto,Arial,sans-serif;color:#000">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr><td align="center" style="padding:32px 16px">
<table role="presentation" width="100%" style="max-width:520px;background:#fff;border-radius:22px;padding:32px" cellpadding="0" cellspacing="0"><tr><td>
<div style="width:56px;height:56px;border-radius:13px;background:linear-gradient(160deg,#FF9F0A,#FF375F);color:#fff;font-size:30px;line-height:56px;text-align:center">🍽</div>
<h1 style="font-size:24px;margin:20px 0 8px">{{.Titulo}}</h1>
{{range .Parrafos}}<p style="font-size:16px;line-height:1.5;color:#1c1c1e;margin:0 0 12px">{{.}}</p>{{end}}
{{if .Destacado}}<p style="font-size:22px;font-weight:700;letter-spacing:1px;background:#F2F2F7;border-radius:12px;padding:14px;text-align:center;font-family:Menlo,monospace">{{.Destacado}}</p>{{end}}
{{if .Boton}}<p style="margin:24px 0"><a href="{{.BotonURL}}" style="background:#0066CC;color:#fff;text-decoration:none;font-weight:600;padding:14px 24px;border-radius:14px;display:inline-block">{{.Boton}}</a></p>{{end}}
<p style="font-size:13px;color:#636366;margin-top:24px">{{.Pie}}</p>
</td></tr></table></td></tr></table></body></html>`))

// Contenido de un correo con el diseño del producto.
type Contenido struct {
	Titulo    string
	Parrafos  []string
	Destacado string
	Boton     string
	BotonURL  string
	Pie       string
}

// Render arma texto plano y HTML a partir del mismo contenido.
func Render(c Contenido) (text, html string, err error) {
	var t strings.Builder
	t.WriteString(c.Titulo + "\n\n")
	for _, p := range c.Parrafos {
		t.WriteString(p + "\n\n")
	}
	if c.Destacado != "" {
		t.WriteString(c.Destacado + "\n\n")
	}
	if c.Boton != "" {
		t.WriteString(c.Boton + ": " + c.BotonURL + "\n\n")
	}
	t.WriteString(c.Pie)
	var h bytes.Buffer
	err = layout.Execute(&h, c)
	return t.String(), h.String(), err
}
