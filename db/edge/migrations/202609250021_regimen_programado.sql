-- F5-14 · Cambio de régimen programado (réplica de configuracion_fiscal): el nodo emite con lo
-- vigente en la fecha de cada comprobante.

-- +goose Up
ALTER TABLE configuracion_fiscal ADD COLUMN cambio_desde TEXT;
ALTER TABLE configuracion_fiscal ADD COLUMN cambio_regimen TEXT;
ALTER TABLE configuracion_fiscal ADD COLUMN cambio_obligado_contabilidad INTEGER;
ALTER TABLE configuracion_fiscal ADD COLUMN cambio_contribuyente_especial TEXT;
ALTER TABLE configuracion_fiscal ADD COLUMN cambio_agente_retencion TEXT;

-- +goose Down
ALTER TABLE configuracion_fiscal DROP COLUMN cambio_agente_retencion;
ALTER TABLE configuracion_fiscal DROP COLUMN cambio_contribuyente_especial;
ALTER TABLE configuracion_fiscal DROP COLUMN cambio_obligado_contabilidad;
ALTER TABLE configuracion_fiscal DROP COLUMN cambio_regimen;
ALTER TABLE configuracion_fiscal DROP COLUMN cambio_desde;
