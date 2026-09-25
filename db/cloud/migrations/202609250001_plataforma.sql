-- F1-02 · Plataforma: planes, tenants, locales, usuarios, permisos y sesiones.
-- Reglas (docs/04 §1, ADR-0008): UUID v7 generados por la app, RLS forzado por tenant,
-- FK compuestas (tenant_id, id) para que ninguna fila apunte a datos de otro tenant.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Rol de la aplicación: no es dueño de nada ni puede saltarse RLS. En producción lo crea
-- la IaC con LOGIN; aquí solo se asegura que exista para poder otorgarle permisos.
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'restpos_app') THEN
    CREATE ROLE restpos_app NOLOGIN NOSUPERUSER NOBYPASSRLS;
  END IF;
END $$;
-- +goose StatementEnd
GRANT USAGE ON SCHEMA public TO restpos_app;

-- Tenant de la transacción actual. NULL si no se fijó: entonces RLS no devuelve filas.
CREATE FUNCTION app_tenant() RETURNS uuid
  LANGUAGE sql STABLE PARALLEL SAFE
  AS $$ SELECT nullif(current_setting('app.tenant_id', true), '')::uuid $$;

-- Mantiene updated_at y version (control optimista y base de la sincronización).
-- +goose StatementBegin
CREATE FUNCTION touch_row() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  NEW.version := OLD.version + 1;
  RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TABLE planes (
  id          uuid PRIMARY KEY,
  codigo      text NOT NULL UNIQUE,
  nombre      text NOT NULL,
  limites     jsonb NOT NULL DEFAULT '{}',
  funciones   text[] NOT NULL DEFAULT '{}',
  created_at  timestamptz NOT NULL DEFAULT now()
);
INSERT INTO planes (id, codigo, nombre, limites, funciones) VALUES
  ('0192a000-0000-7000-8000-000000000001', 'EMPRENDEDOR', 'Emprendedor', '{"cajeros":1,"meseros":1,"dispositivos":2,"impresoras":1,"locales":1}', '{}'),
  ('0192a000-0000-7000-8000-000000000002', 'RESTAURANTE', 'Restaurante', '{"cajeros":null,"meseros":null,"dispositivos":null,"impresoras":null,"locales":1}', '{RECETAS,DIVIDIR_CUENTA,KDS}'),
  ('0192a000-0000-7000-8000-000000000003', 'PRO', 'Pro / Cadena', '{"cajeros":null,"meseros":null,"dispositivos":null,"impresoras":null,"locales":null}', '{RECETAS,DIVIDIR_CUENTA,KDS,MULTI_LOCAL,TRASLADOS,2FA_OBLIGATORIO}');
GRANT SELECT ON planes TO restpos_app;

CREATE TABLE tenants (
  id                  uuid PRIMARY KEY,
  ruc                 char(13) NOT NULL CHECK (ruc ~ '^[0-9]{13}$'),
  razon_social        text NOT NULL CHECK (length(razon_social) BETWEEN 1 AND 300),
  nombre_comercial    text NOT NULL CHECK (length(nombre_comercial) BETWEEN 1 AND 120),
  email_dueno         citext NOT NULL,
  telefono_dueno      text,
  plan_id             uuid NOT NULL REFERENCES planes,
  estado_suscripcion  text NOT NULL DEFAULT 'PRUEBA'
                      CHECK (estado_suscripcion IN ('PRUEBA','ACTIVA','EN_GRACIA','SUSPENDIDA','CANCELADA')),
  config              jsonb NOT NULL DEFAULT '{}',
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  version             int NOT NULL DEFAULT 1
);
-- RF-01-01.5: un solo tenant activo por RUC.
CREATE UNIQUE INDEX tenants_ruc_activo ON tenants (ruc) WHERE estado_suscripcion <> 'CANCELADA';

