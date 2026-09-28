# 00 · Revisión crítica del documento fuente

| Campo | Valor |
|---|---|
| Documento revisado | `docs/fuentes/documento-original.docx` (transcripción en `docs/fuentes/documento-original.md`) |
| Fecha de revisión | 2026-09-24 |
| Estado | Versión 1.0: pendiente de validación por el Product Owner |
| Alcance | Revisión técnica, funcional, legal (SRI / LOPDP) y de planificación |

## 1. Resumen ejecutivo

El documento fuente tiene una **visión de producto sólida y diferenciada**: arquitectura híbrida Cloud-Edge para operar sin internet, facturación SRI asíncrona, bloqueo de mesas, inventario por recetas y un modelo SaaS por niveles. La mayoría de las ideas son correctas y viables.

Sin embargo, **no está listo para usarse como especificación de desarrollo** por cinco motivos:

1. **Está fragmentado.** Mezcla al menos cuatro numeraciones de "fases" distintas (Fase 1-5 de análisis, Fase 1-4 de requisitos, Fase 5-7 de arquitectura), y la sección *"Fase 4: Reportes, Analítica y Cierres"* aparece **duplicada literalmente**.
2. **Tiene contradicciones tecnológicas.** Por ejemplo React, Vue, SolidJS y Next.js aparecen como opciones, y el P12 se firma unas veces en el nodo local y otras en la nube.
3. **Contiene errores técnicos y legales concretos.** Por ejemplo, la clave de acceso descrita tiene 48 dígitos y no 49, se propone *hashear* la contraseña del P12 (así no se podría usar) y se plantea probar Flutter con Cypress (Cypress no prueba apps Flutter nativas).
4. **Le faltan áreas completas.** No define la resolución de conflictos de sincronización, la protección de datos personales (LOPDP), la numeración secuencial por punto de emisión, la observabilidad, la pasarela de pago del SaaS ni las actualizaciones del Nodo Local.
5. **El cronograma de 5 meses no es realista** para el alcance descrito (6 épicas + multi-tenant + SRI + offline + KDS + QR + SaaS).

Esta revisión detalla cada hallazgo con severidad y la corrección adoptada en la documentación normalizada (`docs/01` a `docs/10`).

**Escala de severidad**

- 🔴 **Crítico**: si no se corrige, el sistema falla, es ilegal o es inseguro.
- 🟠 **Alto**: genera retrabajo importante o riesgo operativo.
- 🟡 **Medio**: hay que resolverlo, aunque no bloquea.
- 🔵 **Bajo**: forma, redacción u orden.

---

## 2. Hallazgos de estructura y consistencia

| ID | Sev. | Hallazgo | Corrección adoptada |
|---|---|---|---|
| E-01 | 🟠 | Hay cuatro esquemas de "Fase" superpuestos (análisis, requisitos, arquitectura, negocio). No se sabe qué es una fase de **implementación**. | Se separan **documentos temáticos** (visión, requisitos, arquitectura…) de las **fases de implementación** (`docs/07-plan-de-fases.md`, Fases 0 a 10). |
| E-02 | 🔵 | La sección "Fase 4: Requisitos Detallados de Reportes…" está duplicada palabra por palabra. | Se consolida una sola vez en `requisitos/RF-08-reportes-analitica.md`. |
| E-03 | 🟠 | Los requisitos no tienen identificadores estables (solo HU-01..04 y CU-01..02). Sin IDs no hay trazabilidad requisito → código → prueba. | Todos los requisitos llevan IDs `RF-<módulo>-<nn>` y `RNF-<nn>`, y hay una matriz de trazabilidad en el plan de fases. |
| E-04 | 🟡 | Las métricas de rendimiento se contradicen: la búsqueda de platos se pide en "< 100 ms" (HU-01) y en "< 10 ms" (Req. 3). La comanda se pide en "< 1 s" y la impresión en "< 1,5 s". | Se unifican en `requisitos/RNF-no-funcionales.md` con percentiles medibles (p95/p99) y el punto exacto de medición. |
| E-05 | 🟡 | El esquema de base de datos aparece disperso en cinco secciones, con nombres distintos para lo mismo (`estado` / `estado_general`, `impresora_destino_ip` en categoría y también en detalle). | Hay un único modelo de datos consolidado en `docs/04-modelo-de-datos.md`. |
| E-06 | 🔵 | El texto mezcla la voz del consultor ("tú", "te detallo") con la especificación. | La documentación normalizada usa voz impersonal y formato de especificación. |

