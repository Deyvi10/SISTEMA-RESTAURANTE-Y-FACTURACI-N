# CLAUDE.md (raíz del monorepo)

@documentacion-proyecto/CLAUDE.md

## Rutas

La documentación vive en `documentacion-proyecto/`: toda referencia a `docs/…` en el CLAUDE.md importado significa `documentacion-proyecto/docs/…`. El código vive en la raíz (`apps/`, `packages/`, `tools/`…).

## Cómo trabajar aquí

- Go está en `~/.local/go/bin` si no está en el PATH (el Makefile lo encuentra solo).
- Un solo módulo Go en la raíz: `github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N`.
- Antes de dar algo por terminado: `make lint test`.
- Tickets de impresión: se prueban contra `tools/printer-sim` y con archivos golden (`make golden` y revisar el diff).
- Vectores de prueba que deben coincidir entre Go, TS y Dart van en `packages/testdata/`, no dentro de una prueba.

## Estado

| Ticket | Estado |
|---|---|
| F0-02 Monorepo | ✅ |
| F0-03 Tooling | ✅ Go (vet, golangci, gofumpt, ganchos). TS y Flutter se configuran al crear sus apps |
| F0-04 CI | ✅ `.github/workflows/ci.yml` + nocturno |
| F0-05 `make dev` | ✅ Postgres 17, VersityGW (S3 + Object Lock), Mailpit, toxiproxy, simuladores |
| F0-06 Simulador de impresoras | ✅ `tools/printer-sim` |
| F0-07 Stub del SRI | ✅ `tools/sri-stub` |
| F0-11 Dinero, IDs, reloj | ✅ Go. TS y Dart pendientes (usar `packages/testdata/`) |
| F0-08 Spike SRI | 🟡 Clave de acceso lista. Falta XML + XSD + firma XAdES: requiere `.p12` de pruebas (DP-04) |
| F0-09 Spike impresión | 🟡 Librería ESC/POS lista y probada con el simulador. Falta hardware real (DP-04) |
| F0-10 Spike sync | ✅ 10 000 eventos con caos: 0 pérdidas, 0 duplicados (ADR-0012, `packages/go/edgesync`) |
| F0-12 Sistema de diseño | ✅ Tokens únicos → CSS, Tailwind v4, TS y Flutter; contraste AA verificado en CI; componentes CSS; guía viva (`make ui-docs`) |
| F0-01, F0-13, F0-14 | ⏳ Requieren decisiones, usuarios reales o descargar la ficha del SRI |
