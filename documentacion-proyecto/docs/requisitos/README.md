# 02 · Requisitos: índice y convenciones

## 1. Convención de identificadores

| Prefijo | Significado | Ejemplo |
|---|---|---|
| `RF-<mm>-<nn>` | Requisito funcional `nn` del módulo `mm` | `RF-03-04`: notas por plato |
| `RNF-<nn>` | Requisito no funcional | `RNF-01`: latencia de comanda |
| `CU-<nn>` | Caso de uso | `CU-01`: atención completa en salón |
| `HU-<nn>` | Historia de usuario (heredada del documento fuente) | `HU-03`: facturación a 1 clic |

Los IDs **son permanentes**: no se renumeran. Un requisito eliminado se marca como `~~Retirado~~`, no se borra.

Cada requisito indica:

- **Prioridad** (MoSCoW): `M` = Must (imprescindible para su fase), `S` = Should, `C` = Could, `W` = Won't (por ahora).
- **Fase** de implementación (ver `docs/07-plan-de-fases.md`).
- **Criterios de aceptación** verificables, que se convierten en pruebas automatizadas.

## 2. Documentos

| Archivo | Módulo |
|---|---|
| [RF-01-plataforma-usuarios.md](RF-01-plataforma-usuarios.md) | Tenants, locales, usuarios, roles, permisos, dispositivos |
| [RF-02-nodo-local-impresion.md](RF-02-nodo-local-impresion.md) | Nodo Local, impresoras, estaciones, ruteo |
| [RF-03-salon-comandas.md](RF-03-salon-comandas.md) | Mapa de salón, bloqueo de mesas, toma de pedidos |
| [RF-04-caja-pos.md](RF-04-caja-pos.md) | Cobro, pagos, división de cuentas, turnos, cierre ciego |
| [RF-05-facturacion-sri.md](RF-05-facturacion-sri.md) | Comprobantes electrónicos |
| [RF-06-inventario.md](RF-06-inventario.md) | Insumos, recetas, kardex, stock diario, mermas |
| [RF-07-kds-menu-qr.md](RF-07-kds-menu-qr.md) | Pantalla de cocina y menú QR |
| [RF-08-reportes-analitica.md](RF-08-reportes-analitica.md) | Dashboard, cierres, propinas, exportaciones, auditoría |
| [RF-09-saas.md](RF-09-saas.md) | Planes, suscripciones, aprovisionamiento |
| [RNF-no-funcionales.md](RNF-no-funcionales.md) | Rendimiento, disponibilidad, seguridad, usabilidad |
| [casos-de-uso.md](casos-de-uso.md) | Flujos detallados (CU-01..CU-08) |

## 3. Matriz de roles y permisos por defecto (RBAC)

✅ = permitido por defecto · ⚙️ = configurable por el Admin (permiso granular) · ❌ = no permitido

| Acción | Admin | Cajero | Mesero | Cocina |
|---|:-:|:-:|:-:|:-:|
| Abrir mesa / tomar pedido | ✅ | ✅ | ✅ | ❌ |
| Enviar comanda | ✅ | ✅ | ✅ | ❌ |
| Eliminar ítem **no enviado** | ✅ | ✅ | ✅ | ❌ |
| Eliminar/anular ítem **ya enviado** a cocina | ✅ | ⚙️ | ⚙️ | ❌ |
| Transferir mesa / unir mesas | ✅ | ✅ | ⚙️ | ❌ |
| Imprimir pre-cuenta | ✅ | ✅ | ✅ | ❌ |
| Cobrar y facturar | ✅ | ✅ | ⚙️ | ❌ |
| Dividir cuenta | ✅ | ✅ | ⚙️ | ❌ |
| Aplicar descuento / cortesía | ✅ | ⚙️ | ❌ | ❌ |
| Abrir cajón sin venta | ✅ | ⚙️ | ❌ | ❌ |
| Emitir nota de crédito | ✅ | ⚙️ | ❌ | ❌ |
| Abrir / cerrar turno de caja | ✅ | ✅ | ❌ | ❌ |
| Ver totales esperados antes del cierre | ✅ | ❌ | ❌ | ❌ |
| Marcar pedido listo (KDS) | ✅ | ❌ | ❌ | ✅ |
| Recargar stock diario | ✅ | ⚙️ | ❌ | ⚙️ |
| Configurar menú, precios, recetas | ✅ | ❌ | ❌ | ❌ |
| Toma física de inventario / ajustes | ✅ | ❌ | ❌ | ⚙️ |
| Gestionar personal y dispositivos | ✅ | ❌ | ❌ | ❌ |
| Configurar SRI (P12, puntos de emisión) | ✅ | ❌ | ❌ | ❌ |
| Ver reportes y auditoría | ✅ | ⚙️ (solo su turno) | ❌ | ❌ |

Toda acción marcada ⚙️ y toda acción sensible (anulaciones, descuentos, apertura de cajón, recargas de stock, ajustes) **genera un registro en `auditoria`** (ver RF-08-06).

Cuando un usuario no tiene un permiso, la UI ofrece **"Autorizar con PIN de supervisor"**: un Admin o supervisor digita su PIN en el mismo dispositivo y la acción queda auditada con ambos usuarios.
