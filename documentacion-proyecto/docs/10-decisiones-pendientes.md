# 10 · Decisiones pendientes (para el Product Owner)

Cada decisión tiene una **recomendación**. Al responderlas, se actualiza la columna *Decisión* y, si aplica, el ADR correspondiente pasa a *Aceptado*.

| ID | Pregunta | Opciones | Recomendación | Bloquea | Decisión |
|---|---|---|---|---|---|
| **DP-01** | ¿Proveedor de nube principal? | AWS · Render + S3 · Azure · Proveedor ecuatoriano | **AWS** (RDS PostgreSQL, S3 con Object Lock, KMS, SES) en una sola región. Render es más simple al inicio, pero no ofrece KMS ni Object Lock, así que igual se dependería de AWS. Evaluar la residencia de datos (LOPDP) | F0 | ✅ **Azure** (2026-09-24, Product Owner): PostgreSQL Flexible, Container Apps, Blob con inmutabilidad, Key Vault. Ver ADR-0013 y `13` |
| **DP-02** | ¿Tamaño y composición del equipo? | 1 dev · 2-3 devs · agencia | Define el calendario (`07` §4). Con 1 dev, priorizar sin concesiones el alcance del MVP | Planificación | ⏳ |
| **DP-03** | ¿Sistemas operativos del Nodo Local en el MVP? | Solo Windows · Windows + Linux | **Solo Windows 10/11** en el MVP (es lo que tienen los restaurantes) | F2 | ⏳ |
| **DP-04** | ¿Hardware y credenciales para los spikes? | — | Conseguir: 1 impresora térmica de red (Epson TM-T20 o Xprinter), 1 cajón de dinero, 1 certificado `.p12` de pruebas + RUC habilitado en el ambiente de pruebas del SRI | F0 | ⏳ |
| **DP-05** | ¿Soportar RIMPE Negocio Popular (notas de venta)? | Sí en el MVP · Más adelante · No | **Más adelante**: el ICP inicial factura | F5 | ⏳ |
| **DP-06** | ¿Pasarela para cobrar suscripciones? | Payphone · Kushki · Datafast · PlacetoPay | Cobro manual hasta tener 10 clientes; luego evaluar Kushki o Payphone según cobro recurrente y comisiones | F10 | ⏳ |
| **DP-07** | Validar con un contador o tributarista: plazo de envío, monto máximo de consumidor final, contenido del RIDE, saltos de secuencial, tratamiento de DEVUELTA/NO AUTORIZADO, anulación vs NC | — | **Contratar la revisión** antes de la Fase 5 (checklist en `05` §14) | F5 | ⏳ |
| **DP-08** | ¿ATS dentro del producto? | Sí · No | **No en el MVP**; Excel para el contador | F8 | ⏳ |
| **DP-09** | ¿El menú QR permite pedir (auto-pedido)? | Solo lectura · Pedido · Pedido + pago | **Solo lectura** en la Fase 9; el auto-pedido es un producto aparte | F9 | ⏳ |
| **DP-10** | Nombre comercial y dominio del producto | — | Definir antes de la Fase 2 (el instalador y los certificados usan el dominio) | F2 | ⏳ |
| **DP-11** | Formato numérico en la UI | `$1.250,50` · `$1,250.50` | `$1,250.50` (punto decimal), el uso más extendido en Ecuador; confirmar con usuarios del piloto | F1 | ⏳ |
| **DP-12** | Framework web: ¿confirmar React? | React · SolidJS · Vue | **React + TS + Vite** (ADR-0003) | F1 | ⏳ |
| **DP-13** | ¿Monorepo? | Monorepo · Multi-repo | **Monorepo** (ADR-0001) | F0 | ✅ Adoptado al iniciar la Fase 0 (2026-09-24); un solo módulo Go (ADR-0010) |
| **DP-14** | Momento de descargar inventario | Al enviar a cocina · Al cobrar | **Al enviar** (refleja el consumo real), configurable | F7 | ⏳ |
| **DP-15** | ¿Precios con IVA incluido por defecto? | Sí · No | **Sí** (lo que ve el cliente es lo que paga) | F1 | ⏳ |
| **DP-16** | ¿Hay más documentación fuente por integrar? | — | Si existe (estudios, mockups, notas), agregarla a `docs/fuentes/` y actualizar la revisión | — | ⏳ |
