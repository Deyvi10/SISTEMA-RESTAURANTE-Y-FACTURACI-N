package auth_test

import (
	"context"
	"crypto/ed25519"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/mail"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/testdb"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/tenants"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
)

type fixture struct {
	svc   *auth.Service
	mail  *mail.Memory
	clock *clock.Fake
	alta  tenants.Resultado
	email string
}

func setup(t *testing.T) fixture {
	t.Helper()
	tdb := testdb.New(t)
	m := &mail.Memory{}
	clk := clock.NewFake(time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC))
	_, key, _ := ed25519.GenerateKey(nil)
	svc := &auth.Service{DB: tdb.App, Signer: auth.NewSigner(key, clk.Now), Mail: m, Clock: clk, BackofficeURL: "http://bo"}
	ten := &tenants.Service{DB: tdb.App, Mail: m, BackofficeURL: "http://bo"}
	r, err := ten.Crear(context.Background(), tenants.Alta{
		RUC: "1790011674001", RazonSocial: "Don Pepe S.A.", NombreComercial: "Cevichería Don Pepe",
		NombreDueno: "Pepe Andrade", EmailDueno: "Pepe@DonPepe.ec", Plan: "RESTAURANTE",
	})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{svc: svc, mail: m, clock: clk, alta: r, email: "pepe@donpepe.ec"}
}

func code(err error) string {
	if e, ok := apperr.As(err); ok {
		return e.Code
	}
	return ""
}

