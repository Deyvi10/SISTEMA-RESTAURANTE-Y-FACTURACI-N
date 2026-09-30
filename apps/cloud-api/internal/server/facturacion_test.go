package server_test

import (
	"context"
	"testing"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/facturacion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// F5-02 / F5-06 (pasos 3 a 5): datos del emisor y puntos de emisión por caja.
func TestConfiguracionFiscalYPuntosDeEmision(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "f@f.ec")

	// La primera vez propone los datos del alta: el dueño solo confirma.
	var cfg facturacion.Config
	c.do("GET", "/v1/facturacion", nil, 200, &cfg)
	if cfg.Guardada || cfg.RUC != "1790011674001" || cfg.RazonSocial != "R 1790011674001" || cfg.Ambiente != 1 || cfg.Regimen != "GENERAL" || len(cfg.Pendientes) != 2 {
		t.Fatalf("propuesta: %+v", cfg)
	}
	datos := map[string]any{"ambiente": 1, "razonSocial": "  Distribuidora\ndel Pacífico S.A. ", "direccionMatriz": "Av. Amazonas N34-120",
		"obligadoContabilidad": true, "regimen": "GENERAL", "facturacionActiva": true}

	// No se factura con una caja sin punto de emisión.
	c.do("PUT", "/v1/facturacion", datos, 409, nil)
	datos["facturacionActiva"] = false
	c.do("PUT", "/v1/facturacion", datos, 201, &cfg)
	if !cfg.Guardada || cfg.RazonSocial != "Distribuidora del Pacífico S.A." || !cfg.ObligadoContabilidad {
		t.Fatalf("guardada: %+v", cfg)
	}

	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	caja1 := cajas[0].ID
	c.do("PUT", "/v1/cajas/"+caja1.String()+"/punto-emision", map[string]any{"establecimiento": "1", "puntoEmision": "001"}, 422, nil)
	c.do("PUT", "/v1/cajas/"+caja1.String()+"/punto-emision", map[string]any{"establecimiento": "000", "puntoEmision": "001"}, 422, nil)
	c.do("PUT", "/v1/cajas/"+caja1.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 200, &cfg)
	if cfg.Cajas[0].PuntoID == nil {
		t.Fatalf("sin punto: %+v", cfg)
	}
	primero := *cfg.Cajas[0].PuntoID // copia: el JSON siguiente se decodifica sobre cfg
	if *cfg.Cajas[0].Establecimiento != "001" || *cfg.Cajas[0].PuntoEmision != "001" || len(cfg.Pendientes) != 0 {
		t.Fatalf("punto: %+v", cfg)
	}

	// Una serie no se comparte entre cajas.
	var barra caja.Caja
	c.do("POST", "/v1/cajas", map[string]any{"localId": r.LocalID, "nombre": "Barra"}, 201, &barra)
	c.do("PUT", "/v1/cajas/"+barra.ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 409, nil)
	c.do("PUT", "/v1/cajas/"+barra.ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "002"}, 200, nil)

	// Cambiar la serie crea un punto nuevo (una serie nunca se renumera).
	c.do("PUT", "/v1/cajas/"+caja1.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "003"}, 200, &cfg)
	var nuevo *ids.ID
	for _, p := range cfg.Cajas {
		if p.CajaID == caja1 {
			nuevo = p.PuntoID
		}
	}
	if nuevo == nil || *nuevo == primero {
		t.Fatalf("la serie nueva debía ser otro punto: %v → %v", primero, nuevo)
	}
	var borrado bool
	if err := e.tdb.Admin.QueryRow(context.Background(), `SELECT deleted_at IS NOT NULL FROM puntos_emision WHERE id = $1`, primero).Scan(&borrado); err != nil || !borrado {
		t.Fatalf("el punto anterior queda archivado: %v %v", borrado, err)
	}

	// Ya se puede activar; producción espera la firma y el negocio popular emite notas de venta.
	datos["facturacionActiva"] = true
	c.do("PUT", "/v1/facturacion", datos, 201, &cfg)
	if !cfg.FacturacionActiva {
		t.Fatalf("activa: %+v", cfg)
	}
	datos["ambiente"] = 2
	c.do("PUT", "/v1/facturacion", datos, 409, nil)
	datos["ambiente"], datos["regimen"] = 1, "RIMPE_NEGOCIO_POPULAR"
	c.do("PUT", "/v1/facturacion", datos, 409, nil)

	// Todo llega al nodo por la réplica y queda en la auditoría de la nube.
	var cambios, auditados int
	_ = e.tdb.Admin.QueryRow(context.Background(), `SELECT count(*) FROM sync_cambios WHERE tenant_id = $1 AND tabla IN ('configuracion_fiscal', 'puntos_emision')`, r.TenantID).Scan(&cambios)
	_ = e.tdb.Admin.QueryRow(context.Background(), `SELECT count(*) FROM auditoria WHERE tenant_id = $1 AND accion IN ('FACTURACION_CONFIGURADA', 'PUNTO_EMISION_ASIGNADO')`, r.TenantID).Scan(&auditados)
	if cambios < 5 || auditados != 5 {
		t.Fatalf("réplica %d cambios, auditoría %d", cambios, auditados)
	}
}

// Un nodo que reemplaza a otro (PC nueva) hereda los puntos de emisión del local.
func TestNodoNuevoHeredaLosPuntosDeEmision(t *testing.T) {
	e := newEnv(t)
	c, _ := e.restaurante("1790011674001", "g@g.ec")
	viejo := e.nuevoNodo()
	viejo.activar(c.codigoNodo().Codigo, 200)
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	var cfg facturacion.Config
	c.do("PUT", "/v1/cajas/"+cajas[0].ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 200, &cfg)
	dueno := func() ids.ID {
		var n ids.ID
		if err := e.tdb.Admin.QueryRow(context.Background(), `SELECT nodo_id FROM puntos_emision WHERE id = $1`, *cfg.Cajas[0].PuntoID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if dueno() != viejo.id {
		t.Fatal("el punto nace del nodo activo del local")
	}
	nuevo := e.nuevoNodo()
	nuevo.activar(c.codigoNodo().Codigo, 200)
	if dueno() != nuevo.id {
		t.Fatal("al reemplazar la PC, el nodo nuevo es el dueño de la numeración")
	}
}