## 3. Contradicciones tecnológicas

| ID | Sev. | Contradicción | Decisión propuesta (ver ADRs) |
|---|---|---|---|
| T-01 | 🟠 | Frontend web: "React o Vue 3" (Fase 1), "React o SolidJS" (Fase 4) y "React/Next.js" (menú QR). | **React + TypeScript + Vite** para caja, backoffice y menú QR. SolidJS tiene un ecosistema mucho menor para drag & drop, tablas, gráficos y contratación. → [ADR-0003](adr/0003-frontend-web-react.md) |
| T-02 | 🔴 | Dónde se firma el XML: la Fase 4 dice que lo hace "un servicio de colas en el Servidor Local… lo firma con el certificado P12", y la Fase 7 dice que se firma "en el servidor en la nube… no en la computadora local". | **Se firma en la nube** (el P12 nunca sale de la nube). El Nodo Local **sí** genera la clave de acceso y el secuencial, e imprime el comprobante sin esperar a la nube. → [ADR-0006](adr/0006-motor-fiscal-firma-en-nube.md) |
| T-03 | 🟠 | Redis figura como almacén de "comandas activas", pero en la arquitectura Edge las comandas viven en el Nodo Local (SQLite). También se menciona Redis como cola del worker SRI. | En el Edge se usa SQLite + memoria. En la nube, la cola de trabajos va **sobre PostgreSQL** (encolado transaccional, sin infraestructura extra). Redis queda como opcional para fan-out de tiempo real con varias instancias. → [ADR-0007](adr/0007-colas-sobre-postgresql.md) |
| T-04 | 🟠 | Hosting: Render, AWS, Azure VPS y EcuaHosting (FTP) se mencionan como intercambiables. | Hay que elegir **un** proveedor principal. Queda como decisión pendiente **DP-01** con recomendación. |
| T-05 | 🟡 | "Repositorios aislados en GitHub" para Go, React y Flutter, pero el proyecto ya tiene un repositorio único. | **Monorepo**: contratos compartidos (API/eventos), un único CI y cambios atómicos entre backend y clientes. → [ADR-0001](adr/0001-monorepo.md) |
| T-06 | 🟡 | La app móvil guarda el menú en "SQLite/Hive". | **SQLite (drift)** en el móvil: consultas, índices y transacciones iguales a las del Edge. |
| T-07 | 🟡 | El stock se describe como "tiempo real", pero a la vez se calcula con vistas materializadas que se refrescan "cada pocos minutos". | Ledger inmutable (kardex) **más** una tabla de saldos actualizada en la **misma transacción**. Las vistas materializadas solo se usan para reportes. |

## 4. Errores técnicos

