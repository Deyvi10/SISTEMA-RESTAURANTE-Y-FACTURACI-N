# ADR-0004 · Sincronización Edge-Cloud por propiedad de datos + outbox

- **Estado:** Propuesto
- **Fecha:** 2026-09-24
- **Relacionado con:** hallazgo X-09, `03-arquitectura.md` §3 y §5

## Contexto
El Nodo Local opera sin internet y la nube recibe cambios del backoffice. La "sincronización bidireccional silenciosa" del documento fuente no define qué pasa ante escrituras concurrentes sobre el mismo dato.

## Decisión
1. **Propiedad por dominio** (tabla en `03-arquitectura.md` §3): cada entidad tiene un único escritor. La nube es dueña del catálogo y la configuración; el nodo es dueño de la operación, los secuenciales y los movimientos de venta.
2. **Transactional outbox** en el nodo: cada escritura operativa inserta un evento en `outbox` en la misma transacción.
3. **Push** idempotente nodo → nube con `(node_id, node_seq)`; **pull/push** nube → nodo con cursores por flujo.
4. **Snapshots** de precio e impuesto en las líneas de venta.
5. La única entidad con varios escritores (`clientes`) usa *last-writer-wins* por campo con versión.

## Alternativas consideradas
| Opción | Motivo del descarte |
|---|---|
| CRDTs genéricos | Complejidad innecesaria cuando la propiedad por dominio elimina los conflictos |
| Replicación lógica de PostgreSQL ↔ SQLite | No existe de forma nativa; frágil y difícil de filtrar por tenant |
| Soluciones como PowerSync o ElectricSQL | Dependencia externa en el corazón del producto; se puede reevaluar |

## Consecuencias
- El backoffice no puede editar datos operativos del día (p. ej. una orden abierta); las correcciones se hacen en el local o mediante eventos compensatorios.
- Hace falta monitorear el tamaño y la antigüedad del outbox.
