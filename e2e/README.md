# e2e

Cypress (caja y backoffice) y k6 (carga). La app Flutter se prueba con integration_test/Patrol, no con Cypress.

## Caja (F4-16)

```sh
make e2e
```

`e2e/correr.sh` compila la caja dentro del nodo, arranca un **nodo efímero**
(`apps/edge-node/cmd/nodo-e2e`) y corre `cypress/e2e/caja.cy.ts` en Chrome. El nodo efímero no
necesita la nube ni Postgres: usa una SQLite temporal ya activada, siembra un restaurante ficticio
(cajera, supervisor y mesero con PIN, 6 mesas, 5 platos sin modificadores obligatorios, «Caja 1»,
Efectivo / Tarjeta crédito / Transferencia) e impresoras térmicas simuladas en memoria. Todo se
borra al salir.

Ayudas de la prueba (por defecto en `http://127.0.0.1:7182`):

| Ruta | Para qué |
|---|---|
| `GET /datos` | IDs y PIN sembrados |
| `POST /ordenes` | El mesero envía una orden desde un teléfono emparejado de verdad (Ed25519 + desafío). No se hace desde el navegador porque la PC de caja tiene una sola sesión de usuario |
| `GET /impresiones` | Texto de cada ticket recibido por impresora (comandas, ventas, Cierre Z) |

La suite recorre abrir turno → cobrar mesa en efectivo → pago mixto → dividir la cuenta (un plato
compartido) → cerrar turno. Registra cada pago desde la respuesta del nodo, declara exactamente eso
en el cierre ciego y exige que el Cierre Z salga **Cuadrado** y que su ticket muestre las mismas
ventas por método. También verifica que cada venta salió como documento interno sin valor
tributario. Selectores solo por `data-testid`.

Sin Chrome instalado, indica otro Chromium: `CYPRESS_NAVEGADOR=/ruta/a/chrome make e2e`.
Puertos: `RESTPOS_E2E_HTTP` (caja, 7181) y `RESTPOS_E2E_AUX` (ayudas, 7182).
