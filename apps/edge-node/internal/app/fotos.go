package app

import (
	"context"
	"path/filepath"
	"time"
)

// FotosCada: reintento periódico de las fotos que no se pudieron bajar.
const FotosCada = time.Hour

func (a *App) pedirFotos() {
	select {
	case a.fotosYa <- struct{}{}:
	default:
	}
}

// fotosLoop mantiene la caché de fotos del menú al día (F2-15), en segundo plano.
func (a *App) fotosLoop(ctx context.Context) {
	t := time.NewTicker(FotosCada)
	defer t.Stop()
	for {
		a.SincronizarFotos(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-a.fotosYa:
		}
	}
}

// SincronizarFotos baja las fotos de los productos activos y borra las que sobran.
func (a *App) SincronizarFotos(ctx context.Context) {
	rows, err := a.Store.Read().QueryContext(ctx, `SELECT DISTINCT imagen_key FROM productos WHERE deleted_at IS NULL AND activo = 1 AND imagen_key IS NOT NULL`)
	if err != nil {
		a.Log.Error("fotos: réplica ilegible", "err", err)
		return
	}
	var claves []string
	for rows.Next() {
		var k string
		if rows.Scan(&k) == nil {
			claves = append(claves, k)
		}
	}
	_ = rows.Close()
	n, err := a.fotos.Sincronizar(ctx, claves)
	if err != nil && ctx.Err() == nil {
		a.Log.Warn("fotos del menú incompletas", "err", err)
	}
	if n > 0 {
		archivos, bytes := a.fotos.Tamano()
		a.Log.Info("fotos del menú al día", "nuevas", n, "archivos", archivos, "mb", bytes>>20)
	}
}

func dirFotos(dataDir string) string { return filepath.Join(dataDir, "media") }
