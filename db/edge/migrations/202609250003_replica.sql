-- F2-03 · Réplica local de lo que la nube administra: local, salón, catálogo y personal.
-- Las columnas llevan los mismos nombres que en PostgreSQL: el nodo guarda solo las que
-- conoce e ignora las nuevas (un nodo viejo sigue funcionando con una nube más nueva).
-- Sin claves foráneas: los cambios llegan en el orden de la nube y un borrado en cascada
-- puede llegar antes que el del padre. Dinero y porcentajes en TEXT decimal, nunca REAL.

-- +goose Up
CREATE TABLE locales (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, nombre TEXT NOT NULL, direccion TEXT NOT NULL DEFAULT '',
  codigo_establecimiento TEXT NOT NULL, zona_horaria TEXT NOT NULL DEFAULT 'America/Guayaquil',
  propina_legal_activa INTEGER NOT NULL DEFAULT 0, propina_porcentaje TEXT NOT NULL DEFAULT '10',
  precios_incluyen_iva INTEGER NOT NULL DEFAULT 1, version INTEGER, updated_at TEXT, deleted_at TEXT
);
CREATE TABLE estaciones (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, local_id TEXT NOT NULL, nombre TEXT NOT NULL,
  tipo TEXT NOT NULL DEFAULT 'PRODUCCION', icono TEXT, color TEXT, orden INTEGER NOT NULL DEFAULT 0,
  es_defecto INTEGER NOT NULL DEFAULT 0, version INTEGER, updated_at TEXT, deleted_at TEXT
);
CREATE TABLE zonas (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, local_id TEXT NOT NULL, nombre TEXT NOT NULL,
  orden INTEGER NOT NULL DEFAULT 0, version INTEGER, updated_at TEXT, deleted_at TEXT
);
CREATE TABLE mesas (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, local_id TEXT NOT NULL, zona_id TEXT NOT NULL, nombre TEXT NOT NULL,
  capacidad INTEGER NOT NULL DEFAULT 4, forma TEXT NOT NULL DEFAULT 'CUADRADA', pos_x INTEGER NOT NULL DEFAULT 0,
  pos_y INTEGER NOT NULL DEFAULT 0, ancho INTEGER NOT NULL DEFAULT 1, alto INTEGER NOT NULL DEFAULT 1,
  rotacion INTEGER NOT NULL DEFAULT 0, activa INTEGER NOT NULL DEFAULT 1, version INTEGER, updated_at TEXT, deleted_at TEXT
);
CREATE TABLE tarifas_iva (
  id TEXT PRIMARY KEY, codigo_sri TEXT NOT NULL, porcentaje TEXT NOT NULL, descripcion TEXT NOT NULL,
  vigente_desde TEXT NOT NULL, vigente_hasta TEXT
);
CREATE TABLE categorias (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, nombre TEXT NOT NULL, orden INTEGER NOT NULL DEFAULT 0,
  estacion_id TEXT, color TEXT, icono TEXT, activa INTEGER NOT NULL DEFAULT 1, version INTEGER, updated_at TEXT, deleted_at TEXT
);
CREATE TABLE productos (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, categoria_id TEXT NOT NULL, nombre TEXT NOT NULL, alias TEXT, codigo TEXT,
  descripcion TEXT NOT NULL DEFAULT '', precio TEXT NOT NULL, tarifa_iva_id TEXT NOT NULL, tipo TEXT NOT NULL DEFAULT 'SIMPLE',
  comportamiento_stock TEXT NOT NULL DEFAULT 'NINGUNO', estacion_id TEXT, imagen_key TEXT, activo INTEGER NOT NULL DEFAULT 1,
  orden INTEGER NOT NULL DEFAULT 0, version INTEGER, updated_at TEXT, deleted_at TEXT
);
CREATE INDEX productos_categoria ON productos (categoria_id);
CREATE TABLE grupos_modificadores (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, nombre TEXT NOT NULL, obligatorio INTEGER NOT NULL DEFAULT 0,
  min INTEGER NOT NULL DEFAULT 0, max INTEGER NOT NULL DEFAULT 1, version INTEGER, updated_at TEXT, deleted_at TEXT
);
CREATE TABLE modificadores (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, grupo_id TEXT NOT NULL, nombre TEXT NOT NULL,
  precio_adicional TEXT NOT NULL DEFAULT '0', orden INTEGER NOT NULL DEFAULT 0, activo INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE producto_grupos_modificadores (
  tenant_id TEXT NOT NULL, producto_id TEXT NOT NULL, grupo_id TEXT NOT NULL, orden INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (producto_id, grupo_id)
);
CREATE TABLE notas_rapidas (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, categoria_id TEXT NOT NULL, texto TEXT NOT NULL, orden INTEGER NOT NULL DEFAULT 0
);
-- Personal: sin correo ni contraseña web (la nube no los envía). El PIN se valida en F3-04.
CREATE TABLE usuarios (
  id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, nombre_mostrar TEXT NOT NULL, rol TEXT NOT NULL,
  es_dueno INTEGER NOT NULL DEFAULT 0, pin_hash TEXT, pin_fingerprint TEXT, avatar_key TEXT,
  activo INTEGER NOT NULL DEFAULT 1, version INTEGER, updated_at TEXT
);
CREATE TABLE usuario_locales (
  tenant_id TEXT NOT NULL, usuario_id TEXT NOT NULL, local_id TEXT NOT NULL, PRIMARY KEY (usuario_id, local_id)
);
CREATE TABLE permisos_usuario (
  tenant_id TEXT NOT NULL, usuario_id TEXT NOT NULL, permiso TEXT NOT NULL, concedido INTEGER NOT NULL,
  updated_at TEXT, PRIMARY KEY (usuario_id, permiso)
);

-- +goose Down
DROP TABLE permisos_usuario; DROP TABLE usuario_locales; DROP TABLE usuarios; DROP TABLE notas_rapidas;
DROP TABLE producto_grupos_modificadores; DROP TABLE modificadores; DROP TABLE grupos_modificadores;
DROP TABLE productos; DROP TABLE categorias; DROP TABLE tarifas_iva; DROP TABLE mesas; DROP TABLE zonas;
DROP TABLE estaciones; DROP TABLE locales;
