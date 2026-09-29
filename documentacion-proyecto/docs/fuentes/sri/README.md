# Normativa oficial del SRI (F0-14)

Fuente de verdad del motor fiscal. Los archivos se guardan **tal como se descargaron**, sin
modificar; si el SRI publica una versión nueva se agrega al lado y se anota aquí.

- **Descargado:** 2026-09-29, desde https://www.sri.gob.ec/facturacion-electronica
  (página «Facturación electrónica», enlaces de la biblioteca del SRI).

| Archivo | Qué es | SHA-256 |
|---|---|---|
| `ficha-tecnica-offline-v2.34.pdf` | Ficha Técnica de Comprobantes Electrónicos Esquema Off-line, versión 2.34 (142 págs.) | `7333aebfbdf2cb3ba83f9fc67a7a7f0346ca59506480a260cc42f96dbdfc13c9` |
| `xml-xsd-factura.zip` | XSD y XML de ejemplo de factura 1.0.0, 1.1.0, 2.0.0 y 2.1.0 | `ba1ff0c4e329fe759c3f88dc75f2975780b315b6eb3d0069071b77c1f26fec03` |
| `xml-xsd-nota-credito.zip` | XSD y XML de ejemplo de nota de crédito 1.0.0 y 1.1.0 | `9b7e9c1a240ae39a858aa8fbaf91c41c761417e7bd763ccb20ffc0b81ad1bd43` |
| `xsd-nc-nd-devolucion-iva.zip` | XSD de nota de crédito 1.0.0/1.1.0 y débito 1.0.0 con el campo «Devolución IVA» | `29cf04c7848c27a2f98200582d5595a57b3d7767206cfb04188e373653c55702` |

Nota técnica de la descarga: el sitio no responde por IPv6 ni a clientes sin cabecera de
navegador; con `curl -4 -A "Mozilla/5.0 …"` descarga normal.

## Versión de esquema decidida

**Factura 1.1.0 y nota de crédito 1.1.0.**

- Solo 1.1.0 y 2.1.0 admiten **6 decimales** en `precioUnitario` y `cantidad` (1.0.0 y 2.0.0
  admiten 2; ficha §9.17). El motor de impuestos los necesita: un plato de $15,00 con IVA
  incluido tiene un precio sin IVA de 13,043478 y debe facturar exactamente $15,00.
- 2.1.0 solo agrega, frente a 1.1.0, campos de transporte (guía de remisión sustitutiva) y
  «otros rubros de terceros», que no aplican a un restaurante. Comparado en los XSD oficiales.

## Checklist de `docs/05` §14 contra la ficha

| Punto | Qué dice la ficha v2.34 | Estado |
|---|---|---|
| Ficha y XSD descargados | Esta carpeta | ✅ |
| Versión del esquema | Ver arriba | ✅ factura 1.1.0, NC 1.1.0 |
| Clave de acceso | **49 dígitos**: fecha ddmmaaaa, tipo (Tabla 3), RUC, ambiente (Tabla 4: 1 pruebas, 2 producción), serie 6, secuencial 9, código numérico 8, tipo de emisión (Tabla 2: offline solo 1), dígito módulo 11 con pesos 2..7 (11 → 0, 10 → 1). §5.2 | ✅ coincide con `packages/go/sri`; 18 claves de la ficha como vectores |
| Tipos de comprobante | 01 factura, 03 liquidación, 04 nota de crédito, 05 nota de débito, 06 guía, 07 retención (Tabla 3) | ✅ |
| Identificación del comprador | 04 RUC, 05 cédula, 06 pasaporte, 07 consumidor final (13 nueves), 08 exterior (Tabla 6). NC y ND **no** admiten consumidor final | ✅ |
| Plazo para enviar tras la emisión | La ficha **no** fija las 72 h del documento fuente; solo dice que el SRI procesa en hasta 24 h (§5.12) | 🔎 pendiente: DP-07 (tributarista / resolución vigente) |
| Monto máximo de consumidor final | La v2.22 registra que se «actualizó», pero el texto no da el valor | 🔎 pendiente: DP-07. Hoy es un parámetro (`parametros_globales`, $50 por defecto) |
| Contenido mínimo del RIDE y formato ticket | Anexo 2 (formato RIDE, actualizado en v2.27) y §9.19: el RIDE tiene validez tributaria (Res. 233 de 2018) y admite datos adicionales | 🔎 falta confirmar que el formato ticket de 80 mm cumple el Anexo 2 (F5-05) |
| Saltos de secuencial | §9.18: emitir en orden cronológico y secuencial, sin duplicar secuencia ni clave; no describe cómo tratar un salto | 🔎 pendiente: DP-07 |
| Rechazo y reenvío | §5.10: un comprobante rechazado se corrige y se reenvía **con la misma clave y el mismo secuencial** | ✅ |
| Estados | Recepción: RECIBIDA / DEVUELTA; autorización: PPR (en procesamiento), AUT, NAT; la consulta además da «PENDIENTE DE ANULAR» y «ANULADO» | ✅ |
| Anulación vs nota de crédito | La ficha documenta los estados de anulación y los errores 6000–6005 (p. ej. «la fecha de emisión no corresponde a la de hoy»), pero el procedimiento está en la guía de anulación | 🔎 pendiente: DP-07 |
| Leyendas RIMPE | Anexo 22: etiqueta `<contribuyenteRimpe>` entre `<agenteRetencion>` y `</infoTributaria>`, con «CONTRIBUYENTE RÉGIMEN RIMPE» (27 caracteres) o «CONTRIBUYENTE NEGOCIO POPULAR - RÉGIMEN RIMPE» | ✅ texto; 🔎 qué régimen aplica a cada cliente lo define el onboarding (F5-06) |
| Firma | XAdES-BES v1.3.2, ENVELOPED, UTF-8, **RSA-SHA1**, clave de 2048 bits, PKCS#12; `ds:KeyInfo` con el certificado en base64 y firmado (§6.1–6.8) | ✅ (implementación real requiere el `.p12`, DP-04) |
| Tarifas de IVA | Impuesto IVA = código 2; tarifas: 0 % → 0, 12 % → 2, 14 % → 3, **15 % → 4**, 5 % → 5, no objeto → 6, exento → 7, diferenciado → 8, 13 % → 10 (Tabla 17) | ✅ |
| Totales | Valor total = subtotal + ICE + IRBPNR + IVA + propina; la propina **no puede superar el 10 % del subtotal** (Tabla 21) | ✅ coincide con la caja (F4-10) |
| Decimales | Valores con punto y máximo 2 decimales; precio unitario y cantidad hasta 6 en 1.1.0 (§9.17) | ✅ |
| Tamaño | Envío individual hasta 320 KB por comprobante; lote hasta 500 KB o ~50 comprobantes (§7) | ✅ |
| Servicios web | Pruebas: `https://celcer.sri.gob.ec/comprobantes-electronicos-ws/{RecepcionComprobantesOffline,AutorizacionComprobantesOffline}?wsdl`; producción: mismo camino en `https://cel.sri.gob.ec` (§7.2) | ✅ |
| Revisión por contador o tributarista | — | ⏳ DP-07 |

Hallazgo: dos claves de ejemplo de la ficha tienen el dígito verificador mal (una es de una
consulta RECHAZADA y otra reutiliza el final de otra clave). Recalculado con dos
implementaciones independientes; quedan en `packages/testdata/sri-claves-acceso.json` como
inválidas con la explicación.
