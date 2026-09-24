# RF-09 · SaaS: planes, suscripciones y aprovisionamiento

## RF-09-01 · Planes y feature flags
- **Prioridad:** M · **Fase:** 10
- **Criterios de aceptación:**
  1. Planes definidos por datos (no hardcodeados): Emprendedor, Restaurante, Pro. Cada uno tiene límites (usuarios, dispositivos, impresoras, locales) y funciones habilitadas (recetas, división de cuentas, KDS, multi-local…).
  2. El backend y los clientes consultan un único servicio de **entitlements**; la UI oculta lo no contratado y muestra una invitación a mejorar el plan.
  3. Bajar de plan nunca borra datos; solo deshabilita funciones.

## RF-09-02 · Registro self-service
- **Prioridad:** S · **Fase:** 10
- **Criterios de aceptación:**
  1. Landing → elegir plan → datos (RUC, correo, teléfono) → verificación de correo → pago → tenant aprovisionado automáticamente (RF-01-01).
  2. Periodo de prueba configurable (p. ej. 14 días) sin tarjeta.
  3. Correo de bienvenida con: URL del backoffice, enlace a la app de meseros (tiendas) e instalador del Nodo Local con código de activación.

## RF-09-03 · Cobro recurrente
- **Prioridad:** S · **Fase:** 10
- **Criterios de aceptación:**
  1. Integración con una pasarela que opere en Ecuador (DP-06), con tokenización de tarjeta (el sistema **nunca** almacena el número de tarjeta, lo que reduce el alcance PCI).
  2. Cobro mensual; reintentos ante fallos; periodo de gracia; suspensión automática (con las reglas de RF-01-08).
  3. Emisión automática de la **factura electrónica del SaaS** al restaurante (el SaaS es emisor).

## RF-09-04 · Onboarding asistido del menú
- **Prioridad:** S (importación Excel) / C (IA con foto de la carta) · **Fase:** 10 / 11+
- **Criterios de aceptación:**
  1. Importación de productos, categorías y precios desde una plantilla Excel con validación y previsualización.
  2. (Futuro) Carga de una foto de la carta, extracción de productos y precios con IA y revisión humana antes de guardar.
