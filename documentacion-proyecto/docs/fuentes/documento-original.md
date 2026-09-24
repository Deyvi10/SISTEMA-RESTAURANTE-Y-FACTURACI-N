# Documento fuente original (extracción de texto)

> Transcripción literal de `documento-original.docx` para trazabilidad. **No editar.** Los documentos normativos del proyecto son los de `docs/`; este archivo solo sirve como referencia de origen.

## 1. Stack Tecnológico (Para velocidad extrema)
Para que el sistema sea "súper rápido", soporte múltiples meseros enviando comandas simultáneamente y enrute órdenes a distintas impresoras sin lag, debes abandonar lenguajes pesados y adoptar una arquitectura orientada a eventos.
- Backend (Lógica y API): Go (Golang). Es un lenguaje compilado, creado por Google, diseñado específicamente para alta concurrencia. Permite manejar miles de peticiones simultáneas (meseros, facturación SRI, impresoras) consumiendo muy poca memoria y con tiempos de respuesta de milisegundos.
- Frontend (Panel Web y Caja): React.js o Vue 3. Construyen interfaces de usuario reactivas que no recargan la página, haciendo que el cajero o administrador sienta que usa una aplicación de escritorio.
- Aplicación Móvil (Toma de órdenes): Flutter. Permite compilar código nativo para Android e iOS. Es extremadamente fluido (60-120 fps) e ideal para crear interfaces táctiles muy amigables y de botones grandes para el apuro de los meseros.
- Base de Datos: PostgreSQL para la integridad de los datos financieros e inventario, combinado con Redis (en memoria) para guardar temporalmente las comandas activas y sesiones, garantizando que una orden llegue a la cocina en menos de 1 segundo.
- Comunicación con Impresoras: Uso de WebSockets. En lugar de que el sistema "pregunte" cada 5 segundos si hay una nueva orden, el servidor "empuja" la orden instantáneamente a un nodo local (una pequeña app en la PC del restaurante conectada a las impresoras LAN o USB).
## 2. Análisis de los 10 Mejores Sistemas en Ecuador
A continuación, la investigación de los sistemas más utilizados o adaptados al mercado gastronómico y fiscal ecuatoriano:

| Sistema | Enfoque Principal | Pros | Contras | Oportunidad de Mejora para tu App |
|---|---|---|---|---|
| 1. Illarli | POS Web integral + SRI | Facturación nativa EC, robusto en inventarios. | Curva de aprendizaje moderada, UX algo tradicional. | Interfaz 100% enfocada en reducir clics; menos menús anidados. |
| 2. Restopedia | Operación gastronómica | Manejo excelente de mesas y cuentas divididas. | Puede resultar costoso o pesado para restaurantes muy pequeños. | Modelo SaaS más flexible (freemium o tiers más accesibles). |
| 3. Fudo | SaaS Gastronómico LATAM | UX/UI insuperable, extremadamente fácil de usar y rápido. | La integración con facturación electrónica de Ecuador requiere módulos externos. | Replicar su UX/UI, pero con facturación nativa directa al SRI. |
| 4. Contífico (Siigo) | ERP Contable | Integración contable perfecta, ideal para cadenas. | Es un sistema contable adaptado a restaurantes; lento para el cajero. | Separar el módulo de cajero/mesero (ultrarrápido) del módulo contable. |
| 5. MishkiTap | Menú QR y Auto-pedido | El cliente hace todo desde el celular; reduce personal. | Si el internet falla o el cliente prefiere atención humana, el sistema flaquea. | Modo híbrido fluido: que la app del mesero y el QR del cliente convivan en tiempo real. |
| 6. GastroEc | Gestión administrativa | Buen manejo de recetas y mermas. | Interfaz visualmente desactualizada. | Diseño minimalista y moderno, modo oscuro para bares. |
| 7. Dora POS | Automatización contable | Centraliza operación y facturas. | Complejo para la rotación alta de personal base (meseros). | "Modo entrenamiento" en tu app que guíe al nuevo empleado. |
| 8. Popapp | Integración Delivery | Conecta directo con UberEats, PedidosYa. | Menos fuerte en la gestión compleja del salón y cocina física. | API abierta desde el día 1 para conectar con apps de delivery. |
| 9. OlaClick | Pedidos por WhatsApp | Gratuito en su base, gran adopción rápida. | Carece de control de inventario profundo y multi-impresora. | Incluir un bot de WhatsApp nativo atado al inventario real. |
| 10. Zeta Software | Sucursales y Finanzas | Control estricto de costos y múltiples locales. | Demasiado robusto para el 80% de negocios gastronómicos. | Escalabilidad modular: el cliente paga solo por los módulos que activa. |
## 3. Radiografía Profunda: ILLARLI
Illarli es uno de los referentes en Ecuador por su estabilidad y homologación directa con el Servicio de Rentas Internas (SRI).
### ¿Cómo funciona?
Opera bajo un modelo Cloud (en la nube) pero con capacidades de impresión y conexión de periféricos locales. Utiliza una arquitectura web responsiva. Cuando un mesero toma la orden en una tablet, el sistema registra el pedido en el servidor central y envía la instrucción de impresión a la caja/cocina a través de un servicio de puente local. Maneja facturación electrónica en régimen RIMPE y General.
Puntos Fuertes (Lo que debes replicar):
- Estabilidad Fiscal: No depende de terceros para enviar XMLs al SRI, lo hacen directamente y manejan contingencia offline (si se cae el SRI, la factura se encola).
- Manejo de Recetas e Inventario: Permite descargar inventario en miligramos o gramos por cada plato vendido (ej: descontar 150g de carne por una hamburguesa).
- Multidispositivo real: Funciona en PC, Mac, Android o iOS porque corre desde el navegador.
Puntos Débiles (Donde tu sistema va a ganar):
- Velocidad de Interfaz (UX): Su interfaz web, aunque funcional, requiere varios recargos de pantalla o clics para tareas repetitivas. En hora pico, cada segundo cuenta.
- Configuración de Impresoras: Dependen a veces de configuraciones de red complejas para el usuario final (IPs estáticas en impresoras matriciales/térmicas).
- Curva de adopción: La parametrización inicial de ingredientes, platos y mesas puede ser tediosa sin asistencia de soporte técnico.
### La Estrategia de Mejora (Tu Ventaja Competitiva)
Para que tu sistema sea superior a Illarli, aplicaremos tres principios de ingeniería en las próximas fases de desarrollo:
- "Zero-Setup Printing": Desarrollar un pequeño agente instalable en Windows/Mac que auto-detecte impresoras USB/LAN en la red del restaurante mediante protocolos mDNS, eliminando la configuración de IPs manual. El administrador solo arrastrará en la pantalla: "Impresora 1 -> Cocina Caliente".
- App Nativa Offline-First para Meseros: A diferencia de Illarli que depende de señal WiFi constante para cargar la web, tu app móvil guardará el menú en caché. Si el mesero está en una esquina sin WiFi, toma la orden al instante, y apenas da un paso con señal, la orden se sincroniza silenciosamente mediante WebSockets en background.
- Onboarding Guiado por IA: Un asistente inicial que cargue el menú de un restaurante automáticamente subiendo una foto de la carta física, creando productos y precios en 30 segundos.
FASE 2
## 1. Definición de Roles (Actores del Sistema)
El sistema operará bajo un modelo de control de acceso basado en roles (RBAC). La interfaz (UI) se adaptará automáticamente ocultando menús innecesarios según quién inicie sesión, garantizando que sea "extremadamente fácil de usar".
- Administrador / Propietario (Admin): Tiene acceso total (Web/Móvil). Visualiza reportes en tiempo real, configura el menú (precios, recetas), gestiona el inventario, sube la firma electrónica (P12) para el SRI y mapea las impresoras.
- Mesero: Utiliza exclusivamente la Aplicación Móvil (Smartphone/Tablet). Su interfaz tiene botones grandes y alto contraste. Su único objetivo es despachar rápido: abrir mesas, tomar órdenes, enviar comandas y solicitar pre-cuentas.
- Cajero: Utiliza la interfaz Web de Escritorio o Tablet grande. Gestiona el flujo de dinero, divide cuentas, aplica descuentos, emite la Factura Electrónica autorizada por el SRI y realiza el cierre/arqueo de caja.
- Personal de Cocina / Bar (Cocinero): Interactúa con el sistema de dos formas: mediante los Tickets Físicos (impresos automáticamente) o mediante una pantalla KDS (Kitchen Display System) para marcar los platos como "En preparación" y "Listos".
## 2. Historias de Usuario (User Stories)
Para construir un sistema ágil, redactamos las historias bajo el formato estándar: Como [Rol], quiero [Acción] para [Beneficio]. A cada una le agregamos Criterios de Aceptación (DoD) técnicos para asegurar la velocidad extrema.
### Módulo: Toma de Órdenes (App Móvil)
HU-01: Toma de pedido ultra-rápida y offline
- Descripción: Como Mesero, quiero agregar productos a una mesa incluso si el internet es inestable, para no hacer esperar al cliente.
- Criterios de Aceptación:
  - La app debe guardar el menú completo en el caché del celular al iniciar turno.
  - Si no hay WiFi, la orden se guarda localmente (estado: Pendiente de envío).
  - Al recuperar señal, la app debe sincronizar vía WebSockets en background (sin interrumpir al usuario) en menos de 1 segundo.
  - El buscador de platos debe arrojar resultados en menos de 100 milisegundos a medida que el mesero teclea (búsqueda predictiva local).
### Módulo: Enrutamiento de Comandas (WebSockets)
HU-02: Distribución inteligente de tickets
- Descripción: Como Admin, quiero que las bebidas se impriman en el Bar y la comida en la Cocina de forma automática, para evitar confusiones y tráfico de meseros.
- Criterios de Aceptación:
  - Interfaz Drag & Drop (arrastrar y soltar) para vincular la Categoría "Bebidas" a la "Impresora IP: 192.168.1.10 (Bar)".
  - Cuando un mesero envía una orden mixta, el backend (en Go) debe dividir el JSON de la orden y disparar hilos concurrentes a cada impresora.
  - Tiempo máximo de impresión desde que el mesero presiona "Enviar": 1.5 segundos.
### Módulo: Facturación Electrónica (SRI Ecuador)
HU-03: Facturación a 1 clic
- Descripción: Como Cajero, quiero emitir una factura electrónica válida con el SRI escribiendo solo el número de cédula/RUC del cliente, para agilizar la fila de pagos.
- Criterios de Aceptación:
  - El sistema debe conectarse a una API de Registro Civil/SRI (o base de datos propia de clientes históricos) para autocompletar Nombre, Dirección y Teléfono al ingresar la Cédula/RUC.
  - El XML debe firmarse en el servidor y enviarse al entorno de Producción del SRI en un proceso asíncrono.
  - La interfaz del cajero no debe congelarse mientras el SRI autoriza. El ticket físico se imprime instantáneamente con la leyenda "Clave de acceso en proceso".
  - Opción directa de botón gigante: "Consumidor Final - Efectivo".
### Módulo: Control de Inventarios
HU-04: Descarga de recetas (Mermas automáticas)
- Descripción: Como Admin, quiero que al vender un plato se descuenten sus ingredientes exactos del inventario, para evitar robos y saber cuándo comprar insumos.
- Criterios de Aceptación:
  - Se debe poder crear un producto tipo "Receta" (Ej: Hamburguesa) compuesto por insumos (Ej: 150g carne, 1 pan, 20g queso).
  - El sistema debe alertar visualmente (rojo) cuando el stock de un insumo caiga por debajo del "Stock Mínimo".
## 3. Casos de Uso Detallados (Use Cases)
A continuación, el flujo lógico de los dos procesos más críticos del sistema.
### CU-01: Flujo Completo de Atención en Salón
- Actores: Mesero, Cocina, Cajero.
- Precondiciones: Turno de caja abierto, menú configurado, impresoras conectadas al agente local.
- Flujo Principal:
  - El Mesero abre la App en su celular, selecciona el mapa del salón y toca la "Mesa 4" (que está en color verde/libre). La mesa pasa a color amarillo (Ocupada).
  - El Mesero agrega: 1 Ceviche, 2 Cervezas. Presiona "Enviar Comanda".
  - El sistema enruta en tiempo real: Imprime ticket de Ceviche en Cocina y ticket de Cervezas en Bar.
  - El cliente termina y pide la cuenta. El Mesero presiona "Pre-cuenta". Se imprime un ticket no válido para crédito tributario en la caja para llevárselo al cliente.
  - El cliente paga con Tarjeta. El Mesero informa al Cajero.
  - El Cajero abre la Mesa 4, selecciona método de pago "Tarjeta", ingresa datos del cliente (o Consumidor Final) y presiona "Facturar".
  - El sistema cierra la mesa (vuelve a color verde), actualiza el inventario y envía la factura electrónica al correo del cliente.
