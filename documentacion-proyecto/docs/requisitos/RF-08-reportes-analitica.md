# RF-08 · Reportes, analítica, propinas y auditoría

## RF-08-01 · Dashboard en tiempo real
- **Prioridad:** S · **Fase:** 8
- **Criterios de aceptación:**
  1. Widgets estilo iOS: **Ventas del día** (total y % frente al mismo día de la semana anterior **a la misma hora**), número de tickets, ticket promedio y comensales.
  2. **Mapa del salón en vivo** (mesas libres, ocupadas o esperando cuenta; rojo si llevan más de 45 min ocupadas).
  3. Top 5 productos y top 5 meseros (barras horizontales).
  4. Panel de **alertas críticas**: stock bajo, impresora desconectada, Nodo Local sin conexión, comprobantes sin autorizar, diferencia de caja, certificado por vencer.
  5. Actualización en vivo mediante SSE desde la nube (latencia ≤ 5 s respecto al evento en el local cuando hay internet).
  6. Selector de local (plan Pro: vista consolidada).

## RF-08-02 · Reportes de ventas
- **Prioridad:** M (básicos) · **Fase:** 5 (básicos) / 8 (completos)
- **Criterios de aceptación:**
  1. Ventas por rango de fechas agrupadas por día, hora, producto, categoría, mesero, método de pago, caja o turno.
  2. Listado de Cierres Z con detalle y PDF.
  3. Descuentos, cortesías y anulaciones por usuario.
  4. Todos los reportes se pueden exportar a Excel (`.xlsx`) y CSV.
  5. Los reportes usan la **fecha de negocio** (jornada), no la fecha del reloj.

## RF-08-03 · Reparto de propinas
- **Prioridad:** S · **Fase:** 8
- **Criterios de aceptación:**
  1. Reglas configurables: % para el fondo común (p. ej. cocina 30 %) y el resto repartido entre los meseros según (a) la propina de sus propias órdenes o (b) en partes iguales por turnos trabajados.
  2. Se genera el "Rol de propinas" por periodo en PDF y Excel, con detalle por persona.
  3. Solo se reparte la propina efectivamente **cobrada** (se excluyen propinas retiradas o cuentas revertidas).
  4. El redondeo cuadra al centavo con el total recolectado.

## RF-08-04 · Exportación contable
- **Prioridad:** M · **Fase:** 8
- **Criterios de aceptación:**
  1. Excel de ventas por periodo con columnas: fecha, tipo y número de comprobante, clave de acceso, identificación, cliente, base 0 %, base gravada por tarifa, IVA, propina, descuento, total, forma de pago y estado SRI.
  2. Excel de compras y de movimientos de inventario valorizados.
  3. Plantillas configurables de columnas para distintos sistemas contables (**C**).

## RF-08-05 · ATS (Anexo Transaccional Simplificado)
- **Prioridad:** C · **Fase:** 11+
- **Descripción:** Generación del XML del ATS según el XSD vigente. Requiere validación con un contador y aplica solo a ciertos contribuyentes (ver L-09 y DP-08).

## RF-08-06 · Auditoría inmutable
- **Prioridad:** M · **Fase:** 4 (registro) / 8 (visualización)
- **Criterios de aceptación:**
  1. Se registran como mínimo: login fallido, eliminación o anulación de líneas enviadas, descuentos, cortesías, retiro de propina, apertura de cajón sin venta, reimpresiones, transferencias de mesa, liberación forzada de bloqueo, notas de crédito, recarga de cupos, ajustes de inventario, cambios de precios y permisos, altas y bajas de usuarios y dispositivos, cambios fiscales y cierres Z con diferencias.
  2. Cada registro contiene: tenant, local, usuario, usuario autorizador (si hubo PIN de supervisor), dispositivo, acción, entidad afectada, valor antes y después, monto implicado, motivo y timestamp.
  3. La tabla es **append-only** (la base de datos rechaza UPDATE y DELETE mediante permisos y triggers). Cada registro incluye el hash del anterior (cadena de hash) para detectar manipulación.
  4. **Matriz de fugas**: vista filtrable que resalta en rojo las acciones con impacto económico, ordenadas por monto, usuario o fecha.

## RF-08-07 · Reportes de inventario
- **Prioridad:** S · **Fase:** 8
- **Descripción:** Kardex por insumo, consumo teórico vs real, mermas valorizadas, costo de ventas y margen por plato, y cupos diarios (inicial + recargas − vendido = sobrante).
