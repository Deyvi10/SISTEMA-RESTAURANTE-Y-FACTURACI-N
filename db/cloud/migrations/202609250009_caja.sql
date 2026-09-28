-- F4-03 / F4-06 / F4-09 · Configuración de caja que decide el dueño en la nube y usa el
-- nodo sin internet (docs/03 §3): cajas del local, métodos de pago mapeados al SRI y motivos
-- de descuento. La operación (jornadas, turnos, cobros, cierres) vive en el nodo.

-- +goose Up
-- UUID v7 para las filas que siembra la base (la regla: nunca v4 ni autoincrementales).
-- Toma un v4, le pone los 48 bits del reloj en milisegundos y la versión 7 (bits 52 y 53).
-- +goose StatementBegin
CREATE FUNCTION uuid_v7() RETURNS uuid LANGUAGE sql VOLATILE AS $$
  SELECT encode(set_bit(set_bit(overlay(uuid_send(gen_random_uuid())
    PLACING substring(int8send((extract(epoch FROM clock_timestamp()) * 1000)::bigint) FROM 3) FROM 1 FOR 6), 52, 1), 53, 1), 'hex')::uuid
$$;
-- +goose StatementEnd

-- Parámetros legales globales (no por tenant): llegan a los nodos en el volcado diario.
CREATE TABLE parametros_globales (
  clave        text PRIMARY KEY CHECK (clave ~ '^[a-z_]+$'),
  valor        text NOT NULL,
  descripcion  text NOT NULL,
  updated_at   timestamptz NOT NULL DEFAULT now()
);
GRANT SELECT ON parametros_globales TO restpos_app;
-- USD 50 según la revisión del documento fuente (L-05). Valor provisional hasta la revisión
-- del tributarista (DP-07): se cambia aquí sin tocar código.
INSERT INTO parametros_globales (clave, valor, descripcion) VALUES
  ('consumidor_final_maximo', '50.00', 'Importe total máximo de una factura a consumidor final (USD). Pendiente de validar, DP-07.');

-- Cada caja cobra con su propio turno e imprime en su estación de tipo CAJA (cajón incluido).
-- En F5 se asocia a un punto de emisión del SRI.
CREATE TABLE cajas (
  id                uuid PRIMARY KEY,
  tenant_id         uuid NOT NULL,
  local_id          uuid NOT NULL,
  nombre            text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 40),
  estacion_id       uuid,
  punto_emision_id  uuid,
  activa            boolean NOT NULL DEFAULT true,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  created_by        uuid,
  version           int NOT NULL DEFAULT 1,
  deleted_at        timestamptz,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id),
  FOREIGN KEY (tenant_id, estacion_id) REFERENCES estaciones (tenant_id, id)
);
CREATE UNIQUE INDEX cajas_nombre ON cajas (local_id, lower(nombre)) WHERE deleted_at IS NULL;

