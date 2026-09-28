-- F4-07 · Clientes del restaurante (RF-04-05, docs/04 §7). Los guarda la caja con
-- consentimiento (LOPDP); la nube fusiona los cambios de cada nodo campo por campo y gana
-- el más reciente (last-writer-wins), y responde las búsquedas que el nodo no resuelve.

-- +goose Up
CREATE TABLE clientes (
  id                   uuid PRIMARY KEY,
  tenant_id            uuid NOT NULL,
  tipo_identificacion  char(2) NOT NULL CHECK (tipo_identificacion IN ('04','05','06','08')),
  identificacion       text NOT NULL CHECK (length(identificacion) BETWEEN 3 AND 20),
  razon_social         text NOT NULL CHECK (length(razon_social) BETWEEN 1 AND 300),
  direccion            text,
  email                citext,
  telefono             text,
  consentimiento_at    timestamptz NOT NULL,
  campos_at            jsonb NOT NULL DEFAULT '{}',
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  version              int NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, tipo_identificacion, identificacion)
);
ALTER TABLE clientes ENABLE ROW LEVEL SECURITY;
ALTER TABLE clientes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON clientes USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant());
GRANT SELECT, INSERT, UPDATE ON clientes TO restpos_app;

-- +goose Down
DROP TABLE clientes;
