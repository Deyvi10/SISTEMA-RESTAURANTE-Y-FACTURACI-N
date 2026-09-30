-- F5-07 · Certificado de firma electrónica (.p12) con cifrado de sobre (docs/06 §4) y cola del
-- worker fiscal (ADR-0007: la propia tabla de comprobantes es la cola, con SKIP LOCKED).
--
-- El .p12 y su contraseña se guardan cifrados con una DEK propia (AES-256-GCM); la DEK, cifrada
-- con la KEK. La API solo puede cifrar (llave pública); el worker fiscal descifra en memoria.
-- Cada descifrado queda en certificados_accesos (append-only).

-- +goose Up
CREATE TABLE certificados_firma (
  id                uuid PRIMARY KEY,
  tenant_id         uuid NOT NULL,
  p12_cifrado       bytea NOT NULL,
  password_cifrada  bytea NOT NULL,
  dek_cifrada       bytea NOT NULL,
  kek_id            text NOT NULL,
  titular           text NOT NULL,
  ruc               char(13),
  emisor            text NOT NULL,
  serial            text NOT NULL,
  valido_desde      timestamptz NOT NULL,
  valido_hasta      timestamptz NOT NULL,
  activo            boolean NOT NULL DEFAULT true,
  subido_por        uuid,
  created_at        timestamptz NOT NULL DEFAULT now(),
  reemplazado_at    timestamptz,
  UNIQUE (tenant_id, id)
);
-- Un solo certificado activo por restaurante; los anteriores quedan inactivos (no se borran).
CREATE UNIQUE INDEX certificados_firma_activo ON certificados_firma (tenant_id) WHERE activo;

-- Lo cifrado no cambia nunca: re-subir crea otro certificado (DEK nueva) y desactiva el anterior.
-- +goose StatementBegin
CREATE FUNCTION certificado_inmutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF (NEW.id, NEW.tenant_id, NEW.p12_cifrado, NEW.password_cifrada, NEW.dek_cifrada, NEW.kek_id, NEW.serial, NEW.valido_desde, NEW.valido_hasta)
     IS DISTINCT FROM
     (OLD.id, OLD.tenant_id, OLD.p12_cifrado, OLD.password_cifrada, OLD.dek_cifrada, OLD.kek_id, OLD.serial, OLD.valido_desde, OLD.valido_hasta)
     OR (OLD.activo = false AND NEW.activo = true) THEN
    RAISE EXCEPTION 'un certificado guardado no se modifica ni se reactiva' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER certificados_firma_contenido BEFORE UPDATE ON certificados_firma FOR EACH ROW EXECUTE FUNCTION certificado_inmutable();
CREATE TRIGGER certificados_firma_sin_borrar BEFORE DELETE ON certificados_firma FOR EACH ROW EXECUTE FUNCTION rechazar_modificacion();
CREATE TRIGGER certificados_firma_sin_truncate BEFORE TRUNCATE ON certificados_firma FOR EACH STATEMENT EXECUTE FUNCTION rechazar_modificacion();

CREATE TABLE certificados_accesos (
  id               uuid PRIMARY KEY,
  tenant_id        uuid NOT NULL,
  certificado_id   uuid NOT NULL,
  proceso          text NOT NULL,
  motivo           text NOT NULL,
  comprobante_id   uuid,
  ok               boolean NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, certificado_id) REFERENCES certificados_firma (tenant_id, id)
);
CREATE INDEX certificados_accesos_cert ON certificados_accesos (certificado_id, created_at DESC);
CREATE TRIGGER certificados_accesos_inmutable BEFORE UPDATE OR DELETE ON certificados_accesos FOR EACH ROW EXECUTE FUNCTION rechazar_modificacion();
CREATE TRIGGER certificados_accesos_sin_truncate BEFORE TRUNCATE ON certificados_accesos FOR EACH STATEMENT EXECUTE FUNCTION rechazar_modificacion();

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['certificados_firma','certificados_accesos'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
  END LOOP;
END $$;
-- +goose StatementEnd
GRANT SELECT, INSERT ON certificados_firma TO restpos_app;
GRANT UPDATE (activo, reemplazado_at) ON certificados_firma TO restpos_app;
GRANT SELECT, INSERT ON certificados_accesos TO restpos_app;

-- El XML firmado se guarda una vez: los reintentos envían exactamente el mismo documento.
ALTER TABLE comprobantes ADD COLUMN xml_firmado text;
ALTER TABLE comprobantes ADD COLUMN ultimo_error text;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION comprobante_inmutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF (NEW.id, NEW.tenant_id, NEW.punto_emision_id, NEW.documento_id, NEW.tipo, NEW.ambiente, NEW.serie, NEW.secuencial,
      NEW.clave_acceso, NEW.fecha_emision, NEW.importe_total, NEW.xml, NEW.hash, NEW.hash_valido)
     IS DISTINCT FROM
     (OLD.id, OLD.tenant_id, OLD.punto_emision_id, OLD.documento_id, OLD.tipo, OLD.ambiente, OLD.serie, OLD.secuencial,
      OLD.clave_acceso, OLD.fecha_emision, OLD.importe_total, OLD.xml, OLD.hash, OLD.hash_valido)
     OR (OLD.xml_firmado IS NOT NULL AND NEW.xml_firmado IS DISTINCT FROM OLD.xml_firmado)
     OR (OLD.numero_autorizacion IS NOT NULL AND NEW.numero_autorizacion IS DISTINCT FROM OLD.numero_autorizacion) THEN
    RAISE EXCEPTION 'el contenido tributario de un comprobante no se modifica' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd

-- Cola del worker fiscal: comprobantes listos para su siguiente paso, de todos los restaurantes.
-- Solo devuelve IDs; el trabajo se hace dentro del tenant (RLS) con FOR UPDATE SKIP LOCKED.
-- +goose StatementBegin
CREATE FUNCTION comprobantes_pendientes(p_limite int)
RETURNS TABLE (tenant_id uuid, comprobante_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT c.tenant_id, c.id FROM comprobantes c
  WHERE c.estado IN ('EN_NUBE', 'FIRMADO', 'RECIBIDO')
    AND (c.proximo_intento_at IS NULL OR c.proximo_intento_at <= now())
  ORDER BY c.proximo_intento_at NULLS FIRST, c.recibido_at LIMIT p_limite
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION comprobantes_pendientes(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION comprobantes_pendientes(int) TO restpos_app;

-- Aviso inmediato al worker cuando llega un comprobante (además de su sondeo periódico).
-- +goose StatementBegin
CREATE FUNCTION comprobante_notificar() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('comprobantes', NEW.id::text);
  RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER comprobantes_aviso AFTER INSERT ON comprobantes FOR EACH ROW EXECUTE FUNCTION comprobante_notificar();

-- +goose Down
DROP TRIGGER comprobantes_aviso ON comprobantes;
DROP FUNCTION comprobante_notificar();
DROP FUNCTION comprobantes_pendientes(int);
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION comprobante_inmutable() RETURNS trigger LANGUAGE plpgsql AS $$
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
ALTER TABLE comprobantes DROP COLUMN xml_firmado, DROP COLUMN ultimo_error;
DROP TABLE certificados_accesos, certificados_firma;
DROP FUNCTION certificado_inmutable();
