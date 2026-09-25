package server

import (
	"context"
	"net/http"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/auth"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/httpx"
)

// Paso de la guía «Configura tu restaurante» que ve el dueño al entrar.
type Paso struct {
	ID          string `json:"id"`
	Titulo      string `json:"titulo"`
	Descripcion string `json:"descripcion"`
	Icono       string `json:"icono"`
	Hecho       bool   `json:"hecho"`
	Disponible  bool   `json:"disponible"` // false = llega en una fase posterior
	Ruta        string `json:"ruta"`
}

type Resumen struct {
	NombreComercial string         `json:"nombreComercial"`
	Pasos           []Paso         `json:"pasos"`
	Progreso        int            `json:"progreso"` // 0..100 sobre los pasos disponibles
	Cuentas         map[string]int `json:"cuentas"`
}

// GET /v1/resumen
func resumen(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := auth.MustPrincipal(r.Context())
		res, err := calcularResumen(r.Context(), d, p)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, res)
	}
}

func calcularResumen(ctx context.Context, d *db.DB, p auth.Principal) (Resumen, error) {
	var res Resumen
	var productos, conFoto, categorias, mesas, personal int
	err := d.InTenant(ctx, p.TenantID, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT
			(SELECT nombre_comercial FROM tenants),
			(SELECT count(*) FROM productos WHERE deleted_at IS NULL)::int,
			(SELECT count(*) FROM productos WHERE deleted_at IS NULL AND imagen_key IS NOT NULL)::int,
			(SELECT count(*) FROM categorias WHERE deleted_at IS NULL)::int,
			(SELECT count(*) FROM mesas WHERE deleted_at IS NULL)::int,
			(SELECT count(*) FROM usuarios WHERE activo AND NOT es_dueno)::int`).
			Scan(&res.NombreComercial, &productos, &conFoto, &categorias, &mesas, &personal)
	})
	if err != nil {
		return res, err
	}
	res.Cuentas = map[string]int{"productos": productos, "conFoto": conFoto, "categorias": categorias, "mesas": mesas, "personal": personal}
	res.Pasos = []Paso{
		{"menu", "Arma tu menú", "Agrega al menos 3 platos con su precio.", "utensils", productos >= 3, true, "/menu"},
		{"fotos", "Ponle fotos a tus platos", "Los platos con foto se venden más. Usa las tuyas o elige de la galería.", "camera", productos > 0 && conFoto >= min(3, productos), true, "/menu"},
		{"salon", "Dibuja tu salón", "Crea tus zonas y mesas en segundos.", "armchair", mesas >= 2, true, "/salon"},
		{"personal", "Suma a tu equipo", "Crea a tus meseros con nombre y PIN.", "users", personal >= 1, true, "/personal"},
		{"impresoras", "Conecta tus impresoras", "Cocina y bar imprimen solos. Llega con el Nodo Local.", "printer", false, false, "/impresoras"},
		{"sri", "Activa la facturación SRI", "Sube tu firma electrónica y factura sin esperas.", "receipt", false, false, "/facturacion"},
	}
	total, hechos := 0, 0
	for _, s := range res.Pasos {
		if s.Disponible {
			total++
			if s.Hecho {
				hechos++
			}
		}
	}
	res.Progreso = hechos * 100 / total
	return res, nil
}