- Flujos Alternativos:
  - 1a. Error de Impresora: Si la impresora de cocina se queda sin papel, el nodo local detecta el fallo y envía una notificación push al celular del Mesero y a la pantalla del Cajero.
### CU-02: División de Cuentas (Split Bill) - El dolor de cabeza de los restaurantes
- Actor: Cajero (o Mesero con permisos).
- Flujo Principal:
  - Una mesa de 4 personas pide pagar por separado lo que cada uno consumió.
  - El Cajero abre la mesa y selecciona la opción "Dividir Cuenta / Múltiples Pagos".
  - Aparece una interfaz visual con los ítems de la orden a la izquierda y "Nuevas Cuentas" a la derecha.
  - El Cajero arrastra (drag & drop) los ítems correspondientes a la "Cuenta 1", "Cuenta 2", etc.
  - Se genera una factura electrónica individual para cada cuenta.
- Flujo Alternativo:
  - Pago a partes iguales: El Cajero selecciona "Dividir en partes iguales", ingresa el número de personas (ej. 4) y el sistema divide el total monetario equitativamente, permitiendo cobrar cada parte con diferentes métodos (Ej: 2 pagan en efectivo, 2 con transferencia).
FASE 3
## 1. Arquitectura "Local Edge" (Cero Caídas sin Internet)
Si el internet de tu cliente (ej. Netlife, CNT) se cae en pleno viernes por la noche, el sistema debe seguir operando al 100%.
Para lograrlo, la arquitectura se divide en tres capas:
- El Servidor Cloud (La Nube): Alojado en servicios de alta disponibilidad (como AWS o instancias escalables en Render). Aquí reside la base de datos principal (PostgreSQL). Se encarga de centralizar sucursales, emitir la facturación electrónica (SRI) y mostrar reportes gerenciales remotos.
- El Nodo Local (Edge Node): Esta es la clave. Es un pequeño software ligero (ej. un ejecutable compilado en Go) que se instala en la computadora del cajero (Windows/Linux/Mac).
  - Actúa como un mini-servidor local.
  - Mantiene una réplica de la base de datos (SQLite) sincronizada bidireccionalmente con la nube.
  - Gestiona directamente las impresoras conectadas por USB o red LAN (Ethernet/WiFi).
- Los Dispositivos (Celulares/Tablets): La aplicación móvil (Flutter) de los meseros no se conecta a la nube, se conecta a la IP local del "Nodo Local" a través del router WiFi del restaurante.
Flujo Offline: Si se corta el internet, los celulares siguen enviando comandas al Nodo Local por el WiFi interno. El Nodo Local imprime en la cocina instantáneamente y guarda los datos. Cuando el internet regresa, el Nodo Local sincroniza silenciosamente las ventas hacia el Servidor Cloud (AWS/Render o tu VPS) y encola los XML para el SRI.
## 2. Prevención de Cruce de Mesas (Concurrency Control)
El problema de que dos meseros tomen pedidos a la vez en la misma mesa provoca colapsos y descuadres de inventario. Esto se soluciona combinando Bloqueo Pesimista (Pessimistic Locking) y WebSockets en tiempo real.
### Mecanismo de "Table Locking" (Bloqueo de Mesa):
- Estado de Bloqueo: Cuando el Mesero A entra a la "Mesa 5", la aplicación envía un evento instantáneo (Ping) al Nodo Local.
- Transmisión (Broadcast): El Nodo Local emite un evento WebSocket a todos los dispositivos del restaurante: {"action": "LOCK_TABLE", "table_id": 5, "user": "Mesero A"}.
- Reacción Visual: En la pantalla del Mesero B, la Mesa 5 se pone en color gris oscuro con un ícono de candado y el texto: "Editando: Mesero A". El botón de entrar se deshabilita temporalmente.
- Liberación (Unlock): Cuando el Mesero A presiona "Enviar Comanda" o sale de la pantalla, se emite el evento UNLOCK_TABLE.
- Fail-Safe (Seguro contra fallos): ¿Qué pasa si el celular del Mesero A se apaga o se queda sin batería mientras tenía la mesa abierta? La base de datos local tendrá un campo locked_at (Timestamp). Si el sistema no recibe actividad de ese mesero en 45 segundos, el Nodo Local expira el bloqueo (Timeout) y libera la mesa automáticamente para no paralizar la operación.
## 3. Estructura de Base de Datos para Alta Velocidad
Para que el sistema sincronice datos offline a online sin colisionar (ej. que dos órdenes distintas se guarden como "Orden 50"), queda estrictamente prohibido usar IDs autoincrementables (1, 2, 3...). Todas las tablas principales usarán UUID v4 (Identificadores Únicos Universales generados algorítmicamente).
Aquí tienes el esquema relacional core optimizado:
### Tabla: mesas
- id (UUID, Primary Key)
- numero_nombre (String, ej. "Mesa 10" o "Barra 1")
- estado (Enum: LIBRE, OCUPADA, POR_PAGAR)
- locked_by_user_id (UUID, Nullable - ID del mesero editando)
- locked_at (Timestamp - Para calcular el Timeout de 45 segundos)
### Tabla: ordenes
- id (UUID, Primary Key)
- mesa_id (UUID, Foreign Key)
- mesero_id (UUID, Foreign Key)
- estado_general (Enum: ABIERTA, CERRADA, ANULADA)
- estado_sincronizacion (Enum: LOCAL, SYNCED - Vital para saber qué subir a la nube)
- total_calculado (Decimal)
### Tabla: orden_detalles (Las comandas individuales)
- id (UUID, Primary Key)
- orden_id (UUID, Foreign Key)
- producto_id (UUID, Foreign Key)
- cantidad (Int)
- estado_impresion (Enum: PENDIENTE, IMPRESO, ERROR)
- impresora_destino_ip (String - Para que el Nodo Local sepa dónde enviarlo)
## 4. Estrategia de Despliegue (QA y CI/CD)
Para asegurar que un sistema tan complejo no colapse al recibir actualizaciones, el desarrollo debe incluir un ciclo estricto de pruebas automatizadas y despliegue continuo (CI/CD) antes de tocar el entorno de producción.
- Pruebas End-to-End (E2E): Antes de cada actualización, se deben ejecutar suites de pruebas con Cypress. Estos scripts simularán el comportamiento de múltiples meseros haciendo clics simultáneos en las mesas, comprobando que los WebSockets reaccionen y el bloqueo pesimista funcione. Si Cypress detecta un fallo, el código no se sube.
- Pipelines de Despliegue: Utilizar flujos de trabajo en GitHub Actions. Al hacer un push a la rama principal (main/master), GitHub Actions compilará el código, ejecutará las pruebas de Cypress y construirá imágenes Docker.
- Infraestructura: Una vez pasadas las pruebas, GitHub Actions se conectará mediante llaves SSH y desplegará automáticamente la actualización en el servidor en la nube (ej. Render, AWS o tu VPS en Azure), garantizando un despliegue sin tiempo de inactividad (Zero Downtime Deployment) para los locales que estén operando.
Con esta arquitectura híbrida (UUIDs, Nodo Local, WebSockets y despliegue automatizado), logras un sistema que toma lo mejor de grandes software como Fudo (UI/UX) o Illarli (Facturación), pero eliminando sus vulnerabilidades ante la caída de internet y concurrencia
FASE 4
## 1. Stack Frontend: Velocidad Extrema y Cero Latencia
Para lograr un sistema que no tenga "lag" y soporte el ritmo frenético de un restaurante, debemos dividir la estrategia del Frontend (la cara visible del sistema) según el dispositivo.
- Para la App Móvil (Meseros): Flutter. Es la elección definitiva. Flutter compila directamente a código máquina (ARM) y su nuevo motor gráfico (Impeller) garantiza un rendimiento fluido de 60 a 120 cuadros por segundo. Para lograr el estilo iPhone que buscas, Flutter incluye de forma nativa la librería Cupertino, que replica a la perfección los botones, interruptores, alertas y animaciones del ecosistema iOS.
- Para el Panel Web y Caja (Escritorio/Tablets): React (con Vite) o SolidJS. Para un cajero, la velocidad de tipeo y clics es crítica. SolidJS o React permiten construir interfaces que no recargan la página en absoluto (Single Page Applications). SolidJS, en particular, actualiza directamente los elementos del DOM sin procesos intermedios, haciéndolo estúpidamente rápido para facturar y cobrar.
## 2. Diseño UI/UX: Estilo iOS e Imágenes Interactivas
El diseño visual debe transmitir la elegancia y limpieza de iOS, utilizando el concepto de "Glassmorphism" (efecto cristal) e interacciones táctiles naturales.
- Mapa de Mesas Interactivo: En lugar de una cuadrícula de botones grises, el sistema renderiza un plano 2D de la planta del restaurante. Las mesas son gráficos vectoriales interactivos. Si una mesa tiene un pedido recién enviado, parpadea sutilmente en amarillo; si la comida ya salió de cocina, muestra un pequeño ícono de un plato humeante; si llevan esperando la cuenta más de 10 minutos, cambia a un tono rojo suave.
- Gestos Nativos (Swipe): Eliminaremos los botones de "Eliminar" o "Editar" que ocupan espacio visual. Si el mesero se equivoca al ingresar un plato, simplemente desliza el dedo hacia la izquierda sobre el ítem (Swipe-to-delete) y el celular emite una ligera vibración (respuesta háptica), idéntico a cómo se borran correos en un iPhone.
- Menús Inferiores (Bottom Tabs): La navegación principal de los meseros se ubicará en la parte inferior de la pantalla (estilo app de Apple), con íconos grandes para: Salón (Mapa), Comanda Actual, y Notificaciones (Platos listos).
## 3. Arquitectura "Zero-Click" (Reducción de Interacciones)
Para que el sistema sea amigable y no requiera entrenamiento extenso, la interfaz debe anticipar las acciones del usuario.
- Búsqueda Predictiva de 1 Tecla: En la caja, al teclear la letra "C", el sistema filtra instantáneamente en una ventana flotante: "Ceviche", "Cerveza", "Cola".
- Cobro Rápido con "Billetes Comunes": Cuando el cliente pide la cuenta de $14.50, la pantalla de pago no solo muestra un teclado numérico, sino botones dinámicos basados en la moneda circulante: [$15], [$20]. Al tocar "$20", el sistema calcula el vuelto automáticamente ($5.50), cierra la mesa, abre el cajón de dinero e imprime el recibo en un solo clic.
- Autocompletado de Clientes: En la facturación, al ingresar un RUC o Cédula, el sistema busca en la base local (o en el API del Registro Civil) y llena el Nombre, Dirección y Correo sin que el cajero toque el ratón. Si es un cliente recurrente, al presionar "Enter" se factura directamente.
## 4. Facturación SRI en Background (Asíncrona)
El mayor cuello de botella en los sistemas ecuatorianos es el momento de autorizar la factura electrónica, ya que el cajero debe esperar a que el servidor del SRI responda. Para mantener la velocidad extrema, cambiaremos este paradigma.
- El cajero selecciona los ítems y presiona "Facturar".
- El Frontend cambia instantáneamente el estado de la mesa a "Libre", el sistema local imprime el ticket físico (Tirilla) con la leyenda "Documento en proceso de autorización" y el cajero atiende al siguiente cliente en menos de 2 segundos.
- En segundo plano, un servicio de colas en el Servidor Local toma la transacción, construye el XML, lo firma con el certificado P12 y lo envía al SRI.
- Si el SRI está caído (algo común), el sistema no bloquea la interfaz de caja. Simplemente encola la factura y la reintenta cada 5 minutos hasta obtener la clave de acceso, enviando finalmente el PDF al correo del cliente.
fase 5 
## 1. Módulos Exactos de Desarrollo (El Alcance del Proyecto)
El desarrollo se divide en 6 grandes módulos o Epics. Esta separación permite que distintos desarrolladores (o tú mismo en diferentes etapas) trabajen sin pisarse los pies.
- Módulo 1: Core de Sincronización y Backend (Go & PostgreSQL)
  - Desarrollo de la API REST principal y gestión de WebSockets.
  - Lógica del "Bloqueo Pesimista" (Table Locking) para evitar cruce de mesas.
  - Motor de sincronización bidireccional (Nube <-> Nodo Local).
