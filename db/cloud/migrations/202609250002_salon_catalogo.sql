-- F1-07 / F1-08 · Salón (zonas, mesas, estaciones) y catálogo (categorías, productos,
-- modificadores, notas rápidas, tarifas de IVA). docs/04 §4-5.

-- +goose Up
CREATE TABLE estaciones (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  local_id    uuid NOT NULL,
  nombre      text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 40),
  tipo        text NOT NULL DEFAULT 'PRODUCCION' CHECK (tipo IN ('PRODUCCION','CAJA')),
  icono       text NOT NULL DEFAULT 'cocina',
  color       text NOT NULL DEFAULT 'red',
  orden       int NOT NULL DEFAULT 0,
  es_defecto  boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  created_by  uuid,
  version     int NOT NULL DEFAULT 1,
  deleted_at  timestamptz,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id)
);
CREATE UNIQUE INDEX estaciones_nombre ON estaciones (local_id, lower(nombre)) WHERE deleted_at IS NULL;
-- RF-02-03.4: una estación por defecto por local, para que ninguna comanda se pierda.
CREATE UNIQUE INDEX estaciones_defecto ON estaciones (local_id) WHERE es_defecto AND deleted_at IS NULL;

CREATE TABLE zonas (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  local_id    uuid NOT NULL,
  nombre      text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 40),
  orden       int NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  created_by  uuid,
  version     int NOT NULL DEFAULT 1,
  deleted_at  timestamptz,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id)
);
CREATE UNIQUE INDEX zonas_nombre ON zonas (local_id, lower(nombre)) WHERE deleted_at IS NULL;

-- El estado vivo (libre/ocupada/bloqueada) NO se guarda aquí: lo deriva el Nodo Local.
CREATE TABLE mesas (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  local_id    uuid NOT NULL,
  zona_id     uuid NOT NULL,
  nombre      text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 20),
  capacidad   smallint NOT NULL DEFAULT 4 CHECK (capacidad BETWEEN 1 AND 50),
  forma       text NOT NULL DEFAULT 'CUADRADA' CHECK (forma IN ('CUADRADA','REDONDA','RECTANGULAR')),
  pos_x       smallint NOT NULL DEFAULT 0 CHECK (pos_x BETWEEN 0 AND 99),
  pos_y       smallint NOT NULL DEFAULT 0 CHECK (pos_y BETWEEN 0 AND 99),
  ancho       smallint NOT NULL DEFAULT 1 CHECK (ancho BETWEEN 1 AND 4),
  alto        smallint NOT NULL DEFAULT 1 CHECK (alto BETWEEN 1 AND 4),
  rotacion    smallint NOT NULL DEFAULT 0 CHECK (rotacion IN (0, 90, 180, 270)),
  activa      boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  created_by  uuid,
  version     int NOT NULL DEFAULT 1,
  deleted_at  timestamptz,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id),
  FOREIGN KEY (tenant_id, zona_id) REFERENCES zonas (tenant_id, id)
);
CREATE UNIQUE INDEX mesas_nombre ON mesas (local_id, lower(nombre)) WHERE deleted_at IS NULL;

-- Tarifas de IVA: tabla global versionada por fecha (el IVA cambia por ley). docs/05 §7 🔎
CREATE TABLE tarifas_iva (
  id             uuid PRIMARY KEY,
  codigo_sri     text NOT NULL,
  porcentaje     numeric(5,2) NOT NULL CHECK (porcentaje >= 0),
  descripcion    text NOT NULL,
  vigente_desde  date NOT NULL,
  vigente_hasta  date,
  CHECK (vigente_hasta IS NULL OR vigente_hasta >= vigente_desde)
);
INSERT INTO tarifas_iva (id, codigo_sri, porcentaje, descripcion, vigente_desde, vigente_hasta) VALUES
  ('0192a000-0000-7000-8000-00000000a001', '4', 15, 'IVA 15 %',                '2024-04-01', NULL),
  ('0192a000-0000-7000-8000-00000000a002', '0',  0, 'IVA 0 %',                 '2000-01-01', NULL),
  ('0192a000-0000-7000-8000-00000000a003', '5',  5, 'IVA 5 %',                 '2024-04-01', NULL),
  ('0192a000-0000-7000-8000-00000000a004', '6',  0, 'No objeto de impuesto',   '2000-01-01', NULL),
  ('0192a000-0000-7000-8000-00000000a005', '7',  0, 'Exento de IVA',           '2000-01-01', NULL),
  ('0192a000-0000-7000-8000-00000000a006', '2', 12, 'IVA 12 % (histórico)',    '2000-01-01', '2024-03-31');
GRANT SELECT ON tarifas_iva TO restpos_app;