| ID | Sev. | Error en el documento fuente | Corrección |
|---|---|---|---|
| X-01 | 🔴 | *"La contraseña del P12 se encripta con algoritmos de hashing fuertes"*. Un hash es irreversible: el sistema no podría abrir el P12 para firmar. | **Cifrado simétrico reversible** (AES-256-GCM) con *envelope encryption* y llave maestra en un KMS o gestor de secretos. Nunca se registra en logs. |
| X-02 | 🔴 | *"Se debe construir un script de Cypress que abra dos instancias simuladas de la app [Flutter]"*. Cypress solo automatiza navegadores y no puede controlar una app Flutter nativa. Además, la carrera de bloqueo a nivel de milisegundo no se prueba de forma fiable desde la UI. | La concurrencia se prueba en el **backend** (tests Go con N goroutines contra el Nodo Local, más `-race`). La app Flutter se prueba con `integration_test`/Patrol. Cypress se usa **solo** para la web (caja/backoffice). Hay pruebas de carga con k6. |
| X-03 | 🟠 | *"device_fingerprint: MAC/IMEI enmascarado"*. iOS y Android (10+) **no permiten** leer IMEI ni la MAC real. | **Credencial ligada al dispositivo**: par de llaves generado en el Keystore/Keychain del equipo. El servidor guarda la llave pública y el dispositivo firma un desafío. |
| X-04 | 🔴 | PIN de 4 dígitos con *"hash local y se compara con el almacenado en el caché del dispositivo"*. Hay 10.000 combinaciones: cualquiera con el teléfono rompe un hash bcrypt de 4 dígitos en segundos. | El PIN **se valida en el Nodo Local**, nunca contra un hash guardado en el móvil. Hay límite de intentos, bloqueo temporal y *pepper* del lado servidor. Solo funciona en un dispositivo emparejado. PIN configurable de 4 a 6 dígitos. |
| X-05 | 🟠 | JWT de 12 horas para el mesero y también "tiempos de expiración cortos… evitando que un dispositivo robado mantenga acceso". Un JWT emitido no se puede revocar sin infraestructura adicional. | Access token corto (15 min) + refresh token ligado a dispositivo y turno (hasta 12 h). Existe una lista de revocación (dispositivo o usuario desactivado → se invalida al instante por WebSocket). |
| X-06 | 🟠 | *"Zero-Setup Printing con mDNS"*. La mayoría de impresoras térmicas económicas (Xprinter, Rongta genéricas) **no anuncian mDNS**. | Descubrimiento en capas: mDNS/Bonjour, luego escaneo de la subred en el puerto TCP 9100, luego enumeración USB. También se recomienda (o se guía) reservar la IP por DHCP. |
| X-07 | 🟠 | Se enruta a la impresora por `impresora_destino_ip`, guardada en la categoría **y** en cada `orden_detalle`. Si la IP cambia (DHCP), todo el historial queda apuntando mal. | Modelo categoría → **estación de producción** (Cocina caliente, Bar…) → impresora(s). La línea de orden guarda la estación y no la IP. La IP es un atributo mutable de la impresora. |
| X-08 | 🟠 | UUID **v4** como PK en todas las tablas. Funciona, pero fragmenta los índices B-tree de PostgreSQL por su aleatoriedad total, lo que degrada las inserciones a gran volumen. | **UUID v7** (ordenable por tiempo, RFC 9562). Conserva la generación offline sin colisiones y mejora la localidad del índice. → [ADR-0005](adr/0005-identificadores-uuidv7.md) |
| X-09 | 🔴 | No se define la **resolución de conflictos** de la sincronización bidireccional ("sincronización silenciosa"). ¿Qué pasa si el dueño cambia un precio en la nube mientras el Edge vende sin internet? | **Propiedad de datos por dominio**: la nube es dueña del catálogo y la configuración, y el Edge es dueño de la operación (órdenes, pagos, turnos). Patrón *outbox* con claves de idempotencia, reloj de versión por registro y reglas explícitas por entidad. → [ADR-0004](adr/0004-sincronizacion-edge-cloud.md) |
| X-10 | 🟠 | División de cuenta: *"anula la orden original y crea sub-órdenes"*. Rompe la trazabilidad (la comanda de cocina queda "anulada") y no permite dividir **un** ítem entre dos personas. | La orden se mantiene y se crean **cuentas de cobro** (`cuentas`) con asignaciones de líneas o de fracciones de línea. Cada cuenta genera su propia factura. |
| X-11 | 🟠 | Reserva temporal de stock diario (*soft-lock*) sin expiración. Si el celular del mesero se apaga con el plato en el carrito, ese plato queda retenido para siempre. | Toda reserva tiene un TTL (p. ej. 10 min) renovado por *heartbeat*, igual que el bloqueo de mesa. |
| X-12 | 🟡 | Bloqueo de mesa con timeout de 45 s "sin actividad", pero un mesero puede tardar más de 45 s en tomar un pedido grande. | Mientras la pantalla de la mesa está abierta, la app envía un *heartbeat* cada 10 s. El timeout de 45 s solo corre sin *heartbeats*. |
| X-13 | 🟡 | Backup: "detiene temporalmente la sincronización" y hace un dump `.sql` de SQLite. | Se usa la **Online Backup API** / `VACUUM INTO` de SQLite (copia consistente sin detener la operación). El respaldo se **cifra** antes de subirlo porque contiene datos personales. FTP → SFTP o solo S3. |
| X-14 | 🟡 | Retención de backups: 7 días local y 30 días en S3 (dos cifras distintas). | Se define una política única en `docs/03-arquitectura.md` §9. |
| X-15 | 🟡 | Stock DIARIO "se resetea al hacer el Cierre Z". Con varias cajas o turnos por día, el primer cierre borraría el stock del día. | El reset se ata a la **jornada operativa** (apertura/cierre del día del local), no al cierre de un turno de caja. |
| X-16 | 🟡 | Dinero y cantidades: no se especifica el tipo numérico. Con `float` aparecen errores de redondeo en facturas. | `NUMERIC` en PostgreSQL, *decimal* en Go/Dart/TS y reglas de redondeo del SRI documentadas. Las cantidades de inventario usan `NUMERIC(14,4)`. |

