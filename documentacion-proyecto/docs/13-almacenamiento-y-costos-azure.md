# 13 · Estudio de almacenamiento y costos en Azure

- **Fecha:** 2026-09-24
- **Escenario:** 10 restaurantes × 300 facturas diarias = **3 000 facturas/día** (91 250/mes, 1 095 000/año, 7 665 000 en 7 años).
- **Precios:** API pública de precios de Azure (`prices.azure.com`), región **East US**, pago por uso, en USD y sin impuestos, consultados el 2026-09-24. Los precios de correo (Azure Communication Services) y de ancho de banda **no** salieron de la API: son referencias que hay que confirmar en la calculadora oficial antes de presupuestar 🔎.
- **Decisión relacionada:** [ADR-0013](adr/0013-nube-azure-y-almacenamiento.md).

## 1. Cuánto pesa una factura (medido, no estimado)

Generamos 300 facturas realistas con el formato real del SRI: `<autorizacion>` con el comprobante firmado en XAdES-BES dentro, un certificado X.509 de 2048 bits, 5 líneas de detalle, propina, pago e información adicional. Después medimos cuánto ocupan comprimidas:

| Forma de guardarla | Por factura | Ahorro |
|---|---|---|
| XML autorizado tal cual | **8,71 KB** | — |
| Comprimido suelto (gzip -9 / zstd -19) | 3,5 KB | 2,5× |
| Paquete diario de 300 comprimido junto (zstd) | 0,77 KB | 11× |
| **Cada factura comprimida con un diccionario zstd entrenado** | **0,83 KB** | **10,5×** |
| RIDE PDF A4 (referencia, sin medir) | ~45 KB | — |

**Por qué el diccionario gana.** Todas las facturas repiten lo mismo: el certificado del emisor, los espacios de nombres de la firma, la razón social, la dirección y los nombres de los platos. Un diccionario de 64 KB por emisor guarda esa parte común **una sola vez**. Así se ahorra casi lo mismo que con el paquete diario, pero cada factura sigue siendo un bloque independiente: para descargar una sola basta con leer sus ~850 bytes (lectura por rango HTTP), sin tener que descomprimir el día entero.

La compresión **no pierde datos**. Al descomprimir se obtiene el XML byte por byte idéntico, así que la firma sigue siendo válida. Antes de archivar, el worker comprueba el SHA-256 del resultado contra el original.

## 2. Qué se guarda y dónde

| Dato | Dónde | Cómo | Retención |
|---|---|---|---|
| XML autorizado (fuente legal) | Azure Blob, contenedor `comprobantes`, **GRS** | Un blob por emisor por día (`<tenant>/<ruc>/<aaaa>/<mm>/<dd>.rpz`) con cada factura comprimida por separado con el diccionario del emisor, más su índice (clave de acceso → posición y largo) en PostgreSQL | **7 años**, con política de inmutabilidad (WORM) en el contenedor. Ciclo de vida: Hot 30 días → Cool → Cold al año |
| Diccionario zstd del emisor | El mismo contenedor inmutable | `dicts/<tenant>/<id>.zdict`, versionado y nunca reemplazado | Mientras exista un comprobante que lo use |
| XML reciente | PostgreSQL (`comprobantes.xml_autorizado`, comprimido por TOAST) | Para reenviar y descargar al instante | 90 días; después solo queda en Blob |
| RIDE PDF | **No se guarda** | Se genera al vuelo desde el XML al descargarlo o enviarlo por correo | — |
| Copia local de comprobantes | Nodo Local (SQLite) | XML autorizado + datos para reimprimir el RIDE sin internet | 90 días |
| Fotos del menú | Azure Blob `media`, **LRS**, Hot | WebP sm/md/lg; el original subido no se guarda | Mientras exista el plato |
| Caché de fotos | Nodo Local (disco) | Copia de las WebP del menú para la caja y la app de meseros sin internet | Se sincroniza con el catálogo |
| Respaldo del nodo | Azure Blob `respaldos`, LRS, Cool | SQLite comprimido y cifrado (AES-256-GCM) | 7 diarios + 4 semanales |
| Respaldo local opcional | USB o segundo disco en la PC | Mismo archivo cifrado | Últimos 7 |
| Base central | Azure Database for PostgreSQL Flexible | Respaldos automáticos del servicio | 7 días de PITR (incluidos) |

**Por qué no Archive.** Parece la opción más barata ($0,00099/GB), pero rehidratar un blob tarda horas, y cada blob tiene un mínimo de 180 días y un costo por operación. Con 6 GB en 7 años, Cold cuesta unos **$0,04/mes**, así que Archive ahorraría centavos a cambio de no poder entregar una factura antigua cuando la pidan.

**Por qué GRS solo para comprobantes.** Guardar los XML fiscales en dos regiones cuesta centavos y protege la obligación legal si toda una región falla. Las fotos y los respaldos del nodo se pueden regenerar, así que van en LRS.

## 3. Almacenamiento que usarías

### Comprobantes XML

| Estrategia | Año 1 | Acumulado a 7 años |
|---|---|---|
| Ingenua: XML + PDF, un archivo por factura | 56 GB | **393 GB** |
| Solo XML crudo | 9,1 GB | 64 GB |
| XML comprimido suelto | 3,6 GB | 25 GB |
| **Recomendada: XML + diccionario, sin PDF** | **0,87 GB** | **6,1 GB** |

