# RF-05 · Facturación electrónica SRI

> La especificación técnica completa está en [`docs/05-facturacion-electronica-sri.md`](../05-facturacion-electronica-sri.md). Aquí se listan los requisitos verificables.

## RF-05-01 · Onboarding fiscal guiado
- **Prioridad:** M · **Fase:** 5
- **Criterios de aceptación:**
  1. Asistente paso a paso: (1) subir el `.p12` + contraseña; (2) el sistema valida el certificado (vigencia, cadena, uso de firma) y **extrae el RUC y el titular**; (3) confirmar la razón social, el nombre comercial y la dirección matriz; (4) confirmar el régimen tributario (General / RIMPE Emprendedor / RIMPE Negocio Popular), contribuyente especial y agente de retención, más la obligación de llevar contabilidad; (5) definir cajas → puntos de emisión; (6) emitir una **factura de prueba en ambiente de pruebas** del SRI.
  2. Si el RUC del certificado no coincide con el RUC del tenant, el proceso se bloquea con un mensaje claro.
  3. Se muestra la fecha de caducidad del certificado y se alerta al dueño **30, 15, 7 y 1 días** antes de que venza.
  4. El P12 y su contraseña se almacenan cifrados según `06-seguridad.md` §4. Nunca se muestran ni se descargan de nuevo.

## RF-05-02 · Emisión de factura (tipo 01)
- **Prioridad:** M · **Fase:** 5
- **Criterios de aceptación:**
  1. Al cobrar una cuenta, el **Nodo Local** asigna el secuencial del punto de emisión de la caja, genera la **clave de acceso de 49 dígitos** y persiste el comprobante en estado `EMITIDO_LOCAL` en la misma transacción que el pago.
  2. Imprime el comprobante (RIDE en formato ticket) **inmediatamente**, con la clave de acceso, el código de barras o QR y la leyenda "Comprobante pendiente de autorización del SRI".
  3. El comprobante entra a la cola de sincronización hacia la nube.
  4. La nube construye el XML según el esquema XSD vigente, lo **valida contra el XSD**, lo firma **XAdES-BES** y lo envía al servicio de **Recepción** del SRI.
  5. Consulta el servicio de **Autorización** hasta obtener `AUTORIZADO` o `NO AUTORIZADO`.
  6. Al autorizarse: genera el RIDE en PDF, guarda el XML autorizado y el PDF en almacenamiento inmutable, y envía ambos por correo al comprador (si tiene correo).
  7. Todo el proceso es **idempotente**: reintentar nunca genera dos comprobantes con distinta clave para la misma cuenta.

## RF-05-03 · Consumidor final
- **Prioridad:** M · **Fase:** 5
- **Criterios de aceptación:**
  1. Identificación `9999999999999`, tipo `07`, razón social "CONSUMIDOR FINAL".
  2. Si el importe total supera el **monto máximo vigente** para consumidor final (parámetro global actualizable, ver DP-07), el sistema exige los datos del comprador.

## RF-05-04 · Reintentos y cola fiscal
- **Prioridad:** M · **Fase:** 5
- **Criterios de aceptación:**
  1. Errores transitorios (timeout, HTTP 5xx, SRI no disponible): reintento con **backoff exponencial con jitter**: 1 min, 2, 5, 15, 30 y luego cada 30 min.
  2. Errores de validación (`DEVUELTA` con mensajes de error): **no** se reintenta a ciegas; el comprobante pasa a `REQUIERE_ATENCION` con el mensaje del SRI traducido a lenguaje claro y una alerta al Admin.
  3. Caso especial: si el SRI responde "clave de acceso en procesamiento" o "registrada", se consulta la autorización en lugar de reenviar.
  4. Alerta al dueño si un comprobante lleva más de **12 h** sin autorización (umbral configurable). Alerta crítica al acercarse el plazo legal de envío.
  5. Panel de "Comprobantes pendientes" con acción manual de reintento.

## RF-05-05 · Leyendas y campos por régimen
- **Prioridad:** M · **Fase:** 5
- **Criterios de aceptación:**
  1. Los campos del XML (`contribuyenteRimpe`, `agenteRetencion`, `contribuyenteEspecial`, `obligadoContabilidad`) y las leyendas del RIDE se derivan de la configuración fiscal del tenant.
  2. Los cambios de régimen tienen fecha de vigencia y no alteran comprobantes ya emitidos.

## RF-05-06 · Impuestos
- **Prioridad:** M · **Fase:** 5
- **Criterios de aceptación:**
  1. Cada producto tiene su tarifa de IVA (15 %, 0 %, 5 %, No objeto, Exento…) desde un **catálogo de tarifas versionado por fecha** (el IVA cambia por ley).
  2. Los precios se configuran **con IVA incluido** (práctica habitual en restaurantes) o sin IVA, a elección del tenant; el sistema desglosa correctamente.
  3. Redondeo según la ficha técnica: precio unitario hasta 6 decimales (según la versión del esquema) y totales a 2 decimales. La suma de las líneas cuadra con el total **al centavo**.
  4. ICE u otros impuestos: fuera de alcance inicial (W).

## RF-05-07 · Bóveda de comprobantes
- **Prioridad:** M · **Fase:** 5
- **Criterios de aceptación:**
  1. Listado filtrable (fecha, estado, cliente, tipo, punto de emisión) con acciones "Descargar XML", "Descargar PDF", "Reenviar correo".
  2. Estados visibles: Emitido local · Enviado · Autorizado · No autorizado · Requiere atención · Anulado por NC.
  3. Los XML autorizados se conservan **mínimo 7 años** en almacenamiento con versionado y bloqueo de borrado (Object Lock).

## RF-05-08 · Nota de crédito (tipo 04)
- **Prioridad:** M · **Fase:** 5
- **Criterios de aceptación:**
  1. Se emite referenciando la factura original (tipo, número y fecha de sustento), total o parcial, con motivo.
  2. El monto de la NC no puede exceder el saldo no revertido de la factura.
  3. Mismo flujo asíncrono de firma, envío y autorización.

## RF-05-09 · Ambiente de pruebas por tenant
- **Prioridad:** M · **Fase:** 5
- **Criterios de aceptación:**
  1. Cada tenant tiene un modo `PRUEBAS` / `PRODUCCION`. En pruebas se usa el ambiente 1 del SRI y los RIDE llevan la marca "AMBIENTE DE PRUEBAS - SIN VALIDEZ TRIBUTARIA".
  2. El paso a producción es una acción explícita del Admin con confirmación, y solo es posible tras al menos 1 comprobante autorizado en pruebas.
  3. Los secuenciales de pruebas y producción son independientes.

## RF-05-10 · Guía de trámites externos
- **Prioridad:** S · **Fase:** 10
- **Criterios de aceptación:**
  1. Módulo de ayuda con guías y videos cortos: obtener la firma electrónica **en archivo** (con enlace a la lista oficial de entidades acreditadas), habilitar la facturación electrónica en *SRI en Línea* y cargar el P12 en el sistema.

## RF-05-11 · Otros comprobantes
- **Prioridad:** C · **Fase:** 11+
- **Descripción:** Notas de venta RIMPE (si se decide en DP-05), comprobantes de retención y liquidaciones de compra quedan para evaluación posterior.
