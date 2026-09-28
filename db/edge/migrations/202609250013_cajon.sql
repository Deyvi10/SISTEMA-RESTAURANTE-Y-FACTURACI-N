-- F4-13 · Cajón de dinero (RF-02-07): la apertura sin venta imprime un comprobante corto con
-- el pulso del cajón; la cola de impresión acepta ese tipo (SQLite no cambia un CHECK: se
-- reconstruye la tabla).

-- +goose Up
CREATE TABLE trabajos_impresion_nueva (
  id            TEXT PRIMARY KEY,
  impresora_id  TEXT NOT NULL,
  estacion_id   TEXT,
  comanda_id    TEXT,
  tipo          TEXT NOT NULL CHECK (tipo IN ('COMANDA','ANULACION','REIMPRESION','PRECUENTA','PRUEBA','CIERRE_Z','VENTA','CAJON')),
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
DELETE FROM trabajos_impresion WHERE tipo = 'CAJON';
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
