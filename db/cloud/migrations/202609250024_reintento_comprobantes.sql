-- F5-12 · Reintento manual desde la bóveda. Un comprobante que el SRI DEVOLVIÓ o NO AUTORIZÓ
-- puede volver a firmarse con la firma vigente (p. ej. tras los errores 39/40 de firma) y
-- enviarse con la misma clave y secuencial (ficha §5.10): solo en ese paso se permite borrar
-- el XML firmado. El contenido tributario sigue intacto y lo autorizado no cambia nunca.

-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION comprobante_inmutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF (NEW.id, NEW.tenant_id, NEW.punto_emision_id, NEW.documento_id, NEW.tipo, NEW.ambiente, NEW.serie, NEW.secuencial,
      NEW.clave_acceso, NEW.fecha_emision, NEW.importe_total, NEW.xml, NEW.hash, NEW.hash_valido)
     IS DISTINCT FROM
     (OLD.id, OLD.tenant_id, OLD.punto_emision_id, OLD.documento_id, OLD.tipo, OLD.ambiente, OLD.serie, OLD.secuencial,
      OLD.clave_acceso, OLD.fecha_emision, OLD.importe_total, OLD.xml, OLD.hash, OLD.hash_valido)
     OR (OLD.xml_firmado IS NOT NULL AND NEW.xml_firmado IS DISTINCT FROM OLD.xml_firmado
         AND NOT (NEW.xml_firmado IS NULL AND NEW.estado = 'EN_NUBE' AND OLD.estado IN ('DEVUELTO', 'NO_AUTORIZADO')))
     OR (OLD.numero_autorizacion IS NOT NULL AND NEW.numero_autorizacion IS DISTINCT FROM OLD.numero_autorizacion)
     OR (OLD.estado = 'AUTORIZADO' AND NEW.estado IS DISTINCT FROM 'AUTORIZADO' AND NEW.estado <> 'ANULADO') THEN
    RAISE EXCEPTION 'el contenido tributario de un comprobante no se modifica' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
ALTER TABLE comprobantes DROP CONSTRAINT comprobantes_estado_check;
ALTER TABLE comprobantes ADD CONSTRAINT comprobantes_estado_check
  CHECK (estado IN ('EN_NUBE','FIRMADO','ENVIADO','RECIBIDO','AUTORIZADO','NO_AUTORIZADO','DEVUELTO','REQUIERE_ATENCION','ANULADO'));
CREATE INDEX comprobantes_lista ON comprobantes (tenant_id, recibido_at DESC, id DESC);

-- +goose Down
DROP INDEX comprobantes_lista;
ALTER TABLE comprobantes DROP CONSTRAINT comprobantes_estado_check;
ALTER TABLE comprobantes ADD CONSTRAINT comprobantes_estado_check
  CHECK (estado IN ('EN_NUBE','FIRMADO','ENVIADO','RECIBIDO','AUTORIZADO','NO_AUTORIZADO','DEVUELTO','REQUIERE_ATENCION'));
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
