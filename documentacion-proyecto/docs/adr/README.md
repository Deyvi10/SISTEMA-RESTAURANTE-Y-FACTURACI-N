# Architecture Decision Records (ADR)

Registro de decisiones de arquitectura significativas, en formato [MADR](https://adr.github.io/madr/) simplificado.

- Un ADR **no se edita** una vez *Aceptado*; si la decisión cambia, se crea uno nuevo que lo *Reemplaza*.
- Estados: `Propuesto` → `Aceptado` | `Rechazado` → (`Obsoleto` | `Reemplazado por ADR-XXXX`).
- Plantilla: [`0000-plantilla.md`](0000-plantilla.md).

| ADR | Título | Estado |
|---|---|---|
| [0001](0001-monorepo.md) | Monorepo único para todas las aplicaciones | Propuesto |
| [0002](0002-stack-backend-go-flutter-postgres.md) | Stack base: Go, Flutter, PostgreSQL, SQLite | Propuesto |
| [0003](0003-frontend-web-react.md) | React + TypeScript + Vite para todas las webs | Propuesto |
| [0004](0004-sincronizacion-edge-cloud.md) | Sincronización Edge-Cloud por propiedad de datos + outbox | Propuesto |
| [0005](0005-identificadores-uuidv7.md) | UUID v7 como identificador universal | Propuesto |
| [0006](0006-motor-fiscal-firma-en-nube.md) | Secuencial y clave de acceso en el Edge, firma en la nube | Propuesto |
| [0007](0007-colas-sobre-postgresql.md) | Colas de trabajo sobre PostgreSQL | Propuesto |
| [0008](0008-multitenancy-rls.md) | Multi-tenancy con base de datos compartida + RLS | Propuesto |
| [0010](0010-modulo-go-unico.md) | Un solo módulo Go en la raíz del monorepo | Aceptado |
| [0011](0011-s3-local-versitygw.md) | VersityGW como S3 local de desarrollo | Aceptado |
| [0012](0012-resultado-spike-sync.md) | Resultado del spike de sincronización (F0-10) | Aceptado |
| [0013](0013-nube-azure-y-almacenamiento.md) | Azure como nube principal y almacenamiento optimizado de comprobantes | Aceptado |
| [0014](0014-identidad-del-nodo.md) | Identidad del Nodo Local: llave generada en el nodo y JWT por petición | Aceptado |
