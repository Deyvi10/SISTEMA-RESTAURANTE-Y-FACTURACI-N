# ADR-0007 · Colas de trabajo sobre PostgreSQL

- **Estado:** Propuesto
- **Fecha:** 2026-09-24
- **Relacionado con:** hallazgo T-03

## Contexto
Se necesitan trabajos en segundo plano con reintentos (SRI, correos, PDFs, reportes). El documento fuente sugiere Redis.

## Decisión
Usar una cola sobre PostgreSQL (**River** para Go, basada en `SELECT … FOR UPDATE SKIP LOCKED`). El encolado ocurre **en la misma transacción** que el cambio de negocio: si la transacción falla, el trabajo no existe; si confirma, el trabajo no se pierde.

## Alternativas consideradas
| Opción | Motivo del descarte |
|---|---|
| Redis (asynq) | Infraestructura adicional; sin atomicidad con la base de datos (riesgo de trabajos huérfanos o perdidos) |
| SQS / colas administradas | Acopla al proveedor; mismo problema de atomicidad (requiere outbox) |

## Consecuencias
- Menos piezas que operar.
- Escala de sobra para miles de comprobantes por minuto; si algún día no alcanza, se migra con el patrón outbox.
- Redis queda como opción futura **solo** para fan-out de eventos en tiempo real entre varias instancias de la API (SSE).
