package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// Config del Nodo Local. Casi todo tiene un valor por defecto sensato: el dueño del
// restaurante no configura nada a mano; la identidad llega con la activación (F2-02).
type Config struct {
	DataDir  string // base, logs y caché de fotos
	HTTPAddr string // API LAN + WebSocket + caja web
	NubeURL  string // API de la nube (se puede cambiar solo antes de activar)
}

// Version la fija el build con -ldflags "-X …/app.Version=1.2.3".
var Version = "dev"

// DefaultDataDir: %ProgramData%\RestPOS\Nodo en Windows; /var/lib/restpos-nodo en Linux.
func DefaultDataDir() string {
	if runtime.GOOS == "windows" {
		if pd := os.Getenv("ProgramData"); pd != "" {
			return filepath.Join(pd, "RestPOS", "Nodo")
		}
	}
	return "/var/lib/restpos-nodo"
}

// LoadConfig lee variables de entorno RESTPOS_* con valores por defecto.
func LoadConfig() Config {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	return Config{
		DataDir:  get("RESTPOS_DATA", DefaultDataDir()),
		HTTPAddr: get("RESTPOS_HTTP", ":7080"),
		NubeURL:  get("RESTPOS_NUBE", "http://localhost:8080"),
	}
}

func (c Config) Validate() error {
	if c.DataDir == "" || c.HTTPAddr == "" || c.NubeURL == "" {
		return errors.New("config: RESTPOS_DATA, RESTPOS_HTTP y RESTPOS_NUBE no pueden estar vacíos")
	}
	return nil
}

func (c Config) DBPath() string { return filepath.Join(c.DataDir, "nodo.db") }
