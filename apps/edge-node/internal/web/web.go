// Package web embebe las páginas que sirve el Nodo Local en la LAN (activación, estado).
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var static embed.FS

// Static son los archivos de static/ (tokens.css lo genera `make tokens`).
func Static() fs.FS {
	sub, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
