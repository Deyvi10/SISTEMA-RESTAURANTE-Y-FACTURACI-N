-- F4-12 · Cierre Z en la nube (RF-04-10.5-6): llega del nodo, se guarda inmutable y se envía
-- al dueño en PDF, con alerta crítica si la diferencia supera el umbral del local.

-- +goose Up
-- Umbral de la alerta crítica por local (USD, valor absoluto de la diferencia de un método).
ALTER TABLE locales ADD COLUMN umbral_alerta_cierre numeric(12,2) NOT NULL DEFAULT 5.00 CHECK (umbral_alerta_cierre >= 0);

-- Inmutable: la aplicación solo lee e inserta. hash_valido registra si el hash que calculó el
-- nodo corresponde al contenido recibido.
CREATE TABLE cierres_z (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  local_id       uuid NOT NULL,
  caja_id        uuid NOT NULL,
  turno_id       uuid NOT NULL UNIQUE,
  numero         int NOT NULL CHECK (numero > 0),
  resultado      text NOT NULL CHECK (resultado IN ('CUADRADO','SOBRANTE','FALTANTE')),
  cajero         text NOT NULL,
  fecha_negocio  date NOT NULL,
  cerrado_at     timestamptz NOT NULL,
  datos          jsonb NOT NULL,
  hash           text NOT NULL,
  hash_anterior  text NOT NULL,
  hash_valido    boolean NOT NULL,
  recibido_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (caja_id, numero),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id)
);
CREATE INDEX cierres_z_local ON cierres_z (local_id, cerrado_at DESC);

-- Registro de los correos enviados por cierre (uno por cierre; también inmutable).
CREATE TABLE cierres_z_envios (
  cierre_id      uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  destinatarios  int NOT NULL,
  alerta         boolean NOT NULL,
  enviado_at     timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, cierre_id) REFERENCES cierres_z (tenant_id, id)
);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['cierres_z','cierres_z_envios'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
    EXECUTE format('GRANT SELECT, INSERT ON %I TO restpos_app', t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- El trabajador de correos recorre todos los tenants: solo recibe qué cierre falta enviar y de
-- qué tenant; el contenido lo lee después dentro del tenant, con RLS.
-- +goose StatementBegin
CREATE FUNCTION cierres_z_pendientes(p_limite int)
RETURNS TABLE (tenant_id uuid, cierre_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT c.tenant_id, c.id FROM cierres_z c
  WHERE NOT EXISTS (SELECT 1 FROM cierres_z_envios e WHERE e.cierre_id = c.id)
  ORDER BY c.recibido_at LIMIT p_limite
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION cierres_z_pendientes(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION cierres_z_pendientes(int) TO restpos_app;

-- +goose Down
DROP FUNCTION cierres_z_pendientes(int);
DROP TABLE cierres_z_envios, cierres_z;
ALTER TABLE locales DROP COLUMN umbral_alerta_cierre;
