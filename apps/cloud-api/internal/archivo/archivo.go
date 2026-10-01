// Package archivo guarda los XML autorizados para siempre (F5-11, L-13): cada noche los
// autorizados de un día ya cerrado se comprimen en un solo blob por emisor y día, en el
// contenedor «comprobantes» con inmutabilidad (Object Lock en local, política de Azure en la
// nube). Cada factura es un frame zstd independiente —con el diccionario del emisor cuando ya
// tiene uno— y el índice en PostgreSQL dice dónde está y su SHA-256, así una factura se lee
// con una sola petición por rango.
//
// Antes de indexar se vuelve a leer cada factura desde el almacenamiento, se descomprime y se
// compara el SHA-256 con el original: si algo no cuadra no se indexa nada y la noche siguiente
// se reintenta (el blob fallido queda, porque es inmutable, pero sin índice no se usa).
package archivo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/klauspost/compress/zstd"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/imagenes"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/apperr"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/apps/cloud-api/internal/platform/db"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/clock"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri"
	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/sri/sriws"
)

// MuestrasDiccionario: con cuántas facturas se entrena el diccionario de un emisor (antes,
// zstd sin diccionario).
const MuestrasDiccionario = 1000

// TamanoDiccionario es el tamaño máximo del historial del diccionario (64 KB).
const TamanoDiccionario = 64 << 10

var ErrIntegridad = errors.New("archivo: el comprobante archivado no coincide con el original")

type Archivador struct {
	DB    *db.DB
	Store imagenes.Store
	Log   *slog.Logger
	Now   func() time.Time
	// Muestras para entrenar el diccionario (MuestrasDiccionario si es 0).
	Muestras int
}

type diccionario struct {
	version int
	datos   []byte
}

type pendiente struct {
	id       ids.ID
	clave    string
	firmado  string
	numero   string
	fecha    time.Time
	ambiente int16
}

