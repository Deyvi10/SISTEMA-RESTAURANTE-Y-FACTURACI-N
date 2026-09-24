# ADR-0008 · Multi-tenancy con base de datos compartida + RLS

- **Estado:** Propuesto
- **Fecha:** 2026-09-24
- **Relacionado con:** documento fuente "Fase 5 Multi-Tenant", RNF-20

## Contexto
Se espera escalar de 1 a más de 1 000 restaurantes con costos controlados y aislamiento estricto.

## Decisión
Base de datos PostgreSQL **compartida**, columna `tenant_id` en toda tabla de negocio y **Row-Level Security** forzada (`FORCE ROW LEVEL SECURITY`). La API fija `app.tenant_id` con `SET LOCAL` al inicio de cada transacción, a partir del token. El rol de la aplicación no tiene `BYPASSRLS` ni es dueño de las tablas. Índices compuestos que empiezan por `tenant_id`.

## Alternativas consideradas
| Opción | Motivo del descarte |
|---|---|
| Base de datos por tenant | Operación costosa (migraciones × N, conexiones) |
| Esquema por tenant | Igual de costoso en migraciones; catálogo de PostgreSQL inflado |
| Solo `WHERE tenant_id` en el código | Un olvido expone datos; RLS es la segunda barrera |

## Consecuencias
- Hay que probar el aislamiento en el CI (QA-06).
- Cuidado con los *connection poolers* en modo *transaction* (usar `SET LOCAL`, nunca `SET`).
- Los tenants muy grandes pueden moverse a una instancia dedicada en el futuro sin cambiar el código.
