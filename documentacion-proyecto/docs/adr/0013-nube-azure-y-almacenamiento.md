# ADR-0013 · Azure como nube principal y almacenamiento optimizado de comprobantes

- **Estado:** Aceptado
- **Fecha:** 2026-09-24
- **Relacionado con:** DP-01, ADR-0006, ADR-0011, `03-arquitectura.md` §9, [estudio de costos](../13-almacenamiento-y-costos-azure.md)

## Contexto
DP-01 recomendaba AWS. El Product Owner eligió **Azure**. Además pidió que en el Nodo Local se guarde solo lo que vale la pena tener sin internet, y que en la nube todo ocupe lo menos posible. El escenario de referencia son 10 restaurantes con 300 facturas diarias cada uno.

## Decisión
1. **Servicios de Azure:**

   | Necesidad | Servicio |
   |---|---|
   | Base central | Azure Database for PostgreSQL Flexible Server |
   | API y worker fiscal | Azure Container Apps |
   | Objetos | Azure Blob Storage |
   | P12 y secretos | Key Vault |
   | Backoffice y menú QR | Static Web Apps |
   | Correo | Communication Services Email |
   | Logs | Log Analytics |

   La infraestructura se describe con Terraform en `deploy/azure/`. Las imágenes van a GitHub Container Registry.
2. **Comprobantes:**
   - Solo se conserva el **XML autorizado**. El RIDE PDF **no se guarda**: se genera al vuelo.
   - Cada noche el worker escribe **un blob por emisor por día**. Cada factura va comprimida por separado con un **diccionario zstd entrenado por emisor**.
   - PostgreSQL guarda un índice (clave de acceso → posición y largo) para leer una sola factura por rango HTTP.
   - El contenedor tiene una **política de inmutabilidad** (WORM, equivalente a Object Lock) de 7 años, replicación **GRS** y ciclo de vida Hot 30 días → Cool → Cold al año. Archive no se usa.
   - El XML queda además 90 días en PostgreSQL para acceso inmediato.
3. **Otros objetos:** fotos WebP y respaldos cifrados del nodo en contenedores **LRS**.
4. **Nodo Local:**
   - Guarda una copia de los comprobantes autorizados de los últimos 90 días, para reimprimir y consultar sin internet.
   - Guarda una caché de las fotos del menú.
   - Guarda un respaldo cifrado opcional en USB o en un segundo disco, que se suma al de la nube (3-2-1).
5. **Almacenamiento de objetos en el código:**
   - La interfaz `Store` no cambia. Se agrega un adaptador de Azure Blob.
   - En desarrollo se usa **Azurite**. VersityGW (ADR-0011) sigue disponible para el adaptador S3 durante la transición.

## Alternativas consideradas
| Opción | Motivo del descarte |
|---|---|
| AWS (recomendación original de DP-01) | Técnicamente equivalente. El Product Owner prefiere Azure |
| Guardar XML y PDF por factura | Ocuparía ~393 GB en 7 años frente a ~6 GB, con 2,2 millones de objetos al año |
| Comprimir un paquete diario completo | Ahorro parecido (0,77 KB por factura), pero para entregar una sola factura habría que descomprimir el día entero |
| Nivel Archive | Rehidratar tarda horas y cada blob tiene un mínimo de 180 días. El ahorro sería de centavos |
| Guardar los comprobantes solo en la PC del restaurante | Un disco dañado o un robo harían perder la obligación de conservar 7 años |

## Consecuencias
- El almacenamiento cuesta menos de $1/mes incluso a 7 años. El costo real de Azure es la base de datos y los contenedores: ~$96-101/mes para 10 restaurantes.
- El worker necesita un proceso nocturno de archivo, verificación con SHA-256 y reindexado. Si falla, el XML sigue a salvo en PostgreSQL y el proceso se reintenta.
- Los diccionarios forman parte del archivo legal: se guardan inmutables y nunca se reemplazan (una versión nueva recibe un id nuevo).
- Una factura antigua (en Cold) se entrega en milisegundos, a $0,03/GB de lectura.
- La descarga del PDF (F5-12) siempre lo regenera, así que el RIDE A4 debe producir siempre el mismo resultado a partir del XML.
