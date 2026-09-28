-- F2-01 · Base del Nodo Local: identidad, cursores de sincronización, outbox y auditoría.
-- Convenciones SQLite (docs/04 §1): UUID en TEXT, fechas TEXT RFC 3339 en UTC, dinero TEXT
-- decimal (nunca REAL), enums con CHECK.

-- +goose Up
-- Identidad del nodo: una sola fila. Vacía hasta la activación (F2-02).
CREATE TABLE nodo (
  id             INTEGER PRIMARY KEY CHECK (id = 1),
  nodo_id        TEXT NOT NULL,
  tenant_id      TEXT NOT NULL,
  local_id       TEXT NOT NULL,
  nube_url       TEXT NOT NULL,
  llave_privada  BLOB NOT NULL,
  nombre_local   TEXT NOT NULL DEFAULT '',
  activado_at    TEXT NOT NULL
) STRICT;

-- Cursor por flujo Nube → Nodo (docs/04 §9).
CREATE TABLE inbox_cursores (
  flujo       TEXT PRIMARY KEY,
  cursor      INTEGER NOT NULL DEFAULT 0 CHECK (cursor >= 0),
  updated_at  TEXT NOT NULL
) STRICT;

-- Outbox Nodo → Nube (ADR-0004, ADR-0012). AUTOINCREMENT: un seq nunca se reutiliza.
CREATE TABLE outbox (
  seq         INTEGER PRIMARY KEY AUTOINCREMENT,
  evento_id   TEXT    NOT NULL UNIQUE,
  tipo        TEXT    NOT NULL,
  version     INTEGER NOT NULL CHECK (version >= 1),
  agregado_id TEXT    NOT NULL,
  payload     TEXT    NOT NULL CHECK (json_valid(payload)),
  created_at  TEXT    NOT NULL,
  enviado_at  TEXT
);
CREATE INDEX outbox_pendientes ON outbox (seq) WHERE enviado_at IS NULL;

-- Auditoría local append-only (la cadena de hash llega en F4-15).
CREATE TABLE auditoria (
  id              TEXT PRIMARY KEY,
  usuario_id      TEXT,
  dispositivo_id  TEXT,
  accion          TEXT NOT NULL,
  entidad         TEXT NOT NULL,
  entidad_id      TEXT,
  detalle         TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(detalle)),
  created_at      TEXT NOT NULL
) STRICT;
CREATE TRIGGER auditoria_sin_update BEFORE UPDATE ON auditoria BEGIN SELECT RAISE(ABORT, 'auditoria es append-only'); END;
CREATE TRIGGER auditoria_sin_delete BEFORE DELETE ON auditoria BEGIN SELECT RAISE(ABORT, 'auditoria es append-only'); END;

-- +goose Down
DROP TABLE auditoria;
DROP TABLE outbox;
DROP TABLE inbox_cursores;
DROP TABLE nodo;
