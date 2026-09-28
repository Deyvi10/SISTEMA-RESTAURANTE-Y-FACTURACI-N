# 12 · Backlog por fases (tickets)

> Generado por `docs/backlog/generar.py` desde `docs/backlog/*.json`. **No editar a mano**: cambia el JSON y vuelve a generar.
> Cada ticket traza a sus requisitos (`RF-*`, `RNF-*`, hallazgos de la revisión, ADR). Prioridad MoSCoW; talla orientativa S/M/L/XL.

## Resumen

| Fase | Nombre | Tickets | 1 dev | Equipo de 3 | MVP |
|---|---|---|---|---|---|
| F0 | Fundaciones y spikes de riesgo | 14 | 4-6 sem | 3-4 sem | ✅ |
| F1 | Núcleo Cloud y Backoffice base | 14 | 5-7 sem | 3-4 sem | ✅ |
| F2 | Nodo Local e impresión | 15 | 5-7 sem | 3-5 sem | ✅ |
| F3 | App de meseros | 15 | 6-8 sem | 4-5 sem | ✅ |
| F4 | Caja POS | 16 | 6-8 sem | 4-5 sem | ✅ |
| F5 | Facturación electrónica SRI | 18 | 6-9 sem | 4-6 sem | ✅ |
| F6 | Hardening y piloto | 11 | 4-6 sem | 3-4 sem | ✅ |
| F7 | Inventario y stock diario | 11 | 9-12 sem | 6-8 sem |  |
| F8 | Reportes, analítica y auditoría | 7 | 6-9 sem | 4-6 sem |  |
| F9 | KDS y menú QR | 7 | 5-7 sem | 3-5 sem |  |
| F10 | SaaS comercial | 10 | 6-11 sem | 4-7 sem |  |
| F11 | Expansión (backlog priorizable) | 11 | — | — |  |

**Total:** 149 tickets. Las semanas de F0-F6 vienen de `07` §4; las de F7-F10 reparten entre fases el total que da ese mismo documento (+6-9 meses con 1 dev, +4-6 meses con 3).

## F0 · Fundaciones y spikes de riesgo

**Objetivo:** Dejar el terreno listo (monorepo, CI, entorno local, sistema de diseño) y eliminar las tres incógnitas técnicas mayores antes de construir encima: firma SRI, impresión ESC/POS y sincronización offline.

**Requisitos:** Decisiones DP-01…16 · ADR-0001…0008 · Spikes SRI, impresión y sync

**Definition of Done de la fase:**

- [ ] Los tres spikes tienen su resultado documentado en un ADR.
- [ ] El CI está en verde en main.
- [ ] `make dev` levanta todo el entorno local con un comando.
- [ ] Sistema de diseño iOS publicado (tokens + componentes base) y wireframes validados con usuarios reales.

### F0-01 · Responder decisiones pendientes y aceptar los ADR

**Tipo:** Decisión · **Prioridad:** Must · **Talla:** S · **Área:** `docs`

**Trazabilidad:** DP-01…16, ADR-0001…0008

**Qué se quiere:** Antes de escribir código hay que cerrar las decisiones que bloquean la Fase 0 y la planificación: nube, tamaño del equipo, hardware para los spikes, monorepo y framework web. Con eso los ADR pasan de Propuesto a Aceptado.

**Criterios de aceptación:**

- [ ] DP-01 (nube), DP-02 (equipo), DP-04 (hardware y .p12 de pruebas), DP-12 (React) y DP-13 (monorepo) tienen respuesta en `10-decisiones-pendientes.md`.
- [ ] ADR-0001…0008 cambian a estado Aceptado (o Rechazado con un ADR que lo reemplace).
- [ ] Las decisiones que bloquean fases posteriores (DP-03, 05, 07, 10, 11, 15) tienen fecha límite asignada.
- [ ] El cronograma de `07` §4 se recalibra según el tamaño real del equipo.

**Notas técnicas:**

- Registrar cada respuesta en la columna Decisión con fecha y responsable.

### F0-02 · Esqueleto del monorepo

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** S · **Área:** `ci`, `docs`

**Trazabilidad:** ADR-0001

**Qué se quiere:** Crear la estructura objetivo del repositorio para que las 7 aplicaciones, los paquetes compartidos, los contratos y las migraciones tengan su lugar desde el día uno.

**Criterios de aceptación:**

- [ ] Existen `apps/{cloud-api,edge-node,waiter-app,pos-web,backoffice-web,kds-web,menu-web}`, `packages/{go,ts,dart}`, `contracts/{openapi,events}`, `db/{cloud,edge}/migrations`, `tools/`, `e2e/`, `deploy/`.
- [ ] Archivos de gobierno: LICENSE, CODEOWNERS, SECURITY.md, plantillas de PR (con checklist de la DoD) e issues (épica, historia, bug).
- [ ] .editorconfig y README breve por app con cómo levantarla.
- [ ] Regla documentada: ninguna app importa código de otra; lo compartido va a `packages/`.

**Notas técnicas:**

- Un solo módulo Go en la raíz (ADR-0010) en lugar de go.work.
- pnpm workspaces para las webs y packages/ts.

**Depende de:** F0-01

### F0-03 · Tooling de calidad por lenguaje

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** S · **Área:** `ci`

**Trazabilidad:** 08 §4, 11

**Qué se quiere:** Configurar linters, formateadores y ganchos de pre-commit para que el estilo y los errores básicos nunca lleguen a revisión.

**Criterios de aceptación:**

- [ ] Go: gofumpt + golangci-lint con configuración versionada.
- [ ] TS: ESLint + Prettier + Vitest con `strict: true`.
- [ ] Flutter: very_good_analysis + `dart format`.
- [ ] Pre-commit con commitlint (Conventional Commits y scopes `cloud`, `edge`, `waiter`, `pos`, `backoffice`, `kds`, `menu`, `sri`, `contracts`, `db`, `docs`, `ci`).
- [ ] gitleaks en pre-commit.

**Notas técnicas:**

- Un `Makefile` raíz con `make lint`, `make test`, `make fmt`.

**Depende de:** F0-02

### F0-04 · Pipeline de CI base en GitHub Actions

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `ci`

**Trazabilidad:** 08 §3, RNF-24

**Qué se quiere:** Un CI que en cada PR ejecute lint, pruebas y build solo de las apps afectadas, más los escaneos de seguridad, y que proteja la rama main.

**Criterios de aceptación:**

- [ ] Filtros por ruta: un cambio en `apps/waiter-app/` no dispara el pipeline Go salvo que toque `contracts/`.
- [ ] Jobs: lint → unit + integración (`go test -race`) → verificación de contratos sin diff → seguridad (gitleaks, govulncheck, osv-scanner, CodeQL) → build.
- [ ] main protegida: PR obligatorio, CI en verde, 1 revisión, squash merge, sin push directo.
- [ ] Tiempo del pipeline de un PR típico ≤ 10 min (con caché).

**Notas técnicas:**

- Caché de módulos Go, pnpm store y pub cache.
- Dependabot o Renovate configurado.

**Depende de:** F0-03

### F0-05 · Entorno local con un comando (`make dev`)

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `ci`, `cloud`, `edge`

**Trazabilidad:** 03 §11

**Qué se quiere:** Cualquier desarrollador levanta todas las dependencias del sistema en su máquina con un solo comando, sin cuentas externas.

**Criterios de aceptación:**

- [ ] docker compose con PostgreSQL 16, S3 local con Object Lock (VersityGW, ADR-0011; MinIO ya no publica imágenes), Mailpit, simulador de impresora (F0-06), stub del SRI (F0-07) y toxiproxy.
- [ ] `make dev` levanta todo y aplica migraciones; `make reset` deja datos sintéticos limpios.
- [ ] Documentado en el README raíz con requisitos mínimos.

**Notas técnicas:**

- Datos semilla sintéticos: 1 tenant, 1 local, 20 productos, 10 mesas, 3 meseros.

**Depende de:** F0-02

### F0-06 · Simulador de impresora ESC/POS

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `edge`

**Trazabilidad:** RF-02-04, RF-02-05

**Qué se quiere:** Una herramienta que escuche en TCP 9100 como si fuera una térmica real y muestre lo que se imprimiría, para desarrollar y probar sin hardware.

**Criterios de aceptación:**

- [ ] Recibe bytes ESC/POS y los renderiza como texto y como PNG (negritas, tamaños, corte, código de barras y QR).
- [ ] Responde a `DLE EOT` y permite simular estados: sin papel, tapa abierta, desconectada y latencia.
- [ ] Varias instancias simultáneas (Cocina, Bar, Caja) configurables por puerto.
- [ ] UI web mínima que muestra los tickets recibidos en vivo.

**Notas técnicas:**

- Ubicación: `tools/printer-sim` en Go.

**Depende de:** F0-02

### F0-07 · Stub del SRI (recepción y autorización)

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `sri`

**Trazabilidad:** 05 §10

**Qué se quiere:** Un servicio SOAP falso que imite los web services offline del SRI con escenarios configurables, para probar el motor fiscal sin depender del SRI real.

**Criterios de aceptación:**

- [ ] Implementa `validarComprobante` y `autorizacionComprobante` con los mismos WSDL/respuestas del esquema offline.
- [ ] Escenarios: RECIBIDA, DEVUELTA con mensajes, timeout, HTTP 5xx, EN PROCESO, «clave registrada» y «en procesamiento», AUTORIZADO y NO AUTORIZADO.
- [ ] Escenario seleccionable por cabecera o por la clave de acceso (p. ej. el secuencial define el resultado).

**Notas técnicas:**

- Ubicación: `tools/sri-stub`.

**Depende de:** F0-02

### F0-08 · Spike SRI: firma XAdES-BES en Go con 20 facturas autorizadas

**Tipo:** Spike · **Prioridad:** Must · **Talla:** L · **Área:** `sri`

**Trazabilidad:** ADR-0006, 05 §4 §9, P-02

**Qué se quiere:** Probar lo más riesgoso del proyecto: generar la clave de acceso, construir el XML, firmarlo XAdES-BES en Go y lograr autorizaciones reales en el ambiente de pruebas del SRI.

**Criterios de aceptación:**

- [ ] Clave de acceso de 49 dígitos con módulo 11, probada con casos borde d=10 y d=11.
- [ ] XML de factura válido contra el XSD oficial descargado.
- [ ] 20 facturas distintas autorizadas en el ambiente de pruebas del SRI.
- [ ] Probado con certificados .p12 de al menos 2 entidades certificadoras distintas.
- [ ] ADR-0009 decide: Go nativo (goxmldsig + SignedProperties) o sidecar Java; criterio: 20/20 autorizadas → Go.

**Notas técnicas:**

- Lectura del .p12 con `software.sslmate.com/src/go-pkcs12`.
- Algoritmos de canonicalización y firma según la ficha técnica vigente 🔎.

**Depende de:** F0-14, F0-01

### F0-09 · Spike de impresión: Go → térmica real por TCP y USB

**Tipo:** Spike · **Prioridad:** Must · **Talla:** M · **Área:** `edge`

**Trazabilidad:** RF-02-04, RF-02-07, X-06

**Qué se quiere:** Confirmar que desde Go podemos imprimir con calidad y velocidad en impresoras térmicas reales, incluido USB en Windows y la apertura del cajón.

**Criterios de aceptación:**

- [ ] Texto, negritas, tamaños dobles, corte parcial/total, Code128 y QR en una impresora de red y una USB.
- [ ] Pulso de apertura de cajón por la impresora de caja.
- [ ] Latencia medida de «enviar bytes» a «impreso» documentada (objetivo: margen para p95 ≤ 1,5 s).
- [ ] Lectura de estado `DLE EOT` probada en al menos 1 modelo.
- [ ] ADR con la librería o enfoque elegido (propia o adaptada) y la lista inicial de hardware certificado.

**Notas técnicas:**

- Modelos sugeridos: Epson TM-T20, Xprinter XP-80, Rongta RP80.

**Depende de:** F0-01

### F0-10 · Spike de sincronización outbox SQLite → PostgreSQL

**Tipo:** Spike · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `cloud`

**Trazabilidad:** ADR-0004, 03 §5, QA-07

**Qué se quiere:** Prototipar el patrón transactional outbox + inbox idempotente y demostrar que ningún evento se pierde ni se duplica ante cortes de red.

**Criterios de aceptación:**

- [ ] Prototipo: escritura en SQLite + outbox en la misma transacción → `POST /sync/push` → UPSERT idempotente en PostgreSQL por `(node_id, node_seq)`.
- [ ] 10 000 eventos con cortes y latencias aleatorias (toxiproxy): 0 pérdidas y 0 duplicados.
- [ ] Orden por agregado respetado.
- [ ] ADR con el resultado y los parámetros (lote 500 eventos/1 MB, intervalo 2 s).

**Notas técnicas:**

- `modernc.org/sqlite` sin CGO para compilación cruzada.

**Depende de:** F0-05

### F0-11 · Paquetes de dominio base: dinero, IDs y reloj

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `contracts`

**Trazabilidad:** X-16, ADR-0005, 11 §2

**Qué se quiere:** Crear las piezas que todo el sistema usará: un tipo Money decimal con redondeo half-up, generación de UUID v7 y un reloj inyectable para probar vencimientos.

**Criterios de aceptación:**

- [ ] Go: `packages/go/money` (shopspring/decimal), `ids` (UUID v7), `clock` con implementación falsa para pruebas.
- [ ] Equivalentes en TS (decimal.js) y Dart (decimal).
- [ ] Pruebas property-based (rapid) de suma, reparto y redondeo: nunca se pierde ni se crea un centavo.
- [ ] Prohibido `float` para dinero: regla de linter o revisión documentada.

**Notas técnicas:**

- Función de reparto por *largest remainder* reutilizable (división de cuentas, propinas, IVA).

**Depende de:** F0-03

### F0-12 · Sistema de diseño «iOS» y set de iconos

**Tipo:** Diseño · **Prioridad:** Must · **Talla:** L · **Área:** `ux`

**Trazabilidad:** Visión §3.3, RNF-31

**Qué se quiere:** Definir el lenguaje visual extremadamente limpio estilo iPhone que compartirán la app, la caja, el backoffice, el KDS y el menú QR, con iconografía consistente y agradable.

**Criterios de aceptación:**

- [ ] Tokens de color con la paleta de sistema de iOS (claro y oscuro), tipografía SF Pro con escala Large Title 34 / Title 28 / Headline 17 / Body 17 / Footnote 13, radios 10-22 y espaciado base 4.
- [ ] Materiales «glass» (blur + translucidez) para barras y hojas, con alternativa sólida si el equipo es lento.
- [ ] Iconografía: SF Symbols / CupertinoIcons en Flutter y Lucide (trazo 1,75) en web, más iconos de app en cuadrado redondeado con degradado para módulos y estaciones.
- [ ] Componentes base documentados: Large Title, lista agrupada, celda con chevron, hoja con asa, segmented control, toggle, teclado numérico, badge, toast, swipe actions.
- [ ] Contraste WCAG 2.1 AA y objetivos táctiles ≥ 48×48 dp en ambos temas.
- [ ] Tokens exportados a `packages/ts/ui` (CSS variables + Tailwind) y a un `ThemeData`/`CupertinoThemeData` de Flutter.

**Notas técnicas:**

- Colores semánticos de mesa: Libre = verde, Ocupada = amarillo, Por pagar = azul, Bloqueada = gris con candado.
- Respeta `prefers-reduced-motion` y el modo oscuro del sistema.

