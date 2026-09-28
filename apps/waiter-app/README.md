# waiter-app · RestPOS Meseros

App de meseros en Flutter con estilo iOS (Cupertino), pensada para trabajar sin internet contra el Nodo Local (Fase 3).

| Qué | Dónde |
|---|---|
| Emparejar por QR (llave ed25519 en el llavero seguro) | `lib/features/emparejar` |
| Cuadrícula de personal y PIN estilo iPhone | `lib/features/personal`, `lib/widgets/teclado_pin.dart` |
| Salón en vivo con bloqueos | `lib/features/salon` |
| Toma de pedido, modificadores, pre-cuenta, anular, mover/unir | `lib/features/orden` |
| Catálogo offline y cola «Pendiente de envío» (drift) | `lib/data/db.dart`, `lib/core/estado.dart` |
| Búsqueda predictiva sin tildes | `lib/core/busqueda.dart` |

No hace falta tener Flutter instalado: todo corre en Docker.

```bash
tools/flutter.sh apps/waiter-app "flutter test"                 # pruebas
tools/flutter.sh apps/waiter-app "flutter analyze"
tools/flutter.sh apps/waiter-app "flutter build apk --debug"   # APK para instalar en un Android
```

Cómo probar con un teléfono:

1. En la caja (PC del nodo), abre `http://localhost:7080/emparejar`.
2. En la app, escanea el QR. Si la cámara no sirve, toca «Escribir código» y escribe el código y la IP de la caja.
3. Entra con tu PIN.

Por ahora el Nodo Local habla HTTP dentro del WiFi del restaurante; TLS llega con F2-07 (depende del dominio, DP-10). Mientras tanto, las credenciales van firmadas por dispositivo.
