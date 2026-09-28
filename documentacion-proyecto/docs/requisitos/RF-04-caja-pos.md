# RF-04 · Caja / Punto de venta

## RF-04-01 · Jornada operativa
- **Prioridad:** M · **Fase:** 4
- **Criterios de aceptación:**
  1. El local **abre la jornada** (día de negocio) una vez al día. Todas las ventas, turnos y stock DIARIO pertenecen a una jornada.
  2. La jornada puede cruzar la medianoche (bares); la fecha de negocio es la de apertura.
  3. La jornada solo se puede cerrar cuando no quedan turnos de caja abiertos ni órdenes abiertas (o cuando estas se transfieren explícitamente a la jornada siguiente).

## RF-04-02 · Apertura de turno de caja
- **Prioridad:** M · **Fase:** 4
- **Criterios de aceptación:**
  1. El cajero abre el turno en una **caja** (asociada a un punto de emisión SRI) declarando el **fondo inicial** de efectivo.
  2. Una caja solo puede tener un turno abierto a la vez.
  3. Sin turno abierto no se puede cobrar en esa caja.

## RF-04-03 · Pantalla de cobro "Zero-Click"
- **Prioridad:** M · **Fase:** 4
- **Criterios de aceptación:**
  1. Vista de mesas u órdenes abiertas con búsqueda por número de mesa. La navegación completa es posible **solo con teclado** (atajos documentados).
  2. Búsqueda predictiva de productos para venta directa en mostrador (misma regla que RF-03-05).
  3. **Botones de billetes dinámicos**: para un total de $14,50 se muestran [$14,50 exacto] [$15] [$20] [$50]. Al tocar uno, se calcula el vuelto, se abre el cajón, se emite el comprobante y se libera la mesa **en un solo gesto**.
  4. Botón destacado **"Consumidor final"**, deshabilitado automáticamente si el total supera el límite legal vigente (RF-05-03).
  5. Desde "Facturar" hasta caja libre para el siguiente cliente: **p95 ≤ 2 s**, con o sin internet.

## RF-04-04 · Métodos de pago y pagos mixtos
- **Prioridad:** M · **Fase:** 4
- **Criterios de aceptación:**
  1. Métodos configurables, cada uno mapeado a un código de forma de pago del SRI: Efectivo, Tarjeta de crédito, Tarjeta de débito, Transferencia, DeUna/billeteras, Otros.
  2. Una cuenta puede pagarse con varios métodos (p. ej. $20 en efectivo + $15,40 con tarjeta).
  3. Tarjeta: registro opcional de lote, referencia y últimos 4 dígitos para el cuadre de vouchers.
  4. La integración directa con datafonos (Datafast/Medianet) está **fuera de alcance** en esta versión (C, Fase 11+).

## RF-04-05 · Datos del comprador con autocompletado
- **Prioridad:** M · **Fase:** 4-5
- **Criterios de aceptación:**
  1. Campo único "Cédula / RUC / Pasaporte". Se valida la cédula (10 dígitos, módulo 10) y el RUC (13 dígitos, validación según tipo) **en el cliente** antes de buscar.
  2. Búsqueda en cascada: (a) clientes en el Nodo Local, (b) clientes del tenant en la nube, (c) proveedor externo configurable (opcional, ver L-06 de la revisión).
  3. Si se encuentra, se autocompleta nombre, dirección, correo y teléfono en **≤ 300 ms** en los casos (a) y (b). Con **Enter** se factura.
  4. Si no se encuentra, formulario mínimo: nombre o razón social y correo (dirección y teléfono opcionales).
  5. El cliente queda guardado para futuras compras con consentimiento informado (LOPDP, ver `06-seguridad.md` §7).

