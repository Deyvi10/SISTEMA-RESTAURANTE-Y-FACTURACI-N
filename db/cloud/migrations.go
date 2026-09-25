// Package cloud embebe las migraciones de PostgreSQL para que el binario las aplique
// sin depender de archivos sueltos en el servidor.
package cloud

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
