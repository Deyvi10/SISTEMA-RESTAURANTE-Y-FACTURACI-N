-- F4-12 · Cierre de turno ciego y Cierre Z (RF-04-10, docs/04 §6).

-- +goose Up
-- Pagos de las cuentas (los registra el cobro, F4-05). El Cierre Z los suma por método para
-- calcular lo esperado; existen desde ya para que el cierre sea el mismo antes y después.
CREATE TABLE pagos (
  id              TEXT PRIMARY KEY,
  orden_id        TEXT NOT NULL,
  cuenta_id       TEXT,
  turno_id        TEXT NOT NULL REFERENCES turnos_caja (id),
  metodo_pago_id  TEXT NOT NULL,
  monto           TEXT NOT NULL,                 -- lo aplicado a la cuenta
  recibido        TEXT,                          -- efectivo entregado por el cliente
  vuelto          TEXT,
  referencia      TEXT,
  lote            TEXT,
  ultimos4        TEXT CHECK (ultimos4 IS NULL OR length(ultimos4) = 4),
  created_at      TEXT NOT NULL
) STRICT;
CREATE INDEX pagos_turno ON pagos (turno_id);
CREATE TRIGGER pagos_sin_update BEFORE UPDATE ON pagos BEGIN SELECT RAISE(ABORT, 'pagos es append-only'); END;
CREATE TRIGGER pagos_sin_delete BEFORE DELETE ON pagos BEGIN SELECT RAISE(ABORT, 'pagos es append-only'); END;

-- Cierre Z: inmutable, numerado por caja y encadenado por hash con el anterior de la caja.
CREATE TABLE cierres_z (
  id               TEXT PRIMARY KEY,
  turno_id         TEXT NOT NULL UNIQUE REFERENCES turnos_caja (id),
  caja_id          TEXT NOT NULL,
  numero           INTEGER NOT NULL,
  resultado        TEXT NOT NULL CHECK (resultado IN ('CUADRADO','SOBRANTE','FALTANTE')),
  datos            TEXT NOT NULL,                -- el cierre completo (JSON), lo que se imprime y se sincroniza
  hash             TEXT NOT NULL,
  hash_anterior    TEXT NOT NULL,
  idempotency_key  TEXT NOT NULL UNIQUE,
  generado_at      TEXT NOT NULL,
  UNIQUE (caja_id, numero)
) STRICT;
CREATE TRIGGER cierres_z_sin_update BEFORE UPDATE ON cierres_z BEGIN SELECT RAISE(ABORT, 'cierres_z es append-only'); END;
CREATE TRIGGER cierres_z_sin_delete BEFORE DELETE ON cierres_z BEGIN SELECT RAISE(ABORT, 'cierres_z es append-only'); END;

-- La cola de impresión acepta el Cierre Z (SQLite no cambia un CHECK: se reconstruye la tabla).
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

-- +goose Down
DELETE FROM trabajos_impresion WHERE tipo = 'CIERRE_Z';
CREATE TABLE trabajos_impresion_nueva (
  id            TEXT PRIMARY KEY,
  impresora_id  TEXT NOT NULL,
  estacion_id   TEXT,
  comanda_id    TEXT,
  tipo          TEXT NOT NULL CHECK (tipo IN ('COMANDA','ANULACION','REIMPRESION','PRECUENTA','PRUEBA')),
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
DROP TABLE cierres_z;
DROP TABLE pagos;
