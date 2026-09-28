# ADR-0014 · Identidad del Nodo Local: llave generada en el nodo y JWT por petición

- **Estado:** Aceptado
- **Fecha:** 2026-09-25
- **Relacionado con:** F2-02, F2-04, RF-02-01.3, RF-02-10.4, `06-seguridad.md`

## Contexto
F2-02 decía que el nodo «recibe llave ed25519/certificado» al activarse. Si la nube genera la llave y la envía, la llave privada viaja por la red y queda, al menos por un momento, en la nube.

## Decisión
- **El nodo genera su propio par ed25519 y su UUID v7** antes de canjear el código, y envía solo la llave pública. La privada nunca sale de la PC. En F6-07 se cifrará en disco con DPAPI.
- **Autenticación:** cada petición del nodo lleva un JWT EdDSA de 5 minutos firmado con su llave (`packages/go/nodoauth`). La nube acepta como máximo 10 minutos de vigencia y 2 minutos de desfase de reloj.
- **Estado del nodo en cada petición:** la nube consulta que siga `ACTIVO`, así que revocar un nodo o activar otro para el mismo local le corta el acceso al instante.
- **Código de activación:**
  - 8 caracteres de un alfabeto sin confundibles (31⁸ combinaciones).
  - Vale 30 minutos y se usa una sola vez; solo hay uno vigente por local.
  - En la base se guarda solo su hash.
  - Se bloquea la IP 15 minutos tras 10 códigos erróneos.
- **Canje idempotente:** el nodo guarda su identidad pendiente junto al hash del código. Si la respuesta se pierde, el reintento usa la misma identidad y la nube responde lo mismo, sin gastar el código.
- **Un solo nodo ACTIVO por local:** hay un índice único parcial y la activación revoca al anterior en la misma transacción. Así nunca hay dos nodos emitiendo con el mismo punto de emisión.

## Alternativas consideradas
| Opción | Motivo del descarte |
|---|---|
| La nube genera la llave y la entrega | La llave privada sale de la PC y viaja por la red |
| mTLS con certificado de cliente | Requiere la CA interna (F2-07, que depende de DP-10) y complica los proxies; se puede sumar después sin cambiar la activación |
| Token de larga duración (API key) | No caduca: si se filtra, sirve hasta que alguien lo revoque |

## Consecuencias
- Reactivar un nodo revocado genera una identidad nueva.
- El bloqueo de activación va por IP. Detrás de un proxy inverso (VM con Caddy/Nginx), la API debe leer la IP real del cliente desde un proxy de confianza. Queda pendiente para el despliegue, y lo mismo aplica al bloqueo de login.
