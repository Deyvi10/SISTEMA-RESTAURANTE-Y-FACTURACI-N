# db

Migraciones versionadas `AAAAMMDDHHMM_descripcion.sql` (docs/11 §6). Nunca se edita una migración ya aplicada en main.

- `cloud/migrations`: PostgreSQL. Toda tabla de negocio con `tenant_id` + RLS forzado + prueba QA-06.
- `edge/migrations`: SQLite del Nodo Local.
