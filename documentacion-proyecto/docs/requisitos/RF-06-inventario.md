# RF-06 · Inventario, recetas y stock diario

## RF-06-01 · Insumos y unidades de medida
- **Prioridad:** M · **Fase:** 7
- **Criterios de aceptación:**
  1. Cada insumo tiene nombre, categoría, **unidad de compra** (kg, quintal, caja x24, litro…), **unidad de consumo** (g, ml, unidad) y **factor de conversión** (1 kg = 1000 g; 1 quintal = 45,36 kg = 45 359,24 g).
  2. Catálogo de unidades base (masa, volumen, unidad) con conversiones estándar precargadas y la posibilidad de crear unidades de empaque propias.
  3. Stock mínimo por insumo y costo promedio ponderado.
  4. El stock se muestra en ambas unidades: "5,00 kg (5000 g)".
  5. Todas las cantidades usan decimal exacto (`NUMERIC(14,4)`), nunca coma flotante.

## RF-06-02 · Compras (ingreso de mercadería)
- **Prioridad:** M · **Fase:** 7
- **Criterios de aceptación:**
  1. Registro de compras con proveedor, fecha, número de factura del proveedor, ítems, cantidad en unidad de compra y costo unitario.
  2. Al confirmar, se inserta un movimiento `COMPRA` en el kardex (en unidad de consumo) y se recalcula el **costo promedio ponderado**.
  3. Importación del XML de la factura electrónica del proveedor (**S**): lee el XML autorizado y precarga los ítems.

## RF-06-03 · Recetas (BOM)
- **Prioridad:** M · **Fase:** 7
- **Criterios de aceptación:**
  1. Un producto vendible puede ser **Simple** (descuenta 1 unidad de sí mismo, p. ej. agua embotellada) o **Receta** (descuenta insumos).
  2. Constructor visual: se arrastran insumos o preparaciones al producto y se indica la cantidad en unidad de consumo. Se incluyen empaques (caja de cartón, vaso).
  3. Los **modificadores** también pueden tener receta (Extra queso → +20 g de queso; "Sin cebolla" → −15 g de cebolla).
  4. Costo teórico del plato = Σ(cantidad × costo promedio del insumo), recalculado al cambiar costos.
  5. Alerta cuando el **margen** del plato cae por debajo de un umbral configurable (por defecto 30 %).
  6. Las recetas son **versionadas**: un cambio de receta no altera el costo de ventas pasadas.

## RF-06-04 · Preparaciones (sub-recetas / batching)
- **Prioridad:** S · **Fase:** 7
- **Criterios de aceptación:**
  1. Un ítem tipo `PREPARACION` (Salsa BBQ) tiene su propia receta y rendimiento (1 lote = 5000 ml).
  2. Acción "Producir N lotes": en **una sola transacción** descuenta los insumos (`PRODUCCION_SALIDA`) e ingresa la preparación (`PRODUCCION_ENTRADA`) con el costo calculado.
  3. Se detectan y rechazan ciclos (A usa B y B usa A).
  4. Anidamiento máximo: 3 niveles.

## RF-06-05 · Descarga automática por venta
- **Prioridad:** M · **Fase:** 7
- **Criterios de aceptación:**
  1. Al **enviar** una comanda (momento configurable: al enviar a cocina o al cobrar; por defecto al enviar), se insertan movimientos `VENTA` por cada insumo de la receta (y de sus modificadores).
  2. Anular una línea enviada genera el movimiento inverso **solo si** el usuario indica que el plato no se preparó; si se preparó, se registra como **merma**.
  3. El kardex es **append-only**: nunca se hace UPDATE ni DELETE de un movimiento.
  4. El saldo por insumo se actualiza en la **misma transacción** que el movimiento (tabla de saldos), así el stock es exacto en tiempo real.
  5. Se permite stock negativo (no se frena una venta por inventario desactualizado), pero se marca en rojo y se reporta.
  6. Prueba obligatoria: vender N platos con receta y verificar que el saldo resultante es exacto al 4.º decimal.

## RF-06-06 · Alertas de stock
- **Prioridad:** M · **Fase:** 7
- **Criterios de aceptación:**
  1. Un insumo bajo su stock mínimo aparece en rojo y genera una alerta en el dashboard.
  2. Resumen diario opcional por correo con los insumos por debajo del mínimo.

## RF-06-07 · Toma física ciega y mermas
- **Prioridad:** S · **Fase:** 7
- **Criterios de aceptación:**
  1. Interfaz para tablet: el bodeguero ve la lista de insumos a contar **sin** la cantidad teórica e ingresa lo contado.
  2. Al finalizar, el sistema calcula las diferencias y exige un **motivo** por cada una (Caducidad, Error de porción, Derrame, Robo, Error de conteo…).
  3. Se genera un movimiento `AJUSTE` por diferencia, valorizado al costo promedio.
  4. Reporte de mermas por periodo, motivo e insumo, en dinero.

## RF-06-08 · Comportamiento de stock: PERMANENTE vs DIARIO
- **Prioridad:** M · **Fase:** 7
- **Criterios de aceptación:**
  1. Cada producto vendible tiene `comportamiento_stock`: `NINGUNO` (no controla disponibilidad), `PERMANENTE` (disponibilidad = stock del kardex, p. ej. bebidas) o `DIARIO` (cupo por jornada, p. ej. platos del día).
  2. Productos DIARIO: al abrir la jornada su cupo es **0** y aparecen como "Por habilitar". El Admin o el chef ingresa las cantidades preparadas ("30 Secos de pollo, 15 Ceviches").
  3. Al **cerrar la jornada** (no el turno de caja), el cupo sobrante se registra en el reporte y se reinicia.
  4. Los productos PERMANENTE no requieren acción diaria.

## RF-06-09 · Cupos en tiempo real con reservas temporales
- **Prioridad:** M · **Fase:** 7
- **Criterios de aceptación:**
  1. La tarjeta del producto muestra "Quedan N": verde (> 5), amarillo (≤ 5), rojo (1). Con 0 queda deshabilitada con la etiqueta "Agotado".
  2. Al agregar al carrito, la app solicita una **reserva** al Nodo Local. El nodo descuenta del disponible de forma atómica y difunde el nuevo valor a todos los dispositivos (≤ 300 ms).
  3. Si ya no hay cupo, la reserva se rechaza y la app lo indica al instante.
  4. Al eliminar la línea del carrito, la reserva se libera y se difunde.
  5. Toda reserva vence a los **10 minutos** sin heartbeat de la orden (configurable). Una reserva vencida se libera sola.
  6. Al **enviar la comanda**, la reserva se convierte en consumo definitivo.
  7. Prueba obligatoria: con 2 unidades disponibles y 3 solicitudes simultáneas, exactamente 2 reservas tienen éxito.

## RF-06-10 · Recarga rápida de cupo
- **Prioridad:** M · **Fase:** 7
- **Criterios de aceptación:**
  1. Panel lateral "Platos limitados" en caja y KDS con botón `[+]` y teclado numérico: "+30" en 2 toques.
  2. La recarga se difunde a todos los dispositivos y reactiva el producto en ≤ 1 s.
  3. Cada recarga se **audita** (usuario, hora, cantidad) y aparece en el reporte diario: inicial + recargas − vendidos = sobrante.

## RF-06-11 · Traslados entre locales
- **Prioridad:** C · **Fase:** 11+ (plan Pro)
