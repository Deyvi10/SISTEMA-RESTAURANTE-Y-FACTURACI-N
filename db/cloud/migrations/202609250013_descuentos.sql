-- F4-09 · Descuentos y cortesías (RF-04-07): límite de descuento sin autorización por local
-- (para todos los roles menos el administrador) y, si se quiere, uno propio por persona.

-- +goose Up
ALTER TABLE locales ADD COLUMN descuento_maximo_pct numeric(5,2) NOT NULL DEFAULT 10 CHECK (descuento_maximo_pct BETWEEN 0 AND 100);
ALTER TABLE usuarios ADD COLUMN descuento_maximo_pct numeric(5,2) CHECK (descuento_maximo_pct BETWEEN 0 AND 100);

-- +goose Down
ALTER TABLE usuarios DROP COLUMN descuento_maximo_pct;
ALTER TABLE locales DROP COLUMN descuento_maximo_pct;
