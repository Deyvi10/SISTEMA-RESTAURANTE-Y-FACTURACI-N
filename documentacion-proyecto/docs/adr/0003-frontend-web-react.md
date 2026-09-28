# ADR-0003 · React + TypeScript + Vite para todas las webs

- **Estado:** Propuesto
- **Fecha:** 2026-09-24
- **Relacionado con:** hallazgo T-01, DP-12

## Contexto
El documento fuente alterna entre React, Vue 3, SolidJS y Next.js. Hay 4 frontends web (POS, Backoffice, KDS, Menú QR) que deberían compartir componentes y diseño.

## Decisión
**React + TypeScript (strict) + Vite** para las 4 webs, con un *ui-kit* compartido en `packages/ts/ui`. Librerías base: TanStack Query (datos de servidor), Zustand (estado local), `dnd-kit` (drag & drop: ruteo de impresoras, división de cuentas, recetas, plano de mesas), React Hook Form + Zod, Tailwind CSS + variables de diseño estilo iOS, ECharts o Recharts (dashboard), `qrcode` (generador QR). El POS y el KDS son **PWA** servidas por el Nodo Local.

## Alternativas consideradas
| Opción | Pros | Contras |
|---|---|---|
| SolidJS | Rendimiento de DOM superior | Ecosistema pequeño (DnD, tablas, gráficos), menos talento disponible. La diferencia de rendimiento no es perceptible en la escala del POS si React se usa correctamente |
| Vue 3 | Buen ecosistema | Sin ventaja clara; dividir stacks encarece |
| Next.js para el menú QR | SSR y SEO | El menú QR no necesita SEO; un servidor Node adicional no se justifica. Se puede pre-renderizar con Vite si hiciera falta |

## Consecuencias
- Un solo stack web y componentes reutilizados.
- El rendimiento del POS se garantiza con presupuestos medibles (RNF-05), virtualización de listas y memoización, verificados en el CI.
