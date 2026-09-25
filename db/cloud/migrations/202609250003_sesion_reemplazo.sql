-- La búsqueda de sesión devuelve también a quién reemplazó un refresh rotado, para distinguir
-- una carrera benigna (recarga mientras se renovaba) de la reutilización de un token robado.

-- +goose Up
DROP FUNCTION auth_buscar_sesion(bytea);
-- +goose StatementBegin
CREATE FUNCTION auth_buscar_sesion(p_hash bytea)
RETURNS TABLE (sesion_id uuid, tenant_id uuid, usuario_id uuid, familia uuid, expira_at timestamptz, revocada_at timestamptz, reemplazada_por uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT id, tenant_id, usuario_id, familia, expira_at, revocada_at, reemplazada_por FROM sesiones WHERE refresh_hash = p_hash
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION auth_buscar_sesion(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth_buscar_sesion(bytea) TO restpos_app;

-- +goose Down
DROP FUNCTION auth_buscar_sesion(bytea);
-- +goose StatementBegin
CREATE FUNCTION auth_buscar_sesion(p_hash bytea)
RETURNS TABLE (sesion_id uuid, tenant_id uuid, usuario_id uuid, familia uuid, expira_at timestamptz, revocada_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
  SELECT id, tenant_id, usuario_id, familia, expira_at, revocada_at FROM sesiones WHERE refresh_hash = p_hash
$$;
-- +goose StatementEnd
GRANT EXECUTE ON FUNCTION auth_buscar_sesion(bytea) TO restpos_app;
