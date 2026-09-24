// Package testdata expone los vectores compartidos a las pruebas de Go.
package testdata

import "embed"

//go:embed *.json
var FS embed.FS
