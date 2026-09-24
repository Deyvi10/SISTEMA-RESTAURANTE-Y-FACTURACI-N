# 11 · Convenciones de código y de trabajo

Reglas obligatorias para todo el código del repositorio. El CI verifica automáticamente todo lo que se puede automatizar.

## 1. Idioma

| Elemento | Idioma |
|---|---|
| Documentación, issues, PRs, mensajes al usuario final | Español |
| Código (identificadores, paquetes), commits | **Inglés** para el código técnico. Los **términos de dominio** fiscal y de negocio sin traducción precisa se conservan en español para no perder significado: `claveAcceso`, `puntoEmision`, `comanda`, `cierreZ`, `jornada`, `kardex` |
| Tablas y columnas de base de datos | Español (`docs/04-modelo-de-datos.md`) |
| Comentarios | Español o inglés, pero consistente dentro de cada archivo; explican el **por qué**, no el qué |

> Se mantiene un glosario de dominio ES ↔ identificador en `docs/01-vision-y-alcance.md` §9 y en cada paquete de dominio.

## 2. Principios de diseño

1. **Dominio primero:** la lógica de negocio (impuestos, división de cuentas, clave de acceso, kardex, bloqueos) vive en paquetes de dominio **puros**, sin HTTP, SQL ni UI, y probados exhaustivamente.
2. **Dinero siempre en decimal**, nunca `float`. Tipo `Money` compartido con redondeo explícito.
3. **Tiempo inyectable** (`Clock`) para poder probar vencimientos (bloqueos, reservas, reintentos).
4. **Idempotencia** en todo comando que venga de un cliente (header `Idempotency-Key`).
5. **Sin estado global mutable.** Dependencias explícitas por constructor.
6. **Errores con contexto:** en Go, `fmt.Errorf("…: %w", err)`; errores de dominio tipados que se mapean a códigos HTTP y a mensajes para el usuario en un único lugar.
7. **Fallar ruidosamente en desarrollo y de forma segura en producción:** nunca tragarse un error de un comprobante fiscal.

## 3. Go

- Estructura por app: `cmd/<binario>`, `internal/<dominio>/{domain,app,adapters}`, `internal/platform/...` (config, db, logging).
- Herramientas: `gofumpt`, `golangci-lint` (config en el repo), `go test -race` en el CI, `govulncheck`.
- SQL con `sqlc` (consultas en archivos `.sql`, código generado versionado).
- Logging con `log/slog` en JSON; campos estándar `trace_id`, `tenant_id`, `local_id`, `user_id`.
- Contexto (`context.Context`) como primer parámetro en todo lo que haga I/O.

## 4. TypeScript / React

- `strict: true`, sin `any` (salvo justificación comentada).
- Componentes funcionales, hooks; estado del servidor en TanStack Query y estado de UI en Zustand o local.
- Cliente de API **generado** desde OpenAPI (nunca `fetch` manual a endpoints de negocio).
- Pruebas con Vitest + Testing Library; E2E con Cypress (selectores `data-testid`).
- Accesibilidad: roles ARIA, foco visible, navegación completa por teclado en el POS.

## 5. Flutter / Dart

- `very_good_analysis`, `dart format`.
- Arquitectura por *feature*: `lib/features/<feature>/{data,domain,presentation}`.
- Riverpod para el estado; drift para la persistencia; modelos inmutables (`freezed`).
- Cero lógica de negocio en widgets.

## 6. Base de datos

- Una migración por cambio, con nombre `AAAAMMDDHHMM_descripcion.sql`; nunca editar una migración ya aplicada en `main`.
- Toda tabla nueva de negocio: `tenant_id` + política RLS + prueba de aislamiento (QA-06).
- Índices justificados por una consulta concreta.

## 7. Git y Pull Requests

- **Conventional Commits:** `feat(edge): table locking with heartbeat` · `fix(sri): mod11 check digit when result is 10`. Scopes: `cloud`, `edge`, `waiter`, `pos`, `backoffice`, `kds`, `menu`, `sri`, `contracts`, `db`, `docs`, `ci`, `tools` (simuladores y herramientas de desarrollo), `deps`.
- Ramas cortas: `feat/<id-req>-<slug>` (p. ej. `feat/RF-03-03-table-lock`).
- PR pequeño (idealmente < 400 líneas de diff sin código generado), con:
  - Requisito(s) implementados y criterios cubiertos.
  - Cómo se probó.
  - Capturas o video si hay UI.
  - Checklist de la DoD (`08-calidad-y-cicd.md` §4).
- Squash merge a `main`.

## 8. Documentación viva

- Toda decisión técnica relevante → ADR nuevo.
- Todo cambio de requisito → se edita el `RF-*.md` correspondiente en el mismo PR (con nota de cambio al final del archivo).
- Todo cambio de esquema → actualizar `04-modelo-de-datos.md`.
- Los diagramas se escriben en Mermaid dentro del Markdown (versionables y revisables).