## RF-04-06 · División de cuenta (Split Bill)
- **Prioridad:** M (partes iguales + por ítems) · **Fase:** 4
- **Descripción:** Una orden puede cobrarse en varias **cuentas**, cada una con su propio comprobante, cliente y métodos de pago.
- **Criterios de aceptación:**
  1. **Por ítems:** vista con las líneas de la orden a la izquierda y las cuentas a la derecha (`[+] Nueva cuenta`). Las líneas se asignan arrastrando o tocando. La UI se actualiza sin llamar al servidor.
  2. **Fracción de ítem:** una línea puede dividirse entre N cuentas (p. ej. 1 pizza entre 3). El redondeo se ajusta en la última cuenta para que la suma cuadre al centavo.
  3. **Partes iguales:** se ingresa N personas y el total se divide en N cuentas; el residuo de centavos se asigna a la última.
  4. La confirmación es **atómica**: se crean todas las cuentas o ninguna. La orden original **no se anula**; queda cerrada cuando todas sus cuentas están pagadas.
  5. Cada cuenta se cobra de forma independiente y en cualquier orden; la mesa se libera cuando la última cuenta queda pagada.

## RF-04-07 · Descuentos y cortesías
- **Prioridad:** S · **Fase:** 4
- **Criterios de aceptación:**
  1. Descuento por línea o por cuenta, en % o en monto, con **motivo obligatorio** de una lista configurable.
  2. Cortesía (100 %) con motivo y autorización.
  3. Límite máximo de descuento por rol o usuario.
  4. Los descuentos se reflejan en el XML (campo `descuento` por detalle y totales) según la ficha técnica.
  5. Todo descuento se audita (RF-08-06).

## RF-04-08 · Propina legal (10 % de servicio)
- **Prioridad:** M · **Fase:** 4
- **Criterios de aceptación:**
  1. Se activa o desactiva por local.
  2. Se calcula como 10 % de la **base imponible** (subtotal sin IVA, después de descuentos). **No** incluye el IVA ni grava IVA.
  3. El cajero puede retirar la propina si el cliente la rechaza (acción auditada).
  4. Se registra por orden y mesero para el reparto (RF-08-03) y se informa en el campo `<propina>` del XML.

## RF-04-09 · Movimientos de caja
- **Prioridad:** S · **Fase:** 4
- **Criterios de aceptación:**
  1. Registro de **retiros parciales** (sangrías a la caja fuerte) e **ingresos** no relacionados con ventas, con motivo.
  2. Registro de **gastos menores** pagados desde la caja (con foto del comprobante opcional).

## RF-04-10 · Cierre de turno ciego y Cierre Z
- **Prioridad:** M · **Fase:** 4
- **Criterios de aceptación:**
  1. Al cerrar el turno, el sistema **no muestra** los totales esperados al cajero.
  2. El cajero declara el efectivo contado (con asistente por denominación: billetes de $100…$1 y monedas) y el total de vouchers de tarjeta y de transferencias.
  3. El sistema calcula en el servidor (Nodo Local): esperado = fondo inicial + cobros en efectivo + ingresos − retiros − gastos. Luego muestra **Sobrante** / **Faltante** / **Cuadrado** por método.
  4. Se genera el **Cierre Z** como registro **inmutable** (sin UPDATE ni DELETE), con numeración secuencial por caja.
  5. El Cierre Z se imprime, se sincroniza a la nube, se genera en PDF y se envía por correo al dueño.
  6. Si la diferencia supera un umbral configurable, se genera una alerta crítica al dueño.
  7. Tras el cierre, la caja queda bloqueada hasta abrir un nuevo turno (no "hasta el día siguiente"; los turnos múltiples por día están permitidos).

## RF-04-11 · Reverso de ventas
- **Prioridad:** M · **Fase:** 5
- **Criterios de aceptación:**
  1. Una venta con comprobante **autorizado** solo se revierte emitiendo una **Nota de Crédito** electrónica (RF-05-08) con motivo, permiso y auditoría.
  2. Una venta cuyo comprobante aún **no se ha enviado** al SRI tampoco se borra: se emite igualmente la Nota de Crédito una vez autorizado el comprobante original.
  3. El reverso devuelve el inventario (movimiento `REVERSO_VENTA`) solo si el Admin lo indica (el plato pudo haberse preparado).
