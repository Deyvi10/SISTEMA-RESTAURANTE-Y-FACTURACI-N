-- F5-14 · Leyendas por régimen con vigencia (RF-05-05): un cambio de régimen o de las
-- calificaciones tributarias (contabilidad, contribuyente especial, agente de retención) se
-- programa con fecha. El nodo aplica, para cada comprobante, lo vigente en su fecha de
-- emisión; lo ya emitido no cambia (su XML es inmutable). Llegada la fecha, la nube lo pasa a
-- los campos actuales. Monto máximo para consumidor final: 50 USD (ficha v2.34, §9.10).

-- +goose Up
ALTER TABLE configuracion_fiscal ADD COLUMN cambio_desde date;
ALTER TABLE configuracion_fiscal ADD COLUMN cambio_regimen text CHECK (cambio_regimen IN ('GENERAL', 'RIMPE_EMPRENDEDOR', 'RIMPE_NEGOCIO_POPULAR'));
ALTER TABLE configuracion_fiscal ADD COLUMN cambio_obligado_contabilidad boolean;
ALTER TABLE configuracion_fiscal ADD COLUMN cambio_contribuyente_especial text CHECK (cambio_contribuyente_especial ~ '^[A-Za-z0-9]{3,13}$');
ALTER TABLE configuracion_fiscal ADD COLUMN cambio_agente_retencion text CHECK (cambio_agente_retencion ~ '^[0-9]{1,8}$');
ALTER TABLE configuracion_fiscal ADD CONSTRAINT configuracion_fiscal_cambio_completo
  CHECK ((cambio_desde IS NULL) = (cambio_regimen IS NULL) AND (cambio_desde IS NULL) = (cambio_obligado_contabilidad IS NULL));
UPDATE parametros_globales SET descripcion = 'Importe total máximo de una factura a consumidor final (USD): ficha técnica offline v2.34, §9.10 («mayor a 50 USD» exige los datos del adquirente).'
  WHERE clave = 'consumidor_final_maximo';

-- +goose Down
ALTER TABLE configuracion_fiscal DROP CONSTRAINT configuracion_fiscal_cambio_completo;
ALTER TABLE configuracion_fiscal DROP COLUMN cambio_desde, DROP COLUMN cambio_regimen, DROP COLUMN cambio_obligado_contabilidad,
  DROP COLUMN cambio_contribuyente_especial, DROP COLUMN cambio_agente_retencion;
