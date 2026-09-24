# ADR-0010 · Un solo módulo Go en la raíz del monorepo

- **Estado:** Aceptado
- **Fecha:** 2026-09-24
- **Relacionado con:** ADR-0001, ticket F0-02

## Contexto
F0-02 proponía un `go.work` con un módulo por app (`cloud-api`, `edge-node`, `packages/go`). Con un equipo pequeño eso multiplica los `go.mod`, las versiones de dependencias y la configuración del CI, sin beneficio real: las apps se compilan por separado de todos modos.

## Decisión
Un único `go.mod` en la raíz (`github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N`). Cada binario vive en `apps/<app>/cmd/<binario>` o `tools/<herramienta>` y solo compila los paquetes que importa. La regla de límites de ADR-0001 se mantiene: una app no importa `internal/` de otra; lo compartido va a `packages/go/`.

## Alternativas consideradas
| Opción | Pros | Contras |
|---|---|---|
| `go.work` con varios módulos | Dependencias aisladas por app | Más archivos que mantener, `replace` locales, CI más complejo |
| **Un módulo** | Una versión de cada dependencia, un `go test ./...`, cero fricción | Todas las apps comparten versiones de dependencias |

## Consecuencias
- `go test -race ./...` cubre todo el código Go.
- Si en el futuro una app necesita versiones distintas de una dependencia, se separa en su propio módulo con un ADR nuevo.