**Diseño (estilo iOS):** Referencia: Human Interface Guidelines de Apple. Nada de sombras duras ni bordes gruesos: jerarquía por tipografía, fondos agrupados (#F2F2F7) y separadores finos.

**Depende de:** F0-01

### F0-13 · Wireframes y validación con meseros y cajero

**Tipo:** Diseño · **Prioridad:** Must · **Talla:** M · **Área:** `ux`

**Trazabilidad:** 07 F0, RNF-30

**Qué se quiere:** Diseñar las 5 pantallas críticas y validarlas con personal real antes de programarlas.

**Criterios de aceptación:**

- [ ] Wireframes de: mapa de mesas, toma de pedido, cobro, división de cuenta y cierre ciego.
- [ ] Prueba con 2 meseros y 1 cajero reales; hallazgos registrados como issues.
- [ ] Prototipo navegable en alta fidelidad de al menos mapa de mesas + toma de pedido con el sistema de diseño F0-12.

**Notas técnicas:**

- Herramienta: Figma, con la librería de componentes del sistema de diseño.

**Diseño (estilo iOS):** Medir con cronómetro: un mesero debe poder tomar un pedido de 3 platos con modificadores en menos de 30 s en el prototipo.

**Depende de:** F0-12

### F0-14 · Descargar la normativa SRI vigente

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** S · **Área:** `sri`, `docs`

**Trazabilidad:** 05 §14, CLAUDE.md

**Qué se quiere:** Guardar la ficha técnica de comprobantes electrónicos (esquema offline) y los XSD oficiales como fuente de verdad del motor fiscal.

**Criterios de aceptación:**

- [ ] Ficha técnica vigente y XSD de factura y nota de crédito en `docs/fuentes/sri/` con fecha de descarga.
- [ ] Checklist de `05` §14 revisado: cada 🔎 anotado con lo que dice la ficha.
- [ ] Versión de esquema a usar decidida (1.0.0 / 1.1.0 / 2.x).

## F1 · Núcleo Cloud y Backoffice base

**Objetivo:** API Go multi-tenant con RLS, autenticación web, catálogo, salón en cuadrícula y personal. El dueño configura su restaurante completo desde un backoffice con estética iPadOS.

**Requisitos:** RF-01-01…04 · RF-03-01 (cuadrícula) · base de RF-03-06 · RNF-20

**Definition of Done de la fase:**

- [ ] Un Admin crea su menú completo y su personal desde el backoffice.
- [ ] La prueba de aislamiento entre tenants (QA-06) pasa para todas las tablas.
- [ ] El despliegue a `dev` es automático al hacer merge en main.

### F1-01 · Esqueleto de la API Cloud en Go

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`

**Trazabilidad:** 03 §2, 11 §3

**Qué se quiere:** Base del servicio en la nube con todo lo transversal resuelto: configuración, logs, errores, salud, contratos y trazas.

**Criterios de aceptación:**

- [ ] chi + pgx/v5 + sqlc; estructura `cmd/api`, `cmd/worker`, `internal/<dominio>/{domain,app,adapters}`.
- [ ] Errores estándar RFC 9457 mapeados desde errores de dominio en un único lugar.
- [ ] Logs JSON con `slog` y campos `trace_id`, `tenant_id`, `local_id`, `user_id` (sin datos personales).
- [ ] `/health` y `/ready`; OpenAPI 3.1 en `contracts/openapi/` servido en `/v1/openapi.json`.
- [ ] OpenTelemetry básico y apagado ordenado (graceful shutdown).

**Notas técnicas:**

- Cliente TS generado desde OpenAPI en `packages/ts/api-client`.

**Depende de:** F0-04, F0-11

### F1-02 · Migraciones de plataforma con RLS forzado

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `db`, `cloud`

**Trazabilidad:** 04 §3, ADR-0008, RNF-20

**Qué se quiere:** Crear el esquema base multi-tenant: tenants, planes, locales, usuarios, permisos y sesiones, con aislamiento por fila forzado en la base de datos.

**Criterios de aceptación:**

- [ ] Migraciones goose `AAAAMMDDHHMM_descripcion.sql` para tenants, planes, plan_funciones, locales, usuarios, usuario_locales, permisos_usuario, sesiones, revocaciones.
- [ ] Toda tabla de negocio con `tenant_id` + `ENABLE` y `FORCE ROW LEVEL SECURITY` + política `tenant_isolation`.
- [ ] El rol de la app no es dueño de las tablas ni tiene BYPASSRLS; rol de plataforma separado y auditado.
- [ ] Columnas de auditoría de fila (`created_at`, `updated_at`, `created_by`, `version`) e índices compuestos que empiezan por `tenant_id`.

**Notas técnicas:**

- UUID v7 generados en la aplicación, nunca por la base de datos.

**Depende de:** F1-01

### F1-03 · Middleware de tenant + prueba de aislamiento QA-06

**Tipo:** QA · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`, `db`

**Trazabilidad:** RNF-20, QA-06

**Qué se quiere:** Garantizar que un restaurante jamás vea ni modifique datos de otro, con una prueba automática que cubra todas las tablas presentes y futuras.

**Criterios de aceptación:**

- [ ] Cada transacción abre con `SET LOCAL app.tenant_id` tomado del token (nunca `SET`).
- [ ] Prueba generada que recorre `information_schema` y, para cada tabla con `tenant_id`, verifica que el tenant A no puede leer, insertar, actualizar ni borrar filas de B.
- [ ] Una tabla nueva sin política RLS hace fallar el CI.

**Notas técnicas:**

- testcontainers con PostgreSQL real.

**Depende de:** F1-02

### F1-04 · RBAC con permiso declarado por ruta (QA-11)

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** S · **Área:** `cloud`

**Trazabilidad:** 06 §3, QA-11, requisitos §3

**Qué se quiere:** Un único middleware de autorización y una prueba que falle si alguna ruta de la API no declara qué permiso requiere.

**Criterios de aceptación:**

- [ ] Matriz de roles por defecto (Admin, Cajero, Mesero, Cocina, Bodega) cargada como datos.
- [ ] Cada ruta declara su permiso; el middleware lo verifica en el servidor.
- [ ] Prueba QA-11 recorre el router y falla ante rutas sin permiso.

**Notas técnicas:**

- Permisos en filas (`permisos_usuario`), no en columnas.

**Depende de:** F1-01

### F1-05 · Autenticación web de Admin y Cajero

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`, `backoffice`

**Trazabilidad:** RF-01-02

**Qué se quiere:** El dueño y los cajeros inician sesión en el backoffice de forma segura, con recuperación de contraseña y cambio obligatorio en el primer ingreso.

**Criterios de aceptación:**

- [ ] Login con correo (o RUC para el dueño) y contraseña con Argon2id.
- [ ] 5 intentos fallidos → bloqueo de 15 min por usuario + IP.
- [ ] Recuperación por correo con token de un solo uso válido 30 min.
- [ ] Access token de 15 min + refresh rotativo de 7 días en cookie `HttpOnly; Secure; SameSite=Strict`.
- [ ] Primer inicio con contraseña temporal obliga a cambiarla.

**Notas técnicas:**

- 2FA TOTP opcional queda preparado en el modelo (`totp_secret_cifrado`); se activa en F10-09.

**Diseño (estilo iOS):** Pantalla de login centrada tipo «Iniciar sesión con Apple ID»: tarjeta blanca sobre fondo agrupado, logo en cuadrado redondeado, campos altos de 50 px y botón azul de ancho completo.

**Depende de:** F1-02, F1-04

### F1-06 · Alta de tenant por el Super Admin

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`

**Trazabilidad:** RF-01-01

**Qué se quiere:** Crear un restaurante nuevo con sus datos mínimos y dejarlo listo para configurar, con datos semilla para que no empiece en blanco.

**Criterios de aceptación:**

- [ ] Valida el RUC (13 dígitos, termina en 001, dígito verificador según tipo) antes de guardar.
- [ ] Crea tenant, local por defecto (establecimiento 001) y usuario Admin.
- [ ] Semilla: categorías Entradas, Platos fuertes, Bebidas; estación Cocina; estación Caja; 1 mesa.
- [ ] Contraseña temporal aleatoria ≥ 12 caracteres enviada por correo (Mailpit en local).
- [ ] No puede existir otro tenant activo con el mismo RUC.

**Notas técnicas:**

- CLI `cloud-api tenant create` o pantalla mínima protegida con 2FA.

**Depende de:** F1-05

### F1-07 · Catálogo: categorías, productos, modificadores y tarifas IVA

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `cloud`, `db`

**Trazabilidad:** 04 §4, RF-03-06, RF-05-06

**Qué se quiere:** El modelo y la API del menú: productos con precio exacto, tarifa de IVA versionada, alias para búsqueda, modificadores con reglas y notas rápidas.

**Criterios de aceptación:**

- [ ] Tablas y API CRUD: categorias, productos (`precio NUMERIC(18,6)`, tipo, alias, estación opcional), grupos_modificadores (obligatorio, min, max), modificadores (precio adicional), producto_grupos_modificadores (orden), notas_rapidas.
- [ ] `tarifas_iva` global versionada por fecha con semilla 0 %, 5 %, 15 %, No objeto y Exento 🔎.
- [ ] Configuración por local: precios con IVA incluido (DP-15, por defecto Sí).
- [ ] Borrado lógico (`deleted_at`) y `version` incrementada en cada cambio (base para la sincronización).

**Notas técnicas:**

- Emite evento `catalog.updated` a una tabla de cambios para el flujo Nube → Nodo (F2-03).

**Depende de:** F1-03

### F1-08 · Locales, zonas, mesas y estaciones (cuadrícula)

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`, `db`

**Trazabilidad:** RF-01-03, RF-03-01

**Qué se quiere:** Configurar el local y su salón: zonas, mesas en cuadrícula y estaciones de producción.

**Criterios de aceptación:**

- [ ] Local con nombre, dirección, código de establecimiento, zona horaria, propina legal (activa y %) y precios con IVA incluido.
- [ ] Zonas (Salón, Terraza, Barra) y mesas con nombre, capacidad, forma y posición en cuadrícula.
- [ ] Estaciones tipo PRODUCCION o CAJA.
- [ ] El estado vivo de la mesa NO se guarda en `mesas` (se deriva en el nodo).

**Depende de:** F1-03

### F1-09 · Personal en 10 segundos (API)

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`

**Trazabilidad:** RF-01-04, X-04

**Qué se quiere:** Crear un mesero solo con nombre y PIN, sin correos ni contraseñas, cuidando la seguridad del PIN.

**Criterios de aceptación:**

- [ ] Obligatorios: nombre, rol y PIN de 4 a 6 dígitos; correo solo para Admin y Cajero web.
- [ ] PIN único por local vía `pin_fingerprint` (HMAC) sin revelarlo.
- [ ] Se rechazan PINs triviales (0000, 1234, 1111…).
- [ ] PIN guardado como Argon2id(HMAC-SHA256(pepper, pin || user_id)); pepper en el gestor de secretos.
- [ ] Usuarios nunca se borran; desactivar emite `user.deactivated` para el nodo.

**Depende de:** F1-03

### F1-10 · Shell del Backoffice estilo iPadOS

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `backoffice`

**Trazabilidad:** ADR-0003, RNF-31, RNF-32

**Qué se quiere:** La base visual y técnica del panel del dueño: navegación, temas, formato ecuatoriano y estados vacíos amables.

**Criterios de aceptación:**

- [ ] React + TS strict + Vite; TanStack Query; cliente de API generado; Zustand para estado de UI.
- [ ] Barra lateral tipo iPadOS con iconos de colores por módulo (Inicio, Menú, Salón, Personal, Impresoras, Facturación, Reportes, Ajustes).
- [ ] Tema claro/oscuro automático; formato `$1,250.50` (DP-11) y zona `America/Guayaquil`.
- [ ] Estados vacíos ilustrados con icono y acción principal («Crea tu primer plato»).
- [ ] Accesible por teclado con foco visible.

**Notas técnicas:**

- Componentes del ui-kit `packages/ts/ui` (F0-12).

**Diseño (estilo iOS):** Large Title que se contrae al hacer scroll, barra superior translúcida con blur, listas agrupadas con esquinas de 12 px y separadores con inset.

**Depende de:** F0-12, F1-05

### F1-11 · Backoffice: gestión visual del menú

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `backoffice`

**Trazabilidad:** RF-03-06, Doc. fuente F6 §2

**Qué se quiere:** Crear un plato debe tomar segundos: foto arrastrada, precio, categoría, modificadores con un clic y notas rápidas.

**Criterios de aceptación:**

- [ ] CRUD de categorías (color e icono) con reordenamiento por arrastre (dnd-kit).
- [ ] Formulario de producto con zona de arrastre de imagen, precio con IVA incluido y vista del desglose.
- [ ] Constructor de grupos de modificadores (obligatorio, min, max, precio adicional) reutilizables entre productos.
- [ ] Notas rápidas por categoría («Sin sal», «Extra ají»).
- [ ] Búsqueda y filtro instantáneos en la lista de productos.

**Notas técnicas:**

- React Hook Form + Zod generados desde el contrato.

**Diseño (estilo iOS):** Tarjetas de producto con foto cuadrada redondeada, precio en tipografía tabular y toggles iOS para «Activo» y «Visible en menú QR».

**Depende de:** F1-07, F1-10, F1-13

### F1-12 · Backoffice: salón y personal

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `backoffice`

**Trazabilidad:** RF-01-04, RF-03-01

**Qué se quiere:** Configurar zonas y mesas en una cuadrícula y dar de alta meseros en 10 segundos.

**Criterios de aceptación:**

- [ ] Editor de cuadrícula de mesas por zona (el plano 2D libre es F3-06/S).
- [ ] Alta de mesero con nombre, rol, PIN y foto opcional en una sola hoja modal.
- [ ] Desactivar usuario con confirmación dentro de la página.
- [ ] Validaciones en lenguaje claro (RNF-33).

**Diseño (estilo iOS):** Alta de personal como hoja modal iOS: avatar circular grande arriba, teclado de PIN con puntos animados y botón «Listo» en la esquina.

**Depende de:** F1-08, F1-09, F1-10

### F1-13 · Subida de imágenes optimizada

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** S · **Área:** `cloud`

**Trazabilidad:** 06 §6, 03 §9

**Qué se quiere:** Las fotos del menú se suben directo al almacenamiento de objetos, se limpian y se convierten a WebP en varias resoluciones.

**Criterios de aceptación:**

- [ ] URL firmada para subida directa; límite de tamaño.
- [ ] Validación del tipo real, recodificación a WebP (thumb, medio, grande) y eliminación de EXIF.
- [ ] Entrega por CDN con URL firmada o pública según el caso.

**Depende de:** F1-01

### F1-14 · Despliegue automático a `dev`

**Tipo:** Operación · **Prioridad:** Must · **Talla:** M · **Área:** `ci`

**Trazabilidad:** 08 §3, DP-01, ADR-0013

**Qué se quiere:** Cada merge a main despliega la API, los workers y el backoffice al entorno dev sin intervención.

**Criterios de aceptación:**

- [ ] Dockerfile multi-stage de cloud-api; imagen en GitHub Container Registry.
- [ ] Terraform en `deploy/azure/`: PostgreSQL Flexible, Container Apps (API + worker fiscal), Storage Account con contenedores `comprobantes` (GRS, inmutable), `media` y `respaldos` (LRS) y reglas de ciclo de vida, Key Vault con identidad administrada, Static Web Apps.
- [ ] Migraciones ejecutadas antes del despliegue (patrón expand/contract).
- [ ] Backoffice publicado en Static Web Apps.
- [ ] Alerta de presupuesto de Azure al 80 % y al 100 % del costo mensual previsto en `13`.

**Depende de:** F0-04, F0-01

## F2 · Nodo Local e impresión

**Objetivo:** El corazón offline: binario Go como servicio de Windows con SQLite WAL, sincronización con la nube, hub de tiempo real, descubrimiento de impresoras y ruteo de comandas por estación.

**Requisitos:** RF-02-01…06, 08, 09 (parcial)

**Definition of Done de la fase:**

- [ ] Desde el backoffice se configura «Bebidas → Bar, Platos → Cocina».
- [ ] Un pedido simulado por la API del nodo imprime en 2 impresoras en paralelo con p95 ≤ 1,5 s.
- [ ] Al desconectar una impresora el trabajo se conserva y se imprime al volver.

### F2-01 · Nodo Local como servicio de Windows con SQLite WAL

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `db`

**Trazabilidad:** RF-02-01, 03 §4, RNF-12

**Qué se quiere:** El binario Go que corre en la PC de caja: arranca con el equipo, se recupera solo y persiste todo de forma segura aunque se corte la luz.

**Criterios de aceptación:**

- [ ] Servicio de Windows con reinicio automático y watchdog; arranque ≤ 10 s.
- [ ] SQLite con `journal_mode=WAL`, `synchronous=FULL`, `foreign_keys=ON`, `busy_timeout=5000`.
- [ ] Una sola goroutine escritora serializada + pool de lectores.
- [ ] Migraciones edge versionadas en `db/edge/migrations` aplicadas al arrancar.
- [ ] Prueba: kill -9 a mitad de una transacción no corrompe la base.

**Notas técnicas:**

- `modernc.org/sqlite` (sin CGO). Linux .deb queda como S (DP-03).

**Depende de:** F0-10

### F2-02 · Activación del nodo por código de un solo uso

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `cloud`, `backoffice`

**Trazabilidad:** RF-02-01, RF-02-10.4

**Qué se quiere:** El dueño genera un código en el backoffice, lo escribe en el nodo y este queda ligado a su local con credenciales propias.

**Criterios de aceptación:**

- [ ] Backoffice genera un código de activación de un solo uso con expiración para un local.
- [ ] El nodo recibe llave ed25519/certificado, `tenant_id` y `local_id`.
- [ ] Tras activarse descarga catálogo, configuración, personal y mesas y queda operativo sin reiniciar.
- [ ] Solo un nodo ACTIVO por local; activar uno nuevo revoca el anterior.

**Notas técnicas:**

- Tabla `codigos_activacion_nodo` y `nodos`.

**Diseño (estilo iOS):** En el nodo: pantalla de bienvenida con 8 casillas grandes para el código (como el código de verificación de iOS) y una marca de verificación animada al completar.

**Depende de:** F2-01, F1-08

### F2-03 · Sincronización Nube → Nodo (inbox + cursores)

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `cloud`, `contracts`

**Trazabilidad:** ADR-0004, 03 §3 §5

**Qué se quiere:** El catálogo, la configuración y el personal que el dueño cambia en la nube llegan al nodo de forma idempotente y ordenada.

**Criterios de aceptación:**

- [ ] Flujos por dominio con cursores (`inbox_cursores`); push por WebSocket y pull de respaldo.
- [ ] Aplicación idempotente: reaplicar el mismo cambio no altera nada.
- [ ] Eventos con `type` + `version`; la nube acepta N y N-1.
- [ ] Al reconectar, el nodo empuja su outbox antes de aceptar cambios de catálogo.
- [ ] `user.deactivated` y `device.revoked` se aplican y difunden en < 2 s.

**Depende de:** F2-02, F1-07

### F2-04 · Outbox Nodo → Nube y telemetría de salud

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `cloud`

**Trazabilidad:** 03 §5 §10, RNF-08, RNF-52

**Qué se quiere:** Toda escritura operativa del nodo viaja a la nube sin pérdidas ni duplicados, y la nube sabe en todo momento si el nodo está sano.

**Criterios de aceptación:**

- [ ] Evento en `outbox` dentro de la misma transacción que la escritura; `node_seq` monotónico.
- [ ] `POST /v1/sync/push` en lotes de hasta 500 eventos o 1 MB; ACK con último seq aplicado; `sync_recibidos` evita duplicados.
- [ ] Heartbeat cada 60 s: versión, disco, tamaño y antigüedad del outbox, impresoras, deriva del reloj.
- [ ] Alerta si la deriva del reloj supera 60 s (la fecha va en la clave de acceso).

**Notas técnicas:**

- QA-07 automatizado en el CI con toxiproxy.

**Depende de:** F2-01, F0-10

### F2-05 · Página de estado local del nodo

**Tipo:** Historia · **Prioridad:** Must · **Talla:** S · **Área:** `edge`

**Trazabilidad:** RF-02-01.5, 06 §8

**Qué se quiere:** Una página en la red local para ver de un vistazo si el nodo, la nube, las impresoras y la cola están bien.

**Criterios de aceptación:**

- [ ] `https://<nodo>/estado` con versión, conectividad, tamaño de la cola, impresoras y comprobantes pendientes.
- [ ] No expone datos personales; las acciones requieren PIN de Admin.

**Diseño (estilo iOS):** Estilo «Ajustes > General > Información» de iOS: lista agrupada con puntos de estado verde/amarillo/rojo a la derecha de cada fila.

**Depende de:** F2-01

### F2-06 · Hub WebSocket y contratos de eventos

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `contracts`

**Trazabilidad:** RF-02-08, 03 §6, RNF-03

**Qué se quiere:** El canal de tiempo real entre el nodo y todos los dispositivos del local, con mensajes tipados y generados desde contratos.

**Criterios de aceptación:**

- [ ] JSON Schema por mensaje en `contracts/events/`; tipos generados para Go, TS y Dart.
- [ ] Sobre `{v, id, type, ts, data}`; catálogo inicial de eventos de `03` §6.
- [ ] ≥ 50 dispositivos simultáneos con p95 de difusión ≤ 200 ms (prueba k6).
- [ ] Anuncio mDNS `_restpos._tcp` con IP manual como alternativa.
- [ ] Endpoint de estado de conectividad para el indicador 🟢 🟡 🔴 (RF-02-09.3).

**Notas técnicas:**

- `coder/websocket`.

**Depende de:** F2-01

### F2-07 · TLS en la LAN con CA interna

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `cloud`

**Trazabilidad:** RNF-21, 06 §5

**Qué se quiere:** El tráfico entre la app, la caja y el nodo va cifrado aunque sea dentro del local.

**Criterios de aceptación:**

- [ ] La CA interna de la plataforma emite un certificado al nodo al activarse; rotación anual.
- [ ] La app ancla (pinning) el certificado al emparejar.
- [ ] Caja web en `https://<nodo>.local` validada en Chrome, Edge y Firefox 🔎; alternativa documentada (dominio propio + DNS local).

**Notas técnicas:**

- Depende del dominio del producto (DP-10).

**Depende de:** F2-02

### F2-08 · Descubrimiento de impresoras «Zero-Setup»

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `backoffice`

**Trazabilidad:** RF-02-02, X-06

**Qué se quiere:** El nodo encuentra solo las impresoras del local y el dueño las ve en el backoffice con un botón de prueba.

**Criterios de aceptación:**

- [ ] Descubrimiento en capas: mDNS/Bonjour → escaneo de la subred en TCP 9100 → enumeración USB (Windows).
- [ ] Cada impresora aparece con nombre/modelo, conexión, IP o puerto y botón «Imprimir prueba».
- [ ] Si cambia la IP, se reubica por MAC (tabla ARP) sin intervención.
- [ ] Guía para reservar la IP por DHCP; ancho de papel 58/80 mm.

**Notas técnicas:**

- El hardware detectado es dato del nodo (Nodo → Nube).

**Diseño (estilo iOS):** Lista tipo «Bluetooth» de iOS: spinner mientras busca, cada impresora con icono de impresora en cuadrado de color y estado «Conectada».

**Depende de:** F2-04, F0-09

### F2-09 · Motor ESC/POS y plantillas de ticket

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `edge`

**Trazabilidad:** RF-02-04.4, RF-02-06

**Qué se quiere:** La librería que convierte comandas, anulaciones y pre-cuentas en bytes ESC/POS legibles en 58 y 80 mm.

**Criterios de aceptación:**

- [ ] `packages/go/escpos`: estilos, tamaños, corte, Code128, QR, pulso de cajón y lectura `DLE EOT`.
- [ ] Plantilla de comanda: estación, mesa u orden, mesero, hora, número de comanda, cantidades, producto, modificadores y notas por línea.
- [ ] Plantillas de ANULACIÓN y marca «REIMPRESIÓN» con la hora original.
- [ ] Pruebas golden contra el simulador (F0-06).

**Depende de:** F0-09, F0-06

### F2-10 · Estaciones y ruteo por arrastrar y soltar

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-02-03, X-07

**Qué se quiere:** El dueño arrastra categorías hacia estaciones (Cocina, Bar, Sushi) y cada plato sale en la impresora correcta sin que el mesero elija nada.

**Criterios de aceptación:**

- [ ] CRUD de estaciones y asignación N:M de impresoras (y KDS a futuro).
- [ ] Drag & drop de categorías a estaciones; sobrescritura por producto.
- [ ] Estación de caja para pre-cuentas y comprobantes.
- [ ] Un producto sin estación usa la estación por defecto: nunca se pierde una comanda.
- [ ] La línea de orden guarda la estación, nunca la IP.

**Notas técnicas:**

- dnd-kit.

**Diseño (estilo iOS):** Columnas por estación con su icono (fuego para cocina caliente, copa para bar), categorías como chips que se arrastran con leve escala y sombra al levantar.

**Depende de:** F2-08, F1-11

### F2-11 · Impresión concurrente de comandas con colas persistentes

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `edge`

**Trazabilidad:** RF-02-04, RNF-02

**Qué se quiere:** Una comanda mixta se divide por estación y se imprime en paralelo en menos de 1,5 s, sin que una impresora caída bloquee a las demás.

**Criterios de aceptación:**

- [ ] API del nodo para enviar comanda (usada por la app en F3 y por pruebas).
- [ ] Agrupa por estación, genera ESC/POS y envía con una goroutine por impresora.
- [ ] Cola persistente por estación en `trabajos_impresion`; reintentos; las demás no se bloquean.
- [ ] p95 ≤ 1,5 s de «Enviar» a último byte (medido con el simulador y hardware real).
- [ ] Líneas anuladas tras el envío imprimen ticket de ANULACIÓN.

**Depende de:** F2-09, F2-10

### F2-12 · Detección de fallos de impresora y redirección

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `pos`, `waiter`

**Trazabilidad:** RF-02-05, CU-01 A1

**Qué se quiere:** Si una impresora se queda sin papel o se desconecta, todos se enteran al instante y se puede desviar su cola con un toque.

**Criterios de aceptación:**

- [ ] Detecta sin conexión, sin papel y tapa abierta (`DLE EOT` donde se soporte).
- [ ] Alerta en caja y aviso al mesero dueño de la comanda en < 3 s (evento `printer.status`).
- [ ] Redirigir la cola de una estación a otra impresora con un toque.
- [ ] Al reponer papel se reimprime automáticamente lo pendiente.

**Diseño (estilo iOS):** Banner rojo tipo notificación de iOS que baja desde arriba con icono de impresora y botón «Redirigir».

**Depende de:** F2-11

### F2-13 · Reimpresión auditada

**Tipo:** Historia · **Prioridad:** Should · **Talla:** S · **Área:** `edge`

**Trazabilidad:** RF-02-06

**Qué se quiere:** Reimprimir una comanda, pre-cuenta o comprobante desde la caja, dejando rastro.

**Criterios de aceptación:**

- [ ] Reimpresión disponible por API del nodo (la UI llega en F4).
- [ ] Ticket con «REIMPRESIÓN» y hora original.
- [ ] Registro en auditoría con usuario y dispositivo.

**Depende de:** F2-11

### F2-14 · Instalador MSI firmado

**Tipo:** Operación · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `ci`

**Trazabilidad:** RF-02-01.1, 03 §8

**Qué se quiere:** Instalar el Nodo Local en una PC Windows con un clic.

**Criterios de aceptación:**

- [ ] MSI firmado con Authenticode; registra el servicio, reglas de firewall y desinstalación limpia.
- [ ] Construido y firmado en el CI; publicado en el canal «interno».
- [ ] Asistente con el nombre comercial (DP-10) y enlace a la guía de instalación.

**Depende de:** F2-01, F0-01

### F2-15 · Caché local de fotos del menú

**Tipo:** Técnica · **Prioridad:** Should · **Talla:** S · **Área:** `edge`

**Trazabilidad:** ADR-0013, RF-02-01

**Qué se quiere:** La caja y la app de meseros muestran las fotos de los platos aunque no haya internet, sin descargar cada foto de la nube una y otra vez.

**Criterios de aceptación:**

- [ ] El nodo descarga las WebP sm y md de los productos activos al recibir cambios de catálogo (F2-03) y las sirve en `/media/...` en la LAN.
- [ ] Descarga en segundo plano con límite de ancho de banda; nunca bloquea la operación.
- [ ] Las fotos de productos eliminados se borran de la caché; el tamaño total se muestra en la página de estado.
- [ ] Sin internet y sin foto en caché, se muestra el icono de la categoría.

**Notas técnicas:**

- Las claves de imagen son inmutables (hash en la ruta), así que no hace falta invalidar la caché.

**Depende de:** F2-03

## F3 · App de meseros

**Objetivo:** App Flutter con estilo Cupertino: emparejamiento por QR, login por PIN, mapa de mesas en vivo con bloqueo anti-colisión y toma de pedidos offline-first a una sola mano.

**Requisitos:** RF-01-05, 06 · RF-03-02…10

**Definition of Done de la fase:**

- [ ] 8 dispositivos simultáneos contra un nodo sin internet: 0 platos duplicados y 0 colisiones de mesa.
- [ ] Búsqueda p95 ≤ 50 ms en el móvil de referencia.
- [ ] Test de concurrencia Go + prueba manual guiada en verde.

### F3-01 · Proyecto Flutter base con estilo Cupertino

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `waiter`

**Trazabilidad:** ADR-0002, 11 §5

**Qué se quiere:** La base de la app de meseros: arquitectura por feature, estado, persistencia local, tema iOS y háptica.

**Criterios de aceptación:**

- [ ] Flutter estable con `riverpod`, `drift`, `go_router`, `freezed`, `flutter_secure_storage`, `mobile_scanner`.
- [ ] `lib/features/<feature>/{data,domain,presentation}`; cero lógica de negocio en widgets.
- [ ] Tema Cupertino desde los tokens de F0-12, modo oscuro y tipografía dinámica.
- [ ] Servicio de háptica (selección, éxito, advertencia) centralizado.
- [ ] Android 10+ e iOS 15+ (RNF-40).

**Notas técnicas:**

- Cliente Dart generado en `packages/dart`.

**Depende de:** F0-12, F2-06

### F3-02 · Emparejamiento de dispositivos por QR

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `waiter`, `cloud`, `edge`, `backoffice`

**Trazabilidad:** RF-01-05, X-03

**Qué se quiere:** Una tablet o celular queda ligado al local escaneando un QR, sin usuarios ni contraseñas, con una credencial segura ligada al dispositivo.

**Criterios de aceptación:**

- [ ] Backoffice (o nodo) muestra un QR de un solo uso válido 10 min.
- [ ] La app genera un par de llaves en Keystore/Keychain y envía la llave pública; el dispositivo se registra con nombre editable.
- [ ] Ligado a un solo local; para cambiarlo hay que desemparejar.
- [ ] Revocación efectiva en < 2 s si está conectado y en su siguiente conexión si no.
- [ ] Sin emparejar o revocado, la app no muestra la lista de personal.

**Notas técnicas:**

- Desafío-respuesta firmado para autenticar el dispositivo.

**Diseño (estilo iOS):** Pantalla de bienvenida con el icono de la app grande, texto «Escanea el código de tu restaurante» y visor de cámara con esquinas redondeadas animadas, como al emparejar un Apple Watch.

**Depende de:** F3-01, F2-07

### F3-03 · Descubrimiento del nodo y conexión resiliente

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `waiter`

**Trazabilidad:** RF-02-08.2, RF-02-09.3

**Qué se quiere:** La app encuentra sola el Nodo Local en el WiFi y se reconecta sin que el mesero haga nada.

**Criterios de aceptación:**

- [ ] Descubrimiento por mDNS `_restpos._tcp`; IP manual como alternativa.
- [ ] WebSocket con TLS anclado y reconexión con backoff.
- [ ] Indicador visible 🟢 En línea · 🟡 Sin internet (operando local) · 🔴 Sin Nodo Local.

**Diseño (estilo iOS):** Indicador como píldora en la barra superior, similar a la «Dynamic Island»: se expande 2 s al cambiar de estado y luego se contrae.

**Depende de:** F3-01

### F3-04 · Cuadrícula de personal y login por PIN

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `waiter`, `edge`

**Trazabilidad:** RF-01-06, X-04, X-05

**Qué se quiere:** El mesero toca su foto, escribe su PIN en un teclado gigante y entra en un segundo; el PIN se valida en el nodo, nunca en el teléfono.

**Criterios de aceptación:**

- [ ] Cuadrícula con foto y nombre de meseros y cajeros activos del local.
- [ ] Validación en el Nodo Local con pepper; nunca contra un hash guardado en el móvil.
- [ ] 5 PINs erróneos → bloqueo de 5 min en ese dispositivo + auditoría.
- [ ] Re-PIN al bloquear la pantalla o tras 2 min de inactividad (configurable).
- [ ] Sesión de máx. 12 h o hasta cerrar la jornada; access 15 min + refresh ligado a dispositivo y turno.
- [ ] Cambio rápido de usuario sin perder el estado de las mesas.

**Notas técnicas:**

- Rate limit por usuario y dispositivo; 20 intentos/10 min por dispositivo → alerta al Admin.

**Diseño (estilo iOS):** Idéntico al desbloqueo del iPhone: avatares circulares grandes, al tocar uno se desliza un teclado numérico de botones circulares translúcidos, puntos que se llenan y sacudida con háptica de error si el PIN es incorrecto.

**Depende de:** F3-02, F1-09

### F3-05 · Catálogo local offline en el teléfono

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `waiter`

**Trazabilidad:** RF-03-04.1

**Qué se quiere:** El menú completo vive en el teléfono para que tomar pedidos no dependa de la red.

**Criterios de aceptación:**

- [ ] Esquema drift con categorías, productos, precios, modificadores, notas rápidas, miniaturas y disponibilidad.
- [ ] Descarga al iniciar turno y actualización incremental ante `catalog.updated`.
- [ ] Imágenes en caché con tamaño acotado.

**Depende de:** F3-03

### F3-06 · Mapa de mesas en vivo

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `waiter`, `edge`

**Trazabilidad:** RF-03-02, RF-03-01.2

**Qué se quiere:** El mesero ve el salón entero en tiempo real: qué mesas están libres, ocupadas, por pagar o siendo editadas por otro.

**Criterios de aceptación:**

- [ ] Colores: Libre verde, Ocupada amarillo, Por pagar azul, Bloqueada gris con candado y «Editando: <nombre>».
- [ ] Indicadores: 🍽️ platos listos, ⏱️ tiempo desde la apertura, rojo suave si espera la cuenta > 10 min.
- [ ] Cambios visibles en todos los dispositivos en p95 ≤ 300 ms.
- [ ] Pestañas por zona; vista de cuadrícula (M) y plano 2D con editor en el backoffice (S).

**Notas técnicas:**

- El estado vivo se deriva en el nodo (orden abierta + gestor de bloqueos).

**Diseño (estilo iOS):** Mesas como formas vectoriales redondeadas con número grande; la mesa recién enviada late suavemente en amarillo, las de plato listo muestran un plato humeante; segmented control para zonas.

**Depende de:** F3-04

### F3-07 · Bloqueo pesimista de mesa con heartbeat

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `waiter`

**Trazabilidad:** RF-03-03, X-12, QA-01

**Qué se quiere:** Solo un mesero puede editar una mesa a la vez; si su teléfono muere, la mesa se libera sola.

**Criterios de aceptación:**

- [ ] `LOCK` atómico en el nodo: de dos solicitudes simultáneas exactamente una gana; la otra recibe `LOCKED_BY <usuario>`.
- [ ] Difusión `table.locked` / `table.unlocked` a todos.
- [ ] Heartbeat cada 10 s mientras la pantalla está abierta; expira a los 45 s sin heartbeat.
- [ ] Se libera al enviar la comanda o salir; el Admin puede forzar la liberación (auditado).
- [ ] QA-01: 100 solicitudes simultáneas → exactamente 1 éxito, con `-race`.

**Notas técnicas:**

- Estado en memoria y en `bloqueos_mesa` para sobrevivir a un reinicio.

**Depende de:** F3-06

### F3-08 · Toma de pedido y búsqueda predictiva

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `waiter`

**Trazabilidad:** RF-03-04, RF-03-05, RNF-04

**Qué se quiere:** Agregar platos es instantáneo y encontrar «Ceviche Mixto» basta con escribir «cev».

**Criterios de aceptación:**

- [ ] Agregar un producto responde en ≤ 50 ms (local, sin red).
- [ ] Búsqueda aproximada insensible a tildes y mayúsculas («sec pol» → Seco de Pollo) y por alias («CM»).
- [ ] p95 ≤ 50 ms por pulsación con 500 productos en el móvil de referencia.
- [ ] Resultados priorizan lo más vendido del local.
- [ ] Líneas enviadas y por enviar se distinguen claramente.

**Notas técnicas:**

- Índice de búsqueda en memoria normalizado (sin tildes) construido al cargar el catálogo.

**Diseño (estilo iOS):** Barra de búsqueda tipo Spotlight fija abajo (zona del pulgar), categorías como chips horizontales con icono y productos en tarjetas con foto; al agregar, la tarjeta hace un pequeño rebote y suena la háptica de selección.

**Depende de:** F3-05, F3-07

### F3-09 · Modificadores y notas por línea

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `waiter`

**Trazabilidad:** RF-03-06

**Qué se quiere:** Cada plato lleva sus propios modificadores y notas: la primera hamburguesa «sin cebolla», la segunda «término medio».

**Criterios de aceptación:**

- [ ] Grupos con reglas obligatorio/opcional, mín./máx. y precio adicional.
- [ ] Con modificadores obligatorios, el selector se abre al agregar.
- [ ] Pulsación prolongada sobre una línea abre la edición de modificadores y la nota libre.
- [ ] Cada unidad es una línea independiente si difiere.
- [ ] Notas rápidas por categoría.

**Notas técnicas:**

- Modificadores en tabla propia (`orden_linea_modificadores`) con snapshot de nombre y precio.

**Diseño (estilo iOS):** Hoja inferior con asa (detents medio/completo), opciones como filas con check azul y botón «Agregar · $12.50» fijo abajo.

**Depende de:** F3-08

### F3-10 · Envío de comanda idempotente y cola «Pendiente de envío»

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `waiter`, `edge`

**Trazabilidad:** RF-03-07, RF-03-04.3-5, QA-08, RNF-01

**Qué se quiere:** «Enviar» manda todo en un mensaje, el nodo confirma en 300 ms e imprime por estación; sin red, la orden espera y se envía sola.

**Criterios de aceptación:**

- [ ] Un mensaje con todas las líneas nuevas + `Idempotency-Key`; ACK p95 ≤ 300 ms y liberación del bloqueo.
- [ ] Nodo persiste orden + comanda en una transacción y rutea (F2-11).
- [ ] Sin nodo: la orden queda «Pendiente de envío» y se reenvía sola al reconectar.
- [ ] QA-08: reenviar la misma comanda 5 veces → 1 sola comanda.
- [ ] «Enviar y mantener» y tiempos (Entrada/Fuerte/Postre) con «Marchar fuerte».
- [ ] Una línea enviada no se edita; solo se anula (F3-14).

**Notas técnicas:**

- UUID v7 generado en el dispositivo por línea y por envío.

**Diseño (estilo iOS):** Botón «Enviar» de ancho completo en el borde inferior; al confirmarse, check verde animado + háptica de éxito y la mesa vuelve al mapa.

**Depende de:** F3-08, F2-11

### F3-11 · Gestos y navegación a una mano

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `waiter`

**Trazabilidad:** RF-03-08

**Qué se quiere:** Todo lo frecuente se hace con el pulgar: borrar deslizando, deshacer y cambiar de sección desde abajo.

**Criterios de aceptación:**

- [ ] Swipe a la izquierda sobre una línea no enviada la elimina con háptica y «Deshacer» durante 4 s.
- [ ] Barra de pestañas inferior: Salón, Orden actual, Avisos (con contador).
- [ ] Acciones primarias en la zona del pulgar.

**Diseño (estilo iOS):** Swipe como en Mail de iOS (fondo rojo con icono de papelera que crece), tab bar translúcida con iconos SF Symbols rellenos al estar activos.

**Depende de:** F3-08

### F3-12 · Pre-cuenta

**Tipo:** Historia · **Prioridad:** Must · **Talla:** S · **Área:** `waiter`, `edge`

**Trazabilidad:** RF-03-09

**Qué se quiere:** El mesero pide la pre-cuenta y se imprime en la caja un ticket informativo.

**Criterios de aceptación:**

- [ ] Imprime en la estación de caja con «DOCUMENTO SIN VALOR TRIBUTARIO», detalle, subtotales, IVA, propina legal y total.
- [ ] La mesa pasa a «Por pagar» (azul).
- [ ] Pre-cuenta dividida en partes iguales (informativa).

**Depende de:** F3-10

### F3-13 · Transferencias y uniones de mesas

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `waiter`, `edge`

**Trazabilidad:** RF-03-10, G-07

**Qué se quiere:** Mover una orden o líneas entre mesas, unir mesas y pasar una mesa a otro mesero.

**Criterios de aceptación:**

- [ ] Mover toda la orden de A a B (si B está libre).
- [ ] Mover líneas específicas entre mesas.
- [ ] Unir mesas en una sola orden.
- [ ] Transferir la mesa a otro mesero.
- [ ] Todo auditado y difundido en tiempo real.

**Depende de:** F3-10

### F3-14 · Anulación de líneas enviadas con permiso o PIN de supervisor

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `waiter`, `edge`

**Trazabilidad:** RF-03-07.3, requisitos §3

**Qué se quiere:** Anular un plato ya enviado a cocina solo con permiso, motivo y ticket de anulación.

**Criterios de aceptación:**

- [ ] Requiere permiso `ANULAR_ITEM_ENVIADO` o «Autorizar con PIN de supervisor» en el mismo dispositivo.
- [ ] Motivo obligatorio y marca «¿se preparó?».
- [ ] Imprime ANULACIÓN en la estación correspondiente.
- [ ] Auditoría con ambos usuarios.

**Notas técnicas:**

- Token de autorización de un solo uso ligado a la acción concreta.

**Diseño (estilo iOS):** Alerta de acción destructiva estilo iOS (botón rojo) seguida del teclado de PIN del supervisor en una hoja.

**Depende de:** F3-10

### F3-15 · Prueba de 8 dispositivos sin internet y E2E de la app

**Tipo:** QA · **Prioridad:** Must · **Talla:** M · **Área:** `waiter`, `edge`

**Trazabilidad:** 07 F3 DoD, RNF-07

**Qué se quiere:** Demostrar que el salón funciona con varios meseros a la vez, sin internet, sin duplicados ni colisiones.

**Criterios de aceptación:**

- [ ] 8 dispositivos contra un nodo sin internet: 0 platos duplicados, 0 colisiones de mesa (test Go + prueba manual guiada).
- [ ] `integration_test`/Patrol del flujo emparejar → PIN → mesa → pedido → enviar.
- [ ] Modo avión a mitad de un pedido: la orden se envía al volver.
- [ ] Arranque en frío hasta el PIN ≤ 2 s en el móvil de referencia.

**Depende de:** F3-10, F3-13

## F4 · Caja POS

**Objetivo:** Caja web servida por el Nodo Local: cobro Zero-Click con billetes dinámicos, pagos mixtos, división de cuentas, jornadas, turnos, cierre ciego y Cierre Z inmutable.

**Requisitos:** RF-04-01…10 · RF-01-07 · RF-02-07 · RF-03-11 · RF-08-06 (registro)

**Definition of Done de la fase:**

- [ ] Suite Cypress del flujo de cobro y división en verde.
- [ ] Cuadre al centavo en 1 000 escenarios aleatorios de división (property-based).
- [ ] El Cierre Z coincide con la suma de los pagos en todas las pruebas.

### F4-01 · POS web servida por el Nodo Local

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `pos`, `edge`

**Trazabilidad:** 03 §2, RNF-41, ADR-0003

**Qué se quiere:** La caja es una web React empaquetada dentro del binario del nodo: funciona sin internet y se actualiza con él.

**Criterios de aceptación:**

- [ ] PWA React + TS embebida con `go:embed` y servida en `https://<nodo>.local`.
- [ ] Cliente WebSocket con reconexión e indicador 🟢 🟡 🔴.
- [ ] Marco de atajos de teclado documentado y visible (tecla `?`).
- [ ] Resolución mínima 1280×720; últimas 2 versiones de Chrome, Edge, Safari y Firefox.

**Notas técnicas:**

- Mismo ui-kit que el backoffice (`packages/ts/ui`).

**Diseño (estilo iOS):** Diseño de iPad en horizontal: barra lateral de mesas/órdenes a la izquierda, detalle a la derecha, todo sobre fondo agrupado con tarjetas translúcidas.

**Depende de:** F2-06, F1-10

### F4-02 · Jornada operativa

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `pos`

**Trazabilidad:** RF-04-01, X-15

**Qué se quiere:** El día de negocio del local: se abre una vez, puede cruzar la medianoche y agrupa ventas, turnos y stock diario.

**Criterios de aceptación:**

- [ ] Abrir jornada una vez al día; fecha de negocio = fecha de apertura.
- [ ] Solo se cierra sin turnos abiertos ni órdenes abiertas (o transfiriéndolas explícitamente).
- [ ] Sesiones de meseros se cierran al cerrar la jornada.

**Depende de:** F4-01

### F4-03 · Cajas y turnos de caja

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `pos`

**Trazabilidad:** RF-04-02, G-06

**Qué se quiere:** El cajero abre su turno en una caja con un fondo inicial; sin turno no se cobra.

**Criterios de aceptación:**

- [ ] Cada caja está asociada a un punto de emisión SRI (se usa en F5).
- [ ] Apertura declarando el fondo inicial; una caja tiene como máximo un turno abierto.
- [ ] Sin turno abierto no se puede cobrar en esa caja.
- [ ] Se permiten varios turnos por día.

**Diseño (estilo iOS):** Apertura como hoja con teclado numérico grande y el monto en tipografía de 48 pt, igual que la calculadora de iOS.

**Depende de:** F4-02

### F4-04 · Mesas, órdenes y venta directa en mostrador

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `pos`, `edge`

**Trazabilidad:** RF-04-03.1-2, RF-03-11, G-08

**Qué se quiere:** El cajero ve todas las órdenes abiertas y puede vender directo en mostrador, para llevar o delivery propio.

**Criterios de aceptación:**

- [ ] Vista de mesas y órdenes abiertas con búsqueda por número; navegación solo con teclado.
- [ ] Búsqueda predictiva de productos (misma regla que RF-03-05).
- [ ] Tipos de orden: Mesa, Para llevar, Barra/Mostrador, Delivery propio, con número o nombre corto impreso en la comanda.

**Depende de:** F4-01, F3-10

### F4-05 · Cobro «Zero-Click» con billetes dinámicos

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `pos`, `edge`

**Trazabilidad:** RF-04-03.3-5, RNF-05

**Qué se quiere:** Tocar «$20» calcula el vuelto, abre el cajón, emite el documento y libera la mesa en un solo gesto, en menos de 2 s.

**Criterios de aceptación:**

- [ ] Para $14,50 muestra [$14,50 exacto] [$15] [$20] [$50] (algoritmo con billetes de USD circulantes).
- [ ] Un toque: vuelto, apertura de cajón, documento, impresión y mesa libre.
- [ ] Botón «Consumidor final» destacado, deshabilitado si supera el límite vigente (parámetro global).
- [ ] p95 ≤ 2 s de «Facturar» a caja libre, con o sin internet.

**Notas técnicas:**

- En F4 emite un documento interno sin valor fiscal (F4-16); F5 lo sustituye por el comprobante.

**Diseño (estilo iOS):** Total gigante arriba, fila de «billetes» como botones grandes con verde dólar suave, vuelto que aparece con animación de conteo y check verde final.

**Depende de:** F4-03, F4-04

### F4-06 · Métodos de pago y pagos mixtos

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `pos`, `edge`, `backoffice`

**Trazabilidad:** RF-04-04, 05 §7

**Qué se quiere:** Métodos configurables mapeados al SRI y cuentas pagadas con varios métodos a la vez.

**Criterios de aceptación:**

- [ ] Métodos configurables (Efectivo, Tarjeta crédito/débito, Transferencia, DeUna/billeteras, Otros) con código de forma de pago SRI y si abren cajón.
- [ ] Una cuenta se paga con varios métodos ($20 efectivo + $15,40 tarjeta).
- [ ] Tarjeta: lote, referencia y últimos 4 opcionales para el cuadre.
- [ ] Integración con datafonos fuera de alcance (F11).

**Depende de:** F4-05

### F4-07 · Datos del comprador con validación y autocompletado

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `pos`, `edge`, `cloud`

**Trazabilidad:** RF-04-05, 05 §12, L-06

**Qué se quiere:** Escribir la cédula o el RUC autocompleta el resto en 300 ms; con Enter se factura.

**Criterios de aceptación:**

- [ ] Validación local de cédula (módulo 10) y RUC (según tipo); para sociedades con módulo 11 inválido se advierte pero no se bloquea.
- [ ] Búsqueda en cascada: nodo → nube → proveedor externo opcional.
- [ ] Autocompleta nombre, dirección, correo y teléfono en ≤ 300 ms en local/nube.
- [ ] Formulario mínimo si no existe; guardado con consentimiento informado (LOPDP).
- [ ] Entidad `clientes` sincronizada con last-writer-wins por campo.

**Notas técnicas:**

- Paquete compartido de validación en Go y TS con los mismos vectores de prueba.

**Depende de:** F4-05

### F4-08 · División de cuenta: por ítems, fracciones y partes iguales

**Tipo:** Historia · **Prioridad:** Must · **Talla:** XL · **Área:** `pos`, `edge`

**Trazabilidad:** RF-04-06, CU-02, X-10, QA-05

**Qué se quiere:** Mesas de 8 que pagan por separado sin errores: arrastrar platos a cuentas, dividir una pizza entre 3 o partir el total en N.

**Criterios de aceptación:**

- [ ] Líneas a la izquierda, cuentas a la derecha con «[+] Nueva cuenta»; asignación arrastrando o tocando sin llamar al servidor.
- [ ] Fracción de ítem entre N cuentas con ajuste de centavos en la última.
- [ ] Partes iguales: residuo en la última cuenta.
- [ ] Confirmación atómica; la orden original no se anula y se cierra al pagarse la última cuenta.
- [ ] Cada cuenta con su cliente y métodos; si ya hay cuentas pagadas solo se reagrupan las no pagadas.
- [ ] Property-based: cuadre al centavo en 1 000 escenarios aleatorios.

**Notas técnicas:**

- Tablas `cuentas` y `cuenta_asignaciones` (fracción NUMERIC(7,6)).

**Diseño (estilo iOS):** Cuentas como tarjetas apiladas estilo Wallet, cada una con su color; al soltar un plato la tarjeta se ilumina y su total se anima.

**Depende de:** F4-06

### F4-09 · Descuentos y cortesías

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `pos`, `edge`

**Trazabilidad:** RF-04-07

**Qué se quiere:** Descontar por línea o por cuenta con motivo y límite, y cortesías autorizadas.

**Criterios de aceptación:**

- [ ] Descuento en % o monto por línea o cuenta con motivo de una lista configurable.
- [ ] Cortesía (100 %) con motivo y autorización.
- [ ] Límite máximo por rol o usuario; excederlo pide PIN de supervisor.
- [ ] Todo descuento se audita y se refleja luego en el XML.

**Depende de:** F4-05, F4-14

### F4-10 · Propina legal del 10 %

**Tipo:** Historia · **Prioridad:** Must · **Talla:** S · **Área:** `pos`, `edge`

**Trazabilidad:** RF-04-08, L-11

**Qué se quiere:** El 10 % de servicio se calcula sobre la base imponible, sin IVA, y se registra por mesero.

**Criterios de aceptación:**

- [ ] Activable por local.
- [ ] 10 % de la base imponible después de descuentos; no incluye ni grava IVA.
- [ ] El cajero puede retirarla si el cliente la rechaza (auditado).
- [ ] Registrada por orden y mesero para el reparto (F8-03).

**Depende de:** F4-05

### F4-11 · Movimientos de caja

**Tipo:** Historia · **Prioridad:** Should · **Talla:** S · **Área:** `pos`, `edge`

**Trazabilidad:** RF-04-09

**Qué se quiere:** Registrar retiros a caja fuerte, ingresos no relacionados con ventas y gastos menores.

**Criterios de aceptación:**

- [ ] Retiros parciales, ingresos y gastos con motivo y usuario.
- [ ] Foto opcional del comprobante del gasto.
- [ ] Entran en el cálculo del esperado del cierre.

**Depende de:** F4-03

### F4-12 · Cierre de turno ciego y Cierre Z inmutable

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `pos`, `edge`, `cloud`

**Trazabilidad:** RF-04-10, CU-05

**Qué se quiere:** El cajero declara lo que cuenta sin ver lo esperado; el sistema calcula diferencias y genera un Cierre Z que nadie puede alterar.

**Criterios de aceptación:**

- [ ] La pantalla no muestra totales esperados.
- [ ] Asistente por denominación ($100…$1 y monedas) + vouchers de tarjeta y transferencias.
- [ ] Esperado = fondo + efectivo cobrado + ingresos − retiros − gastos; resultado Sobrante/Faltante/Cuadrado por método.
- [ ] Cierre Z append-only con numeración por caja y hash.
- [ ] Se imprime, se sincroniza y la nube envía el PDF al dueño; alerta crítica si supera el umbral.
- [ ] La caja queda bloqueada hasta abrir un nuevo turno.

**Diseño (estilo iOS):** Asistente paso a paso con contadores `− 3 +` por billete (como el selector de cantidad de la App Store) y resultado final con icono grande verde, amarillo o rojo.

**Depende de:** F4-06, F4-11

### F4-13 · Cajón de dinero

**Tipo:** Historia · **Prioridad:** Must · **Talla:** S · **Área:** `edge`, `pos`

**Trazabilidad:** RF-02-07

**Qué se quiere:** El cajón se abre solo con un cobro en efectivo; abrirlo sin venta requiere permiso y queda auditado.

**Criterios de aceptación:**

- [ ] Pulso ESC/POS por la impresora de caja al cobrar en efectivo.
- [ ] Apertura sin venta con permiso `ABRIR_CAJON` y auditoría.

**Depende de:** F4-05, F2-09

### F4-14 · Permisos granulares y autorización con PIN de supervisor

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `backoffice`, `edge`, `pos`, `waiter`

**Trazabilidad:** RF-01-07

**Qué se quiere:** El dueño decide con interruptores qué puede hacer cada empleado y, en el momento, un supervisor autoriza con su PIN.

**Criterios de aceptación:**

- [ ] Pantalla de interruptores por usuario para las acciones ⚙️ de la matriz RBAC.
- [ ] «Autorizar con PIN de supervisor» genera un token de un solo uso ligado a la acción.
- [ ] Cambios de permisos auditados y sincronizados al nodo.

**Diseño (estilo iOS):** Lista agrupada tipo «Ajustes > Privacidad» con toggles verdes de iOS y descripción corta bajo cada permiso.

**Depende de:** F1-04, F3-04

### F4-15 · Auditoría inmutable con cadena de hash (registro)

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `cloud`, `db`

**Trazabilidad:** RF-08-06.1-3, QA-10

**Qué se quiere:** Todas las acciones sensibles quedan registradas en una tabla que la base de datos no deja modificar ni borrar.

**Criterios de aceptación:**

- [ ] Registro de las acciones de RF-08-06.1 con tenant, local, usuario, autorizador, dispositivo, acción, entidad, antes/después, monto, motivo y hora.
- [ ] Append-only por permisos y triggers en PostgreSQL y SQLite.
- [ ] Cada registro incluye el hash del anterior.
- [ ] QA-10: UPDATE/DELETE en tablas append-only → error de la base de datos.

**Notas técnicas:**

- La visualización y la matriz de fugas llegan en F8-05.

**Depende de:** F4-01

### F4-16 · Documento interno de venta y suite Cypress de caja

**Tipo:** QA · **Prioridad:** Must · **Talla:** M · **Área:** `pos`, `ci`

**Trazabilidad:** 07 F4 DoD

**Qué se quiere:** Mientras no hay SRI, el cobro emite un documento interno; y los flujos de caja quedan protegidos por pruebas E2E.

**Criterios de aceptación:**

- [ ] Documento interno sin valor fiscal, claramente marcado, reemplazable por el comprobante en F5.
- [ ] Cypress (selectores `data-testid`): abrir turno → cobrar mesa → pago mixto → dividir → cerrar turno.
- [ ] El Cierre Z coincide con la suma de pagos en todas las pruebas.

**Depende de:** F4-12, F4-08

## F5 · Facturación electrónica SRI

**Objetivo:** Comprobantes electrónicos reales: secuencial y clave de acceso en el nodo, XML + XAdES-BES + SOAP en la nube, RIDE, bóveda de 7 años, notas de crédito y onboarding fiscal guiado.

**Requisitos:** RF-05-01…09 · RF-04-11 · RF-08-02 (básicos)

**Definition of Done de la fase:**

- [ ] 200 comprobantes (facturas + NC) autorizados en el ambiente de pruebas cubriendo todas las tarifas, consumidor final, propina, descuentos y pagos mixtos.
- [ ] Caída del SRI simulada 2 h sin pérdida.
- [ ] Revisión del contador firmada (checklist `05` §14).

### F5-01 · Revisión normativa con contador o tributarista

**Tipo:** Decisión · **Prioridad:** Must · **Talla:** M · **Área:** `sri`, `docs`

**Trazabilidad:** DP-07, 05 §14, DP-05

**Qué se quiere:** Validar con un experto cada punto marcado 🔎 antes de emitir comprobantes reales.

**Criterios de aceptación:**

- [ ] Checklist `05` §14 completo: esquema, plazo de envío, monto consumidor final, contenido del RIDE, saltos de secuencial, anulación vs NC, leyendas RIMPE, algoritmos de firma.
- [ ] Tratamiento de DEVUELTA/NO AUTORIZADO definido (CU-04).
- [ ] DP-05 (notas de venta RIMPE) decidido.
- [ ] Acta firmada en `docs/fuentes/`.

**Depende de:** F0-14

### F5-02 · Puntos de emisión y secuenciales atómicos en el nodo

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `db`

**Trazabilidad:** RF-05-02.1, L-03, QA-03

**Qué se quiere:** Cada caja tiene su serie (001-001, 001-002) y el nodo asigna números sin huecos ni duplicados, incluso sin internet.

**Criterios de aceptación:**

- [ ] `puntos_emision` con un único nodo dueño; `secuenciales` por (punto, tipo, ambiente).
- [ ] Incremento dentro de la misma transacción SQLite del pago y el comprobante; si falla, no se consume el número.
- [ ] QA-03: 10 000 cobros concurrentes → 0 duplicados, 0 huecos.
- [ ] Secuenciales de pruebas y producción independientes.

**Depende de:** F4-03, F5-01

### F5-03 · Clave de acceso de 49 dígitos

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** S · **Área:** `sri`, `edge`

**Trazabilidad:** RF-05-02, 05 §4, L-01, QA-04

**Qué se quiere:** Generar la clave de acceso correcta (fecha, tipo, RUC, ambiente, serie, secuencial, código numérico, tipo de emisión y dígito verificador).

**Criterios de aceptación:**

- [ ] `packages/go/sri/claveacceso` con los 9 campos y módulo 11 (d=11 → 0, d=10 → 1).
- [ ] Código numérico de 8 dígitos con `crypto/rand`.
- [ ] QA-04: vectores de comprobantes reales autorizados + property-based sobre 10⁶ claves.
- [ ] Fecha en la zona horaria del local.

**Notas técnicas:**

- Reutiliza lo probado en el spike F0-08.

**Depende de:** F0-08

### F5-04 · Motor de cálculo de impuestos y redondeo

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** L · **Área:** `sri`, `edge`, `pos`

**Trazabilidad:** RF-05-06, 05 §8, QA-05

**Qué se quiere:** El cálculo que garantiza que un plato de $15,00 con IVA incluido factura exactamente $15,00.

**Criterios de aceptación:**

- [ ] Precio sin IVA con 6 decimales; totales por línea y por tarifa a 2 decimales half-up.
- [ ] Propina = 10 % de las bases, sin IVA.
- [ ] Ajuste del centavo residual por *largest remainder* respetando tolerancias del SRI 🔎.
- [ ] QA-05: 10 000 facturas aleatorias (precios, cantidades, descuentos, propina, división) cuadran al centavo.
- [ ] Mismo resultado en Go (nodo y nube) y TS (vista previa en caja).

**Depende de:** F0-11, F4-08

### F5-05 · Emisión local del comprobante e impresión del RIDE ticket

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `pos`

**Trazabilidad:** RF-05-02.1-3, 05 §11

**Qué se quiere:** Al cobrar, el nodo emite e imprime al instante el comprobante con su clave de acceso, sin esperar al SRI.

**Criterios de aceptación:**

- [ ] Pago + secuencial + clave + comprobante `EMITIDO_LOCAL` en una transacción.
- [ ] Ticket de 80 mm con los datos del RIDE, clave de acceso (texto + Code128 o QR) y «Comprobante pendiente de autorización del SRI».
- [ ] En ambiente de pruebas: «AMBIENTE DE PRUEBAS - SIN VALIDEZ TRIBUTARIA».
- [ ] El comprobante entra al outbox con hash del contenido tributario.
- [ ] Sustituye el documento interno de F4-16.

**Depende de:** F5-02, F5-03, F5-04

### F5-06 · Onboarding fiscal guiado

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-05-01, L-07

**Qué se quiere:** Un asistente que convierte el trámite fiscal en seis pasos amables, sin que el dueño sepa de XML ni de web services.

**Criterios de aceptación:**

- [ ] Paso 1: subir .p12 + contraseña. Paso 2: validar vigencia, cadena y uso; extraer RUC y titular.
- [ ] Paso 3: confirmar razón social, nombre comercial y dirección matriz.
- [ ] Paso 4: régimen (General, RIMPE Emprendedor, RIMPE Negocio Popular), contribuyente especial, agente de retención, obligado a contabilidad; siempre confirmado por el usuario.
- [ ] Paso 5: cajas → puntos de emisión. Paso 6: factura de prueba en ambiente 1.
- [ ] RUC del certificado distinto al del tenant → bloqueo con mensaje claro.
- [ ] Alertas de vencimiento a 30, 15, 7 y 1 días.

**Diseño (estilo iOS):** Asistente estilo «Configurar tu iPhone»: un paso por pantalla, icono grande arriba, barra de progreso fina, botón «Continuar» abajo y ✓ animado al terminar.

**Depende de:** F5-07, F1-10

### F5-07 · Envelope encryption del certificado P12

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`, `sri`

**Trazabilidad:** 06 §4, RNF-22, X-01

**Qué se quiere:** El .p12 y su contraseña se guardan cifrados de forma reversible y solo el worker fiscal puede descifrarlos, en memoria.

**Criterios de aceptación:**

- [ ] DEK AES-256-GCM por certificado; DEK cifrada con la KEK del KMS (nunca en variables de entorno planas).
- [ ] La API puede cifrar pero no descifrar (IAM de mínimo privilegio).
- [ ] El worker descifra en memoria, firma y borra; cada descifrado queda en un log de acceso.
- [ ] El P12 y la contraseña nunca se muestran, descargan ni registran en logs.

**Depende de:** F1-01, F0-01

### F5-08 · Worker fiscal: construcción y validación del XML

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** L · **Área:** `cloud`, `sri`

**Trazabilidad:** RF-05-02.4, ADR-0007, QA-12

**Qué se quiere:** La nube recibe el comprobante del nodo y construye un XML idéntico en contenido, validado contra el XSD oficial.

**Criterios de aceptación:**

- [ ] Cola River sobre PostgreSQL; encolado en la misma transacción que recibe el comprobante.
- [ ] XML de factura y NC desde las tablas normalizadas; la nube no altera ningún campo tributario (verificación por hash).
- [ ] Validación contra el XSD oficial; QA-12 sobre todos los casos de QA-05.
- [ ] Estados en `comprobantes` y transiciones en `comprobante_eventos` (append-only).

**Depende de:** F5-05, F2-04

### F5-09 · Firma XAdES-BES en producción

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** L · **Área:** `sri`, `cloud`

**Trazabilidad:** 05 §9, ADR-0009

**Qué se quiere:** Firmar cada XML con el certificado del restaurante según el enfoque decidido en el spike.

**Criterios de aceptación:**

- [ ] Firma enveloped sobre `Id=comprobante` con SignedInfo, SignedProperties y KeyInfo según la ficha 🔎.
- [ ] Probado con certificados de ≥ 2 entidades certificadoras.
- [ ] Si el ADR eligió sidecar Java: servicio interno con health check y timeouts.

**Depende de:** F5-08, F5-07

### F5-10 · Cliente SOAP del SRI, máquina de estados y reintentos

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** L · **Área:** `sri`, `cloud`

**Trazabilidad:** RF-05-04, 05 §2 §10, CU-03

**Qué se quiere:** Enviar al SRI y conseguir la autorización con paciencia: reintentos inteligentes y sin reenvíos peligrosos.

**Criterios de aceptación:**

- [ ] Recepción y autorización con URLs en configuración; timeouts 10 s conexión / 30 s respuesta.
- [ ] Backoff exponencial con jitter: 1, 2, 5, 15, 30 min y luego cada 30 min.
- [ ] DEVUELTA con errores → `REQUIERE_ATENCION` con mensaje traducido a lenguaje claro; no se reintenta a ciegas.
- [ ] «Clave registrada / en procesamiento» → consultar autorización, no reenviar.
- [ ] Límite de concurrencia por tenant y global; respuestas crudas guardadas.
- [ ] Idempotente: nunca dos comprobantes con distinta clave para la misma cuenta.

**Depende de:** F5-09, F0-07

### F5-11 · RIDE PDF al vuelo, correo y archivo inmutable comprimido

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`, `sri`

**Trazabilidad:** RF-05-02.6, RF-05-07.3, L-13, ADR-0013

**Qué se quiere:** Al autorizarse, el cliente recibe su PDF y XML por correo, y el XML queda guardado 7 años sin posibilidad de borrarse, ocupando lo mínimo.

**Criterios de aceptación:**

- [ ] RIDE A4 con número y fecha de autorización, generado de forma determinista desde el XML; el PDF no se almacena.
- [ ] Correo al comprador (si tiene) con XML + PDF.
- [ ] XML autorizado en PostgreSQL 90 días y archivado cada noche en Blob `comprobantes`: un blob por emisor por día, cada factura comprimida con el diccionario zstd del emisor, índice (clave → offset, largo, sha256) en PostgreSQL.
- [ ] Antes de indexar se descomprime y se verifica el SHA-256 contra el original; si falla, no se borra nada y se reintenta.
- [ ] Contenedor con política de inmutabilidad ≥ 7 años 🔎, GRS y ciclo de vida Hot 30 d → Cool → Cold; diccionarios inmutables y versionados.
- [ ] El nodo recibe `fiscal.status_changed` y actualiza el estado.

**Notas técnicas:**

- Diccionario de 64 KB entrenado con las primeras ~1 000 facturas del emisor (antes, zstd sin diccionario); medición en `13` §1.
- Lectura de una factura por rango HTTP (`x-ms-range`).

**Depende de:** F5-10

### F5-12 · Bóveda de comprobantes y panel de pendientes

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `backoffice`

**Trazabilidad:** RF-05-07, RF-05-04.5

**Qué se quiere:** El dueño encuentra cualquier factura en segundos y ve cuáles están pendientes o requieren atención.

**Criterios de aceptación:**

- [ ] Lista filtrable por fecha, estado, cliente, tipo y punto de emisión.
- [ ] Acciones: Descargar XML (PostgreSQL o archivo Blob por rango), Descargar PDF (regenerado al vuelo), Reenviar correo, Reintentar.
- [ ] Estados: Emitido local · Enviado · Autorizado · No autorizado · Requiere atención · Anulado por NC.

**Diseño (estilo iOS):** Filas con píldora de estado de color (verde autorizado, gris emitido, naranja requiere atención) y barra de búsqueda tipo iOS con tokens de filtro.

**Depende de:** F5-11

### F5-13 · Nota de crédito y reverso de ventas

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `pos`, `edge`, `sri`

**Trazabilidad:** RF-05-08, RF-04-11, L-10

**Qué se quiere:** Una venta facturada solo se revierte con una Nota de Crédito electrónica, con motivo, permiso y auditoría.

**Criterios de aceptación:**

- [ ] NC tipo 04 referenciando la factura (tipo, número y fecha de sustento), total o parcial, con motivo.
- [ ] El monto no excede el saldo no revertido.
- [ ] Si la factura aún no se autorizó, la NC se emite una vez autorizada la original.
- [ ] El reverso devuelve inventario solo si el Admin lo indica.
- [ ] Mismo flujo asíncrono de firma y autorización.

**Depende de:** F5-10

### F5-14 · Leyendas por régimen y límite de consumidor final

**Tipo:** Historia · **Prioridad:** Must · **Talla:** S · **Área:** `sri`, `cloud`, `edge`

**Trazabilidad:** RF-05-05, RF-05-03, L-05

**Qué se quiere:** El XML y el RIDE llevan las leyendas correctas según el régimen, y el consumidor final respeta el monto máximo legal.

**Criterios de aceptación:**

- [ ] Campos `contribuyenteRimpe`, `agenteRetencion`, `contribuyenteEspecial`, `obligadoContabilidad` derivados de la configuración fiscal con vigencia.
- [ ] Consumidor final: `9999999999999`, tipo 07; si supera el monto máximo vigente se exigen datos.

**Depende de:** F5-06

### F5-15 · Ambientes de pruebas y producción por tenant

**Tipo:** Historia · **Prioridad:** Must · **Talla:** S · **Área:** `backoffice`, `cloud`, `edge`

**Trazabilidad:** RF-05-09, G-12

**Qué se quiere:** Cada restaurante empieza en pruebas y pasa a producción de forma explícita tras una factura autorizada.

**Criterios de aceptación:**

- [ ] Modo PRUEBAS/PRODUCCION por tenant con marca en el RIDE.
- [ ] Paso a producción con confirmación dentro de la página, solo tras ≥ 1 comprobante autorizado en pruebas.
- [ ] Secuenciales independientes por ambiente.

**Depende de:** F5-06

### F5-16 · Alertas fiscales y reportes básicos

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`, `backoffice`

**Trazabilidad:** RF-05-04.4, RF-08-02 (básicos)

**Qué se quiere:** El dueño se entera a tiempo de lo que puede ser un problema legal, y tiene sus reportes mínimos para operar.

**Criterios de aceptación:**

- [ ] Alertas: comprobante > 12 h sin autorizar, crítica al acercarse el plazo legal, certificado por vencer, errores del SRI.
- [ ] Reportes: ventas del día y por periodo, listado de Cierres Z con PDF, exportación Excel de ventas.
- [ ] Reportes por fecha de negocio (jornada).

**Depende de:** F5-11

### F5-17 · Campaña de homologación: 200 comprobantes

**Tipo:** QA · **Prioridad:** Must · **Talla:** M · **Área:** `sri`, `cloud`, `edge`

**Trazabilidad:** 07 F5 DoD

**Qué se quiere:** Demostrar en el ambiente de pruebas del SRI que todos los casos reales se autorizan y que una caída del SRI no pierde nada.

**Criterios de aceptación:**

- [ ] 200 comprobantes (facturas + NC) autorizados cubriendo todas las tarifas, consumidor final, propina, descuentos y pagos mixtos.
- [ ] Caída del SRI simulada 2 h: 0 comprobantes perdidos y autorización automática al volver.
- [ ] Revisión del contador firmada.

**Depende de:** F5-13, F5-14, F5-15

### F5-18 · Copia local de comprobantes autorizados (90 días)

**Tipo:** Historia · **Prioridad:** Must · **Talla:** S · **Área:** `edge`, `pos`

**Trazabilidad:** ADR-0013, RF-05-07

**Qué se quiere:** Desde la caja se puede buscar y reimprimir cualquier factura de los últimos 3 meses aunque no haya internet.

**Criterios de aceptación:**

- [ ] Al recibir `fiscal.status_changed` = AUTORIZADO, el nodo guarda el XML autorizado comprimido y el número y la fecha de autorización.
- [ ] Búsqueda en la caja por número, cliente o fecha; reimpresión del RIDE ticket con la leyenda de autorizado (auditada, F2-13).
- [ ] Retención local de 90 días; la purga (F6-04) no toca lo que no esté confirmado en la nube.
- [ ] Tamaño esperado: ~2,5 MB por restaurante al mes con zstd.

**Depende de:** F5-11, F2-13

## F6 · Hardening y piloto

**Objetivo:** Blindar el sistema para producción (respaldos, auto-update, observabilidad, carga, seguridad, LOPDP) y operar un restaurante piloto real en paralelo 14 días.

**Requisitos:** RF-02-10, 11 · RNF-10…15, 50…54

**Definition of Done de la fase:**

- [ ] El piloto opera 2 fines de semana completos en producción.
- [ ] 0 pérdidas de datos y 0 comprobantes perdidos.
- [ ] ≥ 99 % de autorizaciones en menos de 1 h (fuera de caídas del SRI).
- [ ] Feedback registrado y priorizado. 🚀 = MVP.

### F6-01 · Respaldos cifrados del Nodo Local (nube + USB)

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `cloud`

**Trazabilidad:** RF-02-10.1-2, X-13

**Qué se quiere:** Cada noche el nodo copia su base sin detener la operación, la cifra y la sube a la nube; si hay un USB o un segundo disco, deja ahí también una copia (3-2-1).

**Criterios de aceptación:**

- [ ] Respaldo diario (03:30 hora local) con la Online Backup API / `VACUUM INTO`.
- [ ] Comprimido y cifrado AES-256-GCM antes de subir a `/backups/<tenant>/<local>/`.
- [ ] Retención: 7 diarios + 4 semanales.
- [ ] Copia opcional en USB o segundo disco configurado desde la página de estado del nodo; se conservan los últimos 7 y se avisa si el medio falta o está lleno.
- [ ] Blob `respaldos` en LRS con nivel Cool y regla de ciclo de vida que borra lo que excede la retención.

**Depende de:** F2-04

### F6-02 · Recuperación en una PC nueva en ≤ 15 min

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `cloud`

**Trazabilidad:** CU-08, RF-02-10.3-4, RNF-14

**Qué se quiere:** Si la PC de caja se daña, en otra PC el restaurante vuelve a operar en 15 minutos sin duplicar secuenciales.

**Criterios de aceptación:**

- [ ] Instalar + nuevo código de activación → la nube revoca el nodo anterior.
- [ ] Reconstrucción desde el estado sincronizado + último respaldo.
- [ ] Secuencial inicial = máx(nube, respaldo) + margen configurable 🔎.
- [ ] Los dispositivos redescubren el nodo por mDNS.
- [ ] Simulacro cronometrado ≤ 15 min documentado.

**Depende de:** F6-01, F5-02

### F6-03 · Actualización automática firmada con rollback

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `ci`

**Trazabilidad:** RF-02-11, G-01, 03 §8

**Qué se quiere:** Actualizar 100 restaurantes sin ir local por local, sin interrumpir el servicio y volviendo atrás si algo falla.

**Criterios de aceptación:**

- [ ] Manifest firmado ed25519 + binario Authenticode; el nodo verifica antes de instalar.
- [ ] Solo fuera del horario de servicio (ventana configurable) o manual.
- [ ] Si no pasa el health check en 60 s, vuelve a la versión anterior.
- [ ] Anillos: interno → piloto → 10 % → 100 %.
- [ ] La nube soporta N y N-1 del nodo y la app (RNF-53).

**Depende de:** F2-14

### F6-04 · Purga local de datos operativos

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** S · **Área:** `edge`

**Trazabilidad:** 03 §4 §9

**Qué se quiere:** El nodo se mantiene liviano borrando lo antiguo que ya está a salvo en la nube.

**Criterios de aceptación:**

- [ ] Purga de datos operativos > 60 días ya sincronizados y confirmados.
- [ ] Los comprobantes autorizados se conservan 90 días en local (F5-18) antes de purgarse.
- [ ] Nunca se purga lo no sincronizado.
- [ ] PDFs temporales borrados tras confirmar la subida.

**Depende de:** F2-04

### F6-05 · Observabilidad, alertas y dashboard de salud de nodos

**Tipo:** Operación · **Prioridad:** Must · **Talla:** L · **Área:** `cloud`, `edge`

**Trazabilidad:** 03 §10, G-02, RNF-51, RNF-52

**Qué se quiere:** El equipo sabe antes que el cliente cuando algo va mal en cualquier restaurante.

**Criterios de aceptación:**

- [ ] Métricas OTel: latencia de comanda, impresión fallida, outbox, comprobantes por estado y antigüedad, errores SRI por código, conexiones WS.
- [ ] Trazas API → workers → SRI.
- [ ] Alertas: nodo sin reportar > 10 min en horario, comprobantes > 12 h, 5xx, P12 por vencer, deriva de reloj.
- [ ] Dashboard de salud de todos los nodos.

**Depende de:** F2-04, F5-10

### F6-06 · Pruebas de carga y resiliencia

**Tipo:** QA · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `cloud`

**Trazabilidad:** RNF-01…08, 08 §1

**Qué se quiere:** Comprobar que el sistema cumple sus números al doble de la carga de referencia y sobrevive a fallos.

**Criterios de aceptación:**

- [ ] k6 contra el nodo (WS + HTTP) y la nube con el escenario de referencia ×2 (60 mesas, 16 meseros, 4 cajas, 500 productos).
- [ ] Todos los RNF de rendimiento cumplidos y reportados.
- [ ] Resiliencia: kill -9, corte de luz simulado, toxiproxy nodo ↔ nube ↔ SRI, modo avión en la app.

**Depende de:** F5-17

### F6-07 · Revisión de seguridad y cifrado de la base local

**Tipo:** Operación · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `cloud`

**Trazabilidad:** 06, RNF-20…24

**Qué se quiere:** Cerrar los riesgos de seguridad antes de poner datos reales en producción.

**Criterios de aceptación:**

- [ ] Revisión STRIDE actualizada con hallazgos resueltos.
- [ ] ADR: SQLCipher o cifrado de disco del SO para la SQLite del nodo.
- [ ] Escaneos de dependencias limpios; prueba de aislamiento y simulacro de revocación masiva de dispositivos.

**Depende de:** F5-17

### F6-08 · Runbooks de incidentes

**Tipo:** Operación · **Prioridad:** Must · **Talla:** S · **Área:** `docs`

**Trazabilidad:** 06 §9

**Qué se quiere:** Instrucciones paso a paso para los incidentes más probables.

**Criterios de aceptación:**

- [ ] `docs/runbooks/`: nodo caído, SRI caído, fuga de credenciales, certificado comprometido, revocación masiva, restauración de respaldo.
- [ ] `SECURITY.md` con el contacto de seguridad.

**Depende de:** F6-02

### F6-09 · Kit de instalación, manuales y plan de contingencia

**Tipo:** Operación · **Prioridad:** Must · **Talla:** M · **Área:** `docs`

**Trazabilidad:** 07 F6, RNF-43, 03 §7

**Qué se quiere:** Todo lo que necesita un restaurante para instalar y operar sin llamar al soporte.

**Criterios de aceptación:**

- [ ] Checklist de instalación: red de personal separada (VLAN/SSID), UPS, reserva DHCP de impresoras.
- [ ] Manual de 1 página por rol (mesero, cajero, dueño, cocina).
- [ ] Plan de contingencia en papel si cae el nodo.
- [ ] `docs/hardware-certificado.md` con modelos probados.

**Diseño (estilo iOS):** Manuales con el mismo lenguaje visual: capturas en marco de iPhone/iPad, iconos de color por paso y máximo 5 pasos por tarea.

**Depende de:** F6-02

### F6-10 · Cumplimiento LOPDP para el piloto

**Tipo:** Operación · **Prioridad:** Must · **Talla:** M · **Área:** `docs`, `cloud`

**Trazabilidad:** 06 §7, L-12, RNF-23

**Qué se quiere:** Antes de tratar datos reales de comensales y empleados, cumplir la ley de protección de datos.

**Criterios de aceptación:**

- [ ] Acuerdo de tratamiento de datos firmado con el restaurante piloto.
- [ ] Registro de actividades de tratamiento y proceso de derechos ARCO+ documentado.
- [ ] Política de privacidad visible en backoffice y app.
- [ ] Evaluación de la transferencia internacional según DP-01 🔎.

**Depende de:** F0-01

### F6-11 · Piloto: 14 días en paralelo y paso a producción SRI

**Tipo:** Operación · **Prioridad:** Must · **Talla:** L · **Área:** `negocio`

**Trazabilidad:** 07 F6 DoD, 09 §3

**Qué se quiere:** Un restaurante real usa el sistema junto al suyo durante 14 días y luego factura en producción.

**Criterios de aceptación:**

- [ ] Instalación con el kit; 14 días en paralelo en una caja o área del salón.
- [ ] Paso a producción SRI del restaurante.
- [ ] 2 fines de semana completos: 0 pérdidas de datos, 0 comprobantes perdidos, ≥ 99 % autorizados en < 1 h.
- [ ] Feedback registrado y priorizado; costo unitario por tenant calculado con datos reales.

**Depende de:** F6-03, F6-05, F6-06, F6-07, F6-09, F6-10

## F7 · Inventario y stock diario

**Objetivo:** Inventario por recetas (BOM) con sub-recetas, conversión de unidades, kardex inmutable, toma física ciega y cupos diarios en tiempo real con reservas.

**Requisitos:** RF-06-01…10

**Definition of Done de la fase:**

- [ ] La descarga por receta es exacta al 4.º decimal en la suite E2E.
- [ ] Carrera de cupos (2 disponibles, 3 solicitudes) → 2 éxitos en el 100 % de 1 000 ejecuciones.
- [ ] El reporte diario de cupos cuadra.

### F7-01 · Unidades de medida e insumos

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`, `backoffice`, `db`

**Trazabilidad:** RF-06-01

**Qué se quiere:** Comprar en kilos o quintales y consumir en gramos sin hacer reglas de tres.

**Criterios de aceptación:**

- [ ] Catálogo global de unidades (masa, volumen, unidad) con conversiones precargadas; unidades de empaque propias (caja x24).
- [ ] Insumo con unidad de compra, unidad de consumo y factor (1 quintal = 45 359,24 g).
- [ ] Stock mínimo y costo promedio ponderado.
- [ ] Stock mostrado en ambas unidades: «5,00 kg (5000 g)».
- [ ] Cantidades en NUMERIC(14,4), nunca float.

**Depende de:** F6-11

### F7-02 · Proveedores y compras con costo promedio

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`, `backoffice`

**Trazabilidad:** RF-06-02

**Qué se quiere:** Registrar la mercadería que entra y mantener el costo real de cada insumo.

**Criterios de aceptación:**

- [ ] Compra con proveedor, fecha, número de factura, ítems, cantidad en unidad de compra y costo.
- [ ] Al confirmar: movimiento COMPRA en unidad de consumo y recálculo del costo promedio ponderado.
- [ ] (S) Importar el XML autorizado de la factura del proveedor y precargar ítems.

**Depende de:** F7-03

### F7-03 · Kardex append-only y saldos en la misma transacción

**Tipo:** Técnica · **Prioridad:** Must · **Talla:** M · **Área:** `cloud`, `edge`, `db`

**Trazabilidad:** RF-06-05.3-4, T-07, QA-10

**Qué se quiere:** Un libro mayor de inventario que nunca se edita, con saldos exactos al instante.

**Criterios de aceptación:**

- [ ] `kardex_movimientos` append-only con tipos COMPRA, VENTA, REVERSO_VENTA, PRODUCCION_*, AJUSTE, MERMA, TRASLADO_*, INICIAL.
- [ ] `stock_saldos` actualizado en la misma transacción.
- [ ] Job nocturno de reconciliación saldo vs suma del kardex con alerta ante diferencias.
- [ ] Propiedad: movimientos por venta los escribe el nodo; compras, producción y tomas la nube.

**Depende de:** F7-01

### F7-04 · Recetas versionadas con constructor visual

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-06-03

**Qué se quiere:** Arrastrar insumos a un plato, ver su costo teórico y recibir una alerta si el margen cae.

**Criterios de aceptación:**

- [ ] Producto Simple o Receta; recetas también para modificadores (Extra queso +20 g; Sin cebolla −15 g).
- [ ] Constructor drag & drop con cantidades en unidad de consumo, incluidos empaques.
- [ ] Costo teórico = Σ cantidad × costo promedio, recalculado al cambiar costos.
- [ ] Alerta de margen bajo el umbral (30 % por defecto).
- [ ] Recetas versionadas: un cambio no altera ventas pasadas.

**Diseño (estilo iOS):** Plato al centro como tarjeta con foto; insumos como fichas que se arrastran desde una lista lateral con buscador; anillo de margen tipo «Actividad» de Apple Watch (verde > 30 %, rojo por debajo).

**Depende de:** F7-01

### F7-05 · Preparaciones y producción por lotes

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-06-04

**Qué se quiere:** La salsa BBQ que prepara el chef es insumo de otros platos y se produce por lotes.

**Criterios de aceptación:**

- [ ] Ítem PREPARACION con receta y rendimiento (1 lote = 5000 ml).
- [ ] «Producir N lotes» en una transacción: PRODUCCION_SALIDA de insumos y PRODUCCION_ENTRADA con costo calculado.
- [ ] Detección y rechazo de ciclos; anidamiento máximo 3 niveles.

**Depende de:** F7-04

### F7-06 · Descarga automática de inventario por venta

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `edge`

**Trazabilidad:** RF-06-05, DP-14, QA-09

**Qué se quiere:** Vender un plato descuenta sus ingredientes exactos, incluidas sub-recetas y modificadores.

**Criterios de aceptación:**

- [ ] Al enviar la comanda (o al cobrar, configurable, DP-14) se insertan movimientos VENTA por insumo.
- [ ] Anular una línea enviada: reverso si no se preparó; MERMA si se preparó.
- [ ] Se permite stock negativo, marcado en rojo y reportado.
- [ ] QA-09: recetas anidadas → saldo exacto al 4.º decimal.

**Depende de:** F7-03, F7-05

### F7-07 · Alertas de stock mínimo

**Tipo:** Historia · **Prioridad:** Must · **Talla:** S · **Área:** `cloud`, `backoffice`

**Trazabilidad:** RF-06-06

**Qué se quiere:** Saber cuándo comprar antes de quedarse sin insumos.

**Criterios de aceptación:**

- [ ] Insumo bajo mínimo en rojo y alerta en el dashboard.
- [ ] Resumen diario opcional por correo.

**Depende de:** F7-06

### F7-08 · Toma física ciega y mermas

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-06-07

**Qué se quiere:** El bodeguero cuenta sin ver lo que debería haber, y cada diferencia se explica y se valoriza.

**Criterios de aceptación:**

- [ ] Interfaz para tablet sin la cantidad teórica.
- [ ] Diferencias con motivo obligatorio (Caducidad, Error de porción, Derrame, Robo, Error de conteo).
- [ ] Movimiento AJUSTE valorizado al costo promedio.
- [ ] Reporte de mermas por periodo, motivo e insumo en dinero.

**Diseño (estilo iOS):** Una tarjeta por insumo a pantalla completa con teclado numérico, deslizando para pasar al siguiente, como las fichas de «Recordatorios».

**Depende de:** F7-03

### F7-09 · Stock PERMANENTE vs DIARIO y cupos por jornada

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `edge`, `cloud`, `backoffice`

**Trazabilidad:** RF-06-08, X-15

**Qué se quiere:** Las bebidas llevan stock continuo; los platos del día tienen un cupo que se habilita cada mañana.

**Criterios de aceptación:**

- [ ] `comportamiento_stock`: NINGUNO, PERMANENTE, DIARIO.
- [ ] DIARIO: al abrir la jornada el cupo es 0 y aparecen «Por habilitar»; el chef ingresa cantidades.
- [ ] Al cerrar la jornada (no el turno) el sobrante se reporta y se reinicia.

**Depende de:** F7-06, F4-02

### F7-10 · Reservas de cupo en tiempo real

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `edge`, `waiter`

**Trazabilidad:** RF-06-09, X-11, QA-02

**Qué se quiere:** Si quedan 2 ceviches y 3 meseros los piden a la vez, exactamente 2 lo consiguen y todos lo ven al instante.

**Criterios de aceptación:**

- [ ] Tarjeta «Quedan N»: verde > 5, amarillo ≤ 5, rojo 1; con 0 «Agotado» y deshabilitada.
- [ ] Agregar al carrito reserva de forma atómica en el nodo y difunde en ≤ 300 ms.
- [ ] Eliminar la línea libera la reserva; vence a los 10 min sin heartbeat.
- [ ] Enviar la comanda convierte la reserva en consumo.
- [ ] QA-02: 2 disponibles, 3 solicitudes simultáneas → 2 éxitos en 1 000 ejecuciones.

**Diseño (estilo iOS):** Badge en la esquina de la tarjeta del plato con el número; al agotarse la tarjeta se desatura con transición suave.

**Depende de:** F7-09, F3-08

### F7-11 · Recarga rápida de cupo

**Tipo:** Historia · **Prioridad:** Must · **Talla:** S · **Área:** `pos`, `kds`, `edge`

**Trazabilidad:** RF-06-10, CU-06

**Qué se quiere:** Salió una olla nueva: «+30» en dos toques y el plato vuelve a estar disponible en todos lados.

**Criterios de aceptación:**

- [ ] Panel lateral «Platos limitados» en caja y KDS con [+] y teclado numérico.
- [ ] Difusión y reactivación en ≤ 1 s.
- [ ] Cada recarga auditada; reporte diario inicial + recargas − vendidos = sobrante.

**Depende de:** F7-10

## F8 · Reportes, analítica y auditoría

**Objetivo:** El «cerebro» del dueño: dashboard en vivo con widgets iOS, reportes exportables, reparto de propinas, exportación contable y la matriz de fugas.

**Requisitos:** RF-08-01…07

**Definition of Done de la fase:**

- [ ] Dashboard en vivo con latencia ≤ 5 s.
- [ ] El reparto de propinas cuadra al centavo.
- [ ] Matriz de fugas validada por el dueño piloto.

### F8-01 · Dashboard en tiempo real con widgets iOS

**Tipo:** Historia · **Prioridad:** Should · **Talla:** L · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-08-01

**Qué se quiere:** El dueño abre su iPad desde cualquier lugar y ve el restaurante en vivo.

**Criterios de aceptación:**

- [ ] Widgets: ventas del día vs mismo día de la semana anterior a la misma hora, tickets, ticket promedio, comensales.
- [ ] Mapa del salón en vivo (rojo si > 45 min ocupada).
- [ ] Top 5 productos y top 5 meseros.
- [ ] Actualización por SSE con latencia ≤ 5 s; selector de local.

**Notas técnicas:**

- Redis opcional solo para fan-out SSE con varias instancias (ADR-0007).

**Diseño (estilo iOS):** Widgets de la pantalla de inicio de iOS: tarjetas de 22 px de radio en tamaños pequeño/mediano/grande, número principal enorme, flecha verde o roja de variación y gráficos de barras minimalistas.

**Depende de:** F6-11

### F8-02 · Reportes de ventas completos y exportación

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-08-02

**Qué se quiere:** Ventas por cualquier ángulo y exportables para el contador.

**Criterios de aceptación:**

- [ ] Por día, hora, producto, categoría, mesero, método, caja o turno.
- [ ] Descuentos, cortesías y anulaciones por usuario.
- [ ] Exportación .xlsx y CSV; por fecha de negocio.

**Notas técnicas:**

- excelize en Go.

**Depende de:** F5-16

### F8-03 · Reparto de propinas

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-08-03

**Qué se quiere:** El «rol de propinas» en un clic, exacto al centavo.

**Criterios de aceptación:**

- [ ] Reglas: % para fondo común (p. ej. cocina 30 %) y resto por propina propia o por turnos trabajados.
- [ ] Rol por periodo en PDF y Excel con detalle por persona.
- [ ] Solo propina efectivamente cobrada; cuadre al centavo.

**Depende de:** F8-02

### F8-04 · Exportación contable

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-08-04, DP-08

**Qué se quiere:** El contador recibe todo digerido en Excel.

**Criterios de aceptación:**

- [ ] Excel de ventas: fecha, tipo y número, clave de acceso, identificación, cliente, base 0 %, bases gravadas por tarifa, IVA, propina, descuento, total, forma de pago, estado SRI.
- [ ] Excel de compras e inventario valorizado.
- [ ] (C) Plantillas de columnas por sistema contable.

**Depende de:** F8-02, F7-02

### F8-05 · Visor de auditoría y matriz de fugas

**Tipo:** Historia · **Prioridad:** Must · **Talla:** M · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-08-06.4

**Qué se quiere:** Detectar en segundos quién anuló, regaló o descontó y cuánto dinero implicó.

**Criterios de aceptación:**

- [ ] Vista filtrable por usuario, acción, fecha y monto.
- [ ] Matriz de fugas: acciones con impacto económico en rojo, ordenables por monto.
- [ ] Verificación de la cadena de hash con indicador de integridad.

**Diseño (estilo iOS):** Lista agrupada por día con iconos por tipo de acción (tijeras = anulación, etiqueta = descuento, regalo = cortesía) y monto en rojo alineado a la derecha.

**Depende de:** F4-15

### F8-06 · Reportes de inventario

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-08-07

**Qué se quiere:** Saber dónde se va la mercadería y cuánto gana cada plato.

**Criterios de aceptación:**

- [ ] Kardex por insumo; consumo teórico vs real; mermas valorizadas; costo de ventas y margen por plato; cupos diarios.

**Depende de:** F7-08

### F8-07 · Centro de alertas críticas

**Tipo:** Historia · **Prioridad:** Should · **Talla:** S · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-08-01.4

**Qué se quiere:** Un solo lugar para todo lo que requiere atención.

**Criterios de aceptación:**

- [ ] Stock bajo, impresora desconectada, nodo sin conexión, comprobantes sin autorizar, diferencia de caja, certificado por vencer.
- [ ] Marcar como vista; enlace directo a la pantalla que resuelve cada alerta.

**Diseño (estilo iOS):** Centro de notificaciones de iOS: tarjetas agrupadas por origen con icono de app de color y hora relativa.

**Depende de:** F8-01

## F9 · KDS y menú QR

**Objetivo:** Pantalla de cocina offline servida por el nodo, aviso de «plato listo» al mesero y un menú QR público que muestra lo agotado en tiempo real.

**Requisitos:** RF-07-01…03 · RF-03-12

**Definition of Done de la fase:**

- [ ] KDS operando en el piloto.
- [ ] «Plato listo» llega al mesero en ≤ 1 s.
- [ ] El menú QR refleja «Agotado» en ≤ 5 s con internet.

### F9-01 · KDS: PWA servida por el nodo y emparejada a una estación

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `kds`, `edge`

**Trazabilidad:** RF-07-01.1-2

**Qué se quiere:** Una tablet en la cocina muestra las comandas de su estación y funciona sin internet.

**Criterios de aceptación:**

- [ ] PWA React servida por el nodo; opera offline.
- [ ] Emparejamiento a una estación igual que un dispositivo.
- [ ] Una estación puede tener impresora y KDS a la vez.

**Depende de:** F6-11

### F9-02 · KDS: tarjetas de comandas y estados

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `kds`, `edge`

**Trazabilidad:** RF-07-01.3-7

**Qué se quiere:** El cocinero ve lo que falta, cuánto lleva esperando y marca lo que está listo con un toque.

**Criterios de aceptación:**

- [ ] Tarjetas por orden de llegada: mesa, mesero, tiempo, líneas, modificadores y notas.
- [ ] Color por espera: normal → amarillo > 10 min → rojo > 20 min (configurable).
- [ ] Toque: En preparación → Listo; también por línea.
- [ ] Deshacer el último cambio durante 5 s.

**Diseño (estilo iOS):** Modo oscuro por defecto, tipografía grande legible a 2 m, tarjetas con cronómetro circular que se llena y cambia de color.

**Depende de:** F9-01

### F9-03 · Aviso de «plato listo» al mesero

**Tipo:** Historia · **Prioridad:** Should · **Talla:** S · **Área:** `waiter`, `edge`

**Trazabilidad:** RF-03-12

**Qué se quiere:** El mesero no pregunta en cocina: su teléfono le avisa.

**Criterios de aceptación:**

- [ ] Solo al mesero de la orden en ≤ 1 s.
- [ ] Vibración y aviso verde persistente «Mesa 5: Hamburguesa lista».
- [ ] Historial del turno en la pestaña Avisos.

**Diseño (estilo iOS):** Notificación tipo «Live Activity»: píldora verde que se despliega desde arriba con el icono de plato humeante.

**Depende de:** F9-02

### F9-04 · Métricas de cocina

**Tipo:** Historia · **Prioridad:** Could · **Talla:** S · **Área:** `kds`, `backoffice`

**Trazabilidad:** RF-07-02

**Qué se quiere:** Tiempos de preparación por producto y estación, y comandas por hora.

**Criterios de aceptación:**

- [ ] Promedio de preparación por producto y estación.
- [ ] Comandas por hora en el dashboard.

**Depende de:** F9-02

### F9-05 · Menú QR público de solo lectura

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `menu`, `cloud`

**Trazabilidad:** RF-07-03.1-2,5, DP-09

**Qué se quiere:** El comensal escanea el QR y ve un menú bonito, rápido y siempre actualizado.

**Criterios de aceptación:**

- [ ] Web en `https://<dominio>/m/<slug-local>` servida por CDN; carga inicial ≤ 2 s en 4G.
- [ ] Categorías, productos con foto, descripción, precio con IVA y alérgenos opcionales.
- [ ] Personalización: logo, portada, colores y modo oscuro.
- [ ] Solo lectura (auto-pedido en F11).

**Diseño (estilo iOS):** Catálogo estilo App Store: portada grande, categorías en carrusel horizontal, fichas con foto a sangre y hoja de detalle al tocar.

**Depende de:** F1-13

### F9-06 · Disponibilidad en vivo en el menú QR

**Tipo:** Historia · **Prioridad:** Should · **Talla:** S · **Área:** `menu`, `cloud`, `edge`

**Trazabilidad:** RF-07-03.3

**Qué se quiere:** Lo que se agota en la cocina se ve «Agotado por hoy» en el celular del cliente en segundos.

**Criterios de aceptación:**

- [ ] Evento `stock.changed` nodo → nube → menú en ≤ 5 s con internet.
- [ ] Sin internet en el local: última disponibilidad conocida con nota discreta.

**Depende de:** F9-05, F7-10

### F9-07 · Generador de QR con la marca del local

**Tipo:** Historia · **Prioridad:** Should · **Talla:** S · **Área:** `backoffice`

**Trazabilidad:** RF-07-03.4

**Qué se quiere:** Un QR listo para imprimir en acrílicos, con el logo y los colores del restaurante.

**Criterios de aceptación:**

- [ ] Logo al centro y colores de marca; exportable en PNG, SVG y PDF de alta resolución.
- [ ] Apunta al local y opcionalmente a la mesa.

**Notas técnicas:**

- Librería `qrcode`.

**Depende de:** F9-05

## F10 · SaaS comercial

**Objetivo:** Vender sin intervención: planes y entitlements, registro self-service, cobro recurrente, factura del propio SaaS, consola de plataforma, pentest y documentos legales.

**Requisitos:** RF-09-01…04 · RF-01-08 · RF-05-10 · RNF-34

**Definition of Done de la fase:**

- [ ] Un restaurante nuevo se registra, paga, instala y factura sin intervención del equipo.
- [ ] Pentest externo sin hallazgos críticos abiertos.
- [ ] Términos, privacidad y acuerdo de tratamiento de datos publicados.

### F10-01 · Planes y entitlements

**Tipo:** Historia · **Prioridad:** Must · **Talla:** L · **Área:** `cloud`, `backoffice`, `waiter`, `pos`

**Trazabilidad:** RF-09-01

**Qué se quiere:** Emprendedor, Restaurante y Pro definidos por datos, con un único servicio que decide qué puede usar cada tenant.

**Criterios de aceptación:**

- [ ] Límites (usuarios, dispositivos, impresoras, locales) y funciones (recetas, división, KDS, multi-local) como datos.
- [ ] Backend y clientes consultan un único servicio de entitlements.
- [ ] La UI oculta lo no contratado e invita a mejorar el plan.
- [ ] Bajar de plan nunca borra datos.

**Depende de:** F6-11

### F10-02 · Registro self-service con prueba gratis

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `cloud`, `backoffice`

**Trazabilidad:** RF-09-02

**Qué se quiere:** Un restaurante se registra solo, prueba 14 días y recibe todo lo necesario para empezar.

**Criterios de aceptación:**

- [ ] Landing → plan → datos (RUC, correo, teléfono) → verificación de correo → pago → aprovisionamiento (RF-01-01).
- [ ] Prueba configurable sin tarjeta.
- [ ] Correo de bienvenida con backoffice, app de meseros e instalador del nodo con código.

**Depende de:** F10-01

### F10-03 · Cobro recurrente con pasarela ecuatoriana

**Tipo:** Historia · **Prioridad:** Should · **Talla:** L · **Área:** `cloud`

**Trazabilidad:** RF-09-03, DP-06, G-03

**Qué se quiere:** Cobrar la suscripción mensual con una pasarela que opere en Ecuador, sin guardar tarjetas.

**Criterios de aceptación:**

- [ ] Integración con la pasarela elegida (Kushki o Payphone según DP-06) con tokenización.
- [ ] Cobro mensual, reintentos, periodo de gracia y suspensión según RF-01-08.
- [ ] Nunca se almacena el número de tarjeta.

**Depende de:** F10-02

### F10-04 · Factura electrónica del propio SaaS

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `cloud`, `sri`

**Trazabilidad:** RF-09-03.3, L-15

**Qué se quiere:** El SaaS factura su suscripción a cada restaurante con el mismo motor fiscal.

**Criterios de aceptación:**

- [ ] Tenant «plataforma» con su P12 y punto de emisión en la nube.
- [ ] Factura automática tras cada cobro exitoso.

**Depende de:** F10-03

### F10-05 · Consola de plataforma para el Super Admin

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `cloud`, `backoffice`

**Trazabilidad:** RF-01-08

**Qué se quiere:** Ver y gestionar todos los restaurantes desde un solo lugar.

**Criterios de aceptación:**

- [ ] Tenants con plan, estado, versión del nodo, última conexión y comprobantes pendientes.
- [ ] Suspender/reactivar sin bloquear la emisión de comprobantes ya generados.
- [ ] «Ver como» con auditoría y consentimiento registrado.

**Notas técnicas:**

- Rol de base de datos de plataforma separado y auditado.

**Depende de:** F10-01

### F10-06 · Importación del menú desde Excel

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-09-04.1

**Qué se quiere:** Cargar el menú de un restaurante nuevo en minutos.

**Criterios de aceptación:**

- [ ] Plantilla Excel de productos, categorías y precios.
- [ ] Validación con vista previa y errores por fila antes de guardar.

**Depende de:** F1-11

### F10-07 · Guía de trámites externos

**Tipo:** Historia · **Prioridad:** Should · **Talla:** S · **Área:** `backoffice`, `docs`

**Trazabilidad:** RF-05-10, L-14

**Qué se quiere:** Ayuda integrada para obtener la firma en archivo, habilitar la facturación en SRI en Línea y cargar el P12.

**Criterios de aceptación:**

- [ ] Guías y videos cortos; enlace a la lista oficial de entidades acreditadas (sin nombres fijos).
- [ ] Redirección al asistente fiscal (F5-06).

**Depende de:** F5-06

### F10-08 · Modo entrenamiento

**Tipo:** Historia · **Prioridad:** Should · **Talla:** M · **Área:** `waiter`, `pos`, `edge`

**Trazabilidad:** RNF-34

**Qué se quiere:** Un mesero nuevo practica sin afectar ventas, inventario ni el SRI.

**Criterios de aceptación:**

- [ ] Transacciones de práctica aisladas, claramente marcadas en pantalla y en tickets.
- [ ] No generan comprobantes, kardex ni reportes.

**Diseño (estilo iOS):** Borde naranja en toda la pantalla y píldora «Entrenamiento» fija, como el indicador de grabación de iOS.

**Depende de:** F10-01

### F10-09 · 2FA TOTP (obligatorio en plan Pro)

**Tipo:** Historia · **Prioridad:** Should · **Talla:** S · **Área:** `cloud`, `backoffice`

**Trazabilidad:** RF-01-02.4, 06 §2

**Qué se quiere:** Segundo factor para administradores, obligatorio en el plan Pro y para el Super Admin.

**Criterios de aceptación:**

- [ ] Alta con QR y códigos de recuperación.
- [ ] Obligatorio según plan; siempre obligatorio para el Super Admin.

**Depende de:** F10-01

### F10-10 · Pentest externo y documentos legales

**Tipo:** Operación · **Prioridad:** Must · **Talla:** M · **Área:** `docs`, `negocio`

**Trazabilidad:** 07 F10 DoD, 09 §5, 06 §7

**Qué se quiere:** Listo para vender a escala: seguridad verificada por terceros y contratos publicados.

**Criterios de aceptación:**

- [ ] Pentest externo sin hallazgos críticos abiertos.
- [ ] Términos y condiciones, política de privacidad, acuerdo de tratamiento de datos y SLA publicados.
- [ ] Evaluación de impacto (EIPD) completada.

**Depende de:** F10-02

## F11 · Expansión (backlog priorizable)

**Objetivo:** Funciones de crecimiento que solo se construyen tras decidirlas explícitamente: multi-sucursal, hot standby, ATS, notas de venta RIMPE, datafonos, delivery, WhatsApp, auto-pedido QR e IA.

**Requisitos:** RF-01-03 (multi-local) · RF-05-11 · RF-06-11 · RF-07-04 · RF-08-05 · RF-09-04.2

**Definition of Done de la fase:**

- [ ] Cada épica se prioriza con datos del piloto y de los primeros clientes antes de iniciarse.

### F11-01 · Multi-sucursal y traslados de inventario (Pro)

**Tipo:** Historia · **Prioridad:** Could · **Talla:** XL · **Área:** `cloud`, `backoffice`

**Trazabilidad:** RF-01-03, RF-06-11

**Qué se quiere:** Cadenas con varios locales: vista consolidada y traslados de mercadería entre sucursales.

**Criterios de aceptación:**

- [ ] Movimientos TRASLADO_SALIDA/ENTRADA atómicos entre locales.
- [ ] Dashboard consolidado.

**Depende de:** F10-01

### F11-02 · Nodo en hot standby

**Tipo:** Técnica · **Prioridad:** Could · **Talla:** XL · **Área:** `edge`

**Trazabilidad:** 03 §7.5

**Qué se quiere:** Un segundo nodo en espera que toma el control si falla el principal, sin duplicar secuenciales.

**Criterios de aceptación:**

- [ ] Replicación continua y conmutación controlada con revocación del primario.

**Depende de:** F6-02

### F11-03 · ATS (Anexo Transaccional Simplificado)

**Tipo:** Historia · **Prioridad:** Could · **Talla:** L · **Área:** `sri`, `cloud`

**Trazabilidad:** RF-08-05, DP-08, L-09

**Qué se quiere:** Generar el XML del ATS para los contribuyentes a los que aplique, validado con un contador.

**Criterios de aceptación:**

- [ ] XML según XSD vigente; validación con contador.

**Depende de:** F8-04

### F11-04 · Notas de venta RIMPE Negocio Popular

**Tipo:** Historia · **Prioridad:** Could · **Talla:** L · **Área:** `sri`

**Trazabilidad:** RF-05-11, DP-05, L-08

**Qué se quiere:** Abrir el mercado a negocios populares que emiten notas de venta en lugar de facturas.

**Criterios de aceptación:**

- [ ] Según decisión DP-05 y normativa vigente 🔎.

**Depende de:** F5-17

### F11-05 · Integración con datafonos

**Tipo:** Historia · **Prioridad:** Could · **Talla:** L · **Área:** `pos`, `edge`

**Trazabilidad:** RF-04-04.4

**Qué se quiere:** Cobro con tarjeta directo desde la caja con Datafast/Medianet.

**Criterios de aceptación:**

- [ ] El monto viaja al datafono y el resultado vuelve al pago sin digitar.

**Depende de:** F4-06

### F11-06 · API abierta y delivery (PedidosYa, UberEats)

**Tipo:** Historia · **Prioridad:** Could · **Talla:** XL · **Área:** `cloud`, `edge`

**Trazabilidad:** 01 §6.2

**Qué se quiere:** Recibir pedidos de apps de delivery directamente en la comanda.

**Criterios de aceptación:**

- [ ] API pública versionada con llaves por tenant; pedidos entrantes como órdenes DELIVERY.

**Depende de:** F10-01

### F11-07 · Bot de WhatsApp conectado al inventario

**Tipo:** Historia · **Prioridad:** Could · **Talla:** L · **Área:** `cloud`

**Trazabilidad:** 01 §6.2

**Qué se quiere:** Pedidos por WhatsApp que respetan la disponibilidad real.

**Criterios de aceptación:**

- [ ] Menú y disponibilidad en vivo; pedidos entran como órdenes.

**Depende de:** F11-06

### F11-08 · Auto-pedido desde el menú QR

**Tipo:** Historia · **Prioridad:** Won't (por ahora) · **Talla:** XL · **Área:** `menu`, `edge`

**Trazabilidad:** RF-07-04, DP-09

**Qué se quiere:** El comensal pide (y quizá paga) desde su celular, conviviendo con el mesero.

**Criterios de aceptación:**

- [ ] Requiere definición de producto aparte (DP-09).

**Depende de:** F9-06

### F11-09 · Onboarding con IA desde una foto de la carta

**Tipo:** Historia · **Prioridad:** Could · **Talla:** L · **Área:** `backoffice`, `cloud`

**Trazabilidad:** RF-09-04.2

**Qué se quiere:** Subir una foto de la carta y obtener productos y precios para revisar antes de guardar.

**Criterios de aceptación:**

- [ ] Extracción con IA + revisión humana obligatoria.

**Notas técnicas:**

- Modelo recomendado: la familia Claude más reciente vía API.

**Depende de:** F10-06

### F11-10 · Reservas de mesas y fidelización

**Tipo:** Historia · **Prioridad:** Could · **Talla:** L · **Área:** `cloud`, `waiter`

**Trazabilidad:** 01 §6.2

**Qué se quiere:** Reservas integradas al mapa de mesas y programa de puntos para clientes frecuentes.

**Criterios de aceptación:**

- [ ] Por definir con datos de clientes.

**Depende de:** F10-01

### F11-11 · Nodo Local para Linux

**Tipo:** Técnica · **Prioridad:** Should · **Talla:** M · **Área:** `edge`, `ci`

**Trazabilidad:** RNF-42, DP-03

**Qué se quiere:** Instalar el nodo en Ubuntu con .deb + systemd.

**Criterios de aceptación:**

- [ ] Paquete .deb firmado; servicio systemd; misma suite de pruebas que Windows.

**Depende de:** F6-03
