-- F4-10 · Propina legal del 10 % (RF-04-08): el cajero la retira si el cliente la rechaza
-- (auditado) y cada propina cobrada queda registrada por orden y mesero para el reparto.

-- +goose Up
ALTER TABLE ordenes ADD COLUMN propina_retirada INTEGER NOT NULL DEFAULT 0 CHECK (propina_retirada IN (0, 1));
ALTER TABLE ordenes ADD COLUMN propina_motivo TEXT;

CREATE TABLE propinas (
  id             TEXT PRIMARY KEY,
  orden_id       TEXT NOT NULL REFERENCES ordenes (id),
  documento_id   TEXT NOT NULL UNIQUE,
  mesero_id      TEXT NOT NULL,
  mesero_nombre  TEXT NOT NULL,
  fecha_negocio  TEXT NOT NULL,
  monto          TEXT NOT NULL,
  created_at     TEXT NOT NULL
) STRICT;
CREATE INDEX propinas_mesero ON propinas (fecha_negocio, mesero_id);
CREATE TRIGGER propinas_sin_update BEFORE UPDATE ON propinas BEGIN SELECT RAISE(ABORT, 'propinas es append-only'); END;
CREATE TRIGGER propinas_sin_delete BEFORE DELETE ON propinas BEGIN SELECT RAISE(ABORT, 'propinas es append-only'); END;

-- +goose Down
DROP TABLE propinas;
ALTER TABLE ordenes DROP COLUMN propina_motivo;
ALTER TABLE ordenes DROP COLUMN propina_retirada;
