# ADR-0012 · Resultado del spike de sincronización (F0-10)

- **Estado:** Aceptado
- **Fecha:** 2026-09-24
- **Relacionado con:** ADR-0004, `03-arquitectura.md` §5, QA-07, código en `packages/go/edgesync`

## Contexto
La sincronización Nodo → Nube era uno de los tres riesgos mayores. El criterio del spike: 10 000 eventos con cortes de red → 0 pérdidas y 0 duplicados.

## Resultado
`TestChaosSync` (corre en cada PR con Postgres real):

- 2 nodos en paralelo × 5 000 eventos confirmados = **10 000 eventos**.
- Red con fallas inyectadas en el cliente: ~10 % conexiones rechazadas, ~12 % **ACK perdido después de aplicar** (el caso peligroso), ~5 % HTTP 503, ~8 % latencia alta.
- 4 reinicios por nodo en pleno envío (se cancela el pusher y se cierra la base) y 1 % de transacciones revertidas.
- Resultado: **0 pérdidas, 0 duplicados, orden por nodo exacto**, cursor = total confirmado, y en cada nodo ventas = eventos. En una corrida típica hubo ~600 ACK perdidos, todos deduplicados. Tiempo: ~40 s con `synchronous=FULL` y detector de carreras.

## Decisiones que salen del spike
1. **Cursor contiguo por nodo en lugar de la tabla `sync_recibidos`.** La nube guarda `sync_cursores(nodo_id, ultimo_seq)` y lo bloquea `FOR UPDATE` en la transacción del lote: seq ≤ cursor se ignora (idempotencia), seq = cursor+1 se aplica, un hueco detiene el lote (orden). Da las mismas garantías que `(nodo_id, node_seq)` procesados con estado O(1). `sync_eventos` conserva `UNIQUE (nodo_id, node_seq)` como segunda barrera.
2. **El `node_seq` debe ser contiguo.** Se usa `INTEGER PRIMARY KEY AUTOINCREMENT` en SQLite: el contador forma parte de la transacción, así que un rollback no deja huecos (probado). Nunca se debe asignar el seq fuera de la transacción del negocio.
3. **Lote:** hasta 500 eventos o 1 MB; siempre al menos 1 evento aunque sea grande.
4. **Reintentos:** espera exponencial 1 s → 30 s con jitter ±20 % para que cientos de nodos no reintenten a la vez cuando vuelve internet.
5. **Seguridad del ACK:** si la nube confirma un seq mayor al enviado, el nodo no marca nada y alerta (identidad de nodo duplicada o respaldo viejo restaurado).

## Riesgos abiertos (para F2-04)
- **Evento venenoso:** un evento que la nube no puede aplicar bloquea al nodo, porque el orden es estricto. Mitigación a construir: tabla de cuarentena + alerta, sin saltar el orden de ese agregado.
- La identidad del nodo (mTLS o ed25519) y el `tenant_id` con RLS se agregan en F2-02/F2-04; el spike usa un esquema sin tenant.
- La dirección Nube → Nodo (inbox + cursores por flujo) no fue parte del spike; usa el mismo patrón de cursor.

## Consecuencias
- Se adopta el enfoque de ADR-0004 sin cambios de fondo; el paquete `edgesync` es la base de F2-03/F2-04.
- La prueba de caos queda como regresión obligatoria (QA-07).
