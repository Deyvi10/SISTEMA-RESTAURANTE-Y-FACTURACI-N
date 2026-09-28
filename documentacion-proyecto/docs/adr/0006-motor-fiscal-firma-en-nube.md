# ADR-0006 · Secuencial y clave de acceso en el Edge, firma en la nube

- **Estado:** Propuesto
- **Fecha:** 2026-09-24
- **Relacionado con:** hallazgos T-02, L-02, L-03, X-01 · RF-05

## Contexto
El documento fuente se contradice: a veces firma el XML en el nodo y a veces en la nube. Además se necesita entregar al cliente un comprobante válido **al instante**, incluso sin internet, y proteger el `.p12` (que permite firmar en nombre del contribuyente).

## Decisión
- El **Nodo Local** asigna el **secuencial** (único dueño del punto de emisión), genera la **clave de acceso** (determinística, sin SRI) e imprime el comprobante al cobrar.
- La **nube** construye el XML con los datos exactos del nodo, lo valida contra el XSD, lo **firma con el P12** (que nunca sale de la nube), lo envía al SRI y gestiona la autorización.
- La implementación concreta de XAdES-BES (Go nativo o sidecar) se decide tras el **spike de la Fase 0** en un ADR posterior.

## Alternativas consideradas
| Opción | Motivo del descarte |
|---|---|
| Firmar en el nodo | El P12 quedaría en PCs de restaurante (riesgo de robo o copia); sin internet igual no se puede enviar al SRI, así que firmar localmente no aporta |
| Secuencial asignado en la nube | Imposible emitir sin internet |

## Consecuencias
- Sin internet, los comprobantes quedan "emitidos, pendientes de autorización" hasta que se recupera la conexión; el plazo legal de envío debe vigilarse (DP-07).
- La nube no puede modificar el contenido tributario emitido por el nodo; se verifica con un hash.