## 5. Errores y riesgos legales / fiscales (SRI Ecuador)

> ⚖️ Toda afirmación normativa de este proyecto debe validarse contra la **Ficha Técnica de Comprobantes Electrónicos (esquema offline) vigente** publicada por el SRI y con un contador o tributarista antes de pasar a producción. La normativa cambia (por ejemplo, el IVA pasó del 12 % al 15 % en abril de 2024).

| ID | Sev. | Hallazgo | Corrección |
|---|---|---|---|
| L-01 | 🔴 | **Clave de acceso**: el documento la describe como Fecha(8) + Tipo(2) + RUC(13) + Ambiente(1) + Serie(6) + Secuencial(9) + Código numérico(8) + Dígito verificador(1), lo que **suma 48**, no 49. Falta el campo **Tipo de emisión (1)**, que va antes del dígito verificador. | Estructura correcta de 49 dígitos en `docs/05-facturacion-electronica-sri.md` §4. |
| L-02 | 🔴 | *"Clave de acceso en proceso"* y *"reintenta… hasta obtener la clave de acceso"*. En el esquema offline **la clave de acceso la genera el emisor**, no el SRI, y el número de autorización es la misma clave. Lo que el SRI devuelve es el **estado** (RECIBIDA/DEVUELTA, y luego AUTORIZADO/NO AUTORIZADO). | El ticket impreso **ya lleva** la clave de acceso de 49 dígitos. El reintento busca la **autorización**, no la clave. |
| L-03 | 🔴 | El **secuencial** de factura no está modelado. Si dos cajas, o la nube y el Edge, emiten con el mismo establecimiento + punto de emisión, se duplican los números y el SRI rechaza los comprobantes. | Cada caja física es un **punto de emisión** propio (001-001, 001-002…). El secuencial lo asigna **solo** el Nodo Local dueño de ese punto, de forma atómica y persistente. |
| L-04 | 🟠 | Contingencia: *"hasta por 72 horas"*. El plazo de envío al SRI en el esquema offline debe confirmarse en la normativa vigente. | El plazo es un parámetro configurable. Se alerta al dueño **mucho antes** del vencimiento (p. ej. a las 12 h sin autorizar). Verificar el plazo → **DP-07**. |
| L-05 | 🟠 | *"Consumidor Final - Efectivo" como botón gigante* sin límite. El SRI fija un **monto máximo** para facturas a consumidor final (la normativa reciente lo fija en USD 50; verificar el valor vigente). | Validación bloqueante: si el total supera el límite, el sistema exige los datos del comprador. |
| L-06 | 🟠 | *"API de Registro Civil"* para autocompletar por cédula. No existe una API pública abierta; el acceso requiere convenio (DINARDAP) o un proveedor privado, y los datos obtenidos son datos personales. | Orden de búsqueda: base local de clientes, luego base del tenant en la nube, luego un proveedor externo **opcional** y configurable. Siempre se valida el dígito verificador de cédula (módulo 10) y de RUC (módulo 11) localmente. |
| L-07 | 🟠 | *"Auto-detección de régimen mediante scraping autorizado al SRI"*. El scraping es frágil y puede violar los términos del SRI. | El régimen se **pre-rellena** con una consulta cuando sea posible y **siempre lo confirma el usuario**. Las leyendas se derivan de la configuración, no del scraping. |
| L-08 | 🟠 | Los **RIMPE – Negocio Popular** emiten **notas de venta**, no facturas (salvo excepciones). El documento solo contempla facturas. | Decisión de alcance **DP-05**: soportar notas de venta, o excluir Negocio Popular del mercado objetivo inicial. |
| L-09 | 🟡 | ATS (Anexo Transaccional Simplificado) como botón para todos. El ATS no aplica a todos los contribuyentes, y para comprobantes electrónicos el detalle de ventas tiene reglas particulares. | El ATS pasa a una fase tardía y opcional, validado con un contador (**DP-08**). La exportación Excel sí se incluye en el MVP. |
| L-10 | 🟠 | *"Anular factura"* desde el sistema. Una factura **autorizada** no se borra ni se "anula" libremente: se emite una **Nota de Crédito** o se solicita la anulación en el portal del SRI dentro de los plazos y condiciones vigentes. | Flujo formal de reverso con Nota de Crédito (tipo 04) y auditoría. Nunca hay DELETE. |
| L-11 | 🟡 | *"Propina 10 %"* tratada como obligatoria. El 10 % de servicio aplica a ciertos establecimientos (por categoría turística). No todos los restaurantes lo cobran. | La propina legal es un parámetro por local (activado o no) y se refleja en el campo `<propina>` del XML sin base de IVA. |
| L-12 | 🔴 | **Protección de datos personales** (LOPDP, Registro Oficial 26/05/2021, régimen sancionatorio vigente desde 2023): no se menciona. El sistema guarda cédulas, correos, teléfonos y los PINs y datos laborales de los empleados. | Sección completa en `docs/06-seguridad.md` §7: base legal, finalidad, retención, derechos ARCO, cifrado, contrato de encargado de tratamiento con cada restaurante y transferencia internacional (si la nube está fuera de Ecuador). |
| L-13 | 🟡 | Conservación de comprobantes: borrar los PDFs locales está bien, pero **el emisor debe conservar los XML autorizados** durante el plazo legal (mínimo 7 años según el Código Tributario; verificar). | Bucket S3 con **versionado y Object Lock** (modo *compliance*) para los XML autorizados. Las reglas de ciclo de vida **no** se aplican a comprobantes. |
| L-14 | 🟡 | Entidades certificadoras: el documento lista *Security Data, Uanataca, Banco Central*. La lista de entidades acreditadas cambia. | La guía de onboarding apunta a la lista oficial de entidades acreditadas (ARCOTEL) y no hardcodea nombres. |
| L-15 | 🟡 | Tu propio SaaS también debe **emitir factura electrónica** a cada restaurante por la suscripción. | Se incluye en la Fase 9 (usa el mismo motor fiscal, con el tenant "plataforma"). |