func TestAltaEnviaBienvenidaYExigeCambio(t *testing.T) {
	f := setup(t)
	msg, ok := f.mail.Last()
	if !ok || msg.To != f.email || !strings.Contains(msg.Text, f.alta.PasswordTemporal) {
		t.Fatalf("correo de bienvenida: %+v", msg)
	}
	ctx := context.Background()
	s, err := f.svc.Login(ctx, "  PEPE@donpepe.ec ", f.alta.PasswordTemporal, auth.Cliente{IP: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if !s.Usuario.DebeCambiarPassword || s.Usuario.NombreComercial != "Cevichería Don Pepe" || s.RefreshToken == "" {
		t.Fatalf("sesión: %+v", s.Usuario)
	}
	// También entra con el RUC.
	if _, err := f.svc.Login(ctx, "1790011674001", f.alta.PasswordTemporal, auth.Cliente{IP: "10.0.0.1"}); err != nil {
		t.Fatalf("login con RUC: %v", err)
	}
	p := auth.Principal{UserID: s.Usuario.ID, TenantID: s.Usuario.TenantID}
	nueva, err := f.svc.CambiarPassword(ctx, p, f.alta.PasswordTemporal, "MesaCuatro44", auth.Cliente{})
	if err != nil || nueva.Usuario.DebeCambiarPassword {
		t.Fatalf("cambiar: %v %+v", err, nueva.Usuario)
	}
	// La sesión vieja quedó revocada: su refresh ya no sirve.
	if _, err := f.svc.Refresh(ctx, s.RefreshToken, auth.Cliente{}); code(err) != "SESION_VENCIDA" {
		t.Fatalf("refresh viejo: %v", err)
	}
	if _, err := f.svc.Login(ctx, f.email, "MesaCuatro44", auth.Cliente{}); err != nil {
		t.Fatal(err)
	}
}

func TestRUCDuplicadoYDatosInvalidos(t *testing.T) {
	f := setup(t)
	ten := &tenants.Service{DB: f.svc.DB, Mail: f.mail}
	_, err := ten.Crear(context.Background(), tenants.Alta{RUC: "1790011674001", RazonSocial: "X", NombreComercial: "X", NombreDueno: "X", EmailDueno: "otro@x.ec", Plan: "PRO"})
	if code(err) != "RUC_REGISTRADO" {
		t.Fatalf("RUC duplicado: %v", err)
	}
	_, err = ten.Crear(context.Background(), tenants.Alta{RUC: "1790011674002", Plan: "GRATIS"})
	e, _ := apperr.As(err)
	if e == nil || len(e.Fields) < 5 {
		t.Fatalf("se esperaban errores por campo: %+v", err)
	}
}

func TestBloqueoTrasCincoFallos(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	cli := auth.Cliente{IP: "10.0.0.9"}
	for i := range 5 {
		if _, err := f.svc.Login(ctx, f.email, "incorrecta1", cli); code(err) != "CREDENCIALES_INVALIDAS" {
			t.Fatalf("intento %d: %v", i+1, err)
		}
	}
	if _, err := f.svc.Login(ctx, f.email, f.alta.PasswordTemporal, cli); code(err) != "CUENTA_BLOQUEADA" {
		t.Fatalf("sexto intento con la clave correcta debería estar bloqueado: %v", err)
	}
	// Otra IP no queda bloqueada (el bloqueo es por usuario + IP, RF-01-02.2).
	if _, err := f.svc.Login(ctx, f.email, f.alta.PasswordTemporal, auth.Cliente{IP: "10.0.0.10"}); err != nil {
		t.Fatalf("otra IP: %v", err)
	}
	f.clock.Advance(16 * time.Minute)
	if _, err := f.svc.Login(ctx, f.email, f.alta.PasswordTemporal, cli); err != nil {
		t.Fatalf("pasados 15 min debería poder entrar: %v", err)
	}
}

func TestUsuarioInexistenteMismoMensaje(t *testing.T) {
	f := setup(t)
	_, err1 := f.svc.Login(context.Background(), "nadie@x.ec", "loquesea123", auth.Cliente{})
	_, err2 := f.svc.Login(context.Background(), f.email, "loquesea123", auth.Cliente{})
	if code(err1) != "CREDENCIALES_INVALIDAS" || code(err1) != code(err2) || err1.Error() != err2.Error() {
		t.Fatalf("los mensajes deben ser idénticos: %v / %v", err1, err2)
	}
}

func TestRefreshRotaYDetectaReutilizacion(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s1, _ := f.svc.Login(ctx, f.email, f.alta.PasswordTemporal, auth.Cliente{})
	s2, err := f.svc.Refresh(ctx, s1.RefreshToken, auth.Cliente{})
	if err != nil || s2.RefreshToken == s1.RefreshToken {
		t.Fatalf("rotación: %v", err)
	}
	// Alguien reutiliza el token viejo (robado): se revoca toda la familia.
	if _, err := f.svc.Refresh(ctx, s1.RefreshToken, auth.Cliente{}); code(err) != "SESION_VENCIDA" {
		t.Fatalf("reutilización: %v", err)
	}
	if _, err := f.svc.Refresh(ctx, s2.RefreshToken, auth.Cliente{}); code(err) != "SESION_VENCIDA" {
		t.Fatal("tras detectar reutilización, el token nuevo también debe quedar revocado")
	}
	s3, _ := f.svc.Login(ctx, f.email, f.alta.PasswordTemporal, auth.Cliente{})
	f.clock.Advance(auth.RefreshTTL + time.Minute)
	if _, err := f.svc.Refresh(ctx, s3.RefreshToken, auth.Cliente{}); code(err) != "SESION_VENCIDA" {
		t.Fatal("un refresh vencido debe rechazarse")
	}
}

var tokenRe = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

func TestRecuperacion(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if err := f.svc.SolicitarRecuperacion(ctx, "nadie@x.ec"); err != nil {
		t.Fatal(err)
	}
	antes := len(f.mail.Sent)
	if err := f.svc.SolicitarRecuperacion(ctx, "PEPE@donpepe.ec"); err != nil {
		t.Fatal(err)
	}
	if len(f.mail.Sent) != antes+1 {
		t.Fatal("no se envió el correo de recuperación")
	}
	msg, _ := f.mail.Last()
	m := tokenRe.FindStringSubmatch(msg.Text)
	if m == nil {
		t.Fatalf("correo sin enlace: %s", msg.Text)
	}
	if err := f.svc.Restablecer(ctx, m[1], "corta"); code(err) != "DATOS_INVALIDOS" {
		t.Fatalf("contraseña débil: %v", err)
	}
	if err := f.svc.Restablecer(ctx, m[1], "PlatoDelDia77"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Restablecer(ctx, m[1], "OtraClave2026"); code(err) != "ENLACE_INVALIDO" {
		t.Fatalf("el enlace es de un solo uso: %v", err)
	}
	if _, err := f.svc.Login(ctx, f.email, "PlatoDelDia77", auth.Cliente{}); err != nil {
		t.Fatal(err)
	}
	// Vence a los 30 minutos.
	_ = f.svc.SolicitarRecuperacion(ctx, f.email)
	msg, _ = f.mail.Last()
	f.clock.Advance(31 * time.Minute)
	if err := f.svc.Restablecer(ctx, tokenRe.FindStringSubmatch(msg.Text)[1], "PlatoDelDia88"); code(err) != "ENLACE_INVALIDO" {
		t.Fatalf("enlace vencido: %v", err)
	}
	_ = errors.New
}
