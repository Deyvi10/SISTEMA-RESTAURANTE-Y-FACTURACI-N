-- F5-02 / F5-06 · Configuración fiscal del emisor y puntos de emisión (docs/04 §7).
-- La nube es dueña de la configuración; el nodo dueño de cada punto de emisión asigna sus
-- secuenciales sin internet (CLAUDE.md, docs/03 §3). Fuente normativa: ficha técnica offline
-- v2.34 (documentacion-proyecto/docs/fuentes/sri/).

-- +goose Up
-- Datos tributarios del emisor que van en cada comprobante (infoTributaria / infoFactura).
-- El RUC es el del restaurante (tenants.ruc); razón social y dirección matriz las confirma el
-- dueño en el onboarding (F5-06). Facturar queda apagado hasta que el dueño lo active.
CREATE TABLE configuracion_fiscal (
  tenant_id               uuid PRIMARY KEY REFERENCES tenants,
  ambiente                smallint NOT NULL DEFAULT 1 CHECK (ambiente IN (1, 2)), -- Tabla 4: 1 pruebas, 2 producción
  ruc                     char(13) NOT NULL CHECK (ruc ~ '^[0-9]{10}001$'),
  razon_social            text NOT NULL CHECK (length(razon_social) BETWEEN 1 AND 300 AND razon_social !~ '\n'),
  nombre_comercial        text CHECK (length(nombre_comercial) BETWEEN 1 AND 300 AND nombre_comercial !~ '\n'),
  direccion_matriz        text NOT NULL CHECK (length(direccion_matriz) BETWEEN 1 AND 300 AND direccion_matriz !~ '\n'),
  obligado_contabilidad   boolean NOT NULL DEFAULT false,
  contribuyente_especial  text CHECK (contribuyente_especial ~ '^[A-Za-z0-9]{3,13}$'),
  agente_retencion        text CHECK (agente_retencion ~ '^[0-9]{1,8}$'),
  regimen                 text NOT NULL DEFAULT 'GENERAL' CHECK (regimen IN ('GENERAL', 'RIMPE_EMPRENDEDOR', 'RIMPE_NEGOCIO_POPULAR')),
  facturacion_activa      boolean NOT NULL DEFAULT false,
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  version                 int NOT NULL DEFAULT 1,
  -- El XSD de factura 1.1.0 solo admite la leyenda «CONTRIBUYENTE RÉGIMEN RIMPE»; los negocios
  -- populares emiten notas de venta (DP-05, pendiente): no pueden activar la factura aquí.
  CHECK (NOT facturacion_activa OR regimen <> 'RIMPE_NEGOCIO_POPULAR')
);

-- Serie 001-002: establecimiento y punto. Cada punto tiene un único nodo dueño de su numeración.
CREATE TABLE puntos_emision (
  id                      uuid PRIMARY KEY,
  tenant_id               uuid NOT NULL,
  local_id                uuid NOT NULL,
  codigo_establecimiento  char(3) NOT NULL CHECK (codigo_establecimiento ~ '^[0-9]{3}$' AND codigo_establecimiento <> '000'),
  codigo_punto            char(3) NOT NULL CHECK (codigo_punto ~ '^[0-9]{3}$' AND codigo_punto <> '000'),
  nodo_id                 uuid,
  -- Último secuencial que la nube conoce por tipo y ambiente ({"01-1": 123}), reportado por el
  -- nodo con cada comprobante (F5-05). Un nodo nuevo (PC reemplazada) sigue desde aquí y nunca
  -- repite un número. Lo emitido sin internet y nunca sincronizado lo cubre F6-02.
  ultimos_secuenciales    jsonb NOT NULL DEFAULT '{}',
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  version                 int NOT NULL DEFAULT 1,
  deleted_at              timestamptz,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, local_id) REFERENCES locales (tenant_id, id)
);
CREATE UNIQUE INDEX puntos_emision_serie ON puntos_emision (tenant_id, codigo_establecimiento, codigo_punto) WHERE deleted_at IS NULL;

-- Cada caja emite con su propio punto; un punto no se comparte entre cajas.
ALTER TABLE cajas ADD FOREIGN KEY (tenant_id, punto_emision_id) REFERENCES puntos_emision (tenant_id, id);
CREATE UNIQUE INDEX cajas_punto_emision ON cajas (punto_emision_id) WHERE punto_emision_id IS NOT NULL AND deleted_at IS NULL;

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['configuracion_fiscal','puntos_emision'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = app_tenant()) WITH CHECK (tenant_id = app_tenant())', t);
    EXECUTE format('GRANT SELECT, INSERT, UPDATE ON %I TO restpos_app', t);
    EXECUTE format('CREATE TRIGGER sync_cambio AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION registrar_cambio()', t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP INDEX cajas_punto_emision;
ALTER TABLE cajas DROP CONSTRAINT cajas_tenant_id_punto_emision_id_fkey;
DROP TABLE puntos_emision, configuracion_fiscal;
