-- F5-05 · Emisión local del comprobante electrónico. El documento de venta puede ser la
-- factura (antes solo el documento interno de F4-16) y cada factura guarda su comprobante:
-- serie, secuencial, clave de acceso y el XML sin firmar que la nube firma y envía al SRI
-- (el .p12 nunca sale de la nube). Todo append-only.

-- +goose Up
CREATE TABLE documentos_venta_nueva (
  id                         TEXT PRIMARY KEY,
  tipo                       TEXT NOT NULL DEFAULT 'INTERNO' CHECK (tipo IN ('INTERNO', 'FACTURA')),
  numero                     INTEGER NOT NULL,
  orden_id                   TEXT NOT NULL REFERENCES ordenes (id),
  turno_id                   TEXT NOT NULL REFERENCES turnos_caja (id),
  comprador_tipo             TEXT NOT NULL CHECK (comprador_tipo IN ('04','05','06','07','08')),
  comprador_identificacion   TEXT NOT NULL,
  comprador_nombre           TEXT NOT NULL,
  subtotal                   TEXT NOT NULL,
  iva                        TEXT NOT NULL,
  propina                    TEXT NOT NULL,
  total                      TEXT NOT NULL,
  datos                      TEXT NOT NULL CHECK (json_valid(datos)),
  idempotency_key            TEXT NOT NULL UNIQUE,
  emitido_por                TEXT NOT NULL,
  created_at                 TEXT NOT NULL,
  cuenta_id                  TEXT,
  UNIQUE (tipo, numero)
) STRICT;
INSERT INTO documentos_venta_nueva
  SELECT id, tipo, numero, orden_id, turno_id, comprador_tipo, comprador_identificacion, comprador_nombre,
         subtotal, iva, propina, total, datos, idempotency_key, emitido_por, created_at, cuenta_id
  FROM documentos_venta;
DROP TABLE documentos_venta;
ALTER TABLE documentos_venta_nueva RENAME TO documentos_venta;
CREATE UNIQUE INDEX documentos_venta_orden ON documentos_venta (orden_id, coalesce(cuenta_id, ''));
CREATE INDEX documentos_venta_comprador ON documentos_venta (comprador_identificacion);
CREATE TRIGGER documentos_venta_sin_update BEFORE UPDATE ON documentos_venta BEGIN SELECT RAISE(ABORT, 'documentos_venta es append-only'); END;
CREATE TRIGGER documentos_venta_sin_delete BEFORE DELETE ON documentos_venta BEGIN SELECT RAISE(ABORT, 'documentos_venta es append-only'); END;

CREATE TABLE comprobantes (
  id                TEXT PRIMARY KEY,
  documento_id      TEXT NOT NULL UNIQUE REFERENCES documentos_venta (id),
  tipo              TEXT NOT NULL CHECK (tipo IN ('01', '04')),
  ambiente          INTEGER NOT NULL CHECK (ambiente IN (1, 2)),
  punto_emision_id  TEXT NOT NULL,
  serie             TEXT NOT NULL CHECK (length(serie) = 6),
  secuencial        INTEGER NOT NULL CHECK (secuencial BETWEEN 1 AND 999999999),
  clave_acceso      TEXT NOT NULL UNIQUE CHECK (length(clave_acceso) = 49),
  fecha_emision     TEXT NOT NULL,
  importe_total     TEXT NOT NULL,
  xml               TEXT NOT NULL,
  hash              TEXT NOT NULL,
  created_at        TEXT NOT NULL,
  -- Segunda barrera contra números repetidos (la primera es secuenciales, F5-02).
  UNIQUE (punto_emision_id, tipo, ambiente, secuencial)
) STRICT;
CREATE TRIGGER comprobantes_sin_update BEFORE UPDATE ON comprobantes BEGIN SELECT RAISE(ABORT, 'comprobantes es append-only'); END;
CREATE TRIGGER comprobantes_sin_delete BEFORE DELETE ON comprobantes BEGIN SELECT RAISE(ABORT, 'comprobantes es append-only'); END;

-- RUC del proveedor del sistema de facturación: obligatorio en cada comprobante (Anexo 26 de
-- la ficha, Res. NAC-DGERCGC26-00000027). Llega de la nube con parametros_globales.

-- +goose Down
DROP TABLE comprobantes;
