# ADR-0011 · VersityGW como S3 local de desarrollo

- **Estado:** Aceptado
- **Fecha:** 2026-09-24
- **Relacionado con:** ticket F0-05, hallazgo L-13, `03-arquitectura.md` §11

## Contexto
El entorno local necesita un almacenamiento compatible con S3 **con Object Lock**, para probar desde el inicio que un XML autorizado no se puede borrar. `03` §11 proponía MinIO, pero MinIO dejó de publicar imágenes de contenedor (verificado el 2026-09-24: `minio/minio` y `quay.io/minio/minio` no están disponibles).

## Decisión
Usar **VersityGW** (Apache-2.0) con backend POSIX y directorio de versiones. Verificado en local: crear un bucket con Object Lock en modo COMPLIANCE y luego intentar borrar una versión devuelve `AccessDenied … protected by object lock`, igual que AWS S3.

Esto solo afecta a desarrollo. En la nube se usa S3 del proveedor elegido en DP-01.

## Alternativas consideradas
| Opción | Motivo del descarte |
|---|---|
| MinIO | Sin imágenes publicadas |
| LocalStack | Imagen pesada; el soporte de Object Lock es parcial en la edición gratuita |
| RustFS / SeaweedFS | Viables; VersityGW funcionó a la primera con Object Lock y es más liviano |

## Consecuencias
- Los buckets locales se crean con `deploy/local/s3-init.sh` (comprobantes con Object Lock de 1 día; en producción ≥ 7 años).
- Si VersityGW deja de servir, cualquier S3 compatible con Object Lock lo reemplaza sin cambiar el código.