CREATE TABLE locales (
  id                      uuid PRIMARY KEY,
  tenant_id               uuid NOT NULL REFERENCES tenants,
  nombre                  text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 120),
  direccion               text NOT NULL DEFAULT '',
  codigo_establecimiento  char(3) NOT NULL CHECK (codigo_establecimiento ~ '^[0-9]{3}$' AND codigo_establecimiento <> '000'),
  zona_horaria            text NOT NULL DEFAULT 'America/Guayaquil',
  propina_legal_activa    boolean NOT NULL DEFAULT false,
  propina_porcentaje      numeric(5,2) NOT NULL DEFAULT 10 CHECK (propina_porcentaje BETWEEN 0 AND 100),
  precios_incluyen_iva    boolean NOT NULL DEFAULT true,
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  created_by              uuid,
  version                 int NOT NULL DEFAULT 1,
  deleted_at              timestamptz,
  UNIQUE (tenant_id, id)
);
CREATE UNIQUE INDEX locales_codigo ON locales (tenant_id, codigo_establecimiento) WHERE deleted_at IS NULL;

CREATE TABLE usuarios (
  id                     uuid PRIMARY KEY,
  tenant_id              uuid NOT NULL REFERENCES tenants,
  nombre_mostrar         text NOT NULL CHECK (length(nombre_mostrar) BETWEEN 1 AND 40),
  rol                    text NOT NULL CHECK (rol IN ('ADMIN','CAJERO','MESERO','COCINA','BODEGA')),
  es_dueno               boolean NOT NULL DEFAULT false,
  email                  citext,
  password_hash          text,
  debe_cambiar_password  boolean NOT NULL DEFAULT false,
  pin_hash               text,
  pin_fingerprint        text,
  avatar_key             text,
  activo                 boolean NOT NULL DEFAULT true,
  totp_secret_cifrado    bytea,
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  created_by             uuid,
  version                int NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, id),
  CHECK (rol <> 'ADMIN' OR email IS NOT NULL),
  CHECK (password_hash IS NULL OR email IS NOT NULL),
  CHECK ((pin_hash IS NULL) = (pin_fingerprint IS NULL))
);
-- El correo inicia sesión en cualquier tenant: único entre usuarios activos de toda la plataforma.
CREATE UNIQUE INDEX usuarios_email ON usuarios (email) WHERE email IS NOT NULL AND activo;
-- RF-01-04.2: PIN sin repetir (se aplica a todo el tenant, que es más estricto que por local).
CREATE UNIQUE INDEX usuarios_pin ON usuarios (tenant_id, pin_fingerprint) WHERE pin_fingerprint IS NOT NULL AND activo;
CREATE UNIQUE INDEX usuarios_dueno ON usuarios (tenant_id) WHERE es_dueno;

CREATE TABLE usuario_locales (
  tenant_id   uuid NOT NULL,
  usuario_id  uuid NOT NULL,
  local_id    uuid NOT NULL,
  PRIMARY KEY (usuario_id, local_id),
  FOREIGN KEY (tenant_id, usuario_id) REFERENCES usuarios (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id)
);

-- Permisos en filas y no en columnas: se agregan sin migraciones (docs/04 §3).
CREATE TABLE permisos_usuario (
  tenant_id   uuid NOT NULL,
  usuario_id  uuid NOT NULL,
  permiso     text NOT NULL CHECK (permiso ~ '^[A-Z_]+$'),
  concedido   boolean NOT NULL,
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (usuario_id, permiso),
  FOREIGN KEY (tenant_id, usuario_id) REFERENCES usuarios (tenant_id, id)
);

-- Refresh tokens (solo su hash). «familia» detecta la reutilización de un token rotado.
CREATE TABLE sesiones (
  id              uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL,
  usuario_id      uuid NOT NULL,
  familia         uuid NOT NULL,
  refresh_hash    bytea NOT NULL UNIQUE,
  expira_at       timestamptz NOT NULL,
  revocada_at     timestamptz,
  reemplazada_por uuid,
  user_agent      text,
  ip              inet,
  created_at      timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, usuario_id) REFERENCES usuarios (tenant_id, id)
);
CREATE INDEX sesiones_usuario ON sesiones (usuario_id) WHERE revocada_at IS NULL;

