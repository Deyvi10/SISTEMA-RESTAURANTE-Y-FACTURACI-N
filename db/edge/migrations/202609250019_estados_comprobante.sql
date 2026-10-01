-- F5-11 · Estado de cada comprobante según la nube (réplica de «estados_comprobante»): el nodo
-- sabe si su factura ya está autorizada sin consultar al SRI. Los últimos 90 días llegan
-- también en el volcado.

-- +goose Up
CREATE TABLE estados_comprobante (
  id                   TEXT PRIMARY KEY,
  tenant_id            TEXT,
  local_id             TEXT,
  clave_acceso         TEXT NOT NULL,
  estado               TEXT NOT NULL,
  numero_autorizacion  TEXT,
  fecha_autorizacion   TEXT,
  mensaje              TEXT,
  updated_at           TEXT
);
CREATE INDEX estados_comprobante_clave ON estados_comprobante (clave_acceso);

-- +goose Down
DROP TABLE estados_comprobante;
