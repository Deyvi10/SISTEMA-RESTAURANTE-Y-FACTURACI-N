-- F5-11 · El XML firmado vive en PostgreSQL 90 días; después queda solo en el archivo inmutable
-- (se lee por rango). Solo se borra de la base si está archivado con su SHA-256 verificado.

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
         AND NOT (NEW.xml_firmado IS NULL AND NEW.estado = 'EN_NUBE' AND OLD.estado IN ('DEVUELTO', 'NO_AUTORIZADO'))
         -- Purga a los 90 días: solo si ya está en el archivo inmutable.
         AND NOT (NEW.xml_firmado IS NULL AND OLD.estado IN ('AUTORIZADO', 'ANULADO') AND NEW.estado = OLD.estado
                  AND EXISTS (SELECT 1 FROM archivo_comprobantes a WHERE a.comprobante_id = OLD.id)))
     OR (OLD.numero_autorizacion IS NOT NULL AND NEW.numero_autorizacion IS DISTINCT FROM OLD.numero_autorizacion)
     OR (OLD.estado = 'AUTORIZADO' AND NEW.estado IS DISTINCT FROM 'AUTORIZADO' AND NEW.estado <> 'ANULADO') THEN
    RAISE EXCEPTION 'el contenido tributario de un comprobante no se modifica' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd

-- +goose Down
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