// ArchivarPendientes archiva todos los días ya cerrados que tengan autorizados sin archivar.
func (a *Archivador) ArchivarPendientes(ctx context.Context) (int, error) {
	hoy := a.Now().In(clock.Guayaquil).Format(time.DateOnly)
	type dia struct {
		tenant ids.ID
		fecha  time.Time
	}
	var dias []dia
	if err := a.DB.Global(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT tenant_id, fecha FROM dias_por_archivar($1::date)`, hoy)
		if err != nil {
			return err
		}
		dias, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (dia, error) {
			var d dia
			return d, r.Scan(&d.tenant, &d.fecha)
		})
		return err
	}); err != nil {
		return 0, err
	}
	total := 0
	for _, d := range dias {
		n, err := a.ArchivarDia(ctx, d.tenant, d.fecha)
		if err != nil {
			a.Log.Error("archivo: no se pudo archivar el día", "tenant", d.tenant, "fecha", d.fecha.Format(time.DateOnly), "err", err)
			continue
		}
		total += n
		if err := a.EntrenarSiHaceFalta(ctx, d.tenant); err != nil {
			a.Log.Error("archivo: no se pudo entrenar el diccionario", "tenant", d.tenant, "err", err)
		}
	}
	return total, nil
}

// ArchivarDia guarda en un blob los autorizados de ese día (hora de Ecuador) que falten.
func (a *Archivador) ArchivarDia(ctx context.Context, tenant ids.ID, fecha time.Time) (int, error) {
	var ps []pendiente
	var dic *diccionario
	err := a.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT c.id, c.clave_acceso, c.xml_firmado, c.numero_autorizacion, c.fecha_autorizacion, c.ambiente
			FROM comprobantes c
			WHERE c.estado IN ('AUTORIZADO', 'ANULADO') AND (c.fecha_autorizacion AT TIME ZONE 'America/Guayaquil')::date = $1::date
			  AND NOT EXISTS (SELECT 1 FROM archivo_comprobantes x WHERE x.comprobante_id = c.id)
			ORDER BY c.fecha_autorizacion, c.id`, fecha.Format(time.DateOnly))
		if err != nil {
			return err
		}
		ps, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (pendiente, error) {
			var p pendiente
			return p, r.Scan(&p.id, &p.clave, &p.firmado, &p.numero, &p.fecha, &p.ambiente)
		})
		if err != nil {
			return err
		}
		dic, err = ultimoDiccionario(ctx, tx)
		return err
	})
	if err != nil || len(ps) == 0 {
		return 0, err
	}

	opciones := []zstd.EOption{zstd.WithEncoderLevel(zstd.SpeedBetterCompression), zstd.WithEncoderCRC(true)}
	if dic != nil {
		opciones = append(opciones, zstd.WithEncoderDict(dic.datos))
	}
	enc, err := zstd.NewWriter(nil, opciones...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = enc.Close() }()
	type entrada struct {
		off, largo int64
		sha        string
	}
	var blob []byte
	var bytesXML int64
	entradas := make([]entrada, len(ps))
	for i, p := range ps {
		doc, err := sriws.XMLAutorizado([]byte(p.firmado), p.numero, p.fecha, sri.Ambiente(p.ambiente))
		if err != nil {
			return 0, err
		}
		suma := sha256.Sum256(doc)
		frame := enc.EncodeAll(doc, nil)
		entradas[i] = entrada{off: int64(len(blob)), largo: int64(len(frame)), sha: hex.EncodeToString(suma[:])}
		blob = append(blob, frame...)
		bytesXML += int64(len(doc))
	}
	lote := ids.New()
	key := fmt.Sprintf("%s/%s-%s.zst", tenant, fecha.Format("2006/01/2006-01-02"), lote)
	if err := a.Store.Put(ctx, key, blob, "application/zstd"); err != nil {
		return 0, err
	}
	// Verificación desde el almacenamiento, factura por factura, antes de indexar.
	dec, err := decodificador(dic)
	if err != nil {
		return 0, err
	}
	defer dec.Close()
	for i, e := range entradas {
		frame, err := a.Store.GetRange(ctx, key, e.off, e.largo)
		if err != nil {
			return 0, fmt.Errorf("archivo: releer %s: %w", ps[i].clave, err)
		}
		doc, err := dec.DecodeAll(frame, nil)
		if err != nil {
			return 0, fmt.Errorf("archivo: descomprimir %s: %w", ps[i].clave, err)
		}
		if suma := sha256.Sum256(doc); hex.EncodeToString(suma[:]) != e.sha {
			return 0, fmt.Errorf("%w: %s", ErrIntegridad, ps[i].clave)
		}
	}
	err = a.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO archivo_lotes (id, tenant_id, fecha, blob_key, cantidad, bytes, bytes_xml)
			VALUES ($1, app_tenant(), $2, $3, $4, $5, $6)`, lote, fecha.Format(time.DateOnly), key, len(ps), len(blob), bytesXML); err != nil {
			return err
		}
		var version *int
		if dic != nil {
			version = &dic.version
		}
		for i, p := range ps {
			if _, err := tx.Exec(ctx, `INSERT INTO archivo_comprobantes (comprobante_id, tenant_id, lote_id, clave_acceso, desplazamiento, largo, sha256, diccionario)
				VALUES ($1, app_tenant(), $2, $3, $4, $5, $6, $7)`, p.id, lote, p.clave, entradas[i].off, entradas[i].largo, entradas[i].sha, version); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	a.Log.Info("archivo: día archivado", "tenant", tenant, "fecha", fecha.Format(time.DateOnly), "facturas", len(ps),
		"bytes", len(blob), "xml", bytesXML, "diccionario", dic != nil)
	return len(ps), nil
}

func ultimoDiccionario(ctx context.Context, tx db.Tx) (*diccionario, error) {
	var d diccionario
	err := tx.QueryRow(ctx, `SELECT version, datos FROM diccionarios_zstd ORDER BY version DESC LIMIT 1`).Scan(&d.version, &d.datos)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &d, err
}

func decodificador(d *diccionario) (*zstd.Decoder, error) {
	if d == nil {
		return zstd.NewReader(nil)
	}
	return zstd.NewReader(nil, zstd.WithDecoderDicts(d.datos))
}

// Leer devuelve el XML autorizado archivado, verificado contra su SHA-256.
func (a *Archivador) Leer(ctx context.Context, tenant, comprobante ids.ID) ([]byte, error) {
	var key, sha string
	var off, largo int64
	var dic *diccionario
	err := a.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		var version *int
		err := tx.QueryRow(ctx, `SELECT l.blob_key, x.desplazamiento, x.largo, x.sha256, x.diccionario
			FROM archivo_comprobantes x JOIN archivo_lotes l ON l.id = x.lote_id WHERE x.comprobante_id = $1`, comprobante).
			Scan(&key, &off, &largo, &sha, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil || version == nil {
			return err
		}
		dic = &diccionario{version: *version}
		return tx.QueryRow(ctx, `SELECT datos FROM diccionarios_zstd WHERE version = $1`, *version).Scan(&dic.datos)
	})
	if err != nil {
		return nil, err
	}
	frame, err := a.Store.GetRange(ctx, key, off, largo)
	if err != nil {
		return nil, err
	}
	dec, err := decodificador(dic)
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	doc, err := dec.DecodeAll(frame, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIntegridad, err)
	}
	if suma := sha256.Sum256(doc); hex.EncodeToString(suma[:]) != sha {
		return nil, ErrIntegridad
	}
	return doc, nil
}

// EntrenarSiHaceFalta crea el primer diccionario del emisor cuando ya tiene suficientes
// facturas archivadas. Los diccionarios no cambian nunca: uno nuevo es otra versión.
func (a *Archivador) EntrenarSiHaceFalta(ctx context.Context, tenant ids.ID) error {
	muestras := a.Muestras
	if muestras == 0 {
		muestras = MuestrasDiccionario
	}
	var docs [][]byte
	var version int
	err := a.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		var hay bool
		var archivadas int
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM diccionarios_zstd), (SELECT count(*) FROM archivo_comprobantes)`).Scan(&hay, &archivadas); err != nil {
			return err
		}
		if hay || archivadas < muestras {
			return nil
		}
		version = 1
		rows, err := tx.Query(ctx, `SELECT c.xml_firmado, c.numero_autorizacion, c.fecha_autorizacion, c.ambiente
			FROM archivo_comprobantes x JOIN comprobantes c ON c.id = x.comprobante_id
			WHERE c.xml_firmado IS NOT NULL ORDER BY x.created_at, x.comprobante_id LIMIT $1`, muestras)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p pendiente
			if err := rows.Scan(&p.firmado, &p.numero, &p.fecha, &p.ambiente); err != nil {
				return err
			}
			doc, err := sriws.XMLAutorizado([]byte(p.firmado), p.numero, p.fecha, sri.Ambiente(p.ambiente))
			if err != nil {
				return err
			}
			docs = append(docs, doc)
		}
		return rows.Err()
	})
	if err != nil || len(docs) == 0 {
		return err
	}
	datos, err := Entrenar(docs)
	if err != nil {
		return err
	}
	suma := sha256.Sum256(datos)
	key := fmt.Sprintf("%s/diccionarios/v%d.dict", tenant, version)
	if err := a.Store.Put(ctx, key, datos, "application/octet-stream"); err != nil {
		return err
	}
	return a.DB.InTenant(ctx, tenant, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO diccionarios_zstd (tenant_id, version, zstd_id, datos, sha256, muestras, blob_key)
			VALUES (app_tenant(), $1, $2, $3, $4, $5, $6)`, version, int64(idDiccionario(datos)), datos, hex.EncodeToString(suma[:]), len(docs), key)
		return err
	})
}

// Entrenar arma un diccionario zstd: el historial (hasta 64 KB) sale de la mitad más antigua
// de las muestras y las tablas de entropía de todas, así siempre quedan datos que el
// historial no cubre para medir.
func Entrenar(muestras [][]byte) ([]byte, error) {
	if len(muestras) < 2 {
		return nil, errors.New("archivo: hacen falta al menos 2 muestras para el diccionario")
	}
	var historial []byte
	for i := len(muestras)/2 - 1; i >= 0 && len(historial) < TamanoDiccionario; i-- {
		m := muestras[i]
		if falta := TamanoDiccionario - len(historial); len(m) > falta {
			m = m[len(m)-falta:]
		}
		historial = append(append([]byte{}, m...), historial...)
	}
	id := make([]byte, 4)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	// IDs de diccionario 32768 a 2^31-1: el rango libre para uso privado.
	n := 32768 + binary.BigEndian.Uint32(id)%(1<<31-32768)
	return zstd.BuildDict(zstd.BuildDictOptions{ID: n, Contents: muestras, History: historial, Offsets: [3]int{1, 4, 8},
		Level: zstd.SpeedBetterCompression})
}

func idDiccionario(d []byte) uint32 {
	if len(d) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint32(d[4:8])
}

// Correr archiva al arrancar y luego cada hora (solo hay trabajo una vez al día, pasada la
// medianoche de Ecuador, o si una noche falló).
func (a *Archivador) Correr(ctx context.Context) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		if _, err := a.ArchivarPendientes(ctx); err != nil && ctx.Err() == nil {
			a.Log.Warn("archivo: no se pudieron leer los días pendientes", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
