-- F5-16 · Reportes básicos y alertas fiscales. Las ventas que cobra el nodo (venta.cobrada) se
-- proyectan en «ventas» (append-only) con su fecha de negocio (jornada), base de los reportes;
-- las que ya llegaron se cargan desde sync_eventos. Las alertas por correo se envían una vez
-- por clave y día (alertas_enviadas). El plazo legal de envío depende de DP-07: parámetro vacío.

-- +goose Up
CREATE TABLE ventas (
  documento_id   uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  local_id       uuid NOT NULL,
  fecha_negocio  date NOT NULL,
  emitido_at     timestamptz NOT NULL,
  tipo           text NOT NULL,
  codigo         text NOT NULL,
  mesa           text,
  cajero         text,
  comprador      text,
  subtotal       numeric(14,2) NOT NULL,
  iva            numeric(14,2) NOT NULL,
  propina        numeric(14,2) NOT NULL,
  descuento      numeric(14,2) NOT NULL,
  total          numeric(14,2) NOT NULL,
  pagos          jsonb NOT NULL DEFAULT '[]'
);
CREATE INDEX ventas_fecha ON ventas (tenant_id, fecha_negocio);
CREATE TRIGGER ventas_inmutable BEFORE UPDATE OR DELETE ON ventas FOR EACH ROW EXECUTE FUNCTION rechazar_modificacion();
CREATE TRIGGER ventas_sin_truncate BEFORE TRUNCATE ON ventas FOR EACH STATEMENT EXECUTE FUNCTION rechazar_modificacion();

CREATE TABLE alertas_enviadas (
  tenant_id   uuid NOT NULL,
  clave       text NOT NULL,
  dia         date NOT NULL,
  detalle     text NOT NULL,
  enviado_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, clave, dia)
);
CREATE TRIGGER alertas_enviadas_inmutable BEFORE UPDATE OR DELETE ON alertas_enviadas FOR EACH ROW EXECUTE FUNCTION rechazar_modificacion();

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['ventas','alertas_enviadas'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
    EXECUTE format('GRANT SELECT, INSERT ON %I TO restpos_app', t);
  END LOOP;
END $$;

-- Restaurantes con algo que alertar (para el notificador, que luego trabaja dentro del tenant).
CREATE FUNCTION tenants_con_facturacion()
RETURNS TABLE (tenant_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT tenant_id FROM configuracion_fiscal
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION tenants_con_facturacion() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION tenants_con_facturacion() TO restpos_app;

-- Ventas que ya llegaron antes de esta migración.
INSERT INTO ventas (documento_id, tenant_id, local_id, fecha_negocio, emitido_at, tipo, codigo, mesa, cajero, comprador,
    subtotal, iva, propina, descuento, total, pagos)
SELECT (e.payload->'documento'->>'id')::uuid, e.tenant_id, n.local_id,
       coalesce((e.payload->'documento'->>'fechaNegocio')::date, ((e.payload->'documento'->>'emitidoAt')::timestamptz AT TIME ZONE 'America/Guayaquil')::date),
       (e.payload->'documento'->>'emitidoAt')::timestamptz, e.payload->'documento'->>'tipo', e.payload->'documento'->>'codigo',
       e.payload->'documento'->>'mesa', e.payload->'documento'->>'cajero', e.payload->'documento'->>'comprador',
       coalesce((e.payload->'documento'->'totales'->>'subtotal')::numeric, 0), coalesce((e.payload->'documento'->'totales'->>'iva')::numeric, 0),
       coalesce((e.payload->'documento'->'totales'->>'propina')::numeric, 0), coalesce(nullif(e.payload->'documento'->'totales'->>'descuento', '')::numeric, 0),
       coalesce((e.payload->'documento'->'totales'->>'total')::numeric, 0),
       -- Antes del pago mixto (F4-06) había un solo método por el total.
       CASE WHEN jsonb_array_length(coalesce(e.payload->'documento'->'pagos', '[]')) > 0 THEN e.payload->'documento'->'pagos'
            WHEN e.payload->'documento'->>'metodo' IS NOT NULL
              THEN jsonb_build_array(jsonb_build_object('metodo', e.payload->'documento'->>'metodo', 'monto', e.payload->'documento'->'totales'->>'total'))
            ELSE '[]' END
FROM sync_eventos e JOIN nodos n ON n.id = e.nodo_id
WHERE e.tipo = 'venta.cobrada' AND e.payload->'documento'->>'id' IS NOT NULL
ON CONFLICT DO NOTHING;

INSERT INTO parametros_globales (clave, valor, descripcion) VALUES
  ('plazo_envio_comprobante_horas', '', 'Plazo legal para enviar un comprobante al SRI (horas). Pendiente de DP-07: vacío = sin alerta crítica.')
ON CONFLICT (clave) DO NOTHING;
ALTER TABLE configuracion_fiscal ADD COLUMN alerta_sin_autorizar_horas int NOT NULL DEFAULT 12 CHECK (alerta_sin_autorizar_horas BETWEEN 1 AND 72);

-- +goose Down
ALTER TABLE configuracion_fiscal DROP COLUMN alerta_sin_autorizar_horas;
DELETE FROM parametros_globales WHERE clave = 'plazo_envio_comprobante_horas';
DROP FUNCTION tenants_con_facturacion();
DROP TABLE alertas_enviadas;
DROP TABLE ventas;