- Módulo 2: Nodo Local y Gestor de Hardware (Edge Agent)
  - Pequeño ejecutable instalable en la PC de caja.
  - Detección de impresoras (USB, LAN) y ruteo de comandas (Ticket Bar, Ticket Cocina).
  - Base de datos local (SQLite) para garantizar el funcionamiento offline.
- Módulo 3: App Móvil para Meseros (Flutter)
  - Interfaz con diseño iOS (Cupertino, Glassmorphism).
  - Catálogo con imágenes almacenadas eficientemente (ej. usando AWS S3 para carga rápida).
  - Gestión offline-first: guardar comandas en caché sin internet.
- Módulo 4: Punto de Venta Web (React / SolidJS)
  - Interfaz de cajero "Zero-Click" (búsqueda predictiva, botones de billetes).
  - División de cuentas (Split Bill) con Drag & Drop interactivo.
- Módulo 5: Motor Fiscal (SRI Ecuador)
  - Módulo aislado que recibe la orden, estructura el XML y lo firma con el P12.
  - Sistema de encolamiento (Background Workers) para reintentar enviar facturas si el SRI está caído.
- Módulo 6: Backoffice / Panel Administrativo
  - Gestión de Inventario y Recetas (Mermas exactas por miligramo).
  - Reportes de ventas, cortes de caja y auditoría.
## 2. Cronograma Estimado (Roadmap de 5 Meses)
Un sistema híbrido y robusto toma tiempo. Este cronograma asume un flujo de trabajo profesional, aplicando pruebas automatizadas en cada etapa.

