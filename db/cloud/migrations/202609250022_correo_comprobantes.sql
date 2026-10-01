-- F5-11 · Correo al comprador con el RIDE (PDF) y el XML autorizado. El comprador sale del XML
-- al recibir el comprobante (para la bóveda de F5-12 y para saber a quién escribir); cada
-- intento de envío queda en comprobante_correos (append-only).

-- +goose Up
ALTER TABLE comprobantes ADD COLUMN comprador_identificacion text;
ALTER TABLE comprobantes ADD COLUMN comprador_nombre text;
ALTER TABLE comprobantes ADD COLUMN correo_comprador text;
CREATE INDEX comprobantes_comprador ON comprobantes (tenant_id, comprador_identificacion);

CREATE TABLE comprobante_correos (
  id              uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL,
  comprobante_id  uuid NOT NULL,
  destino         text NOT NULL,
  motivo          text NOT NULL CHECK (motivo IN ('AUTORIZADO', 'REENVIO')),
  ok              boolean NOT NULL,
  error           text,
  solicitado_por  uuid,
  created_at      timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, comprobante_id) REFERENCES comprobantes (tenant_id, id)
);
CREATE INDEX comprobante_correos_comprobante ON comprobante_correos (comprobante_id, created_at DESC);
CREATE TRIGGER comprobante_correos_inmutable BEFORE UPDATE OR DELETE ON comprobante_correos FOR EACH ROW EXECUTE FUNCTION rechazar_modificacion();
CREATE TRIGGER comprobante_correos_sin_truncate BEFORE TRUNCATE ON comprobante_correos FOR EACH STATEMENT EXECUTE FUNCTION rechazar_modificacion();
ALTER TABLE comprobante_correos ENABLE ROW LEVEL SECURITY;
ALTER TABLE comprobante_correos FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON comprobante_correos USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant());
GRANT SELECT, INSERT ON comprobante_correos TO restpos_app;

-- Autorizados con correo del comprador que todavía no se enviaron: hasta 5 intentos
-- automáticos, el siguiente cuando pasen 2^n minutos desde el anterior.
-- +goose StatementBegin
CREATE FUNCTION comprobantes_por_correo(p_limite int)
RETURNS TABLE (tenant_id uuid, comprobante_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT c.tenant_id, c.id FROM comprobantes c
  LEFT JOIN LATERAL (
    SELECT count(*) AS fallos, max(created_at) AS ultimo, bool_or(ok) AS enviado
    FROM comprobante_correos e WHERE e.comprobante_id = c.id AND e.motivo = 'AUTORIZADO'
  ) e ON true
  WHERE c.estado = 'AUTORIZADO' AND c.correo_comprador IS NOT NULL
    AND NOT coalesce(e.enviado, false) AND e.fallos < 5
    AND (e.ultimo IS NULL OR e.ultimo < now() - make_interval(mins => power(2, e.fallos)::int))
  ORDER BY c.fecha_autorizacion LIMIT p_limite
$$;

CREATE FUNCTION comprobante_autorizado_aviso() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('comprobantes_autorizados', NEW.id::text);
  RETURN NULL;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION comprobantes_por_correo(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION comprobantes_por_correo(int) TO restpos_app;
CREATE TRIGGER comprobantes_autorizado_aviso AFTER UPDATE OF estado ON comprobantes FOR EACH ROW
  WHEN (NEW.estado = 'AUTORIZADO' AND OLD.estado IS DISTINCT FROM 'AUTORIZADO') EXECUTE FUNCTION comprobante_autorizado_aviso();

-- +goose Down
DROP TRIGGER comprobantes_autorizado_aviso ON comprobantes;
DROP FUNCTION comprobante_autorizado_aviso();
DROP FUNCTION comprobantes_por_correo(int);
DROP TABLE comprobante_correos;
ALTER TABLE comprobantes DROP COLUMN comprador_identificacion, DROP COLUMN comprador_nombre, DROP COLUMN correo_comprador;
