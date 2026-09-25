-- F2-02 · Identidad pendiente de activación y marca de revocación.
-- La identidad (id + llave ed25519) se genera antes de llamar a la nube y se reutiliza si
-- se reintenta con el mismo código: así una respuesta perdida no deja el código gastado.
-- La llave privada se cifrará con DPAPI en F6-07; nunca sale de esta base.

-- +goose Up
CREATE TABLE identidad_pendiente (
  id             INTEGER PRIMARY KEY CHECK (id = 1),
  codigo_hash    TEXT NOT NULL,
  nodo_id        TEXT NOT NULL,
  llave_privada  BLOB NOT NULL,
  created_at     TEXT NOT NULL
) STRICT;

ALTER TABLE nodo ADD COLUMN revocado_at TEXT;
ALTER TABLE nodo ADD COLUMN nombre_comercial TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE nodo DROP COLUMN nombre_comercial;
ALTER TABLE nodo DROP COLUMN revocado_at;
DROP TABLE identidad_pendiente;
