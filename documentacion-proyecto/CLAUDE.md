# CLAUDE.md: contexto permanente del proyecto

Sistema POS SaaS para restaurantes en Ecuador con facturación electrónica SRI y arquitectura **Cloud-Edge** (opera sin internet mediante un Nodo Local).

## Antes de escribir código

1. Identificar **qué fase** se está implementando (`docs/07-plan-de-fases.md`), **qué ticket** (`docs/12-backlog-tickets.md`, fuente en `docs/backlog/tickets.json`) y **qué requisitos** (`docs/requisitos/RF-*.md`). No implementar funcionalidades de fases futuras sin pedirlo.
2. Revisar los ADR relevantes (`docs/adr/`) y el modelo de datos (`docs/04-modelo-de-datos.md`).
3. Consultar `docs/10-decisiones-pendientes.md`: si el trabajo depende de una decisión sin responder (⏳), preguntar al usuario en vez de asumir.

## Reglas no negociables

- **Dinero y cantidades en decimal exacto**, nunca `float`.
- **IDs UUID v7** generados en el origen; nunca autoincrementales.
- **Toda tabla de negocio** tiene `tenant_id` + RLS + prueba de aislamiento.
- **Tablas append-only** (kardex, auditoría, comprobantes, cierres Z, eventos): nunca UPDATE ni DELETE.
- **Propiedad de datos** según `docs/03-arquitectura.md` §3: la nube es dueña del catálogo y la configuración; el Nodo es dueño de la operación, los secuenciales y la clave de acceso.
- El **`.p12` nunca sale de la nube**; la firma XAdES ocurre solo en el worker fiscal.
- Los **secuenciales SRI** solo los asigna el Nodo Local dueño del punto de emisión, dentro de la transacción del cobro.
- Comandos desde clientes: **idempotentes** (`Idempotency-Key`).
- Nada de secretos ni datos personales en código, logs o fixtures.
- Toda afirmación normativa del SRI se valida contra la ficha técnica en `docs/fuentes/sri/` (ver `docs/05` §14).

## Convenciones

- Código en inglés; términos de dominio fiscal en español (`claveAcceso`, `puntoEmision`, `comanda`). BD en español. Documentación en español.
- Conventional Commits con scopes: `cloud`, `edge`, `waiter`, `pos`, `backoffice`, `kds`, `menu`, `sri`, `contracts`, `db`, `docs`, `ci`, `tools`, `deps`.
- Cada PR: requisito implementado, pruebas de sus criterios de aceptación y documentación actualizada en el mismo PR.
- Detalle completo: `docs/11-convenciones-de-codigo.md` y DoD en `docs/08-calidad-y-cicd.md` §4.

## Stack (propuesto, ver ADR-0002/0003)

Go (API cloud, workers, Nodo Local) · PostgreSQL + RLS · SQLite WAL (Edge) · Flutter (app de meseros) · React + TS + Vite (POS, backoffice, KDS, menú QR) · almacenamiento S3 · GitHub Actions.

## Estado actual

Fase: **pre-Fase 0** (documentación v1.0 en revisión; decisiones pendientes sin responder). Todavía no hay código de aplicación.
