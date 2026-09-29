#!/usr/bin/env bash
# Suite E2E de la caja (F4-16): compila la caja dentro del nodo, arranca un nodo efímero
# con datos sembrados (apps/edge-node/cmd/nodo-e2e) y corre Cypress contra él.
set -euo pipefail
raiz="$(cd "$(dirname "$0")/.." && pwd)"
cd "$raiz"
GO="${GO:-$(command -v go || echo "$HOME/.local/go/bin/go")}"
export RESTPOS_E2E_HTTP="${RESTPOS_E2E_HTTP:-127.0.0.1:7181}" RESTPOS_E2E_AUX="${RESTPOS_E2E_AUX:-127.0.0.1:7182}"
export RESTPOS_E2E_URL="http://$RESTPOS_E2E_HTTP" RESTPOS_E2E_AUX_URL="http://$RESTPOS_E2E_AUX"

# La caja se sirve embebida en el nodo (go:embed): se compila antes que él.
npm run build -w @restpos/pos-web >/dev/null
bin="$(mktemp -d)/nodo-e2e"
"$GO" build -o "$bin" ./apps/edge-node/cmd/nodo-e2e
"$bin" &
nodo=$!
trap 'kill $nodo 2>/dev/null || true; wait $nodo 2>/dev/null || true' EXIT
for _ in $(seq 1 100); do
  curl -sf -o /dev/null "$RESTPOS_E2E_AUX_URL/datos" && break
  sleep 0.1
done
# Chrome (Electron está obsoleto en Cypress 16); CYPRESS_NAVEGADOR admite una ruta a otro Chromium.
npm run caja -w @restpos/e2e -- --browser "${CYPRESS_NAVEGADOR:-chrome}"
