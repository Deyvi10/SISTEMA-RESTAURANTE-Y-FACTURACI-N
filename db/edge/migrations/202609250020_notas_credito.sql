-- F5-13 · Notas de crédito. El nodo las numera (es dueño de los secuenciales del punto) y las
-- guarda en comprobantes como tipo 04 con su factura de sustento; no tienen documento de
-- venta. nc_lineas dice cuánto de cada línea de la factura ya se revirtió (para no pasarse) y
-- devoluciones_nc el dinero devuelto, que el Cierre Z resta de lo cobrado por método.
-- Todo append-only.

-- +goose Up
CREATE TABLE comprobantes_nueva (
  id                TEXT PRIMARY KEY,
  documento_id      TEXT UNIQUE REFERENCES documentos_venta (id),
  tipo              TEXT NOT NULL CHECK (tipo IN ('01', '04')),
  ambiente          INTEGER NOT NULL CHECK (ambiente IN (1, 2)),
  punto_emision_id  TEXT NOT NULL,
  serie             TEXT NOT NULL CHECK (length(serie) = 6),
  secuencial        INTEGER NOT NULL CHECK (secuencial BETWEEN 1 AND 999999999),
  clave_acceso      TEXT NOT NULL UNIQUE CHECK (length(clave_acceso) = 49),
  fecha_emision     TEXT NOT NULL,
  importe_total     TEXT NOT NULL,
  xml               TEXT NOT NULL,
  hash              TEXT NOT NULL,
  created_at        TEXT NOT NULL,
  sustento_id       TEXT REFERENCES comprobantes_nueva (id),
  UNIQUE (punto_emision_id, tipo, ambiente, secuencial),
  -- Una factura tiene su documento de venta; una nota de crédito, su factura de sustento.
  CHECK ((tipo = '01' AND documento_id IS NOT NULL AND sustento_id IS NULL) OR (tipo = '04' AND documento_id IS NULL AND sustento_id IS NOT NULL))
) STRICT;
INSERT INTO comprobantes_nueva (id, documento_id, tipo, ambiente, punto_emision_id, serie, secuencial, clave_acceso, fecha_emision,
    importe_total, xml, hash, created_at)
  SELECT id, documento_id, tipo, ambiente, punto_emision_id, serie, secuencial, clave_acceso, fecha_emision, importe_total, xml, hash, created_at
  FROM comprobantes;
DROP TABLE comprobantes;
ALTER TABLE comprobantes_nueva RENAME TO comprobantes;
CREATE INDEX comprobantes_sustento ON comprobantes (sustento_id) WHERE sustento_id IS NOT NULL;
CREATE INDEX comprobantes_fecha ON comprobantes (fecha_emision);
CREATE TRIGGER comprobantes_sin_update BEFORE UPDATE ON comprobantes BEGIN SELECT RAISE(ABORT, 'comprobantes es append-only'); END;
CREATE TRIGGER comprobantes_sin_delete BEFORE DELETE ON comprobantes BEGIN SELECT RAISE(ABORT, 'comprobantes es append-only'); END;

CREATE TABLE nc_lineas (
  nota_credito_id  TEXT NOT NULL REFERENCES comprobantes (id),
  factura_id       TEXT NOT NULL REFERENCES comprobantes (id),
  indice           INTEGER NOT NULL CHECK (indice >= 0),
  cantidad         TEXT NOT NULL,
  total            TEXT NOT NULL,
  descuento        TEXT NOT NULL,
  iva              TEXT NOT NULL,
  PRIMARY KEY (nota_credito_id, indice)
) STRICT;
CREATE INDEX nc_lineas_factura ON nc_lineas (factura_id);
CREATE TRIGGER nc_lineas_sin_update BEFORE UPDATE ON nc_lineas BEGIN SELECT RAISE(ABORT, 'nc_lineas es append-only'); END;
CREATE TRIGGER nc_lineas_sin_delete BEFORE DELETE ON nc_lineas BEGIN SELECT RAISE(ABORT, 'nc_lineas es append-only'); END;

CREATE TABLE notas_credito (
  id                   TEXT PRIMARY KEY REFERENCES comprobantes (id),
  factura_id           TEXT NOT NULL REFERENCES comprobantes (id),
  motivo               TEXT NOT NULL CHECK (length(motivo) BETWEEN 3 AND 300),
  devolver_inventario  INTEGER NOT NULL CHECK (devolver_inventario IN (0, 1)),
  emitida_por          TEXT NOT NULL,
  autorizada_por       TEXT,
  caja_id              TEXT NOT NULL,
  idempotency_key      TEXT NOT NULL UNIQUE,
  created_at           TEXT NOT NULL
) STRICT;
CREATE TRIGGER notas_credito_sin_update BEFORE UPDATE ON notas_credito BEGIN SELECT RAISE(ABORT, 'notas_credito es append-only'); END;
CREATE TRIGGER notas_credito_sin_delete BEFORE DELETE ON notas_credito BEGIN SELECT RAISE(ABORT, 'notas_credito es append-only'); END;

CREATE TABLE devoluciones_nc (
  id               TEXT PRIMARY KEY,
  nota_credito_id  TEXT NOT NULL UNIQUE REFERENCES notas_credito (id),
  turno_id         TEXT NOT NULL REFERENCES turnos_caja (id),
  metodo_pago_id   TEXT NOT NULL,
  monto            TEXT NOT NULL,
  created_at       TEXT NOT NULL
) STRICT;
CREATE INDEX devoluciones_nc_turno ON devoluciones_nc (turno_id);
CREATE TRIGGER devoluciones_nc_sin_update BEFORE UPDATE ON devoluciones_nc BEGIN SELECT RAISE(ABORT, 'devoluciones_nc es append-only'); END;
CREATE TRIGGER devoluciones_nc_sin_delete BEFORE DELETE ON devoluciones_nc BEGIN SELECT RAISE(ABORT, 'devoluciones_nc es append-only'); END;

-- +goose Down
DROP TABLE devoluciones_nc;
DROP TABLE notas_credito;
DROP TABLE nc_lineas;
