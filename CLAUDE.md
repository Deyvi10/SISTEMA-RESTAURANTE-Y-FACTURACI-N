# CLAUDE.md (raíz del monorepo)

@documentacion-proyecto/CLAUDE.md

## Rutas

La documentación vive en `documentacion-proyecto/`: toda referencia a `docs/…` en el CLAUDE.md importado significa `documentacion-proyecto/docs/…`. El código vive en la raíz (`apps/`, `packages/`, `tools/`…).

## Cómo trabajar aquí

- Go está en `~/.local/go/bin` si no está en el PATH (el Makefile lo encuentra solo).
- Un solo módulo Go en la raíz: `github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N`.
- Antes de dar algo por terminado: `make lint test`.
- Tickets de impresión: se prueban contra `tools/printer-sim` y con archivos golden (`make golden` y revisar el diff).
- Vectores de prueba que deben coincidir entre Go, TS y Dart van en `packages/testdata/`, no dentro de una prueba.

## Estado

| Ticket | Estado |
|---|---|
| F0-02 Monorepo | ✅ |
| F0-03 Tooling | ✅ Go (vet, golangci, gofumpt, ganchos). TS y Flutter se configuran al crear sus apps |
| F0-04 CI | ✅ `.github/workflows/ci.yml` + nocturno |
| F0-05 `make dev` | ✅ Postgres 17, VersityGW (S3 + Object Lock), Azurite (Azure Blob, ADR-0013), Mailpit, toxiproxy, simuladores |
| F0-06 Simulador de impresoras | ✅ `tools/printer-sim` |
| F0-07 Stub del SRI | ✅ `tools/sri-stub` |
| F0-11 Dinero, IDs, reloj | ✅ Go. TS y Dart pendientes (usar `packages/testdata/`) |
| F0-08 Spike SRI | 🟡 Clave de acceso lista. Falta XML + XSD + firma XAdES: requiere `.p12` de pruebas (DP-04) |
| F0-09 Spike impresión | 🟡 Librería ESC/POS lista y probada con el simulador. Falta hardware real (DP-04) |
| F0-10 Spike sync | ✅ 10 000 eventos con caos: 0 pérdidas, 0 duplicados (ADR-0012, `packages/go/edgesync`) |
| F0-12 Sistema de diseño | ✅ Tokens únicos → CSS, Tailwind v4, TS y Flutter; contraste AA verificado en CI; componentes CSS; guía viva (`make ui-docs`) |
| F0-01, F0-13, F0-14 | ⏳ Requieren decisiones, usuarios reales o descargar la ficha del SRI |
| F1-01…F1-09, F1-13 | ✅ API Go (`apps/cloud-api`): RLS forzado + QA-06 en 16 tablas, QA-11, auth completa, catálogo, salón, personal, imágenes WebP + galería de 24 fotos CC0 |
| F1-10…F1-12 | ✅ Backoffice React (`apps/backoffice-web`): login promocional, guía de 5 pasos, menú con fotos, salón, personal con PIN, ajustes |
| F2-01 | ✅ Nodo Local (`apps/edge-node`): servicio de Windows/systemd, SQLite con un solo escritor, watchdog, prueba kill -9 |
| F2-02 | ✅ Activación por código (nube + nodo + pantalla en el backoffice y en el nodo), ADR-0014 |
| F2-03 | ✅ Nube → Nodo: feed de cambios por tenant, long-poll con NOTIFY (33 ms medido), volcado consistente, réplica tolerante a columnas nuevas (ADR-0015) |
| F2-04 | ✅ Push del outbox con tenant/RLS, heartbeat con telemetría y alerta de reloj |
| F1-14 | 🟡 Imagen Docker lista (`deploy/docker/cloud-api.Dockerfile`, `-tags nodynamic`). Falta IaC (Terraform en `deploy/azure/`, ADR-0013) y despliegue a `dev` |

Notas para seguir:
- Detrás de un proxy (VM con Caddy/Nginx) el bloqueo por IP de login y de activación de nodos debe leer la IP real desde un proxy de confianza (pendiente del despliegue).
- `make nodo` corre un Nodo Local en http://localhost:7080 contra la API local.
- El cliente TS del backoffice está escrito a mano (`src/api/types.ts`); generarlo desde OpenAPI queda pendiente (contrato en `contracts/openapi/`).
- Refresh con periodo de gracia de 30 s para carreras benignas (recarga durante la renovación, peticiones simultáneas); fuera de la gracia es robo y revoca la familia.
