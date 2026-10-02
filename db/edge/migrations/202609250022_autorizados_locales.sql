-- F5-18 · Copia local de los comprobantes autorizados (90 días): el XML autorizado que entregó
-- la nube, comprimido con zstd, con su número y fecha de autorización, para consultarlo y
-- reimprimir el RIDE con la leyenda de autorizado aunque no haya internet. Se purga a los 90
-- días; el original queda en el archivo inmutable de la nube.

-- +goose Up
CREATE TABLE autorizados_locales (
  comprobante_id       TEXT PRIMARY KEY REFERENCES comprobantes (id),
  numero_autorizacion  TEXT NOT NULL,
  fecha_autorizacion   TEXT NOT NULL,
  xml_zst              BLOB NOT NULL,
  sha256               TEXT NOT NULL,
  guardado_at          TEXT NOT NULL
) STRICT;
CREATE INDEX autorizados_locales_fecha ON autorizados_locales (fecha_autorizacion);

-- +goose Down
DROP TABLE autorizados_locales;
