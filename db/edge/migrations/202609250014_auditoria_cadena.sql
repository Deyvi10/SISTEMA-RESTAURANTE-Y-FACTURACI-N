-- F4-15 · Auditoría inmutable con cadena de hash (RF-08-06, QA-10). Cada registro guarda el
-- hash del anterior; los registros previos a esta migración quedan sin hash (anteriores a la
-- cadena) y la verificación empieza en el primero que lo tiene.

-- +goose Up
CREATE TABLE auditoria_nueva (
  id              TEXT PRIMARY KEY,
  seq             INTEGER NOT NULL UNIQUE,      -- orden de la cadena
  usuario_id      TEXT,
  autorizado_por  TEXT,                         -- supervisor que dio su PIN
  dispositivo_id  TEXT,
  accion          TEXT NOT NULL,
  entidad         TEXT NOT NULL,
  entidad_id      TEXT,
  antes           TEXT CHECK (antes IS NULL OR json_valid(antes)),
  despues         TEXT CHECK (despues IS NULL OR json_valid(despues)),
  monto           TEXT,
  motivo          TEXT,
  detalle         TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(detalle)),
  created_at      TEXT NOT NULL,
  hash_anterior   TEXT NOT NULL DEFAULT '',
  hash            TEXT NOT NULL DEFAULT ''      -- '' = anterior a la cadena
) STRICT;
INSERT INTO auditoria_nueva (id, seq, usuario_id, dispositivo_id, accion, entidad, entidad_id, detalle, created_at)
  SELECT id, row_number() OVER (ORDER BY created_at, rowid), usuario_id, dispositivo_id, accion, entidad, entidad_id, detalle, created_at FROM auditoria;
DROP TABLE auditoria;
ALTER TABLE auditoria_nueva RENAME TO auditoria;
CREATE INDEX auditoria_accion ON auditoria (accion, created_at);
CREATE TRIGGER auditoria_sin_update BEFORE UPDATE ON auditoria BEGIN SELECT RAISE(ABORT, 'auditoria es append-only'); END;
CREATE TRIGGER auditoria_sin_delete BEFORE DELETE ON auditoria BEGIN SELECT RAISE(ABORT, 'auditoria es append-only'); END;

-- +goose Down
SELECT 1; -- la cadena no se deshace: los registros son inmutables
