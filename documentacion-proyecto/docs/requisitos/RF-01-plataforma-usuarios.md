# RF-01 · Plataforma: tenants, usuarios, roles y dispositivos

## RF-01-01 · Alta de tenant por Super Admin
- **Prioridad:** M · **Fase:** 1
- **Descripción:** El Super Admin crea un restaurante ingresando RUC, razón social, correo y teléfono del dueño y el plan.
- **Criterios de aceptación:**
  1. Se valida el RUC (13 dígitos, que terminen en `001`, dígito verificador según el tipo de contribuyente) antes de guardar.
  2. Se crea el `tenant`, un `local` por defecto (establecimiento `001`) y el usuario Admin.
  3. Se cargan datos semilla: categorías ("Entradas", "Platos fuertes", "Bebidas"), 1 estación "Cocina", 1 mesa de ejemplo.
  4. Se genera una contraseña temporal aleatoria (≥ 12 caracteres) y se envía por correo. En el primer inicio de sesión se **obliga** a cambiarla.
  5. No puede existir otro tenant activo con el mismo RUC.

## RF-01-02 · Inicio de sesión web (Admin y Cajero)
- **Prioridad:** M · **Fase:** 1
- **Criterios de aceptación:**
  1. El usuario inicia sesión con correo (o RUC para el dueño) y contraseña. Las contraseñas se guardan con **Argon2id**.
  2. Tras 5 intentos fallidos la cuenta se bloquea 15 minutos (por usuario + IP).
  3. Existe recuperación de contraseña por correo con token de un solo uso válido 30 minutos.
  4. 2FA (TOTP) opcional para el Admin (**S**, obligatorio en el plan Pro).
  5. La sesión usa un access token corto + un refresh token rotativo en cookie `HttpOnly; Secure; SameSite=Strict`.

## RF-01-03 · Gestión de locales (sucursales)
- **Prioridad:** M (1 local) / S (multi-local, plan Pro) · **Fase:** 1 / 11
- **Criterios de aceptación:**
  1. Cada local tiene nombre, dirección, código de establecimiento SRI (`001`, `002`…), zona horaria y configuración de propina legal.
  2. Cada local tiene exactamente **un** Nodo Local activo.

## RF-01-04 · Gestión de personal en 10 segundos
- **Prioridad:** M · **Fase:** 1
- **Descripción:** El Admin crea un mesero con solo **nombre** y **PIN**.
- **Criterios de aceptación:**
  1. Campos obligatorios: nombre para mostrar, rol y PIN (4 a 6 dígitos). Opcionales: foto y correo. El correo es obligatorio solo para Admin y Cajero con acceso web.
  2. El PIN no puede repetirse dentro del mismo local (evita ambigüedad y colisiones).
  3. Se rechazan PINs triviales (`0000`, `1234`, `1111`…).
  4. Al **desactivar** un usuario, en menos de 2 s desaparece de la pantalla de inicio de todos los dispositivos del local conectados (evento WebSocket) y sus sesiones se revocan.
  5. Los usuarios no se borran físicamente (se conservan por la auditoría); solo se desactivan.

## RF-01-05 · Emparejamiento de dispositivos por QR (Device Binding)
- **Prioridad:** M · **Fase:** 3
- **Criterios de aceptación:**
  1. El Admin genera desde el backoffice (o desde el Nodo Local) un **QR de emparejamiento de un solo uso**, válido 10 minutos.
  2. La app escanea el QR, genera un **par de llaves en el Keystore/Keychain** del dispositivo y envía la llave pública. El servidor registra el dispositivo con un nombre editable ("Tablet Barra 1").
  3. El dispositivo queda ligado a **un** local. Para cambiarlo de local hay que desemparejarlo.
  4. El Admin puede **revocar** un dispositivo; la revocación es efectiva en menos de 2 s si está conectado y en su siguiente conexión si no lo está.
  5. Un dispositivo no emparejado o revocado **no** muestra la lista de personal.

## RF-01-06 · Inicio de sesión por PIN en la app
- **Prioridad:** M · **Fase:** 3
- **Criterios de aceptación:**
  1. La pantalla inicial muestra una cuadrícula con foto y nombre de los usuarios activos con rol Mesero o Cajero del local.
  2. Al tocar un usuario aparece un teclado numérico grande. El PIN se valida **en el Nodo Local** (o en la nube si el dispositivo está fuera del local y el plan lo permite), nunca contra un hash guardado en el teléfono.
  3. Tras 5 PINs erróneos, ese usuario queda bloqueado 5 minutos **en ese dispositivo** y se registra un evento de auditoría.
  4. Al bloquear o apagar la pantalla, o tras N minutos de inactividad (configurable, por defecto 2), se vuelve a pedir el PIN.
  5. La sesión del usuario dura como máximo su turno (12 h por defecto) y se cierra al cerrar la jornada.
  6. Cambio rápido de usuario en el mismo dispositivo sin perder el estado de las mesas.

## RF-01-07 · Permisos granulares
- **Prioridad:** S · **Fase:** 4
- **Criterios de aceptación:**
  1. Pantalla de interruptores estilo iOS por usuario para las acciones ⚙️ de la matriz RBAC (`requisitos/README.md` §3).
  2. Autorización con PIN de supervisor como alternativa en el momento.
  3. Los cambios de permisos quedan auditados.

## RF-01-08 · Consola de plataforma (Super Admin)
- **Prioridad:** S · **Fase:** 10
- **Criterios de aceptación:**
  1. Listado de tenants con plan, estado de suscripción, versión del Nodo Local, última conexión y cantidad de comprobantes pendientes de autorización.
  2. Suspender o reactivar un tenant. Al suspender **no se bloquea la emisión de comprobantes ya generados** (obligación legal); solo se impide operar nuevas ventas tras un periodo de gracia configurable.
  3. Suplantación ("ver como") con auditoría obligatoria y consentimiento registrado.
