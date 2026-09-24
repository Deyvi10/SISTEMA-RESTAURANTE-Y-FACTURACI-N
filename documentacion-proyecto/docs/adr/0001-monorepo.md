# ADR-0001 · Monorepo único para todas las aplicaciones

- **Estado:** Propuesto
- **Fecha:** 2026-09-24
- **Relacionado con:** hallazgo T-05, DP-13

## Contexto
El documento fuente propone "repositorios aislados" por tecnología. El sistema tiene 7 aplicaciones (API, Nodo, App, POS, Backoffice, KDS, Menú QR) que comparten **contratos** (API REST, eventos WebSocket, eventos de sync) y lógica de dominio (cálculo de impuestos, clave de acceso, dinero). Un cambio típico (p. ej. un campo nuevo en la orden) toca 3 o 4 aplicaciones a la vez.

## Decisión
Un solo repositorio con la estructura de `docs/03-arquitectura.md` §12. Los contratos viven en `contracts/` y los clientes se generan. El CI usa filtros por ruta para construir solo lo afectado.

## Alternativas consideradas
| Opción | Pros | Contras |
|---|---|---|
| Multi-repo | Permisos y pipelines aislados | Cambios no atómicos, versiones de contratos desalineadas, más overhead para un equipo pequeño |
| **Monorepo** | Cambios atómicos, un solo PR por feature, contratos siempre sincronizados, documentación junto al código | El CI necesita filtros por ruta; el repo crece |

## Consecuencias
- Un PR puede cambiar el contrato, el backend y los clientes de forma coherente.
- Se requiere disciplina de límites entre `apps/` (no importar código de otra app; lo compartido va a `packages/`).