## 6. Vacíos (lo que falta y es necesario)

| ID | Sev. | Área faltante | Dónde se cubre ahora |
|---|---|---|---|
| G-01 | 🔴 | **Actualización del Nodo Local** (auto-update firmado, rollback, compatibilidad de versiones Edge ↔ Cloud ↔ App). Con 100 restaurantes no se puede ir local por local. | `03-arquitectura.md` §8 |
| G-02 | 🔴 | **Observabilidad**: logs estructurados, métricas, trazas y *health* de cada Nodo Local (¿está vivo?, ¿cuántas facturas sin autorizar tiene?). | `03-arquitectura.md` §10 |
| G-03 | 🟠 | **Pasarela de pago** del SaaS. Stripe no opera con comercios domiciliados en Ecuador; las opciones locales son Payphone, Kushki, Datafast o PlacetoPay. | `09-modelo-de-negocio.md` §4, **DP-06** |
| G-04 | 🟠 | **Funcionamiento del Nodo Local si la PC de caja se apaga** o se reinicia en pleno servicio (punto único de fallo). | `03-arquitectura.md` §7 (servicio del SO, arranque automático, UPS recomendada, modo degradado) |
| G-05 | 🟠 | **Descuentos, cortesías, anulaciones de ítems enviados a cocina** con motivo y permiso. | `requisitos/RF-04-caja-pos.md` |
| G-06 | 🟠 | **Turnos y jornada operativa** (apertura de caja con fondo, retiros parciales/sangrías, cambio de cajero). | `requisitos/RF-04-caja-pos.md` |
| G-07 | 🟡 | **Transferir ítems o mesas** (cambiar de mesa, unir mesas). | `requisitos/RF-03-salon-comandas.md` |
| G-08 | 🟡 | **Pedidos para llevar / delivery / barra** (órdenes sin mesa). | `requisitos/RF-03-salon-comandas.md` |
| G-09 | 🟡 | **Reimpresión** de comandas y tickets, y la plantilla de ticket configurable. | `requisitos/RF-02-nodo-local-impresion.md` |
| G-10 | 🟡 | **Estrategia de versionado de la API** y de los contratos de eventos WebSocket. | `03-arquitectura.md` §6 |
| G-11 | 🟡 | **Accesibilidad e idioma** (es-EC, formato de moneda USD, zona horaria America/Guayaquil). | `requisitos/RNF-no-funcionales.md` |
| G-12 | 🟡 | **Entorno de pruebas SRI** por tenant (ambiente 1) antes de pasar a producción. | `requisitos/RF-05-facturacion-sri.md` (RF-05-09) |
| G-13 | 🟡 | **Términos y condiciones, SLA y soporte** del SaaS. | `09-modelo-de-negocio.md` §5 |

