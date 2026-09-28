//go:build !windows

// Package sistema consulta datos del equipo para la telemetría del nodo.
package sistema

import "golang.org/x/sys/unix"

// DiscoLibreMB devuelve el espacio libre del disco donde está path.
func DiscoLibreMB(path string) int64 {
	var st unix.Statfs_t
	if unix.Statfs(path, &st) != nil {
		return -1
	}
	return int64(st.Bavail) * int64(st.Bsize) >> 20 //nolint:gosec,unconvert // tamaños de disco
}
