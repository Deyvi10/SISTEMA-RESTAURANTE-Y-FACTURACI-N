-- Réplica: cola de Windows de las impresoras instaladas en esta PC.

-- +goose Up
ALTER TABLE impresoras ADD COLUMN nombre_windows TEXT;

-- +goose Down
ALTER TABLE impresoras DROP COLUMN nombre_windows;
