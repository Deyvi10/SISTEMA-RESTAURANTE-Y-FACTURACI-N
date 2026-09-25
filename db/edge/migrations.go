// Package edge embebe las migraciones SQLite del Nodo Local: el binario las aplica al
// arrancar, sin archivos sueltos en la PC del restaurante.
package edge

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
