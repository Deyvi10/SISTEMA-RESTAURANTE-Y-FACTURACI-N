-- F2-02 / F2-04 · Nodos locales, códigos de activación, telemetría y recepción de eventos.
-- docs/04 §3 y §9, ADR-0004, ADR-0012.

-- +goose Up
CREATE TABLE nodos (
  id                   uuid PRIMARY KEY,  -- UUID v7 generado por el propio nodo
  tenant_id            uuid NOT NULL,
  local_id             uuid NOT NULL,
  nombre_equipo        text NOT NULL DEFAULT '' CHECK (length(nombre_equipo) <= 80),
  estado               text NOT NULL DEFAULT 'ACTIVO' CHECK (estado IN ('ACTIVO','REVOCADO')),
  -- Llave pública ed25519: la privada se genera en el nodo y nunca sale de él.
  llave_publica        bytea NOT NULL CHECK (length(llave_publica) = 32),
  version_software     text NOT NULL DEFAULT '',
  ultimo_heartbeat_at  timestamptz,
  heartbeat            jsonb NOT NULL DEFAULT '{}',
  activado_at          timestamptz NOT NULL DEFAULT now(),
  activado_por         uuid,
  revocado_at          timestamptz,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id),
  CHECK ((estado = 'REVOCADO') = (revocado_at IS NOT NULL))
);
-- RF-02-10.4: un solo nodo ACTIVO por local (dos nodos duplicarían secuenciales SRI).
CREATE UNIQUE INDEX nodos_activo_por_local ON nodos (local_id) WHERE estado = 'ACTIVO';

-- Código de un solo uso: se guarda solo su hash.
CREATE TABLE codigos_activacion_nodo (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  local_id     uuid NOT NULL,
  codigo_hash  bytea NOT NULL UNIQUE,
  expira_at    timestamptz NOT NULL,
  usado_at     timestamptz,
  nodo_id      uuid,
  creado_por   uuid NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id),
  FOREIGN KEY (tenant_id, creado_por) REFERENCES usuarios (tenant_id, id)
);

-- Recepción de eventos Nodo → Nube: cursor por nodo y bitácora cruda (ADR-0012).
-- tenant_id toma app_tenant() por defecto: el receptor genérico de edgesync inserta sin
-- conocer el tenant y RLS garantiza que cae en el del nodo autenticado.
CREATE TABLE sync_cursores (
  nodo_id     uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL DEFAULT app_tenant(),
  ultimo_seq  bigint NOT NULL DEFAULT 0 CHECK (ultimo_seq >= 0),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, nodo_id) REFERENCES nodos (tenant_id, id)
);
CREATE TABLE sync_eventos (
  evento_id   uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL DEFAULT app_tenant(),
  nodo_id     uuid   NOT NULL,
  node_seq    bigint NOT NULL,
  tipo        text   NOT NULL,
  version     int    NOT NULL,
  agregado_id uuid   NOT NULL,
  payload     jsonb  NOT NULL,
  orden       bigserial,
  recibido_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (nodo_id, node_seq),
  FOREIGN KEY (tenant_id, nodo_id) REFERENCES nodos (tenant_id, id)
);
CREATE INDEX sync_eventos_recibido ON sync_eventos (recibido_at);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['nodos','codigos_activacion_nodo','sync_cursores','sync_eventos'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
  END LOOP;
END $$;
-- +goose StatementEnd
GRANT SELECT, INSERT, UPDATE ON nodos, codigos_activacion_nodo, sync_cursores TO restpos_app;
-- La bitácora de eventos es append-only para la aplicación.
GRANT SELECT, INSERT ON sync_eventos TO restpos_app;
GRANT USAGE ON SEQUENCE sync_eventos_orden_seq TO restpos_app;

-- Búsquedas previas a conocer el tenant (mismo criterio que auth_buscar_*): solo por una
-- clave que quien pregunta ya posee y devolviendo lo mínimo.
-- +goose StatementBegin
CREATE FUNCTION nodo_buscar_codigo(p_hash bytea)
RETURNS TABLE (codigo_id uuid, tenant_id uuid, local_id uuid, expira_at timestamptz, usado_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT id, tenant_id, local_id, expira_at, usado_at FROM codigos_activacion_nodo WHERE codigo_hash = p_hash
$$;

CREATE FUNCTION auth_buscar_nodo(p_id uuid)
RETURNS TABLE (tenant_id uuid, local_id uuid, llave_publica bytea, estado text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT tenant_id, local_id, llave_publica, estado FROM nodos WHERE id = p_id
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION nodo_buscar_codigo(bytea), auth_buscar_nodo(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION nodo_buscar_codigo(bytea), auth_buscar_nodo(uuid) TO restpos_app;

-- +goose Down
DROP FUNCTION auth_buscar_nodo(uuid), nodo_buscar_codigo(bytea);
DROP TABLE sync_eventos, sync_cursores, codigos_activacion_nodo, nodos;
