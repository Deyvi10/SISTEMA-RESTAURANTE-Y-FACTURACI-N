// Package config lee la configuración de variables de entorno (12-factor).
// En APP_ENV=local hay valores por defecto que apuntan a `make dev`; en cualquier otro
// entorno los secretos son obligatorios y la app no arranca sin ellos.
package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env           string // local, dev, staging, prod
	HTTPAddr      string
	DatabaseURL   string // rol restpos_app: sujeto a RLS
	AdminDBURL    string // rol dueño: solo migraciones y comandos de plataforma
	PublicURL     string // URL pública de la API
	BackofficeURL string // origen permitido por CORS y base de los enlaces en correos
	CookieSecure  bool

	JWTKey    ed25519.PrivateKey
	PINPepper []byte

	SMTPAddr string
	MailFrom string

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool
	S3Bucket    string // imágenes
}

// devKey es una llave fija SOLO para desarrollo local: jamás se acepta fuera de APP_ENV=local.
var devKey = ed25519.NewKeyFromSeed([]byte("restpos-dev-seed-no-usar-en-prod"))

func Load() (Config, error) {
	env := get("APP_ENV", "local")
	local := env == "local"
	def := func(v string) string {
		if local {
			return v
		}
		return ""
	}
	c := Config{
		Env:           env,
		HTTPAddr:      get("HTTP_ADDR", ":8080"),
		DatabaseURL:   get("DATABASE_URL", def("postgres://restpos_app:restpos_app_dev@localhost:5442/restpos")),
		AdminDBURL:    get("DATABASE_ADMIN_URL", def("postgres://restpos_owner:restpos_dev@localhost:5442/restpos")),
		PublicURL:     get("PUBLIC_URL", def("http://localhost:8080")),
		BackofficeURL: get("BACKOFFICE_URL", def("http://localhost:5173")),
		CookieSecure:  get("COOKIE_SECURE", map[bool]string{true: "false", false: "true"}[local]) == "true",
		SMTPAddr:      get("SMTP_ADDR", def("localhost:1025")),
		MailFrom:      get("MAIL_FROM", "RestPOS <no-responder@restpos.local>"),
		S3Endpoint:    get("S3_ENDPOINT", def("localhost:7171")),
		S3AccessKey:   get("S3_ACCESS_KEY", def("restpos")),
		S3SecretKey:   get("S3_SECRET_KEY", def("restpos_s3_dev")),
		S3UseSSL:      get("S3_USE_SSL", map[bool]string{true: "false", false: "true"}[local]) == "true",
		S3Bucket:      get("S3_BUCKET_IMAGENES", "imagenes"),
	}

	var missing []string
	for name, v := range map[string]string{
		"DATABASE_URL": c.DatabaseURL, "PUBLIC_URL": c.PublicURL, "BACKOFFICE_URL": c.BackofficeURL,
		"SMTP_ADDR": c.SMTPAddr, "S3_ENDPOINT": c.S3Endpoint, "S3_ACCESS_KEY": c.S3AccessKey, "S3_SECRET_KEY": c.S3SecretKey,
	} {
		if v == "" {
			missing = append(missing, name)
		}
	}

	switch seed := os.Getenv("JWT_SIGNING_SEED"); {
	case seed != "":
		b, err := base64.StdEncoding.DecodeString(seed)
		if err != nil || len(b) != ed25519.SeedSize {
			return c, errors.New("config: JWT_SIGNING_SEED debe ser 32 bytes en base64")
		}
		c.JWTKey = ed25519.NewKeyFromSeed(b)
	case local:
		c.JWTKey = devKey
	default:
		missing = append(missing, "JWT_SIGNING_SEED")
	}

	switch p := os.Getenv("PIN_PEPPER"); {
	case len(p) >= 32:
		c.PINPepper = []byte(p)
	case p == "" && local:
		c.PINPepper = []byte("restpos-dev-pepper-no-usar-en-produccion")
	default:
		missing = append(missing, "PIN_PEPPER (mínimo 32 caracteres)")
	}

	if len(missing) > 0 {
		return c, fmt.Errorf("config: faltan variables de entorno: %s", strings.Join(missing, ", "))
	}
	return c, nil
}

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