| Fase | Duración | Hitos Principales y Entregables |
|---|---|---|
| Mes 1: Infraestructura y BD | 4 Semanas | Diseño de BD (UUIDs). Configuración de repositorios en GitHub. Creación de los pipelines de integración continua (GitHub Actions). |
| Mes 2: Backend y Edge | 4 Semanas | API en Go terminada. Nodo local capaz de recibir un JSON e imprimir un ticket físico en una impresora térmica de red. |
| Mes 3: Frontend Móvil | 4 Semanas | App en Flutter lista. Autenticación de meseros, apertura de mesas, adición de platos y envío por WebSockets al nodo local. |
| Mes 4: Caja y SRI | 4 Semanas | Interfaz de cajero operativa. Módulo de facturación electrónica firmado y enviando a pruebas (Homologación) del SRI. |
| Mes 5: QA, Pruebas y Piloto | 4 Semanas | Ejecución de pruebas automatizadas End-to-End con Cypress simulando uso intensivo. Despliegue en la nube (ej. instancias en Render y almacenamiento de imágenes en AWS). Instalación en 1 restaurante piloto (Beta). |
## 3. Estructuración del Modelo SaaS (Suscripción Mensual)
En Ecuador, los restaurantes evitan pagar miles de dólares por licencias vitalicias. Prefieren rentar el software. Para competir con los sistemas de la Fase 1, el modelo de precios debe ser escalonado, cobrando según el valor que aporta.
Plan "Emprendedor" (Aprox. $29 - $35 / mes)
- Target: Cafeterías pequeñas, Food Trucks, Huecas.
- Incluye: 1 Usuario Cajero, 1 Usuario Mesero, Facturación Electrónica ilimitada (SRI), App Móvil.
- Limitación: Sin control de inventario de recetas (solo suma y resta de productos simples), 1 sola impresora.
Plan "Restaurante" (Aprox. $59 - $75 / mes) - Tu plan estrella
- Target: Restaurantes medianos, bares, pizzerías.
- Incluye: Usuarios meseros ilimitados, Múltiples impresoras (Bar, Cocina Caliente, Sushi), Control de Inventario Avanzado (Recetas y mermas).
- Bonus: Módulo de cuentas divididas activo.
Plan "Cadena / Pro" (Aprox. $110+ / mes por local)
- Target: Marcas con 2 o más sucursales.
- Incluye: Todo lo anterior + Panel centralizado para ver ventas de todos los locales en tiempo real, traslados de inventario entre sucursales, auditorías de permisos.
Estrategia de Retención: El gran gancho de tu sistema será el Nodo Local. Al no necesitar internet para operar en hora pico, los dueños de restaurantes no cancelarán la suscripción por temor a volver a sistemas 100% web que paralizan su negocio cuando falla su proveedor de internet.
Para evitar que el disco duro de la computadora del restaurante se llene, el sistema se vuelva lento o, en el peor de los casos, se pierda toda la información si la PC se daña o es robada, debemos implementar una Estrategia de Almacenamiento Efímero y Backups en la Nube.
Aquí te detallo la arquitectura exacta para manejar los PDFs y la base de datos sin comprometer la velocidad ni arriesgar los datos del cliente.
## 1. Manejo de PDFs (Facturas y Reportes)
Las facturas electrónicas XML y PDF consumen espacio rápidamente. La regla de oro es no almacenar documentos históricos permanentemente en la máquina local.
El ciclo de vida del PDF debe ser:
- Generación Local: El cajero cobra, el sistema genera el XML/PDF en la memoria de la PC y lo imprime/envía por correo al cliente.
- Subida Inmediata (Background): En segundo plano, el agente de Go toma ese PDF y lo sube a un servicio de almacenamiento de objetos en la nube, como AWS S3. Este servicio es extremadamente económico (centavos por miles de archivos) y está diseñado exactamente para esto.
- Limpieza Local: Una vez que el archivo está seguro en el bucket de AWS S3 y el sistema ha guardado la URL (el enlace) en la base de datos, el archivo físico se elimina del disco duro local del restaurante.
- Consulta Histórica: Si un cliente regresa un mes después pidiendo la copia de su factura, el sistema simplemente abre la URL guardada y descarga el PDF directamente desde la nube.
De esta forma, la carpeta de facturas del restaurante local siempre pesará casi 0 MB, manteniendo la PC rápida y limpia.
## 2. Base de Datos Local (El Nodo Edge)
En la computadora del restaurante estará corriendo una base de datos SQLite (gestionada por tu agente de Go). Para que SQLite sea "súper rápido" y no colapse, se debe activar el modo WAL (Write-Ahead Logging), que permite leer y escribir datos simultáneamente sin bloquear la base.
Para evitar que esta base de datos crezca infinitamente:
- Retención a corto plazo: La base local solo debe almacenar el menú, los usuarios, y las ventas/órdenes de los últimos 30 a 60 días.
- Descarga a la Nube (Offloading): Las ventas antiguas ya fueron sincronizadas a tu base de datos principal en la nube (por ejemplo, tu instancia de PostgreSQL en Render). Por lo tanto, el sistema local puede ejecutar una rutina que borre las órdenes de más de 60 días para mantener el archivo SQLite por debajo de los 50 MB, garantizando búsquedas en milisegundos.
## 3. El Sistema de Backups Automáticos (Disaster Recovery)
A pesar de que el sistema sincroniza constantemente a la nube, es vital tener un respaldo duro (físico y comprimido) de la base de datos local en caso de que la sincronización falle justo antes de un daño del equipo.
El proceso de Backup Comprimido (Ejecutado por un Cron Job local):
- Cuándo: Todos los días a las 3:30 AM (fuera del horario de atención), el agente local de Go detiene temporalmente la sincronización.
- El Volcado (Dump): Ejecuta un comando para extraer toda la información de la base SQLite a un archivo de texto (.sql).
- Compresión: El agente comprime ese archivo en formato .tar.gz o .zip. Una base de datos de texto plano de 50 MB se reduce a apenas 4 o 5 MB al comprimirse.
- Exportación Segura: Este archivo ligero se envía automáticamente fuera de la PC. Puede enviarse al mismo bucket de AWS S3 bajo una carpeta /backups, o incluso cargarse mediante un protocolo FTP a un servidor clásico como EcuaHosting, garantizando redundancia geográfica en Ecuador.
- Rotación (Limpieza): El sistema guarda los respaldos de los últimos 7 días. El día 8, borra el respaldo del día 1. Así, nunca acumulas archivos infinitos.
### ¿Qué pasa si la computadora del restaurante explota o se quema?
El proceso de recuperación será indoloro:
- Instalas tu software en una computadora nueva.
- Ingresas las credenciales del restaurante (RUC y Token).
- El sistema se conecta a la nube, descarga el último backup comprimido, restaura las mesas, el inventario y los usuarios en cuestión de segundos.
- El restaurante vuelve a operar como si nada hubiera pasado.
La Fase 6 define el cerebro del negocio: el Backoffice o Panel Administrativo. A diferencia de los ERPs genéricos de gestión empresarial (como Odoo o SAP), donde el usuario se pierde entre decenas de módulos y configuraciones, este panel debe mantener la filosofía "Zero-Friction" y el diseño estilo iOS, enfocado exclusivamente en la gastronomía.
Toda esta aplicación web (desarrollada en React o SolidJS) vivirá 100% en la nube. Se compilará y desplegará utilizando pipelines de integración continua (como GitHub Actions) hacia plataformas de alto rendimiento (Render o un VPS en Azure), conectándose directamente a la base de datos PostgreSQL central. Esto asegura que el dueño pueda ver su restaurante en tiempo real desde su casa, sin sobrecargar la computadora local del local.
A continuación, la estructura de los módulos clave del Backoffice:
## 1. Dashboard en Tiempo Real (El "Home")
Al iniciar sesión, el propietario no debe ver tablas aburridas, sino Widgets interactivos estilo Apple (tarjetas con bordes redondeados, sombras suaves y colores contrastantes).
- Ventas del Día: Un indicador gigante con el total facturado, comparado porcentualmente con el mismo día de la semana anterior (Ej: $1,250 ⬆ 15% vs jueves pasado).
- Mapa de Calor de Mesas: Una réplica en miniatura del salón que muestra qué mesas están ocupadas, libres o esperando la cuenta en este preciso segundo.
- Top 5 Platos y Top 5 Meseros: Gráficos de barras minimalistas mostrando lo que más se vende y quién está vendiendo más, ideal para calcular comisiones.
- Alertas Críticas: Un panel lateral de notificaciones push (Ej: "Stock crítico: Queso Mozzarella", "Impresora de Cocina Desconectada en Local 1").
## 2. Gestión de Menú y Catálogo (Visual y Rápido)
La creación de un plato debe tomar segundos, no minutos.
- Carga de Imágenes Directa: Un área de Drag & Drop para arrastrar la foto del plato. La imagen se optimiza automáticamente en el navegador y se sube directamente a un bucket de AWS S3 para no saturar la base de datos.
- Categorías Dinámicas: Creación de categorías (Entradas, Platos Fuertes, Bebidas) que se reflejan instantáneamente en la app de los meseros.
- Modificadores (Add-ons): Configuración visual de opciones adicionales. Por ejemplo, al crear "Hamburguesa", se añaden modificadores con un clic: "Término de la carne (Obligatorio)", "Extras: Tocino (+$1.00)".
## 3. Control de Inventario y Recetas (Bill of Materials - BOM)
Este es el módulo técnico más crítico. Para asegurar que la lógica de costeo y descargas no falle, sus flujos deben estar blindados (idealmente con pruebas automatizadas usando Cypress antes de cualquier despliegue).
- Insumos Base: Creación de la materia prima con sus unidades de medida (Ej: Tomate en Kilogramos, Cerveza en Unidades, Carne en Gramos).
- Constructor de Recetas: Una interfaz visual donde el administrador selecciona el plato "Pizza Margarita" y arrastra los insumos: 200g de harina, 50g de salsa, 150g de queso.
- Costeo Automático: El sistema calcula el costo de producción del plato sumando el costo de cada insumo. Si el queso sube de precio en la próxima compra, el sistema alerta si el margen de ganancia de la pizza cae por debajo del 30%.
- Toma Física (Ajuste de Inventario): Interfaz diseñada para usarse desde una tablet, permitiendo al administrador caminar por la bodega ingresando las cantidades reales contadas para ajustar las mermas teóricas.
## 4. Reportes y Cierres de Caja (Caja Fuerte Fiscal)
El dueño necesita ver el dinero y los impuestos de forma transparente.
- Cierre Z (Arqueo): Un reporte diario inalterable que cruza el efectivo esperado vs el ingresado por el cajero, detallando cobros por tarjeta, transferencias y efectivo.
- Bóveda SRI: Una tabla donde se listan todas las facturas electrónicas emitidas. Cada fila tiene dos botones: "Descargar XML" y "Descargar PDF" (obtenidos desde AWS S3).
- Exportación Contable: Un botón de "Exportar a Excel" que genera un archivo plano estructurado, listo para que el contador lo importe a su sistema contable sin tener que digitar factura por factura.
## 5. Gestión de Permisos y Roles (Seguridad)
- Matriz de Accesos: Una tabla de interruptores (Toggles estilo iOS) para habilitar o deshabilitar acciones. Por ejemplo: ¿Puede el cajero aplicar descuentos? [Off]. ¿Puede el mesero anular una cuenta? [Off].
- Registro de Auditoría (Log de Acciones): Un historial inmutable que registra quién hizo qué y a qué hora (Ej: "Cajero Juan eliminó 1 Cerveza de la Mesa 4 a las 21:15").
Entramos a la Fase 7: Cumplimiento Legal (SRI Ecuador), Onboarding Automático y Seguridad.
Para que el sistema sea 100% legal en Ecuador y pueda comercializarse sin fricciones, el proceso de alta fiscal debe ser completamente automático. El dueño de un restaurante no es un experto en impuestos; el software debe abstraer toda la complejidad legal detrás de una interfaz amigable.
## 1. Onboarding Fiscal "Plug & Play" (Configuración Automática)
El objetivo es que el dueño del restaurante no tenga que configurar parámetros técnicos como URLs de Web Services, códigos numéricos o algoritmos. El flujo en el Panel Web será un asistente paso a paso estilo iOS:
- Carga del Certificado: Una pantalla limpia con un botón para subir el archivo de la Firma Electrónica (.p12) y un campo para ingresar la contraseña.
- Extracción de Datos: El backend (en Go) desencripta temporalmente el certificado, extrae automáticamente el RUC y la Razón Social del cliente y los guarda en la base de datos (PostgreSQL).
- Configuración de Puntos de Emisión: El sistema pregunta visualmente: "¿Cuántas cajas registradoras tienes?" Si el usuario responde "1", el sistema configura automáticamente el Establecimiento 001 y el Punto de Emisión 001.
- Auto-Detección de Régimen: Mediante una consulta interna (API o scraping autorizado al SRI), el sistema verifica si el RUC pertenece a RIMPE Emprendedor, RIMPE Negocio Popular o Régimen General, y aplica automáticamente las leyendas obligatorias que deben ir en el PDF ("Contribuyente Régimen RIMPE", "Agente de Retención", etc.).
## 2. Validaciones Legales Obligatorias del Sistema
Para que el software pase cualquier auditoría y funcione sin bloqueos, debe cumplir estrictamente con la Ficha Técnica del SRI:
- Generación de la Clave de Acceso (49 dígitos): El sistema debe generar esta clave algorítmicamente en milisegundos combinando: Fecha (8) + Tipo Comprobante (2) + RUC (13) + Ambiente (1) + Serie (6) + Secuencial (9) + Código Numérico (8) + Dígito Verificador (1, módulo 11).
- Firma Electrónica XAdES-BES: El archivo XML debe ser firmado digitalmente utilizando este estándar criptográfico antes de ser enviado al SRI. Este proceso debe ejecutarse en el servidor en la nube (ej. Render/AWS) mediante un Background Worker, no en la computadora local del restaurante, para garantizar la seguridad del archivo .p12.
- Contingencia Legal (Offline): La ley ecuatoriana permite que, si el sistema del SRI está caído o el restaurante pierde internet, se entregue el comprobante impreso (RIDE) al cliente. El sistema tiene la obligación de almacenar los XML encolados e intentar su transmisión automática hasta por 72 horas.
## 3. Guía Integrada de Trámites para el Cliente
Para evitar sobrecargar tu línea de soporte técnico, el sistema incluirá un módulo de "Ayuda/Onboarding" con videos interactivos cortos y un paso a paso para los trámites que el cliente debe hacer fuera del sistema:
- Obtener la Firma Electrónica (Archivo .p12): Guía de enlaces directos a entidades certificadoras ecuatorianas (Security Data, Uanataca, Banco Central) con la instrucción explícita: "Solicite la firma en ARCHIVO, no en Token USB".
- Autorización en el portal SRI en Línea: Un tutorial visual de 3 pasos indicando cómo el cliente debe ingresar con su clave al SRI, ir a Facturación Electrónica -> Producción -> Autorización y habilitar su cuenta.
- Carga en el Sistema: Redirección automática a la pantalla de configuración del software.
## 4. Hardware Recomendado y Seguridad Anti-Hackeo
Para garantizar el rendimiento extremo y proteger la información:
### Red y Hardware
- Segmentación de Red (VLAN): Es obligatorio exigir al restaurante que la red WiFi de los meseros y cajas esté físicamente (o lógicamente a través del router) separada del WiFi de clientes. Si un cliente descarga torrents, saturará la red y las comandas tardarán en llegar a la impresora.
- Impresoras: Compatibilidad con impresoras térmicas ESC/POS (Epson, Xprinter, Rongta) mediante conexión Ethernet o WiFi (no Bluetooth, por inestabilidad).
- Dispositivos de Meseros: Cualquier dispositivo Android (Android 10+) o iOS, pero la app en Flutter estará optimizada para uso a una sola mano.
### Seguridad del Software (Ciberseguridad)
- Protección de la Firma Electrónica: El archivo .p12 no se guarda como un archivo expuesto. Se encripta con AES-256 y se almacena como un BLOB en la base de datos o en un bucket privado de AWS S3 sin acceso público. La contraseña del P12 se encripta con algoritmos de hashing fuertes.
- Autenticación JWT: Toda la comunicación entre la app móvil, el nodo local y la nube utiliza JSON Web Tokens con tiempos de expiración cortos, evitando que un dispositivo robado mantenga acceso permanente al sistema.
## 1. Visión General y Arquitectura Híbrida
El sistema está diseñado para operar bajo un modelo SaaS Cloud-Edge Híbrido, resolviendo la principal vulnerabilidad de los sistemas de punto de venta en Ecuador: la dependencia de la conexión a internet.
La operación se divide en un servidor central en la nube y un "Nodo Local" ligero instalado en la computadora de caja del restaurante. El Nodo Local intercepta las comandas vía WiFi interno, gestiona el enrutamiento a las impresoras físicas en milisegundos mediante WebSockets, y asegura que la operación del salón (apertura de mesas, pedidos) no se detenga ante cortes del proveedor de internet. La sincronización hacia la nube ocurre de manera asíncrona una vez que la conexión se restablece.
Para prevenir colisiones y descuadres de inventario durante el trabajo concurrente de múltiples meseros, la arquitectura emplea Bloqueo Pesimista (Pessimistic Table Locking) con expiración automática de 45 segundos e identificadores UUID v4 en todas las tablas transaccionales, eliminando el riesgo de duplicación de IDs durante la sincronización bidireccional.
## 2. Stack Tecnológico e Infraestructura
La selección tecnológica prioriza la velocidad de ejecución y la reducción absoluta de latencia.
- Frontend Móvil (Meseros): Flutter. Interfaz táctil de alto rendimiento (60-120 fps) bajo estándares de diseño iOS (Cupertino/Glassmorphism) con almacenamiento de catálogo en caché para operabilidad offline-first.
- Frontend Web (Caja y Backoffice): React o SolidJS. Single Page Applications con búsqueda predictiva de 1 tecla, atajos visuales y gestión de estado sin recargas del DOM.
- Backend y Concurrencia: Go (Golang). Motor de la API REST principal y gestor de WebSockets, optimizado para manejar miles de conexiones simultáneas con un consumo de memoria mínimo.
- Bases de Datos: PostgreSQL para la nube (centralización, analítica y multi-sucursal) y SQLite en modo WAL para los nodos locales, con políticas de purga automática a 60 días para mantener el archivo por debajo de los 50 MB.
- Infraestructura de Alojamiento: Despliegue de los servicios backend y base de datos central en plataformas de alto rendimiento escalables como Render o instancias dedicadas en un Azure VPS.
- Almacenamiento de Archivos Efímeros: Los XML, PDFs de facturas electrónicas y respaldos comprimidos locales diarios se suben a AWS S3 en segundo plano, manteniendo el disco duro del restaurante siempre limpio.
## 3. Especificaciones Críticas de Negocio
El sistema abstrae la complejidad operativa y legal para el usuario final a través de tres flujos automatizados:
- UX "Zero-Click": Las interacciones repetitivas se minimizan mediante gestos nativos (swipe-to-delete), mapas de mesas con retroalimentación visual en tiempo real y botones dinámicos de denominación monetaria para cálculo de vueltos instantáneo.
- Motor Fiscal Asíncrono (SRI): La facturación electrónica no interrumpe el flujo de caja. El cajero cierra la orden, se emite un comprobante físico temporal, y en segundo plano el sistema firma el XML bajo el estándar XAdES-BES utilizando el archivo .p12 encriptado. Si los servidores del SRI presentan intermitencia, un sistema de colas reintenta la transmisión hasta obtener la clave de acceso de 49 dígitos.
- Gestión de Inventario (BOM): Descarga dinámica de materia prima mediante constructores de recetas visuales, alertando sobre variaciones en los márgenes de ganancia frente a las fluctuaciones de costos.
## 4. Estrategia de Despliegue (CI/CD) y Roadmap
Para sostener el modelo SaaS (tiers Emprendedor, Restaurante, Pro) sin degradación del servicio, el ciclo de vida del software debe estar estrictamente automatizado.
El código fuente, gestionado en un repositorio central, se integra con GitHub Actions para orquestar la canalización de CI/CD. Antes de compilar cualquier nueva versión o desplegar actualizaciones en la nube, el pipeline ejecuta suites de pruebas End-to-End (E2E) con Cypress. Estos scripts simulan colisiones de mesas, caídas simuladas de red y facturación masiva. Si los umbrales de latencia se superan o la sincronización falla, el despliegue se detiene automáticamente para proteger los entornos de producción en ejecución.
### Fase 1: Requisitos Detallados de Salón y Comandas
#### 1. Autenticación Ultra-Rápida de Meseros
- El Requisito del Cliente: "Mis meseros no pueden estar escribiendo correos y contraseñas largas cada vez que abren la app o se bloquea el celular. Necesitan entrar en 1 segundo."
- Solución con Programación: Implementar un sistema de autenticación por PIN numérico de 4 dígitos exclusivo para la app móvil, vinculado a un token de sesión de larga duración.
- Implementación Técnica:
  - En Flutter (Frontend), se diseña un teclado numérico gigante en la pantalla de inicio.
  - Al ingresar el PIN, se hace un hash local y se compara con el almacenado en el caché del dispositivo (o se valida contra el Nodo Local de Go).
  - Se utiliza JWT (JSON Web Tokens) para mantener la sesión abierta durante el turno (ej. 12 horas) y forzar el cierre al hacer el corte de caja.
#### 2. Bloqueo Visual de Mesas (Anti-Colisiones)
- El Requisito del Cliente: "Si el mesero Carlos está tomando el pedido de la Mesa 5, la mesera Ana no debe poder abrir esa misma mesa en su celular porque me duplican los platos y descuadran el inventario."
- Solución con Programación: Algoritmo de Bloqueo Pesimista (Pessimistic Locking) gestionado por eventos en tiempo real.
- Implementación Técnica:
  - Al tocar la mesa en Flutter, se emite un evento vía WebSockets. El Nodo Local recibe el evento y emite un broadcast a todos los dispositivos: {"action": "LOCK", "table": 5}.
  - Control de Calidad: Antes de mandar este código a producción, se debe construir un script de Cypress que abra dos instancias simuladas de la app y haga clic exactamente al mismo milisegundo en la misma mesa, validando que el sistema asigne el bloqueo a uno y rechace al otro. Esto se integra en tus flujos de GitHub Actions para que ninguna actualización rompa esta regla.
#### 3. Búsqueda y Filtro de Platos sin Latencia
- El Requisito del Cliente: "El menú tiene 200 platos. El mesero necesita encontrar 'Ceviche Mixto' tecleando solo 'cev' y que aparezca al instante, sin cargar rueditas de espera."
- Solución con Programación: Búsqueda en memoria local (In-Memory Search) y algoritmos de coincidencia aproximada (Fuzzy Search).
- Implementación Técnica:
  - Al iniciar el turno, la app de Flutter descarga el JSON completo del menú desde el servidor principal (ej. alojado en tu instancia de Render o Azure VPS) y lo guarda en la base de datos local del teléfono (SQLite/Hive).
  - La barra de búsqueda consulta únicamente la base de datos local del teléfono. Esto garantiza respuestas en menos de 10 milisegundos, incluso si el internet del restaurante está caído.
