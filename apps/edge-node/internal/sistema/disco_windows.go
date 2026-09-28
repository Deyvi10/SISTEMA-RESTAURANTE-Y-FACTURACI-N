//go:build windows

// Package sistema consulta datos del equipo para la telemetría del nodo.
package sistema

import "golang.org/x/sys/windows"

// DiscoLibreMB devuelve el espacio libre del disco donde está path.
func DiscoLibreMB(path string) int64 {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return -1
	}
	var libre, total, totalLibre uint64
	if windows.GetDiskFreeSpaceEx(p, &libre, &total, &totalLibre) != nil {
		return -1
	}
	return int64(libre >> 20) //nolint:gosec // cabe de sobra
}
