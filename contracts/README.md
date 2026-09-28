# contracts

Fuente de verdad de las interfaces (docs/03 §6). Los tipos de Go, TS y Dart **se generan** desde aquí.

- `openapi/`: REST de la nube y del nodo (`/v1`).
- `events/`: JSON Schema de mensajes WebSocket y de sincronización (campo `v` / `type` + `version`).

## Eventos (F2-06)

- Cada archivo `events/<tipo>.schema.json` describe los datos (`data`) de un evento. Las propiedades se escriben en `snake_case`; `x-event-version` es la versión vigente.
- `_envelope.schema.json` es el sobre común `{v, id, type, ts, data}`.
- `make contracts` genera los tipos:
  - Go: `packages/go/eventos/eventos.gen.go`
  - TypeScript: `packages/ts/contracts/src/eventos.gen.ts`
  - Dart: `packages/dart/restpos_contracts/lib/src/eventos.g.dart`
- El CI falla si los tipos generados no coinciden con los esquemas.
- **Cambios compatibles** (agregar un campo opcional) mantienen la versión. Un cambio incompatible sube `x-event-version` y el nodo sigue aceptando la versión anterior (N y N-1).
