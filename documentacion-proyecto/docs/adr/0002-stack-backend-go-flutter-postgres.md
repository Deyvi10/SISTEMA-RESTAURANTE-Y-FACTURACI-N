# ADR-0002 · Stack base: Go, Flutter, PostgreSQL, SQLite

- **Estado:** Propuesto
- **Fecha:** 2026-09-24
- **Relacionado con:** documento fuente §1, RNF-01…08

## Contexto
Se requiere alta concurrencia (WebSockets, impresión paralela), un binario liviano instalable en PCs de restaurante, una app móvil fluida para Android e iOS, e integridad transaccional para datos financieros.

## Decisión
| Capa | Tecnología | Librerías base (a confirmar en la Fase 0) |
|---|---|---|
| API Cloud y Workers | **Go** (última versión estable) | `chi`, `pgx/v5`, `sqlc`, `goose` (migraciones), `slog`, OpenTelemetry, `shopspring/decimal` |
| Nodo Local | **Go** (mismo lenguaje, código de dominio compartido) | `modernc.org/sqlite` (sin CGO → compilación cruzada simple), `coder/websocket`, librería ESC/POS propia o adaptada |
| Base central | **PostgreSQL 16+** | RLS, `citext`, `pgcrypto` |
| Base local | **SQLite** (WAL) | — |
| App Meseros | **Flutter** (estable), estilo Cupertino | `riverpod`, `drift`, `go_router`, `mobile_scanner`, `flutter_secure_storage` |

## Alternativas consideradas
| Opción | Motivo del descarte |
|---|---|
| Node.js / Python para el backend | Mayor consumo de memoria para el nodo; distribución de un binario único más compleja |
| React Native | Flutter ofrece un rendimiento gráfico más consistente para la UI táctil intensiva y un solo código para tablets |
| Hive en el móvil | Sin consultas relacionales ni índices; drift/SQLite es consistente con el Edge |

## Consecuencias
- El equipo necesita competencia en Go, Dart y TypeScript.
- La firma XAdES en Go es un riesgo (ver ADR-0006 y el spike de la Fase 0).
