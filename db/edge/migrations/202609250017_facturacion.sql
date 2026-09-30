-- F5-02 · Réplica de la configuración fiscal y los puntos de emisión, y los secuenciales, que
-- son del nodo dueño de cada punto (docs/04 §7): se incrementan en la misma transacción del
-- cobro, sin internet, sin huecos ni duplicados. Pruebas y producción llevan numeración aparte.

-- +goose Up
CREATE TABLE configuracion_fiscal (
  tenant_id TEXT PRIMARY KEY, ambiente INTEGER NOT NULL, ruc TEXT NOT NULL, razon_social TEXT NOT NULL,
  nombre_comercial TEXT, direccion_matriz TEXT NOT NULL, obligado_contabilidad INTEGER NOT NULL DEFAULT 0,
  contribuyente_especial TEXT, agente_retencion TEXT, regimen TEXT NOT NULL, facturacion_activa INTEGER NOT NULL DEFAULT 0,
  created_at TEXT, updated_at TEXT, version INTEGER
);
CREATE TABLE puntos_emision (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, local_id TEXT NOT NULL, codigo_establecimiento TEXT NOT NULL,
  codigo_punto TEXT NOT NULL, nodo_id TEXT, ultimos_secuenciales TEXT NOT NULL DEFAULT '{}',
  created_at TEXT, updated_at TEXT, version INTEGER, deleted_at TEXT
);
CREATE TABLE secuenciales (
  punto_emision_id  TEXT NOT NULL,
  tipo_comprobante  TEXT NOT NULL CHECK (tipo_comprobante IN ('01', '04')),
  ambiente          INTEGER NOT NULL CHECK (ambiente IN (1, 2)),
  ultimo            INTEGER NOT NULL CHECK (ultimo BETWEEN 0 AND 999999999),
  PRIMARY KEY (punto_emision_id, tipo_comprobante, ambiente)
) STRICT;

-- +goose Down
DROP TABLE secuenciales;
DROP TABLE puntos_emision;
DROP TABLE configuracion_fiscal;
