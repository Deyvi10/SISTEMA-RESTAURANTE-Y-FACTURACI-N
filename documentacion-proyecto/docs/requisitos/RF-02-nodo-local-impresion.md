# RF-02 · Nodo Local (Edge) e impresión

## RF-02-01 · Instalación y activación del Nodo Local
- **Prioridad:** M · **Fase:** 2
- **Criterios de aceptación:**
  1. Instalador de un clic para Windows 10/11 x64 (MSI firmado). Linux (`.deb` + systemd) es **S**. macOS es **C**.
  2. Se instala como **servicio del sistema operativo**: arranca con el equipo y se reinicia solo si falla.
  3. Activación con un **código de activación** de un solo uso generado en el backoffice para un local específico. El nodo recibe sus credenciales (certificado/llave de dispositivo), el `tenant_id` y el `local_id`.
  4. Tras la activación descarga el catálogo, la configuración, el personal y las mesas, y queda operativo sin reiniciar.
  5. Expone una página local de estado (`http://<ip-nodo>:<puerto>/estado`) con versión, conectividad a la nube, cola de sincronización, impresoras y comprobantes pendientes.

## RF-02-02 · Descubrimiento de impresoras ("Zero-Setup Printing")
- **Prioridad:** M · **Fase:** 2
- **Criterios de aceptación:**
  1. El nodo detecta impresoras por (a) mDNS/Bonjour, (b) escaneo de la subred local en el puerto TCP 9100 y (c) enumeración USB (Windows).
  2. Cada impresora detectada aparece en el backoffice con modelo o nombre, conexión, IP o puerto y un botón **"Imprimir prueba"**.
  3. Si la IP de una impresora de red cambia, el nodo la reubica por su dirección MAC (tabla ARP) y actualiza la configuración sin intervención.
  4. Se muestra una recomendación guiada para reservar la IP en el router (DHCP reservation).
  5. Ancho de papel configurable: 58 mm u 80 mm.

## RF-02-03 · Estaciones y ruteo por arrastrar y soltar
- **Prioridad:** M · **Fase:** 2
- **Criterios de aceptación:**
  1. El Admin crea **estaciones** (Cocina caliente, Cocina fría, Bar, Sushi…) y asigna a cada una una o más impresoras y/o un KDS.
  2. Interfaz drag & drop: se arrastran **categorías** a estaciones. También se puede sobrescribir la estación de un **producto** puntual.
  3. Existe una estación de **caja** para pre-cuentas y comprobantes.
  4. Un producto sin estación asignada usa la estación por defecto del local (nunca se pierde una comanda).

## RF-02-04 · Impresión de comandas concurrente
- **Prioridad:** M · **Fase:** 2
- **Criterios de aceptación:**
  1. Al recibir una comanda, el nodo agrupa las líneas por estación, genera el documento ESC/POS de cada una y envía a las impresoras **en paralelo** (una goroutine por impresora).
  2. **p95 ≤ 1,5 s** desde que el mesero pulsa "Enviar" hasta que la impresora recibe el último byte (red local sana).
  3. Cada estación tiene su **cola persistente** (SQLite). Si una impresora falla, sus trabajos no se pierden y se reintentan; las demás estaciones no se bloquean.
  4. Contenido mínimo del ticket: estación, mesa u orden, mesero, hora, número de comanda, cantidades, producto, modificadores y notas **por línea**, y la marca "REIMPRESIÓN" si corresponde.
  5. Las líneas eliminadas después de enviadas imprimen un ticket de **ANULACIÓN** en la estación correspondiente.

## RF-02-05 · Detección de fallos de impresora
- **Prioridad:** M · **Fase:** 2
- **Criterios de aceptación:**
  1. El nodo detecta sin conexión, sin papel y tapa abierta (vía estado ESC/POS `DLE EOT` cuando la impresora lo soporte).
  2. Ante un fallo: alerta en la pantalla del cajero y notificación al mesero dueño de la comanda en menos de 3 s.
  3. Opción de **redirigir** la cola de una estación a otra impresora con un toque.

## RF-02-06 · Reimpresión
- **Prioridad:** S · **Fase:** 2
- **Criterios de aceptación:**
  1. Se puede reimprimir una comanda, pre-cuenta o comprobante desde la caja; la reimpresión queda auditada.
  2. El ticket reimpreso muestra "REIMPRESIÓN" y la hora original.

## RF-02-07 · Cajón de dinero
- **Prioridad:** M · **Fase:** 4
- **Criterios de aceptación:**
  1. El cajón se abre mediante pulso ESC/POS a través de la impresora de caja al registrar un cobro en efectivo.
  2. Apertura sin venta solo con permiso y siempre auditada.

## RF-02-08 · Servidor local de tiempo real
- **Prioridad:** M · **Fase:** 2
- **Criterios de aceptación:**
  1. El nodo expone una API HTTP y un endpoint WebSocket para la app de meseros, la caja web (cuando está en el local) y el KDS.
  2. Descubrimiento del nodo por la app: mDNS (`_restpos._tcp`) con IP manual como alternativa.
  3. Comunicación cifrada (TLS con certificado emitido por la nube para el nodo, anclado en la app en el momento del emparejamiento).
  4. Soporta ≥ 50 dispositivos conectados simultáneamente con p95 de difusión de eventos ≤ 200 ms.

## RF-02-09 · Operación sin internet
- **Prioridad:** M · **Fase:** 2-5
- **Criterios de aceptación:**
  1. Sin conexión a la nube funcionan: login por PIN, apertura y bloqueo de mesas, pedidos, impresión, pre-cuentas, cobros, emisión de comprobantes (clave de acceso + impresión) y cierre de turno.
  2. Sin internet **no** funcionan (y la UI lo indica claramente): cambios de menú desde el backoffice en la nube, autorización SRI, envío de correos y dashboard remoto.
  3. Un indicador visible en caja y app muestra el estado: 🟢 En línea · 🟡 Sin internet (operando local) · 🔴 Sin Nodo Local.
  4. Al volver la conexión, la sincronización se completa sin intervención humana (ver `03-arquitectura.md` §5).

## RF-02-10 · Respaldo local y recuperación
- **Prioridad:** M · **Fase:** 6
- **Criterios de aceptación:**
  1. Respaldo diario automático (por defecto a las 03:30, hora del local) con la API de backup de SQLite, sin detener la operación.
  2. El respaldo se comprime y **cifra** (AES-256-GCM) antes de subirse a almacenamiento de objetos. Retención: 7 diarios + 4 semanales.
  3. Recuperación en una PC nueva: instalar, activar con un código nuevo, y el nodo reconstruye su estado desde la nube + el último respaldo, en ≤ 15 minutos.
  4. La activación de un nodo nuevo para el mismo local **revoca** al anterior (evita dos nodos emitiendo con el mismo punto de emisión).

## RF-02-11 · Actualización automática del nodo
- **Prioridad:** M · **Fase:** 6
- **Criterios de aceptación:**
  1. El nodo consulta periódicamente si hay una versión nueva; los binarios están **firmados** y el nodo verifica la firma antes de instalar.
  2. La actualización solo se aplica **fuera del horario de servicio** (ventana configurable) o de forma manual.
  3. Si la nueva versión no pasa su *health check* en 60 s, vuelve automáticamente a la anterior.
