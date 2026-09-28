-- Impresoras instaladas en Windows en la PC del nodo (cola del spooler). Se imprime por el
-- spooler en modo RAW: sirve para USB y para cualquier puerto que Windows ya sepa usar.
-- Si la instalada es de red con puerto TCP/IP estándar, se guarda como TCP (más rápido y
-- con estado DLE EOT) y nombre_windows solo recuerda de dónde vino.

-- +goose Up
ALTER TABLE impresoras ADD COLUMN nombre_windows text CHECK (nombre_windows IS NULL OR length(nombre_windows) BETWEEN 1 AND 220);
ALTER TABLE impresoras DROP CONSTRAINT impresoras_conexion_check;
ALTER TABLE impresoras ADD CONSTRAINT impresoras_conexion_check CHECK (conexion IN ('TCP','USB','WINDOWS'));
ALTER TABLE impresoras ADD CONSTRAINT impresoras_windows_check CHECK (conexion <> 'WINDOWS' OR nombre_windows IS NOT NULL);
CREATE UNIQUE INDEX impresoras_windows ON impresoras (local_id, nombre_windows) WHERE nombre_windows IS NOT NULL AND deleted_at IS NULL;

-- +goose Down
DROP INDEX impresoras_windows;
ALTER TABLE impresoras DROP CONSTRAINT impresoras_windows_check;
ALTER TABLE impresoras DROP CONSTRAINT impresoras_conexion_check;
ALTER TABLE impresoras ADD CONSTRAINT impresoras_conexion_check CHECK (conexion IN ('TCP','USB'));
ALTER TABLE impresoras DROP COLUMN nombre_windows;
