# Vectores de prueba compartidos

Casos que **todas** las implementaciones (Go, TypeScript, Dart) deben pasar igual, para que la caja, el nodo y la app nunca discrepen.

| Archivo | Qué contiene |
|---|---|
| `sri-claves-acceso.json` | Claves de 49 dígitos válidas e inválidas, incluidos los casos borde del módulo 11 |
| `identificaciones.json` | Cédulas, RUC (natural, sociedad, público), pasaportes y consumidor final |

Reglas:

- Nunca pongas datos personales reales de comensales o empleados. Los RUC de ejemplo son de entidades públicas o conocidas y los documentos de persona natural son sintéticos.
- Si agregas un caso, agrégalo aquí y no dentro de una sola prueba.
