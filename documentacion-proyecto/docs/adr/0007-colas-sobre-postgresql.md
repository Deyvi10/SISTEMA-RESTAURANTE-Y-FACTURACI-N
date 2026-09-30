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

## Nota de implementación (2026-09-29)
Todavía no se incorporó River: las dos colas que existen usan **la propia tabla de negocio como cola**, con el mismo principio (el trabajo nace en la transacción del cambio y se toma con `FOR UPDATE SKIP LOCKED`):

- **Correo del Cierre Z** (`cierres_z` sin fila en `cierres_z_envios`).
- **Worker fiscal** (F5-07/F5-10, `apps/cloud-api/internal/fiscal`): un comprobante en `EN_NUBE`, `FIRMADO` o `RECIBIDO` con `proximo_intento_at` vencido es un trabajo. La función `comprobantes_pendientes()` (SECURITY DEFINER) da los IDs de todos los restaurantes; cada uno se reclama dentro de su tenant (RLS) con `SKIP LOCKED` y una concesión de 2 min en `proximo_intento_at`, así la llamada al SRI no mantiene una transacción abierta y, si el worker muere, otro lo retoma. `LISTEN comprobantes` despierta al worker al instante. Probado con dos workers a la vez: cada comprobante se descifra, firma y envía una sola vez.

El estado del comprobante *es* el estado del trabajo, sin tabla de trabajos aparte que pueda desincronizarse. River se reconsidera cuando haya trabajos sin tabla de negocio natural (reportes, PDF masivos).