#### 4. Notas de Cocina Específicas por Plato
- El Requisito del Cliente: "No me sirve una nota general para toda la orden. Si piden 3 hamburguesas, el mesero debe poder poner 'Sin cebolla' solo a la primera, y 'Término medio' a la segunda."
- Solución con Programación: Estructuración del JSON de la orden utilizando arreglos (arrays) de objetos independientes, donde cada plato tiene su propio sub-campo de metadatos modificadores.
- Implementación Técnica:
  - En la interfaz iOS-style, al agregar un plato, el mesero hace un toque prolongado (Long Press) sobre el ítem para abrir una ventana modal con botones de modificadores rápidos (Sin Sal, Extra Ají) o un campo de texto libre.
  - A nivel de base de datos relacional, esto se guarda en la tabla orden_detalles en una columna tipo JSONB (soportada por PostgreSQL), lo que permite indexar y leer estas notas sin crear decenas de tablas intermedias.
#### 5. Enrutamiento Condicional de Impresoras (Bar vs. Cocina)
- El Requisito del Cliente: "Cuando el mesero envía el pedido, las cervezas deben imprimirse solas en la barra y los ceviches solos en la cocina. El mesero no debe elegir a qué impresora mandarlo, debe ser automático."
- Solución con Programación: Mapeo de categorías por dirección IP en el agente local.
- Implementación Técnica:
  - En el Panel Administrativo, cada "Categoría" de producto (Ej: Bebidas) tiene un campo impresora_destino_ip.
  - Cuando el mesero presiona "Enviar", manda un solo paquete JSON al Nodo Local.
  - El programa en Go (Nodo Local) recorre el JSON, agrupa los ítems por su impresora_destino_ip, genera los comandos ESC/POS y abre conexiones TCP concurrentes (Goroutines) para enviar los comandos simultáneamente a la impresora 192.168.1.100 (Bar) y a la 192.168.1.101 (Cocina).
#### 6. Notificación de "Plato Listo" al Mesero (KDS)
- El Requisito del Cliente: "El mesero no debe estar dando vueltas por la cocina preguntando si ya salió la comida. Su celular debe avisarle."
- Solución con Programación: Implementación de un Kitchen Display System (KDS) simple en tablet, conectado a la red local.
- Implementación Técnica:
  - La cocina tiene una tablet donde caen los pedidos. El cocinero toca el pedido y cambia el estado a "Listo".
  - El Nodo Local capta el cambio y envía una notificación Push silenciosa (vía WebSockets) específicamente al mesero_id que tomó la orden. El celular del mesero vibra levemente y muestra una alerta verde en la parte superior: Mesa 5: Hamburguesa Lista.
Esta es la matriz de la Fase 1. Desglosa los dolores reales del negocio y los ata directamente a las tecnologías, bases de datos y flujos de CI/CD que harán que funcione sin errores.
### Fase 2: Requisitos Detallados de Caja, Facturación y SRI
#### 1. Autocompletado de Clientes (Anti-filas)
- El Requisito del Cliente: "Cuando cobro al mediodía tengo 10 personas en fila. No puedo estar pidiendo nombres, direcciones y correos y digitando todo manualmente cada vez que alguien quiere factura con datos."
- Solución con Programación: Consumo de APIs gubernamentales (o de terceros) y cacheo de base de datos local para clientes recurrentes.
- Implementación Técnica:
  - En el frontend (React o SolidJS), el input de "Cédula/RUC" tiene un debouncer. Al detectar 10 o 13 dígitos y presionar Enter, el sistema busca primero en la tabla local clientes (SQLite). Si el cliente existe, autocompleta Nombre, Dirección y Email en milisegundos.
  - Si no existe localmente, el backend en Go dispara una petición HTTP concurrente a un servicio validado de consulta de RUCs en Ecuador. La respuesta alimenta los campos y guarda el perfil en la base de datos para futuras compras, reduciendo el tiempo de atención de 1 minuto a 3 segundos.
#### 2. Motor Fiscal Asíncrono (Tolerancia a Caídas del SRI)
- El Requisito del Cliente: "Los fines de mes o en feriados el sistema del SRI siempre colapsa o se pone lentísimo. Si mi sistema se queda pensando esperando al SRI, la caja se paraliza y no puedo cobrar al siguiente cliente."
- Solución con Programación: Arquitectura orientada a eventos con colas de trabajo (Job Queues) y Background Workers.
- Implementación Técnica:
  - El cajero presiona "Facturar". El sistema no intenta conectarse al SRI en ese momento. En su lugar, el estado de la orden cambia a CERRADA_POR_AUTORIZAR.
  - La caja se libera instantáneamente en el frontend web. El nodo local imprime un ticket físico o "RIDE preliminar" con la leyenda: "Comprobante en proceso de autorización SRI".
  - Un worker en Go (una Goroutine leyendo una cola en Redis o memoria) toma el JSON, construye el XML, lo firma (algoritmo XAdES-BES con el .p12) y lo envía. Si el SRI devuelve un Timeout, el worker reencola el trabajo con una política de retroceso exponencial (intenta en 1 min, luego en 5 min, luego en 15 min), garantizando que ningún XML se pierda.
  - Una vez autorizado, el worker sube el PDF/XML a AWS S3 y dispara el correo al cliente.
#### 3. División de Cuentas (Split Bill) Visual y Multimétodo
- El Requisito del Cliente: "Llegan mesas de 8 personas y al final quieren pagar por separado lo que cada uno comió, o dividir la cuenta total entre 3 tarjetas y 2 pagos en efectivo. Si el sistema es complejo, el cajero se equivoca, cobra mal y descuadra la caja."
- Solución con Programación: Gestión de estado inmutable en el frontend (Drag & Drop) y desagregación transaccional en base de datos.
- Implementación Técnica:
  - La interfaz (React) muestra la orden original a la izquierda y un botón estilo iOS: [+] Nueva Sub-cuenta.
  - El cajero arrastra los ítems (ej. 1 Cerveza, 1 Hamburguesa) a la "Cuenta 1". El DOM se actualiza en tiempo real sin llamar al servidor.
  - Al confirmar, el frontend envía un JSON estructurado con un array de sub_ordenes. El backend en PostgreSQL ejecuta una Transacción ACID: anula la orden original y crea las sub-órdenes vinculadas al mismo mesa_id. Si una falla, se hace un rollback de todo para evitar cobros fantasma.
  - Cada sub-cuenta puede tener su propio método de pago y cliente facturado de manera totalmente independiente.
#### 4. Gestión Legal de Propinas (10% de Servicio Ecuatoriano)
- El Requisito del Cliente: "El SRI me exige que el 10% de servicio (propina) no grave IVA, debe ir detallado por separado en la factura, y ese dinero no es del restaurante, debe ir a un reporte aparte para repartirlo a los meseros quincenalmente."
- Solución con Programación: Segregación matemática a nivel de base de datos y exclusión de impuestos directos.
- Implementación Técnica:
  - La tabla ordenes incluye columnas independientes: subtotal_base_cero, subtotal_iva, monto_iva, y monto_propina_ley.
  - En el backend (Go), el cálculo es estricto: la propina es exactamente el 10% de la suma de (subtotal_base_cero + subtotal_iva), sin incluir el monto del IVA en la ecuación.
  - Al generar el XML de facturación, este rubro se mapea en el nodo específico de <propina> exigido por la ficha técnica del SRI, evitando que el contador tenga problemas por ingresos no justificados.
#### 5. Arqueo y Cierre Z a Prueba de Manipulación
- El Requisito del Cliente: "Al final del turno, el cajero tiene que entregarme el cierre. Necesito que el sistema cruce lo que se facturó vs lo que realmente hay en la gaveta, y que el cajero no pueda modificar facturas pasadas para robar."
- Solución con Programación: Tablas de inmutabilidad (Append-only) y flujos de auditoría.
- Implementación Técnica:
  - Ninguna factura pagada puede ser borrada de la base de datos (PostgreSQL). Solo puede ser anulada, lo que genera un registro de auditoría (anulado_por_user_id, motivo, timestamp) y emite una Nota de Crédito al SRI automáticamente si ya estaba autorizada.
  - El módulo de "Cierre de Caja" obliga al cajero a ingresar de forma ciega (Blind Close) la cantidad de billetes y vouchers de tarjeta que tiene en la mano.
  - El sistema procesa esto en el servidor, calcula la diferencia (Faltante/Sobrante) y genera el Cierre Z. Este reporte se renderiza como PDF, se respalda en AWS S3 y se envía instantáneamente por correo electrónico al dueño del restaurante.
#### 1. Conversión de Unidades Automática (Compras vs. Consumo)
- El Requisito del Cliente: "Yo compro la carne por Quintales (100 lbs) o Kilos, pero en la cocina las hamburguesas me consumen 150 gramos. No quiero estar con una calculadora haciendo reglas de tres para saber cuánto me queda."
- Solución con Programación: Matrices de equivalencia y factor de conversión en la base de datos.
- Implementación Técnica:
  - En la tabla insumos de PostgreSQL, se crean tres columnas clave: unidad_compra (Ej: Kilo), unidad_consumo (Ej: Gramo) y factor_conversion (Ej: 1000).
  - Cuando el dueño ingresa una factura de compra de "5 Kilos de Carne", el backend en Go multiplica automáticamente 5 x 1000 y suma 5000 gramos al Kardex.
  - En el panel web (React/SolidJS), el dueño ve una barra visual que dice: Stock: 5.0 Kilos (5000 gramos).
#### 2. Constructor Visual de Recetas (BOM - Bill of Materials)
- El Requisito del Cliente: "Quiero que al vender una 'Hamburguesa Especial' se descuente el pan, la carne, la rebanada de queso, y hasta el empaque de cartón."
- Solución con Programación: Relación de muchos-a-muchos (Many-to-Many) con atributos de cantidad.
- Implementación Técnica:
  - Se crea una tabla puente llamada receta_detalles que conecta un producto_id (Hamburguesa) con múltiples insumo_id (Carne, Pan, Caja).
  - En el frontend, esto se diseña como una interfaz de Drag & Drop. El administrador arrastra el insumo hacia el plato y digita "150".
  - Crucial: Como ya dominas herramientas de automatización como Cypress, este es el módulo donde debes escribir pruebas rigurosas E2E. Un script que cree un plato, le asigne insumos, simule una venta y valide que la resta matemática en la base de datos sea exacta hasta el último decimal.
#### 3. Sub-Recetas (Preparaciones Previas o Batching)
- El Requisito del Cliente: "La Salsa BBQ no la compro hecha. El lunes el chef prepara una olla de 5 Litros usando tomate, azúcar, vinagre y humo líquido. Luego, cada alitas que vendo usa 50 ml de esa salsa."
- Solución con Programación: Estructura recursiva de inventario (Un producto que es receta e insumo a la vez).
- Implementación Técnica:
  - El sistema permite clasificar un ítem como PREPARACION.
  - El lunes, el chef ingresa al sistema y presiona: "Producir 5 Litros de Salsa BBQ".
  - El backend ejecuta una transacción ACID: Resta los tomates y el azúcar de la bodega, y Suma 5000 ml al stock de "Salsa BBQ". Luego, el plato "Alitas" simplemente tiene como ingrediente "50 ml de Salsa BBQ".
#### 4. Descarga Inmediata y Segura (Kardex Inmutable)
- El Requisito del Cliente: "Si vendo 50 cervezas el viernes por la noche, quiero que el sistema me diga exactamente cuántas me quedan en tiempo real, para saber si debo mandar a comprar más a la licorería."
- Solución con Programación: Arquitectura Event Sourcing (Libro Mayor o Ledger). Regla de oro: El stock nunca se actualiza (UPDATE), solo se insertan movimientos (INSERT).
- Implementación Técnica:
  - En lugar de tener una tabla con un campo stock_actual que se sobrescribe (lo cual causa errores si dos meseros venden al mismo tiempo), se utiliza una tabla kardex_movimientos (id, insumo_id, tipo: IN/OUT, cantidad, orden_id).
  - El stock real de la cerveza se calcula en milisegundos sumando todas las entradas y restando las salidas. Para que esto sea rápido en PostgreSQL, se crean Vistas Materializadas (Materialized Views) que pre-calculan estos totales cada pocos minutos, manteniendo el servidor (ya sea en Azure o Render) libre de sobrecargas de CPU.
