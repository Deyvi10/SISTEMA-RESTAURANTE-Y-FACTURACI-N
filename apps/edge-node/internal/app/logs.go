package app

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// maxLog: al arrancar, si el log supera este tamaño se rota (se conservan 3 anteriores).
const maxLog = 20 << 20

// OpenLog abre <data>/logs/nodo.log en JSON (y también en la consola si interactive).
// Nunca registra datos personales: solo UUID y códigos (docs/03 §10).
func OpenLog(dataDir string, interactive bool) (*slog.Logger, io.Closer, error) {
	dir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, nil, err
	}
	path := filepath.Join(dir, "nodo.log")
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxLog {
		for i := 2; i >= 1; i-- {
			_ = os.Rename(fmt.Sprintf("%s.%d", path, i), fmt.Sprintf("%s.%d", path, i+1))
		}
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640) //nolint:gosec // ruta fija dentro del directorio de datos
	if err != nil {
		return nil, nil, err
	}
	var w io.Writer = f
	if interactive {
		w = io.MultiWriter(f, os.Stdout)
	}
	return slog.New(slog.NewJSONHandler(w, nil)), f, nil
}