## 7. Planificación

| ID | Sev. | Hallazgo | Corrección |
|---|---|---|---|
| P-01 | 🔴 | Cronograma de **5 meses** para todo el alcance. Solo el motor fiscal (XAdES-BES en Go, que tiene pocas librerías maduras, más la homologación) y la sincronización offline bidireccional son proyectos de meses cada uno. | Plan por **fases incrementales con un MVP pilotable** en `07-plan-de-fases.md`. Las estimaciones se presentan como rangos y dependen del tamaño del equipo. |
| P-02 | 🟠 | Los riesgos técnicos más altos (firma XAdES en Go, impresión ESC/POS, sincronización) se dejan para los meses 2 a 4. | **Fase 0 con spikes**: se prueban primero los tres riesgos mayores y, si algo no funciona, se cambia el plan antes de construir encima. |
| P-03 | 🟡 | El MVP no está definido: todo parece obligatorio para salir. | El MVP (Fases 0-5) permite operar salón + caja + facturación en un restaurante piloto. Inventario, KDS, QR y SaaS self-service vienen después. |

## 8. Lo que el documento acierta (y se conserva)

- Arquitectura **Cloud-Edge** con Nodo Local como ventaja competitiva central.
- **Go** para backend y Edge (binario único, gran concurrencia, compilación cruzada Windows/Linux/macOS).
- **Flutter** para la app de meseros y **PostgreSQL + RLS** para el multi-tenant.
- Identificadores generados en el cliente (UUID) para permitir operar offline.
- Facturación **asíncrona**: la caja nunca espera al SRI.
- Kardex **inmutable** (solo INSERT), auditoría de acciones críticas y cierre de caja **ciego**.
- Emparejamiento de dispositivo por **QR** más **PIN** por mesero.
- Stock DIARIO frente a PERMANENTE, recetas con sub-recetas y factores de conversión de unidades.
- Estrategia comercial de demostración "anti-caídas" y piloto en paralelo.

## 9. Próximo paso

Revisar y responder las **decisiones pendientes** en [`10-decisiones-pendientes.md`](10-decisiones-pendientes.md). Con esas respuestas los ADRs pasan de *Propuesto* a *Aceptado* y puede empezar la Fase 0.
