-- F2-10 / F2-11 · Réplica de impresoras y estaciones, órdenes de la nube y colas de impresión.

-- +goose Up
CREATE TABLE impresoras (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, local_id TEXT NOT NULL, nombre TEXT NOT NULL,
  conexion TEXT NOT NULL DEFAULT 'TCP', host TEXT, puerto INTEGER, mac TEXT, usb_id TEXT, modelo TEXT NOT NULL DEFAULT '',
  ancho_papel INTEGER NOT NULL DEFAULT 80, origen TEXT NOT NULL DEFAULT 'MANUAL', activa INTEGER NOT NULL DEFAULT 1,
  version INTEGER, updated_at TEXT, deleted_at TEXT
);
CREATE TABLE estacion_impresoras (
  tenant_id TEXT NOT NULL, estacion_id TEXT NOT NULL, impresora_id TEXT NOT NULL, local_id TEXT NOT NULL,
  PRIMARY KEY (estacion_id, impresora_id)
);
CREATE TABLE comandos_nodo (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, local_id TEXT NOT NULL, tipo TEXT NOT NULL,
  datos TEXT NOT NULL DEFAULT '{}', creado_por TEXT, created_at TEXT NOT NULL, ejecutado_at TEXT, resultado TEXT
);
-- Qué órdenes ya ejecutó este nodo (la réplica puede traer la misma orden varias veces).
CREATE TABLE comandos_ejecutados (
  id TEXT PRIMARY KEY, ejecutado_at TEXT NOT NULL, resultado TEXT NOT NULL
) STRICT;

-- Comandas enviadas a producción (la orden completa llega en F3/F4; aquí basta para imprimir,
-- reimprimir y anular). La idempotencia la da idempotency_key.
CREATE TABLE comandas (
  id               TEXT PRIMARY KEY,
  idempotency_key  TEXT NOT NULL UNIQUE,
  numero           INTEGER NOT NULL,
  fecha_negocio    TEXT NOT NULL,
  mesa             TEXT NOT NULL,
  mesero_id        TEXT,
  mesero           TEXT NOT NULL,
  lineas           TEXT NOT NULL CHECK (json_valid(lineas)),
  enviada_at       TEXT NOT NULL,
  UNIQUE (fecha_negocio, numero)
) STRICT;

-- Contadores por día (número de comanda visible por jornada).
CREATE TABLE contadores (
  clave TEXT PRIMARY KEY, valor INTEGER NOT NULL
) STRICT;

-- Cola persistente: un trabajo por impresora. Nunca se borra lo pendiente (RF-02-04.3).
CREATE TABLE trabajos_impresion (
  id            TEXT PRIMARY KEY,
  impresora_id  TEXT NOT NULL,
  estacion_id   TEXT,
  comanda_id    TEXT,
  tipo          TEXT NOT NULL CHECK (tipo IN ('COMANDA','ANULACION','REIMPRESION','PRECUENTA','PRUEBA')),
  payload       BLOB NOT NULL,
  documento     TEXT CHECK (documento IS NULL OR json_valid(documento)), -- para volver a armar el ticket si se redirige a otro ancho de papel
  comando_id    TEXT, -- orden de la nube que originó el trabajo (imprimir prueba)
  estado        TEXT NOT NULL DEFAULT 'PENDIENTE' CHECK (estado IN ('PENDIENTE','IMPRESO','CANCELADO')),
  intentos      INTEGER NOT NULL DEFAULT 0,
  ultimo_error  TEXT,
  created_at    TEXT NOT NULL,
  impreso_at    TEXT
) STRICT;
CREATE INDEX trabajos_pendientes ON trabajos_impresion (impresora_id, created_at) WHERE estado = 'PENDIENTE';
CREATE INDEX trabajos_comanda ON trabajos_impresion (comanda_id);

-- Anulaciones de líneas ya enviadas (append-only: una línea se anula una sola vez).
CREATE TABLE anulaciones_comanda (
  comanda_id     TEXT NOT NULL,
  linea_id       TEXT NOT NULL,
  motivo         TEXT NOT NULL,
  usuario_id     TEXT,
  autorizado_por TEXT,
  created_at     TEXT NOT NULL,
  PRIMARY KEY (comanda_id, linea_id)
) STRICT;
CREATE TRIGGER anulaciones_sin_update BEFORE UPDATE ON anulaciones_comanda BEGIN SELECT RAISE(ABORT, 'anulaciones_comanda es append-only'); END;
CREATE TRIGGER anulaciones_sin_delete BEFORE DELETE ON anulaciones_comanda BEGIN SELECT RAISE(ABORT, 'anulaciones_comanda es append-only'); END;

-- Redirección temporal: la cola de una estación va a otra impresora (RF-02-05.3).
CREATE TABLE redirecciones (
  estacion_id   TEXT PRIMARY KEY,
  impresora_id  TEXT NOT NULL,
  desde         TEXT NOT NULL
) STRICT;

-- +goose Down
DROP TABLE redirecciones; DROP TABLE anulaciones_comanda; DROP TABLE trabajos_impresion; DROP TABLE contadores; DROP TABLE comandas;
DROP TABLE comandos_ejecutados; DROP TABLE comandos_nodo; DROP TABLE estacion_impresoras; DROP TABLE impresoras;
