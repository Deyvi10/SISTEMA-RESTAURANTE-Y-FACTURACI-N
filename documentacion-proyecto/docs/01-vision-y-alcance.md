# 01 · Visión y alcance del producto

| Campo | Valor |
|---|---|
| Producto | Sistema Restaurante y Facturación (nombre comercial por definir, ver DP-10) |
| Tipo | SaaS B2B vertical para gastronomía en Ecuador |
| Versión del documento | 1.0 |
| Fuente | `docs/fuentes/documento-original.md` (normalizado y corregido según `00-revision-documento-fuente.md`) |

## 1. Declaración de visión

> Ser el punto de venta para restaurantes **más rápido y confiable de Ecuador**: un sistema que **nunca se detiene aunque se caiga el internet**, que factura electrónicamente ante el SRI sin hacer esperar al cajero y que el personal aprende a usar en minutos.

## 2. Problema

| Dolor del restaurante | Consecuencia |
|---|---|
| Los sistemas 100 % web se paralizan cuando cae el internet (proveedores como CNT o Netlife), justo en hora pico. | Pedidos a gritos, errores, ventas perdidas y facturación a mano. |
| El SRI se pone lento o colapsa a fin de mes y en feriados, y la caja se queda "pensando". | Filas, clientes molestos, cajero bloqueado. |
| Dos meseros toman la misma mesa. | Platos duplicados, inventario descuadrado. |
| Configurar impresoras requiere técnicos (IPs estáticas). | Costos de soporte y dependencia del proveedor. |
| Inventario manual, sin recetas. | Robos y mermas invisibles, compras a ciegas. |
| Cierres de caja manipulables. | Fugas de dinero. |
| La alta rotación de meseros exige usuarios y claves nuevas cada semana. | Fricción administrativa. |

## 3. Propuesta de valor (diferenciadores)

1. **Operación offline real** mediante el Nodo Local (Edge): las comandas, la impresión, el bloqueo de mesas y el cobro siguen funcionando sin internet.
2. **Facturación SRI asíncrona**: el cajero atiende al siguiente cliente en menos de 2 s; la autorización ocurre en segundo plano con reintentos.
3. **UX "Zero-Click"** estilo iOS: búsqueda predictiva, billetes rápidos, gestos, botones grandes.
4. **Impresión casi sin configuración**: auto-descubrimiento de impresoras y ruteo por estación mediante arrastrar y soltar.
5. **Inventario por recetas (BOM)** con sub-recetas, conversión de unidades, stock diario y kardex inmutable.
6. **Anti-fraude**: cierre ciego, auditoría inmutable y matriz de fugas.
7. **Acceso por PIN** en dispositivos emparejados por QR: un mesero nuevo trabaja en 10 segundos.

## 4. Análisis competitivo (resumen)

| Competidor | Fortaleza | Debilidad que se explota |
|---|---|---|
| Illarli | Facturación SRI nativa, inventario | UX tradicional, configuración de impresoras compleja, depende de la web |
| Restopedia | Mesas y cuentas divididas | Costoso y pesado para locales pequeños |
| Fudo | Mejor UX/UI de LATAM | Facturación EC mediante módulos externos |
| Contífico (Siigo) | Contabilidad | Lento para el cajero |
| MishkiTap | Auto-pedido QR | Falla sin internet |
| GastroEc | Recetas y mermas | Interfaz desactualizada |
| Dora POS | Automatización contable | Complejo para personal rotativo |
| Popapp | Integración con delivery | Débil en salón y cocina |
| OlaClick | Pedidos por WhatsApp, gratuito | Sin inventario profundo ni multi-impresora |
| Zeta Software | Multi-sucursal y costos | Sobredimensionado para el 80 % del mercado |

**Referencia principal a superar: Illarli.** Hay que igualar su estabilidad fiscal y superarlo en velocidad de UX, configuración de impresoras y operación offline.

## 5. Usuarios y roles

| Rol | Dispositivo principal | Objetivo |
|---|---|---|
| **Super Admin** (dueño del SaaS) | Web (consola de plataforma) | Alta de tenants, planes, soporte, salud de los nodos |
| **Administrador / Propietario** | Web (backoffice), móvil | Configurar menú, inventario, impresoras, personal, SRI y ver reportes |
| **Cajero** | Web POS (PC o tablet grande) | Cobrar, dividir cuentas, facturar, abrir y cerrar caja |
| **Mesero** | App Flutter (celular o tablet) | Abrir mesas, tomar pedidos, enviar comandas, pedir pre-cuenta |
| **Cocina / Bar** | Ticket impreso o KDS (tablet) | Preparar y marcar pedidos como listos |
| **Bodeguero** (opcional) | Tablet | Toma física de inventario, producción de sub-recetas |
| **Cliente final** (comensal) | Navegador (menú QR) | Ver el menú vivo con disponibilidad real |

## 6. Alcance

### 6.1 Dentro del alcance (producto completo)

Organizado por **módulo funcional**. El detalle está en `docs/requisitos/`.

