-- F4-15 · Auditoría inmutable con cadena de hash (RF-08-06, QA-10). Guarda la cadena de cada
-- Nodo Local (verificando contenido y eslabón) y la de la nube por restaurante (acciones del
-- panel web). Inmutable por permisos (la aplicación solo lee e inserta) y por triggers (ni
-- el dueño de la base puede modificar ni borrar).

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION rechazar_modificacion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION '% es append-only: no se permite %', TG_TABLE_NAME, TG_OP USING ERRCODE = 'insufficient_privilege';
END $$;
-- +goose StatementEnd

CREATE TABLE auditoria (
  id              uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL,
  local_id        uuid,
  origen          text NOT NULL CHECK (origen IN ('NODO','NUBE')),
  nodo_id         uuid,                       -- la cadena de origen NODO es por nodo
  seq             bigint NOT NULL CHECK (seq > 0),
  usuario_id      uuid,
  autorizado_por  uuid,
  dispositivo_id  uuid,
  accion          text NOT NULL,
  entidad         text NOT NULL,
  entidad_id      text,
  antes           jsonb,
  despues         jsonb,
  monto           numeric(14,2),
  motivo          text,
  detalle         jsonb,
  created_at      timestamptz NOT NULL,
  hash_anterior   text NOT NULL,
  hash            text NOT NULL,
  datos           text NOT NULL,              -- el registro tal cual se selló, para verificarlo
  hash_valido     boolean NOT NULL,           -- el hash corresponde al contenido
  cadena_valida   boolean NOT NULL,           -- el anterior es el último recibido de ese origen
  recibido_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  CHECK ((origen = 'NODO') = (nodo_id IS NOT NULL))
);
CREATE UNIQUE INDEX auditoria_seq_nodo ON auditoria (nodo_id, seq) WHERE origen = 'NODO';
CREATE UNIQUE INDEX auditoria_seq_nube ON auditoria (tenant_id, seq) WHERE origen = 'NUBE';
CREATE INDEX auditoria_fecha ON auditoria (tenant_id, created_at DESC);
CREATE INDEX auditoria_accion ON auditoria (tenant_id, accion, created_at DESC);

ALTER TABLE auditoria ENABLE ROW LEVEL SECURITY;
ALTER TABLE auditoria FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON auditoria USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant());
GRANT SELECT, INSERT ON auditoria TO restpos_app;

-- QA-10 también por triggers en todas las tablas append-only de la nube.
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['auditoria','sync_eventos','cierres_z','cierres_z_envios'] LOOP
    EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION rechazar_modificacion()', t || '_inmutable', t);
    EXECUTE format('CREATE TRIGGER %I BEFORE TRUNCATE ON %I FOR EACH STATEMENT EXECUTE FUNCTION rechazar_modificacion()', t || '_sin_truncate', t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER sync_eventos_inmutable ON sync_eventos;
DROP TRIGGER sync_eventos_sin_truncate ON sync_eventos;
DROP TRIGGER cierres_z_inmutable ON cierres_z;
DROP TRIGGER cierres_z_sin_truncate ON cierres_z;
DROP TRIGGER cierres_z_envios_inmutable ON cierres_z_envios;
DROP TRIGGER cierres_z_envios_sin_truncate ON cierres_z_envios;
DROP TABLE auditoria;
DROP FUNCTION rechazar_modificacion();
