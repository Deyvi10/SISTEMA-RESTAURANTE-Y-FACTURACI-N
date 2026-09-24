# RF-07 · KDS (pantalla de cocina) y menú QR

## RF-07-01 · KDS básico
- **Prioridad:** S · **Fase:** 9
- **Criterios de aceptación:**
  1. Aplicación web (PWA) servida por el Nodo Local, pensada para una tablet en la estación. Funciona sin internet.
  2. Se empareja a una **estación** (igual que un dispositivo, RF-01-05).
  3. Las comandas aparecen como tarjetas en orden de llegada, con mesa, mesero, tiempo transcurrido, líneas, modificadores y notas.
  4. Colores por tiempo de espera: normal → amarillo (> 10 min) → rojo (> 20 min), con umbrales configurables.
  5. Toque en una tarjeta: **En preparación** → **Listo**. También se puede marcar por línea.
  6. "Listo" dispara la notificación al mesero (RF-03-12).
  7. Deshacer el último cambio de estado (5 s).
  8. Una estación puede tener impresora **y** KDS a la vez.

## RF-07-02 · Métricas de cocina
- **Prioridad:** C · **Fase:** 9
- **Descripción:** Tiempo promedio de preparación por producto y estación, y comandas por hora.

## RF-07-03 · Menú QR "vivo" (solo lectura)
- **Prioridad:** S · **Fase:** 9
- **Criterios de aceptación:**
  1. Web ligera servida desde la nube/CDN (`https://<dominio>/m/<slug-local>`), responsive y con carga inicial ≤ 2 s en 4G.
  2. Muestra categorías, productos con foto, descripción y precio (IVA incluido), alérgenos opcionales y la etiqueta **"Agotado por hoy"** en tiempo real.
  3. La disponibilidad se sincroniza del Nodo Local a la nube (evento `stock.changed`). Si el nodo está sin internet, el menú muestra la última disponibilidad conocida con una nota discreta.
  4. Generador de QR en el backoffice: con logo al centro y colores de marca, exportable en PNG/SVG/PDF de alta resolución para imprimir. El QR apunta al local (y opcionalmente a la mesa).
  5. Personalización: logo, portada, colores y modo oscuro.

## RF-07-04 · Auto-pedido desde el QR
- **Prioridad:** W (por ahora) · **Fase:** 11+
- **Descripción:** El comensal pide y/o paga desde su celular y el pedido convive con el del mesero. Requiere una definición de producto aparte (ver DP-09).
