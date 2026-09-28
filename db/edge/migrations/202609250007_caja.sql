-- F4-02 / F4-03 / F4-11 · Caja en el Nodo Local (docs/04 §6).
-- Réplica de la configuración de la nube (cajas, métodos de pago, motivos, parámetros) y
-- la operación que es del nodo: jornadas, turnos de caja y movimientos de efectivo.

-- +goose Up
-- ---------- Réplica (mismas columnas que la nube, sin FK) ----------
CREATE TABLE parametros_globales (
  clave TEXT PRIMARY KEY, valor TEXT NOT NULL, descripcion TEXT NOT NULL, updated_at TEXT
);
CREATE TABLE cajas (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, local_id TEXT NOT NULL, nombre TEXT NOT NULL,
  estacion_id TEXT, punto_emision_id TEXT, activa INTEGER NOT NULL DEFAULT 1,
  created_at TEXT, updated_at TEXT, created_by TEXT, version INTEGER, deleted_at TEXT
);
CREATE TABLE metodos_pago (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, nombre TEXT NOT NULL, tipo TEXT NOT NULL,
  codigo_forma_pago_sri TEXT NOT NULL, abre_cajon INTEGER NOT NULL DEFAULT 0, pide_referencia INTEGER NOT NULL DEFAULT 0,
  icono TEXT, orden INTEGER NOT NULL DEFAULT 0, activo INTEGER NOT NULL DEFAULT 1,
  created_at TEXT, updated_at TEXT, version INTEGER, deleted_at TEXT
);
CREATE TABLE motivos_descuento (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, nombre TEXT NOT NULL, tipo TEXT NOT NULL,
  orden INTEGER NOT NULL DEFAULT 0, activo INTEGER NOT NULL DEFAULT 1, created_at TEXT, version INTEGER, deleted_at TEXT
);

-- ---------- Operación (dueño: el nodo) ----------

-- El día de negocio: se abre una vez, puede cruzar la medianoche y agrupa órdenes y turnos.
CREATE TABLE jornadas (
  id             TEXT PRIMARY KEY,
  fecha_negocio  TEXT NOT NULL UNIQUE,          -- fecha local de la apertura (YYYY-MM-DD)
  abierta_at     TEXT NOT NULL,
  abierta_por    TEXT NOT NULL,
  cerrada_at     TEXT,
  cerrada_por    TEXT,
  CHECK ((cerrada_at IS NULL) = (cerrada_por IS NULL))
) STRICT;
CREATE UNIQUE INDEX jornadas_una_abierta ON jornadas ((1)) WHERE cerrada_at IS NULL;

-- Las órdenes pertenecen a la jornada en que se abrieron (las que se transfieren al cerrar
-- quedan sin jornada y las adopta la siguiente).
ALTER TABLE ordenes ADD COLUMN jornada_id TEXT;

CREATE TABLE turnos_caja (
  id             TEXT PRIMARY KEY,
  caja_id        TEXT NOT NULL,
  jornada_id     TEXT NOT NULL REFERENCES jornadas (id),
  cajero_id      TEXT NOT NULL,
  cajero_nombre  TEXT NOT NULL,
  fondo_inicial  TEXT NOT NULL,
  abierto_at     TEXT NOT NULL,
  cerrado_at     TEXT,
  estado         TEXT NOT NULL DEFAULT 'ABIERTO' CHECK (estado IN ('ABIERTO','CERRADO')),
  CHECK ((estado = 'CERRADO') = (cerrado_at IS NOT NULL))
) STRICT;
-- Una caja tiene como máximo un turno abierto.
CREATE UNIQUE INDEX turnos_caja_abierto ON turnos_caja (caja_id) WHERE estado = 'ABIERTO';
CREATE INDEX turnos_caja_jornada ON turnos_caja (jornada_id);

-- Retiros a caja fuerte, ingresos ajenos a ventas y gastos menores (append-only).
CREATE TABLE movimientos_caja (
  id              TEXT PRIMARY KEY,
  turno_id        TEXT NOT NULL REFERENCES turnos_caja (id),
  tipo            TEXT NOT NULL CHECK (tipo IN ('RETIRO','INGRESO','GASTO')),
  monto           TEXT NOT NULL,
  motivo          TEXT NOT NULL CHECK (length(motivo) BETWEEN 3 AND 200),
  usuario_id      TEXT NOT NULL,
  usuario_nombre  TEXT NOT NULL,
  autorizado_por  TEXT,
  foto            TEXT,                          -- clave del archivo del comprobante (opcional)
  idempotency_key TEXT NOT NULL UNIQUE,
  created_at      TEXT NOT NULL
) STRICT;
CREATE INDEX movimientos_caja_turno ON movimientos_caja (turno_id);
CREATE TRIGGER movimientos_caja_sin_update BEFORE UPDATE ON movimientos_caja BEGIN SELECT RAISE(ABORT, 'movimientos_caja es append-only'); END;
CREATE TRIGGER movimientos_caja_sin_delete BEFORE DELETE ON movimientos_caja BEGIN SELECT RAISE(ABORT, 'movimientos_caja es append-only'); END;

-- +goose Down
DROP TABLE movimientos_caja;
DROP TABLE turnos_caja;
ALTER TABLE ordenes DROP COLUMN jornada_id;
DROP TABLE jornadas;
DROP TABLE motivos_descuento;
DROP TABLE metodos_pago;
DROP TABLE cajas;
DROP TABLE parametros_globales;
