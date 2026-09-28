-- F4-07 · Datos del comprador (RF-04-05): clientes del restaurante en el nodo y documentos
-- de venta a nombre de un comprador identificado.

-- +goose Up
-- Clientes guardados con consentimiento (LOPDP). Se buscan aquí primero; los que vienen de
-- la nube quedan en caché. campos_at guarda cuándo cambió cada campo: la nube fusiona campo
-- por campo y gana el más reciente (last-writer-wins).
CREATE TABLE clientes (
  id                   TEXT PRIMARY KEY,
  tipo_identificacion  TEXT NOT NULL CHECK (tipo_identificacion IN ('04','05','06','08')),
  identificacion       TEXT NOT NULL,
  razon_social         TEXT NOT NULL CHECK (length(razon_social) BETWEEN 1 AND 300),
  direccion            TEXT,
  email                TEXT,
  telefono             TEXT,
  consentimiento_at    TEXT,
  campos_at            TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(campos_at)),
  origen               TEXT NOT NULL DEFAULT 'NODO' CHECK (origen IN ('NODO','NUBE')),
  created_at           TEXT NOT NULL,
  updated_at           TEXT NOT NULL,
  UNIQUE (tipo_identificacion, identificacion)
) STRICT;

-- El comprador del documento pasa a guardarse con su código SRI (07 = consumidor final).
CREATE TABLE documentos_venta_nueva (
  id                         TEXT PRIMARY KEY,
  tipo                       TEXT NOT NULL DEFAULT 'INTERNO' CHECK (tipo IN ('INTERNO')),
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
  UNIQUE (tipo, numero)
) STRICT;
INSERT INTO documentos_venta_nueva
  SELECT id, tipo, numero, orden_id, turno_id, CASE comprador_tipo WHEN 'CONSUMIDOR_FINAL' THEN '07' ELSE comprador_tipo END,
         comprador_identificacion, comprador_nombre, subtotal, iva, propina, total, datos, idempotency_key, emitido_por, created_at
  FROM documentos_venta;
DROP TABLE documentos_venta;
ALTER TABLE documentos_venta_nueva RENAME TO documentos_venta;
CREATE UNIQUE INDEX documentos_venta_orden ON documentos_venta (orden_id);
CREATE INDEX documentos_venta_comprador ON documentos_venta (comprador_identificacion);
CREATE TRIGGER documentos_venta_sin_update BEFORE UPDATE ON documentos_venta BEGIN SELECT RAISE(ABORT, 'documentos_venta es append-only'); END;
CREATE TRIGGER documentos_venta_sin_delete BEFORE DELETE ON documentos_venta BEGIN SELECT RAISE(ABORT, 'documentos_venta es append-only'); END;

-- +goose Down
DROP TABLE clientes;
