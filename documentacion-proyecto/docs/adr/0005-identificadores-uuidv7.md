# ADR-0005 · UUID v7 como identificador universal

- **Estado:** Propuesto
- **Fecha:** 2026-09-24
- **Relacionado con:** hallazgo X-08

## Contexto
Los registros se crean en la nube, en el nodo y en los teléfonos, a veces sin conexión. El documento fuente propone UUID v4 (correcto en unicidad), pero los v4 aleatorios fragmentan los índices B-tree y degradan las inserciones a gran volumen.

## Decisión
Todas las PK son **UUID v7** (RFC 9562): los primeros 48 bits son el timestamp en ms y el resto es aleatorio. Se generan en el origen (Go: `google/uuid` v1.6+ `NewV7`; Dart y TS: librerías equivalentes). Los números visibles para humanos (número de orden del día, número de comanda, secuencial SRI) son campos **aparte**, asignados por el nodo.

## Consecuencias
- Inserciones con localidad de índice similar a un autoincremental.
- El UUID revela el momento de creación (aceptable; no es un dato sensible).
- No se debe usar el UUID como fuente de "hora oficial" (se usan los campos `created_at`).
