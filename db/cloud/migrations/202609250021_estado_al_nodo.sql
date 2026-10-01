-- F5-11 · El estado de cada comprobante vuelve al nodo (fiscal.status_changed): cada cambio
-- de estado se publica en el feed de réplica como la tabla «estados_comprobante», solo con
-- lo que el nodo necesita (sin el XML: la copia autorizada local es F5-18).

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION estado_comprobante_json(c comprobantes) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
  SELECT jsonb_build_object(
    'id', c.id, 'tenant_id', c.tenant_id, 'local_id', c.local_id, 'clave_acceso', c.clave_acceso, 'estado', c.estado,
    'numero_autorizacion', c.numero_autorizacion, 'fecha_autorizacion', c.fecha_autorizacion,
    'mensaje', coalesce(c.ultimo_error, nullif(concat_ws(' ', c.mensajes_sri->0->>'Identificador', c.mensajes_sri->0->>'Mensaje'), '')),
    'updated_at', c.updated_at)
$$;

CREATE FUNCTION publicar_estado_comprobante() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE n bigint;
BEGIN
  INSERT INTO sync_seq_tenant AS s (tenant_id, ultimo) VALUES (NEW.tenant_id, 1)
    ON CONFLICT (tenant_id) DO UPDATE SET ultimo = s.ultimo + 1
    RETURNING s.ultimo INTO n;
  INSERT INTO sync_cambios (tenant_id, seq, tabla, op, local_id, datos)
    VALUES (NEW.tenant_id, n, 'estados_comprobante', 'U', NEW.local_id, estado_comprobante_json(NEW));
  PERFORM pg_notify('sync_cambios', NEW.tenant_id::text);
  RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER comprobantes_estado_nodo AFTER INSERT ON comprobantes FOR EACH ROW EXECUTE FUNCTION publicar_estado_comprobante();
CREATE TRIGGER comprobantes_estado_nodo_cambio AFTER UPDATE OF estado ON comprobantes FOR EACH ROW
  WHEN (OLD.estado IS DISTINCT FROM NEW.estado) EXECUTE FUNCTION publicar_estado_comprobante();

-- +goose Down
DROP TRIGGER comprobantes_estado_nodo_cambio ON comprobantes;
DROP TRIGGER comprobantes_estado_nodo ON comprobantes;
DROP FUNCTION publicar_estado_comprobante();
DROP FUNCTION estado_comprobante_json(comprobantes);
