-- F3-02 · Dispositivos emparejados (teléfonos, tablets, caja, KDS). El emparejamiento ocurre
-- en el Nodo Local (funciona sin internet) y llega a la nube con un evento; la revocación
-- se decide en la nube y vuelve al nodo por el feed de cambios (docs/03 §3).

-- +goose Up
CREATE TABLE dispositivos (
  id              uuid PRIMARY KEY,  -- UUID v7 generado por el dispositivo
  tenant_id       uuid NOT NULL,
  local_id        uuid NOT NULL,
  nombre          text NOT NULL CHECK (length(nombre) BETWEEN 1 AND 60),
  tipo            text NOT NULL DEFAULT 'MOVIL' CHECK (tipo IN ('MOVIL','TABLET','KDS','POS')),
  llave_publica   bytea NOT NULL CHECK (length(llave_publica) = 32),
  plataforma      text NOT NULL DEFAULT '',
  version_app     text NOT NULL DEFAULT '',
  estado          text NOT NULL DEFAULT 'AUTORIZADO' CHECK (estado IN ('AUTORIZADO','REVOCADO')),
  emparejado_at   timestamptz NOT NULL,
  ultimo_uso_at   timestamptz,
  revocado_at     timestamptz,
  revocado_por    uuid,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id),
  CHECK ((estado = 'REVOCADO') = (revocado_at IS NOT NULL))
);
ALTER TABLE dispositivos ENABLE ROW LEVEL SECURITY;
ALTER TABLE dispositivos FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON dispositivos USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant());
GRANT SELECT, INSERT, UPDATE ON dispositivos TO restpos_app;
CREATE TRIGGER sync_cambio AFTER INSERT OR UPDATE OR DELETE ON dispositivos FOR EACH ROW EXECUTE FUNCTION registrar_cambio();

-- +goose Down
DROP TABLE dispositivos;
