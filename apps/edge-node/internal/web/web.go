// Package web embebe las páginas que sirve el Nodo Local en la LAN (activación, estado, caja).
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

//go:embed all:pos
var pos embed.FS

// Pos es la caja web compilada por `make pos` (apps/pos-web). Sin compilar solo trae .gitkeep.
func Pos() fs.FS {
	sub, err := fs.Sub(pos, "pos")
	if err != nil {
		panic(err)
	}
	return sub
}
