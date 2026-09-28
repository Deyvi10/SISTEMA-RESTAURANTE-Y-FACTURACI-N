-- F4-08 · División de cuenta (RF-04-06, docs/04 §6): una orden se reparte en cuentas; cada
-- plato va a una o varias con un peso (pizza entre 3: peso 1 en cada una). Una cuenta pagada
-- guarda lo que pagó y queda congelada; las abiertas se pueden reagrupar.

-- +goose Up
CREATE TABLE cuentas (
  id            TEXT PRIMARY KEY,
  orden_id      TEXT NOT NULL REFERENCES ordenes (id),
  numero        INTEGER NOT NULL CHECK (numero > 0),
  estado        TEXT NOT NULL DEFAULT 'ABIERTA' CHECK (estado IN ('ABIERTA','PAGADA')),
  documento_id  TEXT,
  pagado        TEXT CHECK (pagado IS NULL OR json_valid(pagado)), -- lo que pagó (plato a plato, por tarifa y servicio)
  created_at    TEXT NOT NULL,
  pagada_at     TEXT,
  UNIQUE (orden_id, numero),
  CHECK ((estado = 'PAGADA') = (pagado IS NOT NULL AND documento_id IS NOT NULL))
) STRICT;

CREATE TABLE cuenta_asignaciones (
  cuenta_id       TEXT NOT NULL REFERENCES cuentas (id),
  orden_linea_id  TEXT NOT NULL REFERENCES orden_lineas (id),
  peso            INTEGER NOT NULL CHECK (peso BETWEEN 1 AND 1000),
  PRIMARY KEY (cuenta_id, orden_linea_id)
) STRICT;

-- Un documento por cuenta (sin división, uno por orden como hasta ahora).
ALTER TABLE documentos_venta ADD COLUMN cuenta_id TEXT;
DROP INDEX documentos_venta_orden;
CREATE UNIQUE INDEX documentos_venta_orden ON documentos_venta (orden_id, coalesce(cuenta_id, ''));

-- +goose Down
DROP INDEX documentos_venta_orden;
CREATE UNIQUE INDEX documentos_venta_orden ON documentos_venta (orden_id);
ALTER TABLE documentos_venta DROP COLUMN cuenta_id;
DROP TABLE cuenta_asignaciones;
DROP TABLE cuentas;