CREATE TABLE categorias (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL REFERENCES tenants,
  nombre      text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 40),
  orden       int NOT NULL DEFAULT 0,
  estacion_id uuid,
  color       text NOT NULL DEFAULT 'orange',
  icono       text NOT NULL DEFAULT 'utensils',
  activa      boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  created_by  uuid,
  version     int NOT NULL DEFAULT 1,
  deleted_at  timestamptz,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, estacion_id) REFERENCES estaciones (tenant_id, id)
);
CREATE UNIQUE INDEX categorias_nombre ON categorias (tenant_id, lower(nombre)) WHERE deleted_at IS NULL;

CREATE TABLE productos (
  id                    uuid PRIMARY KEY,
  tenant_id             uuid NOT NULL,
  categoria_id          uuid NOT NULL,
  nombre                text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 80),
  alias                 text CHECK (alias IS NULL OR length(alias) BETWEEN 1 AND 12),
  codigo                text CHECK (codigo IS NULL OR length(codigo) BETWEEN 1 AND 25),
  descripcion           text NOT NULL DEFAULT '' CHECK (length(descripcion) <= 300),
  precio                numeric(18,6) NOT NULL CHECK (precio >= 0),
  tarifa_iva_id         uuid NOT NULL REFERENCES tarifas_iva,
  tipo                  text NOT NULL DEFAULT 'SIMPLE' CHECK (tipo IN ('SIMPLE','RECETA','PREPARACION','INSUMO_VENDIBLE')),
  comportamiento_stock  text NOT NULL DEFAULT 'NINGUNO' CHECK (comportamiento_stock IN ('NINGUNO','PERMANENTE','DIARIO')),
  estacion_id           uuid,
  imagen_key            text,
  activo                boolean NOT NULL DEFAULT true,
  visible_menu_qr       boolean NOT NULL DEFAULT true,
  orden                 int NOT NULL DEFAULT 0,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  created_by            uuid,
  version               int NOT NULL DEFAULT 1,
  deleted_at            timestamptz,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, categoria_id) REFERENCES categorias (tenant_id, id),
  FOREIGN KEY (tenant_id, estacion_id) REFERENCES estaciones (tenant_id, id)
);
CREATE INDEX productos_categoria ON productos (tenant_id, categoria_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX productos_alias ON productos (tenant_id, upper(alias)) WHERE alias IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX productos_codigo ON productos (tenant_id, codigo) WHERE codigo IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE grupos_modificadores (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL REFERENCES tenants,
  nombre      text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 40),
  obligatorio boolean NOT NULL DEFAULT false,
  min         smallint NOT NULL DEFAULT 0,
  max         smallint NOT NULL DEFAULT 1,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  created_by  uuid,
  version     int NOT NULL DEFAULT 1,
  deleted_at  timestamptz,
  UNIQUE (tenant_id, id),
  CHECK (min >= 0 AND max >= 1 AND max >= min AND max <= 20),
  CHECK (NOT obligatorio OR min >= 1)
);

CREATE TABLE modificadores (
  id                uuid PRIMARY KEY,
  tenant_id         uuid NOT NULL,
  grupo_id          uuid NOT NULL,
  nombre            text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 40),
  precio_adicional  numeric(18,6) NOT NULL DEFAULT 0 CHECK (precio_adicional >= 0),
  orden             int NOT NULL DEFAULT 0,
  activo            boolean NOT NULL DEFAULT true,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, grupo_id) REFERENCES grupos_modificadores (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE producto_grupos_modificadores (
  tenant_id    uuid NOT NULL,
  producto_id  uuid NOT NULL,
  grupo_id     uuid NOT NULL,
  orden        int NOT NULL DEFAULT 0,
  PRIMARY KEY (producto_id, grupo_id),
  FOREIGN KEY (tenant_id, producto_id) REFERENCES productos (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, grupo_id) REFERENCES grupos_modificadores (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE notas_rapidas (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  categoria_id  uuid NOT NULL,
  texto         text NOT NULL CHECK (length(texto) BETWEEN 1 AND 30),
  orden         int NOT NULL DEFAULT 0,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, categoria_id) REFERENCES categorias (tenant_id, id) ON DELETE CASCADE
);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['estaciones','zonas','mesas','categorias','productos','grupos_modificadores','modificadores','producto_grupos_modificadores','notas_rapidas'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON %I TO restpos_app', t);
  END LOOP;
  FOREACH t IN ARRAY ARRAY['estaciones','zonas','mesas','categorias','productos','grupos_modificadores'] LOOP
    EXECUTE format('CREATE TRIGGER touch BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION touch_row()', t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE notas_rapidas, producto_grupos_modificadores, modificadores, grupos_modificadores, productos, categorias, tarifas_iva, mesas, zonas, estaciones;