-- Métodos de pago del restaurante con su código de forma de pago del SRI (docs/05 §7).
CREATE TABLE metodos_pago (
  id                     uuid PRIMARY KEY,
  tenant_id              uuid NOT NULL,
  nombre                 text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 30),
  tipo                   text NOT NULL CHECK (tipo IN ('EFECTIVO','TARJETA_CREDITO','TARJETA_DEBITO','TRANSFERENCIA','BILLETERA','OTRO')),
  codigo_forma_pago_sri  char(2) NOT NULL CHECK (codigo_forma_pago_sri IN ('01','15','16','17','18','19','20','21')),
  abre_cajon             boolean NOT NULL DEFAULT false,
  pide_referencia        boolean NOT NULL DEFAULT false, -- lote, referencia y últimos 4 (opcionales)
  icono                  text NOT NULL DEFAULT 'caja',
  orden                  int NOT NULL DEFAULT 0,
  activo                 boolean NOT NULL DEFAULT true,
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  version                int NOT NULL DEFAULT 1,
  deleted_at             timestamptz,
  UNIQUE (tenant_id, id),
  -- El efectivo es la única forma «sin sistema financiero» y la que abre el cajón.
  CHECK ((tipo = 'EFECTIVO') = (codigo_forma_pago_sri = '01'))
);
CREATE UNIQUE INDEX metodos_pago_nombre ON metodos_pago (tenant_id, lower(nombre)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX metodos_pago_efectivo ON metodos_pago (tenant_id) WHERE tipo = 'EFECTIVO' AND deleted_at IS NULL;

-- Motivos que el cajero elige al descontar o regalar (RF-04-07).
CREATE TABLE motivos_descuento (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  nombre      text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 40),
  tipo        text NOT NULL DEFAULT 'DESCUENTO' CHECK (tipo IN ('DESCUENTO','CORTESIA')),
  orden       int NOT NULL DEFAULT 0,
  activo      boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  version     int NOT NULL DEFAULT 1,
  deleted_at  timestamptz,
  UNIQUE (tenant_id, id)
);
CREATE UNIQUE INDEX motivos_descuento_nombre ON motivos_descuento (tenant_id, tipo, lower(nombre)) WHERE deleted_at IS NULL;

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['cajas','metodos_pago','motivos_descuento'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
    EXECUTE format('GRANT SELECT, INSERT, UPDATE ON %I TO restpos_app', t);
    EXECUTE format('CREATE TRIGGER sync_cambio AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION registrar_cambio()', t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- Valores iniciales para los restaurantes que ya existen (los nuevos los reciben al darse de alta).
-- +goose StatementBegin
CREATE FUNCTION sembrar_caja(t uuid) RETURNS void LANGUAGE sql AS $$
  INSERT INTO metodos_pago (id, tenant_id, nombre, tipo, codigo_forma_pago_sri, abre_cajon, pide_referencia, icono, orden) VALUES
    (uuid_v7(), t, 'Efectivo',       'EFECTIVO',        '01', true,  false, 'efectivo',      1),
    (uuid_v7(), t, 'Tarjeta crédito', 'TARJETA_CREDITO', '19', false, true,  'tarjeta',       2),
    (uuid_v7(), t, 'Tarjeta débito',  'TARJETA_DEBITO',  '16', false, true,  'tarjeta',       3),
    (uuid_v7(), t, 'Transferencia',   'TRANSFERENCIA',   '20', false, true,  'transferencia', 4),
    (uuid_v7(), t, 'DeUna',           'BILLETERA',       '20', false, true,  'billetera',     5);
  INSERT INTO motivos_descuento (id, tenant_id, nombre, tipo, orden) VALUES
    (uuid_v7(), t, 'Cliente frecuente', 'DESCUENTO', 1),
    (uuid_v7(), t, 'Promoción',         'DESCUENTO', 2),
    (uuid_v7(), t, 'Demora en el servicio', 'DESCUENTO', 3),
    (uuid_v7(), t, 'Plato con problema', 'CORTESIA', 4),
    (uuid_v7(), t, 'Invitación de la casa', 'CORTESIA', 5);
  INSERT INTO cajas (id, tenant_id, local_id, nombre, estacion_id)
    SELECT uuid_v7(), t, l.id, 'Caja 1',
      (SELECT e.id FROM estaciones e WHERE e.local_id = l.id AND e.tipo = 'CAJA' AND e.deleted_at IS NULL ORDER BY e.orden LIMIT 1)
    FROM locales l WHERE l.tenant_id = t AND l.deleted_at IS NULL;
$$;
-- +goose StatementEnd
-- La migración corre como dueño: sembrar cada tenant con su RLS activo.
-- +goose StatementBegin
DO $$
DECLARE t uuid;
BEGIN
  FOR t IN SELECT id FROM tenants LOOP
    PERFORM set_config('app.tenant_id', t::text, true);
    PERFORM sembrar_caja(t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION sembrar_caja(uuid);
DROP FUNCTION uuid_v7();
DROP TABLE motivos_descuento, metodos_pago, cajas, parametros_globales;