#### 5. Toma Física y Cálculo de Mermas (Pérdidas)
- El Requisito del Cliente: "El sistema dice que tengo 10 Kilos de carne, pero abro la refrigeradora y hay 8 Kilos porque se dañaron 2. Necesito cuadrar eso y saber cuánta plata perdí."
- Solución con Programación: Módulo de Auditoría de Inventario de "Ciego" y conciliación de costos.
- Implementación Técnica:
  - Toma Ciega: El bodeguero entra al sistema desde una tablet. El sistema le pide "Cuenta la carne", pero no le dice cuánto debería haber. El bodeguero ingresa: "8 Kilos".
  - El sistema detecta la discrepancia (Faltan 2 Kilos). Le exige al bodeguero seleccionar un motivo (Ej: "Caducidad", "Error de porcionamiento", "Derrame").
  - El backend registra un movimiento tipo AJUSTE_MERMA por -2 Kilos. Multiplica esos 2 Kilos por el costo promedio de compra de esa carne, y lo envía a un "Reporte de Mermas". Así el dueño sabe que este mes perdió $15 en carne dañada.
### 6. Control de Stock Diario en Tiempo Real (Evitar el "Ya no hay")
- El Requisito del Cliente: "Hoy preparé solo 30 'Secos de Pollo' y 15 'Ceviches'. Mis meseros necesitan ver en su celular exactamente cuántos quedan antes de ofrecerlos, y si piden el último, que desaparezca o se bloquee en los celulares de los demás al instante."
- Solución con Programación: Asignación de "Cupos Diarios", Reservas Temporales en Memoria y Sincronización por WebSockets.
- Implementación Técnica:Paso 1: La Configuración del Dueño (Backoffice)
  - En el panel web, el administrador tiene una pantalla de "Stock de Hoy". Busca el plato (ej: Seco de Pollo) y activa un interruptor (Toggle iOS): [Activar Límite Diario] -> Cantidad: 30.
- Paso 2: Interfaz Visual del Mesero (Prevención)
  - En la app móvil (Flutter), la tarjeta del "Seco de Pollo" mostrará una pequeña etiqueta visual (Badge) en la esquina superior que dice "Quedan 30".
  - A medida que el número baja, el color cambia: Verde (seguro), Amarillo (quedan menos de 5), y Rojo vibrante (Queda 1).
  - Cuando llega a 0, el botón del plato se pone gris opaco (Disabled) y muestra un letrero de "Agotado". El mesero no puede ni siquiera hacerle clic.
- Paso 3: El "Bloqueo Suave" (Soft-Locking) para Alta Concurrencia
  - Aquí está la magia técnica: ¿Qué pasa si quedan 2 Ceviches y tres meseros los están pidiendo al mismo tiempo?
  - Utilizamos Reservas Temporales. En el instante en que el Mesero A toca el Ceviche y lo añade a su carrito de compras (incluso antes de enviar la comanda a cocina), la app envía un evento rápido (Ping) al Nodo Local por WiFi.
  - El Nodo Local (Go) resta "1" de forma temporal y avisa por WebSocket a los demás celulares: "Atención, ahora solo queda 1 Ceviche".
  - Si el Mesero A se arrepiente y borra el Ceviche de su carrito (haciendo swipe para eliminar), el Nodo Local devuelve ese Ceviche al contador general y los demás celulares vuelven a mostrar "Quedan 2".
  - Confirmación Final (Hard-Lock): Cuando el mesero finalmente presiona "Enviar Comanda", el inventario diario se descuenta de forma permanente.
### 7. Recarga Rápida de Lote en Caliente (Hot-Reload de Platos Agotados)
- El Requisito del Cliente: "A veces a las 2 de la tarde se me acaba el 'Seco de Pollo', el sistema lo bloquea en cero, pero el cocinero me avisa que acaba de sacar una olla nueva con 30 platos más. No tengo tiempo de ir a configuraciones complejas para volver a habilitarlo, necesito sumarlos en 2 segundos."
- Solución con Programación: Botón de "Acción Rápida" (Quick Action) en la pantalla principal de Caja/KDS y emisión de eventos de reactivación por WebSockets.
- Implementación Técnica:Paso 1: La Interfaz de Recarga (Cero Fricción)
  - En la pantalla del Cajero (React/SolidJS) o en la tablet de la Cocina, existirá un panel lateral colapsable llamado "Platos Limitados".
  - Ahí aparecerá el "Seco de Pollo" marcado en rojo con un gran botón de [ + ].
  - Al presionarlo, se abre un pequeño teclado numérico en pantalla (tipo calculadora). El administrador digita "30" y presiona "Confirmar". (Solo toma 2 clics).
- Paso 2: Reactivación en Tiempo Real (La Magia de los WebSockets)
  - Al confirmar, el frontend envía el evento al Nodo Local (Go): {"action": "REFILL_STOCK", "plato_id": "seco_pollo", "cantidad": 30}.
  - El Nodo Local suma los 30 platos a su memoria temporal e inmediatamente dispara un broadcast a todos los celulares de los meseros.
  - Efecto Visual: El botón gris de "Agotado" en los celulares de los meseros mágicamente cobra vida, vuelve a encenderse en color verde y la etiqueta cambia a "Quedan 30". Los meseros pueden seguir vendiendo en ese mismo instante sin tener que reiniciar la aplicación.
- Paso 3: Seguridad Anti-Fraude (Auditoría Invisible)
  - Para evitar que un cajero deshonesto agregue platos falsos para venderlos por fuera y quedarse con el dinero, esta acción de recarga registra una huella silenciosa en la base de datos (PostgreSQL).
  - El sistema guardará un registro: "A las 14:15, el usuario 'Cajero_Admin' inyectó +30 unidades al plato Seco de Pollo". Al final del día, el dueño verá en su reporte que el plato inició con 30, se recargaron 30, y se vendieron 60, cuadrando el dinero perfectamente.
### Fase 4: Requisitos Detallados de Reportes, Analítica y Cierres Contables
El Backoffice (Panel Administrativo en la Nube) es donde el dueño del restaurante pasa la mayor parte de su tiempo. La regla de oro aquí es: El dueño no es contador ni programador; necesita ver gráficos claros, números exactos y detectar fugas de dinero en segundos.
Aquí tienes la matriz técnica para el manejo del dinero y la analítica:
#### 1. Cierre de Caja "Ciego" (Anti-Robo y Cuadre Exacto)
- El Requisito del Cliente: "Si el sistema le dice al cajero que debe haber $500 en efectivo, y el cajero tiene $520 en la gaveta, se guarda los $20 al bolsillo y me cuadra la caja perfecta. Necesito evitar eso."
- Solución con Programación: Máquina de estados finitos (State Machine) para el flujo de Cierre Z y validación oculta.
- Implementación Técnica:
  - Al final del turno, el cajero presiona "Cerrar Caja". El sistema oculta cuánto se vendió.
  - La pantalla muestra un formulario estilo calculadora. El cajero tiene que contar físicamente los billetes, monedas y sumar los vouchers de tarjetas de crédito, y digitar: "Tengo $480 en Efectivo y $120 en Tarjeta".
  - Al presionar "Procesar", el backend en Go cruza los datos digitados contra la suma de la tabla ordenes (filtrada por el ID del turno actual).
  - El sistema genera un Cierre Z inmutable que arroja el resultado: [Sobrante de $20] o [Faltante de $5]. Este reporte se envía instantáneamente por correo al dueño y bloquea la caja hasta el día siguiente.
#### 2. Distribución de Propinas (Tronco Común vs. Individual)
- El Requisito del Cliente: "El 10% de servicio por ley se reparte a fin de mes, pero nosotros lo dividimos así: 70% para los meseros (según quién vendió qué) y 30% para la cocina. Hacer esto a mano en Excel me toma horas."
- Solución con Programación: Algoritmos de agregación ponderada y reportes parametrizables.
- Implementación Técnica:
  - En la base de datos (PostgreSQL), cada orden tiene su columna monto_propina_ley vinculada a un mesero_id.
  - En el Panel de Configuración, el dueño define reglas de distribución: Ej. Fondo Cocina = 30%.
  - El motor de base de datos ejecuta una consulta con GROUP BY por mesero_id sobre un rango de fechas. Calcula el 10% total recolectado, extrae el 30% para el fondo de cocina, y el resto lo asigna a cada mesero proporcionalmente a sus ventas.
  - El dueño solo hace un clic en "Generar Rol de Propinas" y obtiene un PDF exacto para pagarles.
#### 3. Panel de Analítica en Tiempo Real (Live Dashboard)
- El Requisito del Cliente: "Si estoy de viaje en otro país, quiero abrir mi iPad, ver cómo está el restaurante en vivo, qué mesas están llenas y cuánto vamos facturando, sin tener que llamar al administrador."
- Solución con Programación: Server-Sent Events (SSE) o WebSockets conectando la base de datos central en la nube con el Frontend del Backoffice.
- Implementación Técnica:
  - El Nodo Local (Edge) sincroniza las ventas a la nube (AWS/Render) en background.
  - La aplicación web del dueño (React/SolidJS) mantiene una conexión SSE abierta con el servidor en la nube.
  - Se renderizan componentes gráficos interactivos:
    - Termómetro de Ventas: Una barra de progreso que compara las ventas de hoy a las 14:00 vs las ventas del martes pasado a la misma hora.
    - Top Platos: Un gráfico circular (Chart.js) que muestra los 5 platos más vendidos.
    - Mapa de Calor: Un pequeño plano donde las mesas se pintan de rojo si llevan más de 45 minutos ocupadas (indicador de rotación lenta).
#### 4. Exportación Contable y Anexo Transaccional (ATS - SRI)
- El Requisito del Cliente: "Mi contador me cobra carísimo porque tiene que pasar a mano todas las ventas al sistema contable. Necesito que el sistema le dé todo digerido en Excel o en el formato del SRI."
- Solución con Programación: Generadores de archivos planos (CSV/Excel) y XML según esquemas XSD gubernamentales.
- Implementación Técnica:
  - En el módulo "Contabilidad", existe un selector de fechas (Ej: 1 al 31 de Agosto).
  - El backend usa una librería (como excelize en Go) para generar un archivo .xlsx (Excel) con columnas estandarizadas: Fecha, Comprobante, Cédula, Nombre, Base 0%, Base 12/15%, IVA, Total.
  - Adicionalmente, se incluye un botón "Generar ATS". El backend compila la información de compras y ventas en el formato XML específico que exige el SRI (Anexo Transaccional Simplificado), permitiendo al contador subirlo directamente al portal del gobierno sin digitar un solo número.
#### 5. Auditoría de Mermas y Descuadres (Log de Trazabilidad)
- El Requisito del Cliente: "Si a fin de mes perdí plata, quiero saber si fue porque el cajero anuló 50 facturas, si el mesero regaló platos, o si se pudrieron 20 kilos de carne."
- Solución con Programación: Registro de eventos (Event Logging) de nivel administrativo.
- Implementación Técnica:
  - Toda acción crítica (Anular factura, Aplicar Descuento a una mesa, Eliminar un ítem ya enviado a cocina, Hacer ajuste de inventario manual) requiere insertar una fila en la tabla auditoria_critica.
  - El panel muestra la "Matriz de Fugas": Una tabla roja donde el dueño puede ver: "25 de Agosto - Cajero Carlos - Anuló Mesa 4 ($45) - Motivo: Cliente se fue sin pagar". Esto permite tomar acciones administrativas inmediatas.
