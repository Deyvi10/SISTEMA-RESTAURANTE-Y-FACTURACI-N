-- F5-13 · Notas de crédito en la nube. Cada NC apunta a su factura de sustento; el worker
-- fiscal no la envía al SRI hasta que la factura esté AUTORIZADA (si la factura aún no se
-- autorizó, la NC espera). Cuando se autoriza la NC que revierte todo lo que quedaba, la
-- factura pasa a ANULADO («Anulado por NC» en la bóveda).

-- +goose Up
ALTER TABLE comprobantes ADD COLUMN sustento_id uuid;
ALTER TABLE comprobantes ADD COLUMN revierte_todo boolean NOT NULL DEFAULT false;
ALTER TABLE comprobantes ADD CONSTRAINT comprobantes_sustento FOREIGN KEY (tenant_id, sustento_id) REFERENCES comprobantes (tenant_id, id);
ALTER TABLE comprobantes ADD CONSTRAINT comprobantes_nc_con_sustento CHECK ((tipo = '04') = (sustento_id IS NOT NULL));
CREATE INDEX comprobantes_por_sustento ON comprobantes (sustento_id) WHERE sustento_id IS NOT NULL;
-- La NC no tiene documento de venta propio.
ALTER TABLE comprobantes ALTER COLUMN documento_id DROP NOT NULL;
ALTER TABLE comprobantes ADD CONSTRAINT comprobantes_factura_con_documento CHECK ((tipo = '01') = (documento_id IS NOT NULL));

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION comprobantes_pendientes(p_limite int)
RETURNS TABLE (tenant_id uuid, comprobante_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT c.tenant_id, c.id FROM comprobantes c
  WHERE c.estado IN ('EN_NUBE', 'FIRMADO', 'RECIBIDO')
    AND (c.proximo_intento_at IS NULL OR c.proximo_intento_at <= now())
    AND (c.sustento_id IS NULL OR EXISTS (SELECT 1 FROM comprobantes s WHERE s.id = c.sustento_id AND s.estado IN ('AUTORIZADO', 'ANULADO')))
  ORDER BY c.proximo_intento_at NULLS FIRST, c.recibido_at LIMIT p_limite
$$;

-- Una factura anulada por NC también se archiva (su XML autorizado se conserva igual).
CREATE OR REPLACE FUNCTION dias_por_archivar(p_hasta date)
RETURNS TABLE (tenant_id uuid, fecha date)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT DISTINCT c.tenant_id, (c.fecha_autorizacion AT TIME ZONE 'America/Guayaquil')::date
  FROM comprobantes c
  WHERE c.estado IN ('AUTORIZADO', 'ANULADO') AND c.fecha_autorizacion IS NOT NULL
    AND (c.fecha_autorizacion AT TIME ZONE 'America/Guayaquil')::date < p_hasta
    AND NOT EXISTS (SELECT 1 FROM archivo_comprobantes a WHERE a.comprobante_id = c.id)
  ORDER BY 2, 1
$$;

-- La NC que esperaba sale en cuanto su factura se autoriza.
CREATE OR REPLACE FUNCTION comprobante_autorizado_aviso() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('comprobantes_autorizados', NEW.id::text);
  IF EXISTS (SELECT 1 FROM comprobantes n WHERE n.sustento_id = NEW.id AND n.estado = 'EN_NUBE') THEN
    PERFORM pg_notify('comprobantes', NEW.id::text);
  END IF;
  RETURN NULL;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION dias_por_archivar(p_hasta date)
RETURNS TABLE (tenant_id uuid, fecha date)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT DISTINCT c.tenant_id, (c.fecha_autorizacion AT TIME ZONE 'America/Guayaquil')::date
  FROM comprobantes c
  WHERE c.estado = 'AUTORIZADO' AND c.fecha_autorizacion IS NOT NULL
    AND (c.fecha_autorizacion AT TIME ZONE 'America/Guayaquil')::date < p_hasta
    AND NOT EXISTS (SELECT 1 FROM archivo_comprobantes a WHERE a.comprobante_id = c.id)
  ORDER BY 2, 1
$$;
CREATE OR REPLACE FUNCTION comprobante_autorizado_aviso() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('comprobantes_autorizados', NEW.id::text);
  RETURN NULL;
END $$;
CREATE OR REPLACE FUNCTION comprobantes_pendientes(p_limite int)
RETURNS TABLE (tenant_id uuid, comprobante_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT c.tenant_id, c.id FROM comprobantes c
  WHERE c.estado IN ('EN_NUBE', 'FIRMADO', 'RECIBIDO')
    AND (c.proximo_intento_at IS NULL OR c.proximo_intento_at <= now())
  ORDER BY c.proximo_intento_at NULLS FIRST, c.recibido_at LIMIT p_limite
$$;
-- +goose StatementEnd
ALTER TABLE comprobantes DROP CONSTRAINT comprobantes_factura_con_documento;
ALTER TABLE comprobantes ALTER COLUMN documento_id SET NOT NULL;
DROP INDEX comprobantes_por_sustento;
ALTER TABLE comprobantes DROP CONSTRAINT comprobantes_nc_con_sustento, DROP CONSTRAINT comprobantes_sustento;
ALTER TABLE comprobantes DROP COLUMN sustento_id, DROP COLUMN revierte_todo;
