package app

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/impresion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/edge-node/internal/sistema"
)

// EstadoNodo es la página de estado local (F2-05, RF-02-01.5). No expone datos personales:
// solo números, estados y nombres del local y de las impresoras.
type EstadoNodo struct {
	Version        string             `json:"version"`
	Arranque       time.Time          `json:"arranque"`
	Hora           time.Time          `json:"hora"`
	Activado       bool               `json:"activado"`
	Revocado       bool               `json:"revocado"`
	Restaurante    string             `json:"restaurante"`
	Local          string             `json:"local"`
	Conectividad   Conectividad       `json:"conectividad"`
	DerivaSeg      int64              `json:"derivaSegundos"`
	AlertaReloj    bool               `json:"alertaReloj"`
	OutboxPend     int                `json:"outboxPendientes"`
	OutboxEdadSeg  int64              `json:"outboxAntiguedadSeg"`
	DiscoLibreMB   int64              `json:"discoLibreMb"`
	BaseMB         int64              `json:"baseMb"`
	Impresoras     []impresion.Estado `json:"impresoras"`
	UltimaBusqueda *time.Time         `json:"ultimaBusqueda"`
	Buscando       bool               `json:"buscando"`
	Encontradas    int                `json:"encontradas"`
	// Los comprobantes electrónicos llegan en la Fase 5; hasta entonces siempre 0.
	ComprobantesPendientes int   `json:"comprobantesPendientes"`
	FotosArchivos          int   `json:"fotosArchivos"`
	FotosKB                int64 `json:"fotosKb"`
}

func (a *App) estadoNodo(ctx context.Context) EstadoNodo {
	s := a.salud.Copia()
	e := EstadoNodo{Version: Version, Arranque: a.Inicio.UTC(), Hora: a.Clock.Now(), Conectividad: a.conectividad(ctx),
		DerivaSeg: s.DerivaSeg, AlertaReloj: s.AlertaReloj, DiscoLibreMB: sistema.DiscoLibreMB(a.Cfg.DataDir),
		Impresoras: a.motor.Estados(ctx)}
	if id, err := a.Identidad(ctx); err == nil && id != nil {
		e.Activado, e.Revocado = id.Activo(), !id.Activo()
		e.Restaurante, e.Local = id.NombreComercial, id.NombreLocal
	}
	if st, err := a.outbox.Stats(ctx); err == nil {
		e.OutboxPend = st.Pending
		if !st.OldestUnsent.IsZero() {
			e.OutboxEdadSeg = int64(a.Clock.Now().Sub(st.OldestUnsent) / time.Second)
		}
	}
	for _, f := range []string{a.Cfg.DBPath(), a.Cfg.DBPath() + "-wal"} {
		if fi, err := os.Stat(f); err == nil {
			e.BaseMB += fi.Size() >> 20
		}
	}
	archivos, bytes := a.fotos.Tamano()
	e.FotosArchivos, e.FotosKB = archivos, bytes>>10
	ultima, encontradas, enCurso := a.busqueda.copia()
	if !ultima.IsZero() {
		e.UltimaBusqueda = &ultima
	}
	e.Buscando, e.Encontradas = enCurso, len(encontradas)
	return e
}

func (a *App) handleEstado(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, a.estadoNodo(r.Context()))
}

// handleBuscarAhora dispara una búsqueda de impresoras desde la página de estado (solo en esta PC).
func (a *App) handleBuscarAhora(w http.ResponseWriter, r *http.Request) {
	a.syncMu.Lock()
	activo := a.syncCancel != nil
	a.syncMu.Unlock()
	if !activo {
		writeProblem(w, http.StatusConflict, "SIN_ACTIVAR", "Activa el nodo para buscar impresoras.")
		return
	}
	a.pedirBusqueda()
	w.WriteHeader(http.StatusAccepted)
}
