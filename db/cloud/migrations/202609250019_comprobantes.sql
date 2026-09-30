-- F5-05 / F5-08 · Comprobantes emitidos por los nodos (docs/04 §7). El nodo manda el XML sin
-- firmar con su hash; la nube lo verifica, lo firma (el .p12 no sale de aquí) y lo envía al
-- SRI. El contenido tributario no se modifica nunca; solo avanza el estado, y cada transición
-- queda en comprobante_eventos (append-only).

-- +goose Up
CREATE TABLE comprobantes (
  id                   uuid PRIMARY KEY,
  tenant_id            uuid NOT NULL,
  local_id             uuid NOT NULL,
  nodo_id              uuid NOT NULL,
  punto_emision_id     uuid NOT NULL,
  documento_id         uuid NOT NULL,
  tipo                 char(2) NOT NULL CHECK (tipo IN ('01', '04')),
  ambiente             smallint NOT NULL CHECK (ambiente IN (1, 2)),
  serie                char(6) NOT NULL CHECK (serie ~ '^[0-9]{6}$'),
  secuencial           int NOT NULL CHECK (secuencial BETWEEN 1 AND 999999999),
  clave_acceso         char(49) NOT NULL UNIQUE CHECK (clave_acceso ~ '^[0-9]{49}$'),
  fecha_emision        date NOT NULL,
  importe_total        numeric(12,2) NOT NULL CHECK (importe_total >= 0),
  xml                  text NOT NULL,
  hash                 char(64) NOT NULL,
  -- El hash llegó igual al del XML: si no, no se firma ni se envía (requiere atención).
  hash_valido          boolean NOT NULL,
  estado               text NOT NULL CHECK (estado IN ('EN_NUBE','FIRMADO','ENVIADO','RECIBIDO','AUTORIZADO','NO_AUTORIZADO','DEVUELTO','REQUIERE_ATENCION')),
  intentos             int NOT NULL DEFAULT 0,
  proximo_intento_at   timestamptz,
  mensajes_sri         jsonb NOT NULL DEFAULT '[]',
  numero_autorizacion  char(49),
  fecha_autorizacion   timestamptz,
  xml_autorizado_key   text,
  recibido_at          timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (punto_emision_id, tipo, ambiente, secuencial)
);
CREATE INDEX comprobantes_pendientes ON comprobantes (proximo_intento_at) WHERE estado NOT IN ('AUTORIZADO', 'NO_AUTORIZADO', 'DEVUELTO', 'REQUIERE_ATENCION');
CREATE INDEX comprobantes_fecha ON comprobantes (tenant_id, fecha_emision DESC);

-- El contenido tributario es inmutable: solo cambian el estado y lo que devuelve el SRI.
-- +goose StatementBegin
CREATE FUNCTION comprobante_inmutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF (NEW.id, NEW.tenant_id, NEW.punto_emision_id, NEW.documento_id, NEW.tipo, NEW.ambiente, NEW.serie, NEW.secuencial,
      NEW.clave_acceso, NEW.fecha_emision, NEW.importe_total, NEW.xml, NEW.hash, NEW.hash_valido)
     IS DISTINCT FROM
     (OLD.id, OLD.tenant_id, OLD.punto_emision_id, OLD.documento_id, OLD.tipo, OLD.ambiente, OLD.serie, OLD.secuencial,
      OLD.clave_acceso, OLD.fecha_emision, OLD.importe_total, OLD.xml, OLD.hash, OLD.hash_valido) THEN
    RAISE EXCEPTION 'el contenido tributario de un comprobante no se modifica' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER comprobantes_contenido BEFORE UPDATE ON comprobantes FOR EACH ROW EXECUTE FUNCTION comprobante_inmutable();
CREATE TRIGGER comprobantes_sin_borrar BEFORE DELETE ON comprobantes FOR EACH ROW EXECUTE FUNCTION rechazar_modificacion();
CREATE TRIGGER comprobantes_sin_truncate BEFORE TRUNCATE ON comprobantes FOR EACH STATEMENT EXECUTE FUNCTION rechazar_modificacion();

CREATE TABLE comprobante_eventos (
  id              uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL,
  comprobante_id  uuid NOT NULL,
  estado          text NOT NULL,
  detalle         jsonb NOT NULL DEFAULT '{}',
  created_at      timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, comprobante_id) REFERENCES comprobantes (tenant_id, id)
);
CREATE INDEX comprobante_eventos_comprobante ON comprobante_eventos (comprobante_id, created_at);
CREATE TRIGGER comprobante_eventos_inmutable BEFORE UPDATE OR DELETE ON comprobante_eventos FOR EACH ROW EXECUTE FUNCTION rechazar_modificacion();
CREATE TRIGGER comprobante_eventos_sin_truncate BEFORE TRUNCATE ON comprobante_eventos FOR EACH STATEMENT EXECUTE FUNCTION rechazar_modificacion();

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['comprobantes','comprobante_eventos'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
  END LOOP;
END $$;
-- +goose StatementEnd
GRANT SELECT, INSERT, UPDATE ON comprobantes TO restpos_app;
GRANT SELECT, INSERT ON comprobante_eventos TO restpos_app;

-- +goose Down
DROP TABLE comprobante_eventos, comprobantes;
DROP FUNCTION comprobante_inmutable();