La estrategia recomendada ocupa **98 % menos** que la ingenua, y cada factura se sigue pudiendo descargar al instante.

### Todo lo demás

| Dato | Tamaño |
|---|---|
| Fotos del menú (10 × ~80 platos × 3 tamaños WebP) | ~176 MB |
| Respaldos de los 10 nodos (11 copias × ~60 MB) | ~6,4 GB, que rotan y no crecen |
| PostgreSQL: datos operativos (~6 KB por factura con sus líneas, pagos y eventos) + XML de 90 días | ~7 GB en el año 1 y ~45 GB en el año 7 |

## 4. Cuánto pagarías al mes

### Almacenamiento (lo que pediste optimizar)

| Concepto | Año 1 | Año 7 |
|---|---|---|
| Comprobantes (GRS: Hot $0,0458, Cool $0,0334, Cold $0,0081 por GB) | $0,03 | $0,07 |
| Escrituras de comprobantes (10 blobs/día) | < $0,01 | < $0,01 |
| Fotos (Hot LRS $0,0208/GB) | < $0,01 | < $0,01 |
| Respaldos de nodos (Cool LRS $0,0152/GB) | $0,10 | $0,10 |
| **Total Blob Storage** | **≈ $0,15** | **≈ $0,20** |
| *Comparación: estrategia ingenua en Hot LRS* | $1,20 | $9,10 |

**Conclusión honesta:** el almacenamiento casi no cuesta. Lo que pesa en la factura de Azure es la **base de datos** y los **contenedores que están siempre encendidos**.

### Factura completa de Azure

| Servicio | Arranque (1-5 restaurantes) | **Recomendado para 10** |
|---|---|---|
| PostgreSQL Flexible | B1ms: $12,41 | **B2s (2 vCPU, 4 GB): $49,64** |
| Disco de PostgreSQL (32 GB, $0,115/GB) | $3,68 | $3,68 |
| Container Apps: API (0,5 vCPU, 1 GiB) + worker fiscal (0,25 vCPU, 0,5 GiB), siempre encendidos, descontada la cuota gratis mensual | $33,03 | $33,03 |
| Container Apps: solicitudes (~3,3 M/mes, 2 M gratis) | $0,53 | $0,53 |
| Blob Storage (tabla anterior) | $0,15 | $0,15 |
| Key Vault (certificados .p12 y secretos) | $0,30 | $0,30 |
| Log Analytics (5 GB/mes gratis; después $2,30/GB) | $0-5 | $0-5 |
| Correo: ~32 000 envíos/mes (35 % de compradores con email, XML + PDF adjuntos) 🔎 | ~$8,20 | ~$8,20 |
| DNS | $0,50 | $0,50 |
| Backoffice (Static Web Apps, plan gratis) | $0 | $0 |
| Registro de imágenes (GitHub Container Registry en lugar de ACR Basic, que cuesta $5) | $0 | $0 |
| Salida de datos (primeros 100 GB/mes gratis) 🔎 | $0 | $0 |
| **Total mensual** | **≈ $59-64** | **≈ $96-101** |
| **Por restaurante** | — | **≈ $10** |

### Qué hace subir la factura (y cuándo)

| Cambio | Costo extra |
|---|---|
| Alta disponibilidad de PostgreSQL con zona redundante (duplica el servidor) | +$53. No hace falta al inicio: si la nube cae, el Nodo Local sigue facturando y encola |
| Pasar a PostgreSQL B2ms (más de ~25 restaurantes) | +$50 |
| Disco de PostgreSQL de 64 GB (hacia el año 5; o antes, si no se archivan los datos operativos de más de 2 años) | +$3,68 |
| Enviar correo a todos los compradores en lugar del 35 % | +$15 🔎 |
| Menú QR muy visitado: más de 100 GB/mes de fotos | ~$0,087 por GB extra 🔎; ahí conviene poner Front Door o un CDN |

## 5. Cómo se ahorró espacio (resumen)

1. **No guardar PDFs.** El RIDE se genera desde el XML cuando alguien lo pide. Esto solo ahorra el 84 % del espacio.
2. **Diccionario zstd por emisor.** Pasa de 8,7 KB a 0,83 KB por factura y cada factura sigue siendo accesible por separado.
3. **Un blob por emisor por día**, no uno por factura: 3 650 objetos al año en lugar de 2,2 millones, con muchas menos operaciones y metadatos.
4. **Ciclo de vida Hot → Cool → Cold** automático por antigüedad. Archive no se usa.
5. **Fotos solo en WebP optimizado.** El original subido nunca se guarda (F1-13).
6. **XML de más de 90 días fuera de PostgreSQL.** La base guarda solo el índice, así que no crece por los comprobantes.

## 6. Pendientes antes de presupuestar en firme

- 🔎 Confirmar con un contador (F5-01) el plazo exacto de conservación y si el XML comprimido cuenta como conservación del original (se recupera byte por byte).
- 🔎 Confirmar los precios de Communication Services Email y de salida de datos en la calculadora de Azure.
- 🔎 Región: East US es la más barata. Brazil South queda más cerca (menos latencia a Ecuador) pero cuesta ~30-50 % más. Hay que decidirlo junto con la evaluación LOPDP de transferencia internacional (`06` §5).
- Volver a medir el tamaño real de las facturas con el certificado del piloto (F0-08).
