# RF-03 · Salón y comandas (app de meseros)

## RF-03-01 · Configuración del plano del salón
- **Prioridad:** M (cuadrícula) / S (plano 2D libre) · **Fase:** 1 / 3
- **Criterios de aceptación:**
  1. El Admin crea **zonas** (Salón, Terraza, Barra) y **mesas** con nombre, capacidad y forma (cuadrada, redonda, rectangular).
  2. Editor de plano 2D: arrastrar, rotar y redimensionar mesas sobre una grilla. Se guarda la posición (x, y, rotación) relativa a la zona.
  3. Los cambios se reflejan en todos los dispositivos sin reiniciar la app.

## RF-03-02 · Mapa de mesas en vivo
- **Prioridad:** M · **Fase:** 3
- **Criterios de aceptación:**
  1. Colores de estado: **Libre** (verde), **Ocupada** (amarillo), **Por pagar / pre-cuenta impresa** (azul), **Bloqueada por otro usuario** (gris con candado y "Editando: <nombre>").
  2. Indicadores secundarios: 🍽️ platos listos por entregar; ⏱️ tiempo desde la apertura; rojo suave si la mesa espera la cuenta más de N minutos (configurable, por defecto 10).
  3. Los cambios de estado de cualquier mesa se ven en todos los dispositivos en **p95 ≤ 300 ms**.

## RF-03-03 · Bloqueo pesimista de mesa (anti-colisión)
- **Prioridad:** M · **Fase:** 3
- **Descripción:** Solo un usuario puede editar una mesa a la vez.
- **Criterios de aceptación:**
  1. Al entrar a una mesa, la app solicita `LOCK` al Nodo Local. El nodo concede el bloqueo de forma **atómica**: si dos solicitudes llegan a la vez, exactamente una obtiene el bloqueo y la otra recibe `LOCKED_BY <usuario>`.
  2. El nodo difunde `table.locked` a todos los dispositivos.
  3. Mientras la pantalla de la mesa está abierta, la app envía un *heartbeat* cada 10 s.
  4. El bloqueo se libera al enviar la comanda, al salir de la mesa, o automáticamente a los **45 s sin heartbeat** (celular apagado o sin batería).
  5. Un Admin puede forzar la liberación (auditado).
  6. Prueba obligatoria: test de concurrencia en Go con ≥ 100 solicitudes simultáneas sobre la misma mesa → exactamente 1 éxito.

## RF-03-04 · Toma de pedido offline-first
- **Prioridad:** M · **Fase:** 3
- **Criterios de aceptación:**
  1. Al iniciar el turno (y ante cada cambio), la app guarda en su SQLite local el catálogo completo: categorías, productos, precios, modificadores, imágenes en miniatura y disponibilidad.
  2. Agregar un producto responde en **≤ 50 ms** (operación local, sin red).
  3. Si el Nodo Local no está alcanzable, la orden queda en el teléfono como **"Pendiente de envío"** con un indicador visible, y se envía automáticamente al recuperar la conexión, sin que el mesero haga nada.
  4. Cada línea y cada envío llevan un **UUID generado en el dispositivo** y una clave de idempotencia: reenviar la misma comanda nunca duplica platos.
  5. El mesero ve claramente qué líneas están **enviadas** y cuáles están **por enviar**.

> Nota de diseño: el escenario offline principal del sistema es "sin internet". El escenario "el celular sin WiFi interna" es secundario, ya que el mesero no puede imprimir en cocina hasta reconectarse; la app debe dejarlo claro con un aviso.

## RF-03-05 · Búsqueda predictiva de productos
- **Prioridad:** M · **Fase:** 3
- **Criterios de aceptación:**
  1. Búsqueda local con coincidencia aproximada, insensible a tildes y mayúsculas (`cev` → "Ceviche Mixto"; `sec pol` → "Seco de Pollo").
  2. **p95 ≤ 50 ms** por pulsación con un catálogo de 500 productos en el móvil de referencia (ver escenario en `RNF-no-funcionales.md`).
  3. También se busca por código corto o alias configurable ("CM" → Ceviche Mixto).
  4. Los resultados priorizan los productos más vendidos del local.