Con estas 4 fases, tenemos cubierto el 100% de la funcionalidad de negocio, operativa, técnica y legal de Ecuador. Hemos diseñado un sistema que no se cae sin internet, maneja concurrencia, evita robos, factura automáticamente y entrega data limpia al contador.
### Fase 4: Requisitos Detallados de Reportes, Analítica y Cierres Contables
El Backoffice (Panel Administrativo en la Nube) es donde el dueño del restaurante pasa la mayor parte de su tiempo. La regla de oro aquí es: El dueño no es contador ni programador; necesita ver gráficos claros, números exactos y detectar fugas de dinero en segundos.
Aquí tienes la matriz técnica para el manejo del dinero y la analítica:
#### 1. Cierre de Caja "Ciego" (Anti-Robo y Cuadre Exacto)
- El Requisito del Cliente: "Si el sistema le dice al cajero que debe haber $500 en efectivo, y el cajero tiene $520 en la gaveta, se guarda los $20 al bolsillo y me cuadra la caja perfecta. Necesito evitar eso."
- Solución con Programación: Máquina de estados finitos (State Machine) para el flujo de Cierre Z y validación oculta.
- Implementación Técnica:
  - Al final del turno, el cajero presiona "Cerrar Caja". El sistema oculta cuánto se vendió.
  - La pantalla muestra un formulario estilo calculadora. El cajero tiene que contar físicamente los billetes, monedas y sumar los vouchers de tarjetas de crédito, y digitar: "Tengo $480 en Efectivo y $120 en Tarjeta".
  - Al presionar "Procesar", el backend en Go cruza los datos digitados contra la suma de la tabla ordenes (filtrada por el ID del turno actual).
  - El sistema genera un Cierre Z inmutable que arroja el resultado: [Sobrante de $20] o [Faltante de $5]. Este reporte se envía instantáneamente por correo al dueño y bloquea la caja hasta el día siguiente.
#### 2. Distribución de Propinas (Tronco Común vs. Individual)
- El Requisito del Cliente: "El 10% de servicio por ley se reparte a fin de mes, pero nosotros lo dividimos así: 70% para los meseros (según quién vendió qué) y 30% para la cocina. Hacer esto a mano en Excel me toma horas."
- Solución con Programación: Algoritmos de agregación ponderada y reportes parametrizables.
- Implementación Técnica:
  - En la base de datos (PostgreSQL), cada orden tiene su columna monto_propina_ley vinculada a un mesero_id.
  - En el Panel de Configuración, el dueño define reglas de distribución: Ej. Fondo Cocina = 30%.
  - El motor de base de datos ejecuta una consulta con GROUP BY por mesero_id sobre un rango de fechas. Calcula el 10% total recolectado, extrae el 30% para el fondo de cocina, y el resto lo asigna a cada mesero proporcionalmente a sus ventas.
  - El dueño solo hace un clic en "Generar Rol de Propinas" y obtiene un PDF exacto para pagarles.
#### 3. Panel de Analítica en Tiempo Real (Live Dashboard)
- El Requisito del Cliente: "Si estoy de viaje en otro país, quiero abrir mi iPad, ver cómo está el restaurante en vivo, qué mesas están llenas y cuánto vamos facturando, sin tener que llamar al administrador."
- Solución con Programación: Server-Sent Events (SSE) o WebSockets conectando la base de datos central en la nube con el Frontend del Backoffice.
- Implementación Técnica:
  - El Nodo Local (Edge) sincroniza las ventas a la nube (AWS/Render) en background.
  - La aplicación web del dueño (React/SolidJS) mantiene una conexión SSE abierta con el servidor en la nube.
  - Se renderizan componentes gráficos interactivos:
    - Termómetro de Ventas: Una barra de progreso que compara las ventas de hoy a las 14:00 vs las ventas del martes pasado a la misma hora.
    - Top Platos: Un gráfico circular (Chart.js) que muestra los 5 platos más vendidos.
    - Mapa de Calor: Un pequeño plano donde las mesas se pintan de rojo si llevan más de 45 minutos ocupadas (indicador de rotación lenta).
#### 4. Exportación Contable y Anexo Transaccional (ATS - SRI)
- El Requisito del Cliente: "Mi contador me cobra carísimo porque tiene que pasar a mano todas las ventas al sistema contable. Necesito que el sistema le dé todo digerido en Excel o en el formato del SRI."
- Solución con Programación: Generadores de archivos planos (CSV/Excel) y XML según esquemas XSD gubernamentales.
- Implementación Técnica:
  - En el módulo "Contabilidad", existe un selector de fechas (Ej: 1 al 31 de Agosto).
  - El backend usa una librería (como excelize en Go) para generar un archivo .xlsx (Excel) con columnas estandarizadas: Fecha, Comprobante, Cédula, Nombre, Base 0%, Base 12/15%, IVA, Total.
  - Adicionalmente, se incluye un botón "Generar ATS". El backend compila la información de compras y ventas en el formato XML específico que exige el SRI (Anexo Transaccional Simplificado), permitiendo al contador subirlo directamente al portal del gobierno sin digitar un solo número.
#### 5. Auditoría de Mermas y Descuadres (Log de Trazabilidad)
- El Requisito del Cliente: "Si a fin de mes perdí plata, quiero saber si fue porque el cajero anuló 50 facturas, si el mesero regaló platos, o si se pudrieron 20 kilos de carne."
- Solución con Programación: Registro de eventos (Event Logging) de nivel administrativo.
- Implementación Técnica:
  - Toda acción crítica (Anular factura, Aplicar Descuento a una mesa, Eliminar un ítem ya enviado a cocina, Hacer ajuste de inventario manual) requiere insertar una fila en la tabla auditoria_critica.
  - El panel muestra la "Matriz de Fugas": Una tabla roja donde el dueño puede ver: "25 de Agosto - Cajero Carlos - Anuló Mesa 4 ($45) - Motivo: Cliente se fue sin pagar". Esto permite tomar acciones administrativas inmediatas.
Con estas 4 fases, tenemos cubierto el 100% de la funcionalidad de negocio, operativa, técnica y legal de Ecuador. Hemos diseñado un sistema que no se cae sin internet, maneja concurrencia, evita robos, factura automáticamente y entrega data limpia al contador.
Entramos a la Fase 5: Arquitectura Multi-Tenant y Orquestación SaaS. Esta es la piedra angular para que tu sistema escale de 1 a 1,000 restaurantes sin que los datos se crucen y manteniendo los costos de infraestructura bajo control.
Para un sistema SaaS moderno, de alto rendimiento y con nodos locales (Edge Nodes), la mejor estrategia no es crear una base de datos entera por cada cliente (lo cual sería insostenible de administrar), sino utilizar un modelo de Base de Datos Compartida con Aislamiento Lógico.
A continuación, el diseño técnico de esta arquitectura:
## 1. Aislamiento Lógico en PostgreSQL (Row-Level Security)
Para garantizar que el Restaurante A jamás vea las ventas del Restaurante B, implementaremos un modelo Multi-Tenant utilizando una columna identificadora y políticas de seguridad a nivel de motor de base de datos.
- El Identificador Universal (tenant_id): Toda tabla transaccional o maestra (mesas, productos, ordenes, usuarios) tendrá obligatoriamente una columna tenant_id (UUID).
- Seguridad a Nivel de Fila (RLS - Row Level Security): Para evitar que un error de programación en Go olvide poner el filtro WHERE tenant_id = X y exponga datos, activaremos RLS nativo de PostgreSQL.
- Funcionamiento: Cuando el backend de Go abre una conexión a la base de datos, inyecta el contexto de la sesión actual. PostgreSQL evalúa la política RLS y automáticamente oculta cualquier fila que no pertenezca al tenant_id de esa sesión, haciendo literalmente imposible la fuga cruzada de datos, incluso ante errores humanos de código.
## 2. Autenticación y Enrutamiento del Nodo Local
¿Cómo sabe el pequeño programa instalado en la PC del restaurante a qué cuenta de la nube debe enviar las facturas?
- API Keys por Tenant: Al momento en que un restaurante compra su suscripción, el Backoffice en la nube genera una Edge_API_Key encriptada y un tenant_id.
- El Handshake Inicial: Cuando el dueño instala el programa (Nodo Local) en su Windows/Mac, el sistema le pide esa llave. El nodo se conecta a la nube, valida la firma, descarga el catálogo de productos específico de su tenant_id y configura su base SQLite local.
- Sincronización Blindada: Cada vez que el Nodo Local envía una venta a la nube, el payload (JSON) viaja firmado por un JSON Web Token (JWT) que contiene el tenant_id. El servidor en Go lee el token y enruta los datos a la partición correcta en milisegundos.
## 3. Orquestación del Despliegue (CI/CD) e Infraestructura
Para mantener este nivel de complejidad bajo control, el flujo de desarrollo y despliegue debe estar automatizado al 100%, conectando repositorios de código con infraestructura elástica.
- Control de Versiones y Pruebas Automatizadas: Todo el código fuente de los distintos módulos (Go, React, Flutter) vivirá en repositorios aislados en GitHub. Al realizar un Push a la rama de producción, se disparan flujos de GitHub Actions. Estos flujos levantarán entornos efímeros para ejecutar tus scripts de prueba End-to-End con Cypress (simulando clics en el cajero y cruce de mesas). Si una prueba falla, el despliegue se aborta, protegiendo a los clientes.
- Backend y Bases de Datos: Una vez que las pruebas pasan, GitHub Actions construye los contenedores Docker y despliega la API REST escrita en Go directamente en servicios en la nube de alta disponibilidad como Render o en tu propio entorno escalable dentro de un Azure VPS (manejado vía llaves SSH).
- Almacenamiento de Archivos: Las imágenes del menú, los reportes en PDF y los backups diarios en formato .tar.gz que envían los nodos locales se dirigen automáticamente a buckets seguros en AWS S3, configurados con políticas de ciclo de vida (ej. borrar backups con más de 30 días) para optimizar costos operativos.
## 4. Aprovisionamiento Automático (Onboarding Cero Fricción)
En un SaaS escalable, tú como dueño del software no debes intervenir manualmente cuando entra un nuevo cliente. Todo debe ser un flujo "Self-Service".
- Registro y Pago: El cliente entra a tu landing page, selecciona el Plan "Restaurante", paga con tarjeta de crédito, y el sistema captura sus datos.
- Scripts de Migración en Caliente: El backend recibe el pago, inserta un nuevo registro en la tabla maestra tenants y genera su tenant_id (UUID v4).
- Seed Data (Datos Semilla): El sistema inyecta automáticamente información básica en el entorno del cliente para que no empiece con un sistema en blanco: categorías predeterminadas ("Bebidas", "Platos Fuertes"), 1 mesa de ejemplo y 1 usuario administrador.
- Entrega: El cliente recibe un correo de bienvenida con:
  - La URL de acceso a su Panel Web (Backoffice).
  - El enlace para descargar la app móvil de los meseros.
  - El instalador de un clic del "Nodo Local" y su llave única de activación.
Con esto concluye la estructura técnica y operativa del software. Tienes el diseño de bases de datos, prevención de colisiones concurrentes, contingencia offline, motor fiscal asíncrono para el SRI y la infraestructura de despliegue multi-tenant.
### 1. Nivel Super Administrador -> Dueño del Restaurante
Como dueño del SaaS (Super Admin), tu objetivo es que el cliente (el dueño del restaurante) empiece a usar el sistema sin fricción.
- Creación: Tú ingresas la Cédula o RUC del cliente. El sistema crea automáticamente el "Tenant" (el espacio de ese restaurante).
- Credenciales: El nombre de usuario del dueño será su Cédula/RUC.
- Contraseña Segura: El sistema genera una contraseña temporal alfanumérica aleatoria (Ej: X7k9P2m) y se la envía automáticamente por correo o WhatsApp. Al iniciar sesión por primera vez en su Panel Web, el sistema le obliga a cambiarla por una que él recuerde.
### 2. Nivel Dueño del Restaurante -> Meseros (La Optimización)
Aquí corregimos un cuello de botella: los meseros no deberían tener correos corporativos ni contraseñas alfanuméricas. La rotación de personal en restaurantes es muy alta; el dueño necesita crear un usuario en 10 segundos.
- Flujo del Dueño: Entra a su panel web, va a "Personal", hace clic en "Nuevo Mesero" y solo llena dos cosas: Nombre (Ej. Carlos) y un PIN de 4 dígitos (Ej. 1024). No hay correos, no hay contraseñas largas. El usuario ya está listo para trabajar.
### 3. Nivel Mesero -> Aplicación Móvil (Device Pairing)
Meencionaste que el mesero ponga su usuario/contraseña en su celular y luego el PIN. La optimización extrema es eliminar el primer paso por completo. Si un restaurante usa 3 tablets que se rotan entre los meseros, o si el mesero descargó la app en su celular, usamos un modelo de "Dispositivo Emparejado" (Device Binding).
¿Cómo funciona la App Móvil sin pedir usuario/contraseña al mesero?
- El Emparejamiento (Una sola vez): Cuando el mesero abre la app por primera vez, la pantalla le pide un "Código de Activación del Local" o escanear un Código QR. El dueño del restaurante le muestra ese QR desde su panel web.
- El Entorno Cerrado: Al escanearlo, el celular queda bloqueado y conectado exclusivamente a la base de datos de ese restaurante (Ej: Cevichería Don Pepe).
- La Pantalla de Inicio Rápida: A partir de ese momento, cada vez que se abra la app, no hay campos de texto. Solo aparece una cuadrícula bonita con los nombres y fotos de los meseros activos (Carlos, Ana, Luis).
- Acceso en 1 segundo: Carlos toca su foto, se despliega un teclado numérico gigante estilo iOS, digita 1 0 2 4 y entra a tomar pedidos. Si apaga la pantalla y la vuelve a prender, la app le pide el PIN nuevamente para proteger la sesión.
Ventajas de esta optimización:
- Cero fricción tecnológica: Un mesero nuevo que entra a trabajar el viernes por la noche no necesita memorizar un usuario o pedir que le reseteen la clave. El dueño le dice "Tu PIN es 8899" y empieza a trabajar.
- Dispositivos compartidos: Si un celular se queda sin batería, Carlos simplemente agarra la tablet de la barra, toca su nombre, pone su PIN y sus mesas están ahí intactas.
- Seguridad: Si un mesero es despedido, el dueño simplemente desactiva a "Carlos" en el panel. Su nombre desaparece instantáneamente de todas las pantallas de inicio de los celulares mediante WebSockets.
Para soportar la optimización de "Emparejamiento de Dispositivos" (QR) y el "Inicio de Sesión por PIN", el esquema de la base de datos debe ser flexible. No todos los usuarios tendrán correo y contraseña, y no todos los dispositivos tendrán acceso a la red del restaurante.
Aquí tienes el diagrama relacional optimizado para PostgreSQL, aplicando el aislamiento Multi-Tenant (tenant_id) en cada tabla.
### 1. Tabla tenants (Los Restaurantes - Clientes del SaaS)
Esta tabla es gestionada exclusivamente por ti (el Super Administrador). Define la existencia del restaurante en la nube.