CREATE TABLE tokens_recuperacion (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  usuario_id  uuid NOT NULL,
  token_hash  bytea NOT NULL UNIQUE,
  expira_at   timestamptz NOT NULL,
  usado_at    timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, usuario_id) REFERENCES usuarios (tenant_id, id)
);

-- Intentos de inicio de sesión (RF-01-02.2). Previo a la autenticación: no pertenece a un tenant.
CREATE TABLE intentos_login (
  clave           text PRIMARY KEY, -- hash de (usuario, ip): no guarda correos ni IPs en claro
  fallos          int NOT NULL DEFAULT 0,
  bloqueado_hasta timestamptz,
  updated_at      timestamptz NOT NULL DEFAULT now()
);
GRANT SELECT, INSERT, UPDATE, DELETE ON intentos_login TO restpos_app;

-- RLS: tenants se filtra por id; el resto por tenant_id.
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenants USING (id = app_tenant()) WITH CHECK (id = app_tenant());
-- INSERT permitido: la política WITH CHECK solo deja crear el tenant de la transacción actual.
GRANT SELECT, INSERT, UPDATE ON tenants TO restpos_app;

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['locales','usuarios','usuario_locales','permisos_usuario','sesiones','tokens_recuperacion'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON %I TO restpos_app', t);
  END LOOP;
  FOREACH t IN ARRAY ARRAY['tenants','locales','usuarios'] LOOP
    EXECUTE format('CREATE TRIGGER touch BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION touch_row()', t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- Búsquedas previas a conocer el tenant (login, refresh, recuperación). Son SECURITY DEFINER
-- para cruzar tenants, pero devuelven lo mínimo y solo por una clave que el usuario ya posee.
-- +goose StatementBegin
CREATE FUNCTION auth_buscar_login(p_login text)
RETURNS TABLE (usuario_id uuid, tenant_id uuid, password_hash text, activo boolean, debe_cambiar_password boolean, estado_tenant text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT u.id, u.tenant_id, u.password_hash, u.activo, u.debe_cambiar_password, t.estado_suscripcion
  FROM usuarios u JOIN tenants t ON t.id = u.tenant_id
  WHERE u.password_hash IS NOT NULL AND (
        (p_login ~ '@' AND u.email = p_login::citext AND u.activo)
     OR (p_login ~ '^[0-9]{13}$' AND u.es_dueno AND t.ruc = p_login AND t.estado_suscripcion <> 'CANCELADA'))
  LIMIT 1
$$;

CREATE FUNCTION auth_buscar_sesion(p_hash bytea)
RETURNS TABLE (sesion_id uuid, tenant_id uuid, usuario_id uuid, familia uuid, expira_at timestamptz, revocada_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT id, tenant_id, usuario_id, familia, expira_at, revocada_at FROM sesiones WHERE refresh_hash = p_hash
$$;

CREATE FUNCTION auth_buscar_token_recuperacion(p_hash bytea)
RETURNS TABLE (token_id uuid, tenant_id uuid, usuario_id uuid, expira_at timestamptz, usado_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT id, tenant_id, usuario_id, expira_at, usado_at FROM tokens_recuperacion WHERE token_hash = p_hash
$$;

CREATE FUNCTION auth_buscar_por_email(p_email text)
RETURNS TABLE (usuario_id uuid, tenant_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT id, tenant_id FROM usuarios WHERE email = p_email::citext AND activo AND password_hash IS NOT NULL LIMIT 1
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION auth_buscar_login(text), auth_buscar_sesion(bytea), auth_buscar_token_recuperacion(bytea), auth_buscar_por_email(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth_buscar_login(text), auth_buscar_sesion(bytea), auth_buscar_token_recuperacion(bytea), auth_buscar_por_email(text) TO restpos_app;

-- +goose Down
DROP FUNCTION auth_buscar_por_email(text), auth_buscar_token_recuperacion(bytea), auth_buscar_sesion(bytea), auth_buscar_login(text);
DROP TABLE intentos_login, tokens_recuperacion, sesiones, permisos_usuario, usuario_locales, usuarios, locales, tenants, planes;
DROP FUNCTION touch_row(), app_tenant();
