-- F5-11 · Archivo inmutable de los XML autorizados. Cada noche los autorizados de un día se
-- comprimen (un frame zstd por factura, con el diccionario del emisor si ya tiene uno) en un
-- solo blob por emisor y día en el contenedor «comprobantes» (Object Lock / inmutabilidad).
-- El índice dice dónde está cada factura (blob, desplazamiento, largo) y su SHA-256: se lee
-- una sola factura con una petición por rango. Todo es append-only.

-- +goose Up
CREATE TABLE diccionarios_zstd (
  tenant_id    uuid NOT NULL,
  version      int NOT NULL CHECK (version >= 1),
  zstd_id      bigint NOT NULL CHECK (zstd_id BETWEEN 32768 AND 2147483647),
  datos        bytea NOT NULL,
  sha256       char(64) NOT NULL,
  muestras     int NOT NULL,
  blob_key     text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, version)
);

CREATE TABLE archivo_lotes (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  fecha        date NOT NULL,
  blob_key     text NOT NULL UNIQUE,
  cantidad     int NOT NULL CHECK (cantidad > 0),
  bytes        bigint NOT NULL,
  bytes_xml    bigint NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id)
);

CREATE TABLE archivo_comprobantes (
  comprobante_id   uuid PRIMARY KEY,
  tenant_id        uuid NOT NULL,
  lote_id          uuid NOT NULL,
  clave_acceso     char(49) NOT NULL UNIQUE,
  desplazamiento   bigint NOT NULL CHECK (desplazamiento >= 0),
  largo            int NOT NULL CHECK (largo > 0),
  sha256           char(64) NOT NULL,
  diccionario      int,
  created_at       timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, comprobante_id) REFERENCES comprobantes (tenant_id, id),
  FOREIGN KEY (tenant_id, lote_id) REFERENCES archivo_lotes (tenant_id, id),
  FOREIGN KEY (tenant_id, diccionario) REFERENCES diccionarios_zstd (tenant_id, version)
);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['diccionarios_zstd','archivo_lotes','archivo_comprobantes'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
    EXECUTE format('GRANT SELECT, INSERT ON %I TO restpos_app', t);
    EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION rechazar_modificacion()', t || '_inmutable', t);
    EXECUTE format('CREATE TRIGGER %I BEFORE TRUNCATE ON %I FOR EACH STATEMENT EXECUTE FUNCTION rechazar_modificacion()', t || '_sin_truncate', t);
  END LOOP;
END $$;

-- Días por archivar: autorizados sin índice, de días ya cerrados (hora de Ecuador).
CREATE FUNCTION dias_por_archivar(p_hasta date)
RETURNS TABLE (tenant_id uuid, fecha date)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT DISTINCT c.tenant_id, (c.fecha_autorizacion AT TIME ZONE 'America/Guayaquil')::date
  FROM comprobantes c
  WHERE c.estado = 'AUTORIZADO' AND c.fecha_autorizacion IS NOT NULL
    AND (c.fecha_autorizacion AT TIME ZONE 'America/Guayaquil')::date < p_hasta
    AND NOT EXISTS (SELECT 1 FROM archivo_comprobantes a WHERE a.comprobante_id = c.id)
  ORDER BY 2, 1
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION dias_por_archivar(date) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION dias_por_archivar(date) TO restpos_app;

-- +goose Down
DROP FUNCTION dias_por_archivar(date);
DROP TABLE archivo_comprobantes, archivo_lotes, diccionarios_zstd;
