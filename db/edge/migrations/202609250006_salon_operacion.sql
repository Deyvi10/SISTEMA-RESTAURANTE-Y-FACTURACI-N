-- F3 · Dispositivos, sesiones por PIN, bloqueos de mesa y órdenes (docs/04 §3, §5, §6).
-- Dinero y cantidades en TEXT decimal; fechas TEXT RFC 3339 UTC.

-- +goose Up
ALTER TABLE nodo ADD COLUMN pin_pepper BLOB;

-- Réplica desde la nube: solo interesa saber si un dispositivo fue revocado.
CREATE TABLE dispositivos (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, local_id TEXT NOT NULL, nombre TEXT NOT NULL, tipo TEXT NOT NULL,
  estado TEXT NOT NULL, emparejado_at TEXT, ultimo_uso_at TEXT, revocado_at TEXT
);

-- Emparejados en ESTE nodo: es lo que se usa para autenticar (funciona sin internet).
CREATE TABLE dispositivos_nodo (
  id             TEXT PRIMARY KEY,
  nombre         TEXT NOT NULL,
  tipo           TEXT NOT NULL CHECK (tipo IN ('MOVIL','TABLET','KDS','POS')),
  llave_publica  BLOB NOT NULL CHECK (length(llave_publica) = 32),
  plataforma     TEXT NOT NULL DEFAULT '',
  version_app    TEXT NOT NULL DEFAULT '',
  emparejado_at  TEXT NOT NULL,
  revocado_at    TEXT
) STRICT;

CREATE TABLE codigos_emparejamiento (
  codigo_hash  TEXT PRIMARY KEY,
  expira_at    TEXT NOT NULL,
  usado_at     TEXT,
  dispositivo_id TEXT
) STRICT;

CREATE TABLE sesiones_dispositivo (
  token_hash     TEXT PRIMARY KEY,
  dispositivo_id TEXT NOT NULL,
  creada_at      TEXT NOT NULL,
  expira_at      TEXT NOT NULL
) STRICT;
CREATE INDEX sesiones_dispositivo_disp ON sesiones_dispositivo (dispositivo_id);

-- Sesión de un usuario en un dispositivo tras su PIN (máx. 12 h o hasta cerrar la jornada).
CREATE TABLE sesiones_usuario (
  token_hash     TEXT PRIMARY KEY,
  usuario_id     TEXT NOT NULL,
  dispositivo_id TEXT NOT NULL,
  creada_at      TEXT NOT NULL,
  expira_at      TEXT NOT NULL,
  cerrada_at     TEXT
) STRICT;
CREATE INDEX sesiones_usuario_usuario ON sesiones_usuario (usuario_id);

-- Intentos de PIN por dispositivo (5 seguidos → 5 min; 20 en 10 min → alerta).
CREATE TABLE intentos_pin (
  dispositivo_id  TEXT PRIMARY KEY,
  fallos          INTEGER NOT NULL DEFAULT 0,
  bloqueado_hasta TEXT,
  ventana_desde   TEXT NOT NULL,
  en_ventana      INTEGER NOT NULL DEFAULT 0
) STRICT;

-- Autorización de supervisor de un solo uso, ligada a una acción concreta (F3-14).
CREATE TABLE autorizaciones (
  token_hash     TEXT PRIMARY KEY,
  accion         TEXT NOT NULL,
  referencia     TEXT NOT NULL,
  autorizado_por TEXT NOT NULL,
  dispositivo_id TEXT NOT NULL,
  expira_at      TEXT NOT NULL,
  usado_at       TEXT
) STRICT;

-- Bloqueo pesimista de mesa (F3-07): sobrevive a un reinicio del nodo.
CREATE TABLE bloqueos_mesa (
  mesa_id              TEXT PRIMARY KEY,
  usuario_id           TEXT NOT NULL,
  usuario_nombre       TEXT NOT NULL,
  dispositivo_id       TEXT NOT NULL,
  adquirido_at         TEXT NOT NULL,
  ultimo_heartbeat_at  TEXT NOT NULL
) STRICT;

-- Órdenes (dueño: el nodo).
CREATE TABLE ordenes (
  id            TEXT PRIMARY KEY,
  mesa_id       TEXT,
  tipo          TEXT NOT NULL DEFAULT 'MESA' CHECK (tipo IN ('MESA','LLEVAR','BARRA','DELIVERY')),
  mesero_id     TEXT NOT NULL,
  mesero_nombre TEXT NOT NULL,
  numero_corto  INTEGER NOT NULL,
  fecha_negocio TEXT NOT NULL,
  estado        TEXT NOT NULL DEFAULT 'ABIERTA' CHECK (estado IN ('ABIERTA','PRECUENTA','CERRADA','ANULADA')),
  comensales    INTEGER,
  abierta_at    TEXT NOT NULL,
  precuenta_at  TEXT,
  cerrada_at    TEXT,
  unida_a       TEXT,
  version       INTEGER NOT NULL DEFAULT 1,
  CHECK (tipo <> 'MESA' OR mesa_id IS NOT NULL)
) STRICT;
-- Una sola orden abierta por mesa.
CREATE UNIQUE INDEX ordenes_mesa_abierta ON ordenes (mesa_id) WHERE estado IN ('ABIERTA','PRECUENTA') AND mesa_id IS NOT NULL;

CREATE TABLE orden_lineas (
  id               TEXT PRIMARY KEY,
  orden_id         TEXT NOT NULL REFERENCES ordenes (id),
  producto_id      TEXT NOT NULL,
  producto_nombre  TEXT NOT NULL,
  precio_unitario  TEXT NOT NULL,
  tarifa_iva_id    TEXT NOT NULL,
  porcentaje_iva   TEXT NOT NULL,
  cantidad         TEXT NOT NULL,
  nota             TEXT NOT NULL DEFAULT '',
  tiempo           TEXT NOT NULL DEFAULT '',
  estacion_id      TEXT NOT NULL,
  comanda_id       TEXT,
  estado           TEXT NOT NULL CHECK (estado IN ('EN_ESPERA','ENVIADA','ANULADA')),
  creada_por       TEXT NOT NULL,
  creada_at        TEXT NOT NULL,
  anulada_por      TEXT,
  anulada_autoriza TEXT,
  anulada_motivo   TEXT,
  anulada_at       TEXT,
  se_preparo       INTEGER
) STRICT;
CREATE INDEX orden_lineas_orden ON orden_lineas (orden_id);

CREATE TABLE orden_linea_modificadores (
  id                TEXT PRIMARY KEY,
  orden_linea_id    TEXT NOT NULL REFERENCES orden_lineas (id),
  modificador_id    TEXT NOT NULL,
  nombre            TEXT NOT NULL,
  precio_adicional  TEXT NOT NULL,
  cantidad          INTEGER NOT NULL DEFAULT 1
) STRICT;
CREATE INDEX orden_linea_mod_linea ON orden_linea_modificadores (orden_linea_id);

ALTER TABLE comandas ADD COLUMN orden_id TEXT;

-- +goose Down
ALTER TABLE comandas DROP COLUMN orden_id;
DROP TABLE orden_linea_modificadores; DROP TABLE orden_lineas; DROP TABLE ordenes; DROP TABLE bloqueos_mesa;
DROP TABLE autorizaciones; DROP TABLE intentos_pin; DROP TABLE sesiones_usuario; DROP TABLE sesiones_dispositivo;
DROP TABLE codigos_emparejamiento; DROP TABLE dispositivos_nodo; DROP TABLE dispositivos;
ALTER TABLE nodo DROP COLUMN pin_pepper;
