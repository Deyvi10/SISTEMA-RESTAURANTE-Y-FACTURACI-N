-- F4-04 · Órdenes sin mesa (RF-03-11): Para llevar, Barra/Mostrador y Delivery propio se
-- identifican con su número del día y un nombre corto opcional que sale en la comanda.

-- +goose Up
ALTER TABLE ordenes ADD COLUMN etiqueta TEXT CHECK (etiqueta IS NULL OR length(etiqueta) BETWEEN 1 AND 20);
CREATE INDEX ordenes_sin_mesa_abiertas ON ordenes (abierta_at) WHERE mesa_id IS NULL AND estado IN ('ABIERTA','PRECUENTA');

-- +goose Down
DROP INDEX ordenes_sin_mesa_abiertas;
ALTER TABLE ordenes DROP COLUMN etiqueta;