## RF-03-06 · Modificadores y notas por línea
- **Prioridad:** M · **Fase:** 3
- **Criterios de aceptación:**
  1. Cada producto puede tener **grupos de modificadores** con reglas: obligatorio u opcional, mínimo y máximo de selecciones, y precio adicional (p. ej. "Término de la carne" obligatorio 1 de 4; "Extras" opcional 0 a 3, Tocino +$1,00).
  2. Si un producto tiene modificadores obligatorios, al agregarlo se abre directamente el selector.
  3. Pulsación prolongada sobre una línea abre la edición de modificadores y la **nota libre de esa línea**.
  4. Cada unidad es una línea independiente cuando tiene modificadores o notas distintas (3 hamburguesas → 3 líneas si difieren).
  5. Notas rápidas configurables por categoría ("Sin sal", "Extra ají", "Sin cebolla").

## RF-03-07 · Envío de comanda
- **Prioridad:** M · **Fase:** 3
- **Criterios de aceptación:**
  1. "Enviar" manda al Nodo Local **un** mensaje con todas las líneas nuevas. El nodo responde con acuse (ACK) en ≤ 300 ms, rutea por estación (RF-02-04) y libera el bloqueo de la mesa.
  2. Opción **"Enviar y mantener"** (para bebidas primero y platos después) y marcas de **tiempos** (Entrada / Fuerte / Postre) con envío diferido "Marchar fuerte".
  3. Una línea enviada no se puede editar; solo se puede anular (con permiso, auditado, con ticket de anulación en la estación).

## RF-03-08 · Gestos y navegación
- **Prioridad:** S · **Fase:** 3
- **Criterios de aceptación:**
  1. *Swipe* a la izquierda sobre una línea **no enviada** la elimina con respuesta háptica y opción "Deshacer" durante 4 s.
  2. Barra de pestañas inferior: **Salón**, **Orden actual**, **Avisos** (con contador).
  3. Todas las acciones primarias se pueden realizar con una sola mano (zona del pulgar).

## RF-03-09 · Pre-cuenta
- **Prioridad:** M · **Fase:** 3
- **Criterios de aceptación:**
  1. "Pre-cuenta" imprime en la estación de caja un ticket con la leyenda **"DOCUMENTO SIN VALOR TRIBUTARIO"**, detalle, subtotales, IVA, propina legal (si aplica) y total.
  2. La mesa pasa a estado **Por pagar**.
  3. Opción de pre-cuenta dividida en partes iguales (informativa).

## RF-03-10 · Transferencias y uniones
- **Prioridad:** S · **Fase:** 3
- **Criterios de aceptación:**
  1. Mover toda una orden de la Mesa A a la Mesa B (si B está libre).
  2. Mover líneas específicas entre mesas.
  3. Unir mesas (una orden para varias mesas físicas).
  4. Transferir la mesa a otro mesero.
  5. Todas estas acciones quedan auditadas y difundidas en tiempo real.

## RF-03-11 · Órdenes sin mesa
- **Prioridad:** S · **Fase:** 4
- **Criterios de aceptación:**
  1. Tipos de orden: **Mesa**, **Para llevar**, **Barra/Mostrador**, **Delivery propio**.
  2. Las órdenes sin mesa se identifican con un número o nombre corto del cliente, que se imprime en la comanda.

## RF-03-12 · Notificación de "plato listo"
- **Prioridad:** S · **Fase:** 9
- **Criterios de aceptación:**
  1. Cuando la cocina marca una comanda (o línea) como lista en el KDS, el nodo notifica **solo al mesero de esa orden** en ≤ 1 s.
  2. El teléfono vibra y muestra un aviso verde persistente: "Mesa 5: Hamburguesa lista".
  3. La pestaña *Avisos* conserva el historial del turno.