| Código | Módulo |
|---|---|
| RF-01 | Plataforma: tenants, usuarios, roles, permisos, dispositivos |
| RF-02 | Nodo Local (Edge) e impresión |
| RF-03 | Salón y comandas (app de meseros) |
| RF-04 | Caja / POS: cobros, cuentas divididas, turnos, cierre ciego |
| RF-05 | Facturación electrónica SRI |
| RF-06 | Inventario, recetas y stock diario |
| RF-07 | KDS y menú QR |
| RF-08 | Reportes, analítica, propinas, auditoría |
| RF-09 | SaaS: planes, suscripciones, aprovisionamiento |

### 6.2 Fuera de alcance (por ahora)

Estos puntos quedan registrados para el futuro y **no** se construyen hasta que se decida explícitamente:

- Contabilidad completa (libro diario, balances). El sistema **exporta** al software contable.
- Nómina y roles de pago (excepto el reporte de reparto de propinas).
- Auto-pedido del comensal desde el QR (el menú QR del MVP es **de solo lectura**, ver DP-09).
- Integración con apps de delivery (UberEats, PedidosYa), bot de WhatsApp y onboarding por IA con foto de la carta. Son candidatos para la fase de expansión (Fase 10+).
- Traslados de inventario entre sucursales (plan Pro, fase posterior).
- Reservas de mesas y programas de fidelización.

## 7. Supuestos y restricciones

| Tipo | Descripción |
|---|---|
| Supuesto | El restaurante tiene al menos una PC (Windows 10+ o Linux) que permanece encendida durante el servicio y funciona como Nodo Local. |
| Supuesto | El restaurante tiene un router WiFi; se recomienda una red separada para el personal (VLAN o SSID aparte). |
| Supuesto | Impresoras térmicas ESC/POS por Ethernet, WiFi o USB (no Bluetooth). |
| Supuesto | El cliente obtiene su firma electrónica **en archivo .p12** (no token USB). |
| Restricción | Moneda USD, zona horaria `America/Guayaquil`, idioma `es-EC`. |
| Restricción | Cumplimiento obligatorio de la normativa SRI vigente y de la LOPDP. |
| Restricción | Android 10+ e iOS 15+ para la app de meseros (validar, ver RNF). |

## 8. Métricas de éxito del producto

| Métrica | Objetivo |
|---|---|
| Tiempo de "Enviar comanda" a ticket impreso | p95 ≤ 1,5 s en red local |
| Tiempo de "Facturar" a caja libre | p95 ≤ 2 s |
| Disponibilidad operativa del salón (con o sin internet) | ≥ 99,9 % en horario de servicio |
| Facturas autorizadas por el SRI dentro de 1 h | ≥ 99 % (fuera de caídas del SRI) |
| Tiempo para dar de alta a un mesero | ≤ 10 s |
| Tiempo de onboarding de un restaurante nuevo | ≤ 1 día hábil (con migración asistida del menú) |
| Retención mensual de clientes (churn) | ≤ 3 % mensual |

## 9. Glosario

| Término | Definición |
|---|---|
| **Tenant** | Un restaurante (o cadena) cliente del SaaS; unidad de aislamiento de datos. |
| **Local / Sucursal** | Ubicación física de un tenant; tiene su propio Nodo Local. |
| **Nodo Local / Edge** | Software en Go instalado en la PC del local que actúa como servidor local. |
| **Nube / Cloud** | Backend central (API Go + PostgreSQL) multi-tenant. |
| **Estación** | Destino de producción (Cocina caliente, Bar, Sushi…) asociado a una o más impresoras o KDS. |
| **Comanda** | Conjunto de líneas de una orden enviadas a producción en un mismo envío. |
| **Orden** | Consumo abierto de una mesa (o para llevar), compuesto por líneas. |
| **Cuenta** | Agrupación de líneas (o fracciones) que se cobran juntas; genera un comprobante. |
| **Pre-cuenta** | Ticket informativo sin valor tributario. |
| **Turno de caja** | Periodo entre la apertura y el cierre de una caja por un cajero. |
| **Jornada operativa** | Día de negocio del local (puede cruzar la medianoche). |
| **Cierre Z** | Reporte inmutable de cierre de turno con cuadre ciego. |
| **Punto de emisión** | Serie `EEE-PPP` del SRI (establecimiento + punto) con secuencial propio. |
| **Clave de acceso** | Identificador de 49 dígitos de un comprobante electrónico, generado por el emisor. |
| **RIDE** | Representación Impresa del Documento Electrónico. |
| **XAdES-BES** | Estándar de firma XML exigido por el SRI. |
| **Kardex** | Libro mayor inmutable de movimientos de inventario. |
| **BOM / Receta** | Lista de insumos (con cantidades) que componen un producto. |
| **Preparación / Sub-receta** | Ítem que se produce con insumos y a su vez es insumo de otras recetas. |
| **KDS** | Kitchen Display System: pantalla de cocina. |
| **RLS** | Row-Level Security de PostgreSQL. |
| **LOPDP** | Ley Orgánica de Protección de Datos Personales (Ecuador). |
