-- F4-05 · Cobro en caja con documento interno de venta (F4-16: sin valor tributario; F5 lo
-- reemplaza por el comprobante electrónico).

-- +goose Up
-- Documento de la venta: inmutable, numerado por nodo y con el detalle tal como se imprimió.
CREATE TABLE documentos_venta (
  id                         TEXT PRIMARY KEY,
  tipo                       TEXT NOT NULL DEFAULT 'INTERNO' CHECK (tipo IN ('INTERNO')),
  numero                     INTEGER NOT NULL,
  orden_id                   TEXT NOT NULL REFERENCES ordenes (id),
  turno_id                   TEXT NOT NULL REFERENCES turnos_caja (id),
  comprador_tipo             TEXT NOT NULL CHECK (comprador_tipo IN ('CONSUMIDOR_FINAL')),
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
-- Una orden se cobra una sola vez (la división de cuentas de F4-08 cambia esta regla).
CREATE UNIQUE INDEX documentos_venta_orden ON documentos_venta (orden_id);
CREATE TRIGGER documentos_venta_sin_update BEFORE UPDATE ON documentos_venta BEGIN SELECT RAISE(ABORT, 'documentos_venta es append-only'); END;
CREATE TRIGGER documentos_venta_sin_delete BEFORE DELETE ON documentos_venta BEGIN SELECT RAISE(ABORT, 'documentos_venta es append-only'); END;

-- El pago apunta al documento que lo originó.
ALTER TABLE pagos ADD COLUMN documento_id TEXT; -- documentos_venta.id

-- La cola de impresión acepta el documento de venta.
CREATE TABLE trabajos_impresion_nueva (
  id            TEXT PRIMARY KEY,
  impresora_id  TEXT NOT NULL,
  estacion_id   TEXT,
  comanda_id    TEXT,
  tipo          TEXT NOT NULL CHECK (tipo IN ('COMANDA','ANULACION','REIMPRESION','PRECUENTA','PRUEBA','CIERRE_Z','VENTA')),
  payload       BLOB NOT NULL,
  documento     TEXT CHECK (documento IS NULL OR json_valid(documento)),
  comando_id    TEXT,
  estado        TEXT NOT NULL DEFAULT 'PENDIENTE' CHECK (estado IN ('PENDIENTE','IMPRESO','CANCELADO')),
  intentos      INTEGER NOT NULL DEFAULT 0,
  ultimo_error  TEXT,
  created_at    TEXT NOT NULL,
  impreso_at    TEXT
) STRICT;
INSERT INTO trabajos_impresion_nueva SELECT id, impresora_id, estacion_id, comanda_id, tipo, payload, documento, comando_id, estado, intentos, ultimo_error, created_at, impreso_at FROM trabajos_impresion;
DROP TABLE trabajos_impresion;
ALTER TABLE trabajos_impresion_nueva RENAME TO trabajos_impresion;
CREATE INDEX trabajos_pendientes ON trabajos_impresion (impresora_id, created_at) WHERE estado = 'PENDIENTE';
CREATE INDEX trabajos_comanda ON trabajos_impresion (comanda_id);

-- +goose Down
DELETE FROM trabajos_impresion WHERE tipo = 'VENTA';
CREATE TABLE trabajos_impresion_nueva (
  id            TEXT PRIMARY KEY,
  impresora_id  TEXT NOT NULL,
  estacion_id   TEXT,
  comanda_id    TEXT,
  tipo          TEXT NOT NULL CHECK (tipo IN ('COMANDA','ANULACION','REIMPRESION','PRECUENTA','PRUEBA','CIERRE_Z')),
  payload       BLOB NOT NULL,
  documento     TEXT CHECK (documento IS NULL OR json_valid(documento)),
  comando_id    TEXT,
  estado        TEXT NOT NULL DEFAULT 'PENDIENTE' CHECK (estado IN ('PENDIENTE','IMPRESO','CANCELADO')),
  intentos      INTEGER NOT NULL DEFAULT 0,
  ultimo_error  TEXT,
  created_at    TEXT NOT NULL,
  impreso_at    TEXT
) STRICT;
INSERT INTO trabajos_impresion_nueva SELECT id, impresora_id, estacion_id, comanda_id, tipo, payload, documento, comando_id, estado, intentos, ultimo_error, created_at, impreso_at FROM trabajos_impresion;
DROP TABLE trabajos_impresion;
ALTER TABLE trabajos_impresion_nueva RENAME TO trabajos_impresion;
CREATE INDEX trabajos_pendientes ON trabajos_impresion (impresora_id, created_at) WHERE estado = 'PENDIENTE';
CREATE INDEX trabajos_comanda ON trabajos_impresion (comanda_id);
ALTER TABLE pagos DROP COLUMN documento_id;
DROP TABLE documentos_venta;
