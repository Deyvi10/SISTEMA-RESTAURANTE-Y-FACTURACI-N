# Usuarios de prueba

Todos son **ficticios** y solo existen en tu entorno local (la demo «Cevichería Don Pepe» y el nodo
efímero de las pruebas E2E). No sirven en producción.

## 1. Levantar el entorno

```sh
make dev     # Postgres, correo (Mailpit), simuladores
make api     # API de la nube en http://localhost:8080
make demo    # crea la demo «Cevichería Don Pepe» (solo la primera vez)
make bo      # backoffice en http://localhost:5173
make pos     # compila la caja dentro del nodo
make nodo    # Nodo Local en http://localhost:7080 (caja en /pos/)
```

Si un PIN deja de funcionar: `go run ./apps/cloud-api/cmd/cloud-api demo -pines` (los repone).

## 2. Backoffice (panel web del dueño)

| Dónde | Usuario | Contraseña |
|---|---|---|
| http://localhost:5173 | `demo@donpepe.ec` | `DonPepe2026` |

## 3. Caja (http://localhost:7080/pos/) y app de meseros

Se elige la persona en la pantalla y se escribe el PIN.

| Persona | Rol | PIN | Qué puede probar |
|---|---|---|---|
| Pepe Andrade | Dueño (ADMIN) | `4826` | Todo: cobrar, dividir, descuentos sin tope, abrir cajón, cerrar turno. También autoriza como **supervisor** (anulaciones, cortesías, descuentos grandes) |
| Luis P. | Caja principal (Cajero) | `7391` | En la computadora principal: toma pedidos de mesas y mostrador (con observaciones por plato) y cobra; abre y cierra turno, pago mixto, dividir cuentas, retiros/ingresos/gastos |
| Carlos M. | Mesero | `8899` | Tomar pedidos en la app de meseros, pre-cuenta, mover/unir mesas |
| Ana R. | Mesera | `1024` | Igual que Carlos (sirve para probar dos meseros a la vez) |
| María C. | Cocina | `5821` | Existe en la demo; su pantalla (KDS) llega en una fase posterior |

Correos que envía el sistema (Cierre Z con PDF, alertas): **Mailpit** en http://localhost:8025.

## 4. Nodo efímero de las pruebas Cypress (`make e2e`)

Solo existe mientras corre la suite (caja en http://127.0.0.1:7181/pos/, se borra al terminar).

| Persona | Rol | PIN |
|---|---|---|
| Eva Caja | Cajera | `1357` |
| Sol Admin | Administrador (supervisor) | `2468` |
| Carlos Mesa | Mesero | `8899` |

Para abrirlo a mano y probarlo en el navegador: `go run ./apps/edge-node/cmd/nodo-e2e`
(ver `e2e/README.md`).

## 5. Recorrido sugerido en la caja

1. Entra como **Luis P.** (`7391`) → pestaña **Turno** → *Abrir turno* con $50.
2. Toca una mesa libre para tomar el pedido desde la misma PC (o envíalo desde la app de meseros).
   Cada plato tiene su campo de observaciones («sin cebolla», «poco picante»).
3. **Mesas** → toca la mesa → cobra con un billete (teclas `1`–`4`), `M` para pago mixto,
   `V` para dividir la cuenta, `D` para descuento (si supera el límite pide el PIN de Pepe).
   Al cobrar, el ticket del cliente sale en la impresora de la caja (Panel › Impresoras ›
   «Impresora de la caja»; con `make dev` usa «Caja (simulador)», puerto 9102, y se ve en
   http://localhost:8090).
4. **Turno** → *Cerrar turno*: cuenta billetes y monedas y declara las tarjetas → Cierre Z.

Pulsa `?` en la caja para ver todos los atajos de teclado.
