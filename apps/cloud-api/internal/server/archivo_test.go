package server_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/archivo"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/caja"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/facturacion"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/imagenes"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// almacenQueCorrompe cambia un byte de lo que guarda (un disco o una red que fallan).
type almacenQueCorrompe struct{ imagenes.Memory }

func (a *almacenQueCorrompe) Put(ctx context.Context, key string, data []byte, ct string) error {
	malo := append([]byte{}, data...)
	malo[len(malo)/2] ^= 0xff
	return a.Memory.Put(ctx, key, malo, ct)
}

// autorizados deja n facturas autorizadas (por el stub) y devuelve sus IDs y cómo emitir más.
func (e *env) autorizados(c *cliente, n int) ([]ids.ID, func(sec int64) ids.ID) {
	e.t.Helper()
	nodo := e.nuevoNodo()
	nodo.activar(c.codigoNodo().Codigo, 200)
	var cajas []caja.Caja
	c.do("GET", "/v1/cajas", nil, 200, &cajas)
	var cfg facturacion.Config
	c.do("PUT", "/v1/cajas/"+cajas[0].ID.String()+"/punto-emision", map[string]any{"establecimiento": "001", "puntoEmision": "001"}, 200, &cfg)
	ahora := time.Now()
	c.subirP12(p12De(e.t, cfg.RUC, ahora.Add(-time.Hour), ahora.AddDate(1, 0, 0)), claveP12, 201)
	emitir := func(sec int64) ids.ID {
		id := nodo.comprobante(cfg.RUC, *cfg.Cajas[0].PuntoID, sec, false).ID
		drenar(e.t, e.worker("fiscal@archivo"))
		return id
	}
	var out []ids.ID
	for i := range n {
		out = append(out, emitir(int64(100+i)))
	}
	return out, emitir
}

// F5-11: los autorizados de un día cerrado quedan en un blob, se leen por rango y se verifican.
func TestArchivoDeAutorizados(t *testing.T) {
	e := newEnv(t)
	c, r := e.restaurante("1790011674001", "arch@w.ec")
	comp, emitir := e.autorizados(c, 4)
	ctx := context.Background()
	store := &imagenes.Memory{}
	manana := time.Now().Add(24 * time.Hour)
	a := &archivo.Archivador{DB: e.tdb.App, Store: store, Log: slog.Default(), Now: func() time.Time { return manana }, Muestras: 3}

	// El día en curso no se archiva todavía.
	hoy := &archivo.Archivador{DB: e.tdb.App, Store: store, Log: slog.Default(), Now: time.Now}
	if n, err := hoy.ArchivarPendientes(ctx); err != nil || n != 0 {
		t.Fatalf("día abierto: %d %v", n, err)
	}

	// Un almacenamiento que corrompe: no se indexa nada y se reintenta.
	malo := &archivo.Archivador{DB: e.tdb.App, Store: &almacenQueCorrompe{}, Log: slog.Default(), Now: a.Now}
	if n, _ := malo.ArchivarPendientes(ctx); n != 0 {
		t.Fatal("con el blob corrupto no se indexa")
	}
	var indexados int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*) FROM archivo_comprobantes WHERE tenant_id = $1`, r.TenantID).Scan(&indexados)
	if indexados != 0 {
		t.Fatal("quedó índice de un blob corrupto")
	}

	if n, err := a.ArchivarPendientes(ctx); err != nil || n != 4 {
		t.Fatalf("archivar: %d %v", n, err)
	}
	if n, _ := a.ArchivarPendientes(ctx); n != 0 {
		t.Fatal("lo archivado no se vuelve a archivar")
	}
	// Cada factura se lee del blob por rango y es exactamente el XML autorizado.
	for _, id := range comp {
		doc, err := a.Leer(ctx, r.TenantID, id)
		if err != nil {
			t.Fatal(err)
		}
		var esperado facturacion.Documentos
		if err := e.tdb.App.InTenant(ctx, r.TenantID, func(tx db.Tx) error {
			var err error
			esperado, err = facturacion.LeerDocumentos(ctx, tx, id)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if string(doc) != string(esperado.XML) {
			t.Fatal("el XML archivado no es el autorizado")
		}
	}
	var lotes, cantidad int
	var key string
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT count(*), max(cantidad), max(blob_key) FROM archivo_lotes WHERE tenant_id = $1`, r.TenantID).Scan(&lotes, &cantidad, &key)
	if lotes != 1 || cantidad != 4 {
		t.Fatalf("un blob por emisor y día: %d lotes, %d facturas", lotes, cantidad)
	}
	if ok, _ := store.Exists(ctx, key); !ok {
		t.Fatal("el blob no está en el almacenamiento")
	}
	// Con 3 muestras ya se entrenó el diccionario del emisor; lo siguiente se comprime con él.
	var version int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT version FROM diccionarios_zstd WHERE tenant_id = $1`, r.TenantID).Scan(&version)
	if version != 1 {
		t.Fatal("falta el diccionario del emisor")
	}

	// La siguiente factura (autorizada «ayer» para el reloj del archivador) va con el diccionario.
	otra := emitir(200)
	if n, err := a.ArchivarPendientes(ctx); err != nil || n != 1 {
		t.Fatalf("segundo lote: %d %v", n, err)
	}
	var conDic int
	_ = e.tdb.Admin.QueryRow(ctx, `SELECT diccionario FROM archivo_comprobantes WHERE comprobante_id = $1`, otra).Scan(&conDic)
	if doc, err := a.Leer(ctx, r.TenantID, otra); err != nil || conDic != 1 || len(doc) == 0 {
		t.Fatalf("lectura con diccionario: %v (diccionario %d)", err, conDic)
	}

	// Inmutable: ni el índice ni los lotes cambian.
	if _, err := e.tdb.Admin.Exec(ctx, `UPDATE archivo_comprobantes SET largo = 1 WHERE comprobante_id = $1`, comp[0]); err == nil {
		t.Fatal("el índice no se modifica")
	}
	if _, err := e.tdb.Admin.Exec(ctx, `DELETE FROM diccionarios_zstd WHERE tenant_id = $1`, r.TenantID); err == nil {
		t.Fatal("un diccionario no se borra")
	}

	// Si alguien altera el blob, la lectura lo detecta.
	_ = store.Put(ctx, key, []byte("x"), "application/zstd")
	if _, err := a.Leer(ctx, r.TenantID, comp[0]); err == nil {
		t.Fatal("un blob alterado no se entrega")
	}
	if _, err := a.Leer(ctx, r.TenantID, ids.New()); err == nil || errors.Is(err, archivo.ErrIntegridad) {
		t.Fatalf("lo que no está archivado: %v", err)
	}
}
