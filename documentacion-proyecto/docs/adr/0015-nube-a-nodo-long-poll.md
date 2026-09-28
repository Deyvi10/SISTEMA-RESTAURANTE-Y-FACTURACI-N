# ADR-0015 · Nube → Nodo: feed de cambios por tenant con long-poll

- **Estado:** Aceptado
- **Fecha:** 2026-09-25
- **Relacionado con:** F2-03, ADR-0004, `03-arquitectura.md` §5

## Contexto
ADR-0004 proponía llevar a los nodos los cambios de catálogo, salón y personal «por WebSocket y pull de respaldo». Un WebSocket entre la nube y cada nodo obliga a mantener conexiones largas a través de los proxies y del balanceador, a gestionar la reconexión y a tener un segundo protocolo además del pull, que igual hace falta.

## Decisión
- **Feed de cambios en PostgreSQL:**
  - El trigger `registrar_cambio` escribe cada INSERT, UPDATE o DELETE de las tablas replicadas en `sync_cambios`.
  - Cada cambio lleva un `seq` por tenant **sin huecos**: el contador se incrementa bloqueando la fila del tenant hasta el commit. Así ningún cambio se confirma «detrás» de uno que el nodo ya leyó.
- **Un solo endpoint:** `GET /v1/sync/pull?desde=N&esperar=25[&volcado=1]`.
  - Si no hay nada nuevo, la nube espera hasta 25 s (long-poll).
  - El trigger hace `pg_notify` y la nube despierta al instante a los nodos del tenant.
  - Medido en local: un cambio de precio llega a la base del nodo en **33 ms**.
- **Volcado completo** en una sola foto (`REPEATABLE READ`) cuando:
  - el nodo se activa;
  - toca el repaso diario;
  - el cursor del nodo está adelantado (la nube se restauró);
  - ya se purgaron cambios que el nodo necesita.
- **Filtrado por local:** los cambios de otros locales no se envían, pero el cursor avanza igual.
- **Minimización:** el nodo nunca recibe `password_hash`, `totp_secret_cifrado` ni correos.
- **Orden de sincronización:** antes de cada pull, el nodo envía su outbox pendiente (docs/03 §5.6).

## Alternativas consideradas
| Opción | Motivo del descarte |
|---|---|
| WebSocket Nube → Nodo | Conexiones largas a través de proxies, reconexión y un segundo protocolo que no mejora la latencia frente al long-poll |
| Pull cada N segundos | Latencia de N segundos y más carga |
| `bigserial` global como cursor | Las transacciones se confirman en otro orden que el de sus números: un nodo podría saltarse un cambio |

## Consecuencias
- Las escrituras del backoffice de un mismo tenant se serializan en el contador. Su volumen es bajo (personas editando el menú), así que no se nota.
- `sync_cambios` crece. Hay que purgar lo que tenga más de 30 días (F6); un nodo más atrasado recibe un volcado completo.
- La réplica del nodo guarda solo las columnas que conoce, así que una nube más nueva no rompe a un nodo viejo.
