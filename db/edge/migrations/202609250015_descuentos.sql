-- F4-09 · Descuentos y cortesías (RF-04-07). Un descuento por línea y uno por cuenta (la
-- orden completa) a la vez; quitarlo lo marca (la historia queda en la auditoría).

-- +goose Up
-- Réplica de los límites que decide el dueño.
ALTER TABLE locales ADD COLUMN descuento_maximo_pct TEXT;
ALTER TABLE usuarios ADD COLUMN descuento_maximo_pct TEXT;

CREATE TABLE descuentos (
  id              TEXT PRIMARY KEY,
  orden_id        TEXT NOT NULL REFERENCES ordenes (id),
  linea_id        TEXT REFERENCES orden_lineas (id),   -- NULL = descuento de la cuenta
  tipo            TEXT NOT NULL CHECK (tipo IN ('PORCENTAJE','MONTO')),
  valor           TEXT NOT NULL,                        -- % (0–100) o USD, decimal exacto
  cortesia        INTEGER NOT NULL DEFAULT 0 CHECK (cortesia IN (0, 1)),
  motivo_id       TEXT NOT NULL,
  motivo          TEXT NOT NULL,
  usuario_id      TEXT NOT NULL,
  usuario_nombre  TEXT NOT NULL,
  autorizado_por  TEXT,
  created_at      TEXT NOT NULL,
  quitado_at      TEXT,
  quitado_por     TEXT
) STRICT;
CREATE UNIQUE INDEX descuentos_vigente ON descuentos (orden_id, coalesce(linea_id, '')) WHERE quitado_at IS NULL;

-- +goose Down
DROP TABLE descuentos;
ALTER TABLE usuarios DROP COLUMN descuento_maximo_pct;
ALTER TABLE locales DROP COLUMN descuento_maximo_pct;
