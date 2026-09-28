-- F2-03 · Feed de cambios Nube → Nodo (docs/03 §3 y §5).
-- Un trigger registra cada cambio de catálogo, salón y personal en sync_cambios con un
-- número de secuencia POR TENANT sin huecos: el contador se incrementa con un UPDATE que
-- bloquea la fila del tenant hasta el commit, así los números se confirman en orden y un
-- nodo nunca se salta un cambio que se confirme tarde. El volumen de cambios del backoffice
-- es bajo, así que serializar por tenant no cuesta nada.

-- +goose Up
CREATE TABLE sync_seq_tenant (
  tenant_id  uuid PRIMARY KEY REFERENCES tenants,
  ultimo     bigint NOT NULL DEFAULT 0 CHECK (ultimo >= 0)
);

CREATE TABLE sync_cambios (
  tenant_id   uuid NOT NULL,
  seq         bigint NOT NULL,
  tabla       text NOT NULL,
  op          char(1) NOT NULL CHECK (op IN ('U','D')),
  local_id    uuid,          -- NULL = aplica a todo el tenant
  datos       jsonb NOT NULL, -- fila completa (sin datos sensibles); en 'D', la fila borrada
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, seq)
);
CREATE INDEX sync_cambios_antiguedad ON sync_cambios (created_at);

ALTER TABLE sync_seq_tenant ENABLE ROW LEVEL SECURITY;
ALTER TABLE sync_seq_tenant FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON sync_seq_tenant USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant());
ALTER TABLE sync_cambios ENABLE ROW LEVEL SECURITY;
ALTER TABLE sync_cambios FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON sync_cambios USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant());
GRANT SELECT, INSERT, UPDATE ON sync_seq_tenant TO restpos_app;
GRANT SELECT, INSERT ON sync_cambios TO restpos_app;

-- +goose StatementBegin
CREATE FUNCTION registrar_cambio() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  fila jsonb;
  t uuid;
  loc uuid;
  n bigint;
BEGIN
  fila := CASE WHEN TG_OP = 'DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END;
  -- El nodo no necesita credenciales web ni datos de contacto (minimización, LOPDP).
  IF TG_TABLE_NAME = 'usuarios' THEN
    fila := fila - 'password_hash' - 'totp_secret_cifrado' - 'email' - 'debe_cambiar_password';
  END IF;
  t := (fila->>'tenant_id')::uuid;
  loc := CASE WHEN TG_TABLE_NAME = 'locales' THEN (fila->>'id')::uuid ELSE (fila->>'local_id')::uuid END;
  INSERT INTO sync_seq_tenant AS s (tenant_id, ultimo) VALUES (t, 1)
    ON CONFLICT (tenant_id) DO UPDATE SET ultimo = s.ultimo + 1
    RETURNING s.ultimo INTO n;
  INSERT INTO sync_cambios (tenant_id, seq, tabla, op, local_id, datos)
    VALUES (t, n, TG_TABLE_NAME, CASE WHEN TG_OP = 'DELETE' THEN 'D' ELSE 'U' END, loc, fila);
  -- Despierta a los nodos del tenant que esperan en long-poll (se entrega al hacer commit).
  PERFORM pg_notify('sync_cambios', t::text);
  RETURN NULL;
END $$;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['locales','estaciones','zonas','mesas','categorias','productos','grupos_modificadores',
    'modificadores','producto_grupos_modificadores','notas_rapidas','usuarios','usuario_locales','permisos_usuario'] LOOP
    EXECUTE format('CREATE TRIGGER sync_cambio AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION registrar_cambio()', t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['locales','estaciones','zonas','mesas','categorias','productos','grupos_modificadores',
    'modificadores','producto_grupos_modificadores','notas_rapidas','usuarios','usuario_locales','permisos_usuario'] LOOP
    EXECUTE format('DROP TRIGGER sync_cambio ON %I', t);
  END LOOP;
END $$;
-- +goose StatementEnd
DROP FUNCTION registrar_cambio();
DROP TABLE sync_cambios, sync_seq_tenant;
