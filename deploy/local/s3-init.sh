#!/bin/sh
# Crea los buckets locales. «comprobantes» lleva Object Lock en modo COMPLIANCE:
# ni siquiera el administrador puede borrar un XML autorizado antes de tiempo (L-13).
set -eu
S3="aws --endpoint-url http://s3:7070"
until $S3 s3api list-buckets >/dev/null 2>&1; do sleep 1; done
crear() { b="$1"; shift; $S3 s3api head-bucket --bucket "$b" >/dev/null 2>&1 || $S3 s3api create-bucket --bucket "$b" "$@"; }
crear comprobantes --object-lock-enabled-for-bucket
# En local la retención es de 1 día para poder limpiar; en producción es ≥ 7 años.
$S3 s3api put-object-lock-configuration --bucket comprobantes \
  --object-lock-configuration '{"ObjectLockEnabled":"Enabled","Rule":{"DefaultRetention":{"Mode":"COMPLIANCE","Days":1}}}'
crear imagenes
crear respaldos
echo "Buckets listos: comprobantes (Object Lock), imagenes, respaldos"
