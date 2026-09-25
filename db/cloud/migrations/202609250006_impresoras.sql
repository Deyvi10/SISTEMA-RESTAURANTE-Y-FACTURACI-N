-- F2-08 / F2-10 · Impresoras del local, su asignación a estaciones y órdenes de la nube
-- para el nodo (p. ej. «imprimir prueba» desde el backoffice). docs/04 §5, RF-02-02, RF-02-03.
--
-- Propiedad por columnas (docs/03 §3): el hardware (host, mac, modelo) lo informa el nodo;
-- el nombre, el ancho de papel y las estaciones los decide el dueño en la nube. El estado
-- vivo (sin papel, desconectada…) no se guarda aquí: viaja en el heartbeat del nodo.

-- +goose Up
CREATE TABLE impresoras (
  id                uuid PRIMARY KEY,
  tenant_id         uuid NOT NULL,
  local_id          uuid NOT NULL,
  nombre            text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 40),
  conexion          text NOT NULL DEFAULT 'TCP' CHECK (conexion IN ('TCP','USB')),
  host              text CHECK (host IS NULL OR length(host) BETWEEN 1 AND 253),
  puerto            int  CHECK (puerto IS NULL OR puerto BETWEEN 1 AND 65535),
  mac               text CHECK (mac IS NULL OR mac ~ '^[0-9a-f]{2}(:[0-9a-f]{2}){5}$'),
  usb_id            text,
  modelo            text NOT NULL DEFAULT '',
  ancho_papel       smallint NOT NULL DEFAULT 80 CHECK (ancho_papel IN (58, 80)),
  origen            text NOT NULL DEFAULT 'MANUAL' CHECK (origen IN ('MANUAL','DETECTADA')),
  activa            boolean NOT NULL DEFAULT true,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  created_by        uuid,
  version           int NOT NULL DEFAULT 1,
  deleted_at        timestamptz,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id),
  CHECK (conexion <> 'TCP' OR (host IS NOT NULL AND puerto IS NOT NULL)),
  CHECK (conexion <> 'USB' OR usb_id IS NOT NULL)
);
CREATE UNIQUE INDEX impresoras_red ON impresoras (local_id, host, puerto) WHERE conexion = 'TCP' AND deleted_at IS NULL;
CREATE UNIQUE INDEX impresoras_nombre ON impresoras (local_id, lower(nombre)) WHERE deleted_at IS NULL;

-- N:M: una estación imprime en una o varias impresoras (RF-02-03.1).
CREATE TABLE estacion_impresoras (
  tenant_id     uuid NOT NULL,
  estacion_id   uuid NOT NULL,
  impresora_id  uuid NOT NULL,
  local_id      uuid NOT NULL,
  PRIMARY KEY (estacion_id, impresora_id),
  FOREIGN KEY (tenant_id, estacion_id) REFERENCES estaciones (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, impresora_id) REFERENCES impresoras (tenant_id, id) ON DELETE CASCADE
);

-- Órdenes de la nube para el nodo de un local. Viajan por el mismo feed de cambios y el
-- nodo informa el resultado con un evento. Solo se ejecutan si son recientes (10 min).
CREATE TABLE comandos_nodo (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  local_id      uuid NOT NULL,
  tipo          text NOT NULL CHECK (tipo IN ('IMPRIMIR_PRUEBA','BUSCAR_IMPRESORAS')),
  datos         jsonb NOT NULL DEFAULT '{}',
  creado_por    uuid NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  ejecutado_at  timestamptz,
  resultado     text,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id)
);
CREATE INDEX comandos_nodo_pendientes ON comandos_nodo (local_id, created_at) WHERE ejecutado_at IS NULL;

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['impresoras','estacion_impresoras','comandos_nodo'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
    EXECUTE format('CREATE TRIGGER sync_cambio AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION registrar_cambio()', t);
  END LOOP;
END $$;
-- +goose StatementEnd
CREATE TRIGGER touch BEFORE UPDATE ON impresoras FOR EACH ROW EXECUTE FUNCTION touch_row();
GRANT SELECT, INSERT, UPDATE, DELETE ON impresoras, estacion_impresoras TO restpos_app;
GRANT SELECT, INSERT, UPDATE ON comandos_nodo TO restpos_app;

-- +goose Down
DROP TABLE comandos_nodo, estacion_impresoras, impresoras;
