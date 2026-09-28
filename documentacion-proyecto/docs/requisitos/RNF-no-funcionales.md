# RNF · Requisitos no funcionales

Todas las métricas son **medibles** y tienen un punto de medición definido. "p95 ≤ X" significa que el 95 % de las mediciones deben ser menores o iguales a X en el escenario de referencia.

**Escenario de referencia (hardware):**

- **Nodo Local:** PC con Intel Core i3 de 8.ª generación (o equivalente), 8 GB RAM, SSD, Windows 10.
- **Móvil de referencia:** Android de gama media, 4 GB RAM (p. ej. Samsung Galaxy A15 o equivalente).
- **Red:** router WiFi 5 (802.11ac), 20 dispositivos conectados.
- **Carga:** 30 mesas, 8 meseros activos, 2 cajas, catálogo de 500 productos.

## Rendimiento

| ID | Requisito | Métrica | Medición |
|---|---|---|---|
| RNF-01 | Envío de comanda | ACK al mesero p95 ≤ 300 ms | App → Nodo → App |
| RNF-02 | Impresión de comanda | p95 ≤ 1,5 s | Toque en "Enviar" → último byte a la impresora |
| RNF-03 | Difusión de eventos (bloqueo, estados, cupos) | p95 ≤ 300 ms | Evento en nodo → recibido en todos los dispositivos |
| RNF-04 | Búsqueda de productos | p95 ≤ 50 ms por pulsación | En el móvil de referencia |
| RNF-05 | Cobro y liberación de caja | p95 ≤ 2 s | "Facturar" → caja lista para el siguiente cliente |
| RNF-06 | API en la nube | p95 ≤ 250 ms, p99 ≤ 800 ms | Endpoints de lectura/escritura estándar |
| RNF-07 | Arranque de la app al PIN | ≤ 2 s en frío | Móvil de referencia |
| RNF-08 | Sincronización al recuperar internet | 1 día de operación (≈ 500 órdenes) sincronizado en ≤ 2 min | Nodo → Nube |

## Disponibilidad y resiliencia

| ID | Requisito |
|---|---|
| RNF-10 | La operación del salón y la caja **no depende de internet** (ver RF-02-09). |
| RNF-11 | Disponibilidad de la nube ≥ 99,5 % mensual (objetivo inicial), ≥ 99,9 % a partir de 100 tenants. |
| RNF-12 | **Cero pérdida** de órdenes, pagos y comprobantes ante: corte de internet, reinicio del Nodo Local, corte de luz (con escritura transaccional en SQLite WAL y `synchronous=FULL` para tablas fiscales). |
| RNF-13 | RPO (pérdida máxima de datos) del Nodo Local ante destrucción de la PC: ≤ datos no sincronizados. Con internet, la sincronización es continua (≤ 30 s). Sin internet, depende del último respaldo. Se recomienda una UPS. |
| RNF-14 | RTO (tiempo de recuperación) del Nodo Local en una PC nueva: ≤ 15 min. |
| RNF-15 | Todas las operaciones de escritura desde clientes son **idempotentes** (clave de idempotencia). |

## Seguridad (detalle en `06-seguridad.md`)

| ID | Requisito |
|---|---|
| RNF-20 | Aislamiento multi-tenant con RLS en PostgreSQL; pruebas automatizadas de fuga entre tenants en el CI. |
| RNF-21 | Todo el tráfico cifrado (TLS 1.2+), incluido el tráfico LAN App ↔ Nodo. |
| RNF-22 | Secretos (P12, contraseñas del P12, llaves) cifrados con envelope encryption; nunca en logs ni en el repositorio. |
| RNF-23 | Cumplimiento de la LOPDP (Ecuador). |
| RNF-24 | Dependencias escaneadas (SCA) y análisis estático (SAST) en cada PR. |

## Usabilidad

| ID | Requisito |
|---|---|
| RNF-30 | Un mesero nuevo toma su primer pedido correcto en ≤ 5 minutos sin capacitación formal (prueba de usabilidad con usuarios reales). |
| RNF-31 | Objetivos táctiles ≥ 48×48 dp; contraste WCAG 2.1 AA; modo oscuro para bares. |
| RNF-32 | Idioma `es-EC`, moneda USD (formato de separadores según DP-11), zona horaria `America/Guayaquil`. |
| RNF-33 | Mensajes de error en lenguaje claro, sin códigos técnicos; siempre indican qué hacer. |
| RNF-34 | "Modo entrenamiento" (**S**, Fase 10): transacciones de práctica que no afectan ventas, inventario ni SRI. |

## Compatibilidad

| ID | Requisito |
|---|---|
| RNF-40 | App de meseros: Android 10+ e iOS 15+; teléfonos y tablets. |
| RNF-41 | Web (caja/backoffice): últimas 2 versiones de Chrome, Edge, Safari y Firefox. Resolución mínima de caja: 1280×720. |
| RNF-42 | Nodo Local: Windows 10/11 x64 (M), Ubuntu 22.04+ (S), macOS 13+ (C). |
| RNF-43 | Impresoras: ESC/POS por TCP 9100 y USB. Modelos certificados: lista a mantener en `docs/hardware-certificado.md` (Epson TM-T20, Xprinter XP-80, Rongta RP80 como punto de partida). |

## Mantenibilidad y operación

| ID | Requisito |
|---|---|
| RNF-50 | Cobertura de pruebas: ≥ 80 % en los dominios críticos (fiscal, dinero, inventario, sincronización, bloqueos) y ≥ 60 % global. |
| RNF-51 | Logs estructurados (JSON) con `trace_id`, `tenant_id` y `local_id`; sin datos personales ni secretos. |
| RNF-52 | Cada Nodo Local reporta telemetría de salud a la nube cada 60 s cuando hay internet. |
| RNF-53 | Compatibilidad de versiones: la nube soporta la versión N y N-1 del Nodo Local y de la app simultáneamente. |
| RNF-54 | Migraciones de base de datos versionadas, reversibles cuando sea posible y ejecutadas automáticamente en el despliegue. |