| Columna | Tipo de Dato | Descripción / Regla de Negocio |
|---|---|---|
| id | UUID (PK) | Identificador único del restaurante (Ej. Cevichería Don Pepe). |
| ruc | String (13) | RUC legal para la facturación electrónica. |
| razon_social | String | Nombre legal de la empresa. |
| qr_pairing_token | UUID | Token estático o rotativo que se convierte en el Código QR para enlazar los celulares de los meseros. |
| estado_suscripcion | Enum | ACTIVA, SUSPENDIDA (Si no pagan, se bloquea el acceso). |
### 2. Tabla dispositivos_vinculados (Device Binding)
Controla qué celulares o tablets físicas tienen permiso de conectarse a la base de datos del restaurante, evitando que un ex-empleado use la app desde su casa.

| Columna | Tipo de Dato | Descripción / Regla de Negocio |
|---|---|---|
| id | UUID (PK) | ID único del registro. |
| tenant_id | UUID (FK) | Llave foránea hacia el restaurante. Base del Multi-Tenant. |
| nombre_dispositivo | String | Ej: "iPhone de Carlos", "Tablet Barra 1". |
| device_fingerprint | String | Huella digital única del hardware del celular (MAC/IMEI enmascarado) para evitar clonación. |
| estado | Enum | AUTORIZADO, REVOCADO (El dueño puede revocar un celular robado con un clic). |
### 3. Tabla usuarios (Dueños, Cajeros, Meseros)
Esta tabla unifica a todo el personal, pero usa campos Nullables (opcionales) dependiendo del rol.

| Columna | Tipo de Dato | Descripción / Regla de Negocio |
|---|---|---|
| id | UUID (PK) | ID del usuario. |
| tenant_id | UUID (FK) | Vinculación estricta al restaurante (Aislamiento RLS). |
| nombre_mostrar | String | El nombre que aparece en la cuadrícula de la App (Ej: "Carlos M."). |
| rol | Enum | ADMIN, CAJERO, MESERO, COCINA. |
| email | String (Null) | Obligatorio solo para ADMIN y CAJERO. Nulo para meseros. |
| password_hash | String (Null) | Clave alfanumérica encriptada. Nulo para meseros. |
| pin_hash | String (Null) | El PIN de 4 dígitos encriptado (Bcrypt). Obligatorio para CAJERO y MESERO. |
| avatar_url | String (Null) | Foto del mesero para la interfaz visual de inicio rápido. |
| activo | Boolean | true / false. Si es false, desaparece de la app al instante. |
### 4. Tabla permisos_granulares (Opcional - Para control estricto)
En lugar de crear decenas de roles complejos, se usa una tabla puente si el dueño quiere personalizar qué puede hacer un empleado específico (Ej. Un cajero novato vs. un cajero de confianza).

| Columna | Tipo de Dato | Descripción / Regla de Negocio |
|---|---|---|
| usuario_id | UUID (FK) | ID del cajero o mesero. |
| puede_anular_item | Boolean | ¿Puede borrar un plato que ya se envió a cocina? |
| puede_dar_descuento | Boolean | ¿Puede aplicar descuentos porcentuales a la cuenta final? |
| puede_abrir_cajon | Boolean | ¿Puede disparar la apertura de la gaveta de dinero sin facturar? |
### ¿Cómo fluye esto en el código (Go + Flutter)?
- Emparejamiento (Pasa 1 vez): El mesero instala la app y escanea el QR. La app lee el qr_pairing_token, captura el device_fingerprint del celular y hace una petición a la API en Go. Go inserta el registro en dispositivos_vinculados y devuelve un Token JWT de Dispositivo. El celular guarda este token.
- Carga de Pantalla Inicial: Al abrir la app, Flutter envía el Token de Dispositivo. La API verifica que el dispositivo no esté REVOCADO. Si está bien, le devuelve la lista de la tabla usuarios donde rol = MESERO y activo = true para pintar las fotos en la pantalla.
- El Toque Final (Login): Carlos toca su foto, digita "1024". La app manda {usuario_id: "uuid-de-carlos", pin: "1024"}. El servidor compara el pin_hash. Si es correcto, devuelve un Token JWT de Usuario con una expiración de 12 horas, habilitando los botones de tomar órdenes.
Para conseguir los primeros 10 clientes de pago en el sector gastronómico ecuatoriano, la estrategia debe alejarse del marketing digital masivo y enfocarse en la venta consultiva presencial. El dueño de un restaurante tradicional, al igual que los administradores de reservas de canchas deportivas o locales de repuestos comerciales, es pragmático: solo invierte en herramientas que no interrumpan su operación diaria y que resuelvan problemas inmediatos.
## 1. El Perfil de Cliente Ideal (ICP) Inicial
No apuntes a grandes cadenas ni a franquicias todavía; sus ciclos de venta toman meses y exigen integraciones contables complejas. Tampoco apuntes a cafeterías de una sola persona.
- El blanco perfecto: Restaurantes independientes de alto tráfico (hamburgueserías, pizzerías, cevicherías), con 5 a 15 mesas, 2 a 4 meseros y un cajero.
- El dolor actual: Usan sistemas web que se cuelgan los fines de semana, facturan a mano cuando se cae el portal del SRI, o los meseros chocan entre sí gritando pedidos hacia la cocina.
## 2. La Demostración "Anti-Caídas" (El Caballo de Troya)
Los dueños de restaurantes no compran software leyendo PDFs ni viendo presentaciones en PowerPoint. Tienes que demostrar la arquitectura Edge-Cloud en vivo, directamente en su local.
- El Kit de Venta: Visita el local en horario de baja afluencia (típicamente martes o miércoles entre las 15:30 y 17:00). Lleva contigo una laptop (tu Nodo Local), un teléfono celular (App Flutter de mesero), un router básico de $20 y una impresora térmica pequeña.
- El Momento "Wow": Conecta tus equipos a tu propio router. Ingresa un pedido desde el celular y muestra cómo se imprime en menos de 1 segundo. Luego, desconecta el cable de internet del router. Ingresa otro pedido y muéstrale al dueño cómo el sistema sigue imprimiendo y bloqueando mesas a la perfección sin conexión a la nube. Esa única acción cierra más ventas que cualquier discurso sobre facturación.
## 3. Oferta de Riesgo Cero para "Early Adopters"
Los primeros 10 clientes están asumiendo un riesgo al confiar en un software nuevo. Tu objetivo no es hacerte rico con ellos, sino convertirlos en casos de éxito documentados.
- Cero Costo de Implementación: Elimina el cobro inicial por instalación o configuración del menú (que la competencia suele cobrar entre $100 y $300).
- Transición en Paralelo: No les pidas que apaguen su sistema actual el primer día. Ofréceles correr tu sistema en paralelo durante 14 días para una caja específica o un área del salón.
- Cláusula de Éxito: El acuerdo debe ser claro: si el sistema demuestra ser más rápido y no se cae, firman el contrato de suscripción mensual (SaaS) y te graban un video testimonial de 30 segundos.
## 4. Expansión por Clústers Geográficos
En Ecuador, el sector gastronómico funciona por recomendación de boca en boca ("Word of Mouth").
- Cierra tu primer cliente en una zona comercial concurrida (ej. una plaza de comidas o una calle viva).
- Asegúrate de que la operación sea impecable. La fluidez de la facturación SRI asíncrona y el control exacto de comandas generará curiosidad.
- Utiliza a ese primer cliente como palanca: camina hacia los restaurantes vecinos y diles: "Implementamos el nuevo sistema de comandas anti-caídas en el local de al lado, y logramos que despachen 3 veces más rápido los viernes. Me gustaría mostrarle cómo funciona".
## 5. Abordaje de Objeciones Comunes
- "Ya tengo un sistema contable": Aclara que tu software es un Punto de Venta (POS) ultrarrápido diseñado para la trinchera del restaurante, no un software de contabilidad pesado. Muestra el botón de "Exportar ATS" para calmar a su contador.
- "Me da miedo perder mi menú": Ofrécele el servicio de migración de datos. Toma fotos de su carta física y túmbate el trabajo manual de cargar los primeros 50 platos e ingredientes al sistema para que él empiece a operar al día siguiente sin esfuerzo.
## 1. Lógica de Inventario: Perecibles vs. Percha
Para que las aguas duren varios días y los secos de pollo se reinicien, modificaremos la estructura del producto añadiendo una regla de comportamiento.
En la tabla de productos, añadiremos un campo llamado comportamiento_stock con dos opciones:
- PERMANENTE (Bebidas, Empacados): El inventario es continuo. Si ingresas 100 aguas el lunes y vendes 20, el martes el sistema amanece con 80. Solo aumenta cuando el administrador registra una nueva compra a proveedores.
- DIARIO (Platos Preparados): El stock se asigna cada mañana (Ej: 30 Secos). Al realizar el "Cierre de Caja Z" en la noche, el backend en Go resetea automáticamente este valor a 0. Al día siguiente, el sistema exige que el chef indique cuántos platos nuevos se prepararon antes de habilitarlos para la venta.
## 2. El Menú QR "Vivo" y Atractivo
El QR no apuntará a un PDF estático, sino a una aplicación web ligera (React/Next.js) alojada en la nube, que lee el inventario del Nodo Local.
- Generador de QR Integrado: En el Backoffice, el dueño tendrá un botón "Generar QR". El sistema creará un código de alta resolución con el logo del restaurante en el centro (usando librerías como qrcode.react) y un diseño en los colores de la marca, listo para imprimir y poner en las mesas (acrílicos).
- Sincronización Mágica: Cuando el cliente escanea el QR, la web carga el menú en formato de catálogo digital estilo iOS (imágenes grandes, navegación suave).
- Ocultamiento Automático: Si el Nodo Local registra que la última agua se vendió, o que el Seco de Pollo llegó a 0, el backend envía una señal y ese producto desaparece instantáneamente del menú QR del cliente, o se oscurece con una etiqueta de "Agotado por hoy". Así, el cliente nunca pedirá algo que ya no existe.
## 3. Implementación en la Operación
Para que esto sea rápido de usar:
- En la mañana: El administrador abre su panel, la sección de productos DIARIO está en cero. Digita "30 Ceviches, 20 Churrascos" y presiona Confirmar.
- Los productos PERMANENTE: No se tocan. El sistema ya sabe que quedan 80 aguas del día anterior.
- Durante el día: Los meseros y los clientes (vía QR) consumen de ambas bolsas de inventario de manera transparente.
Esta separación técnica elimina el trabajo manual repetitivo y garantiza que el menú que ve el cliente siempre sea la realidad exacta de la cocina en ese segundo.
