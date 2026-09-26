#!/bin/sh
# Corre Flutter en Docker (no hace falta tenerlo instalado). La caché de paquetes vive en
# un volumen para no descargar todo en cada corrida. Uso: tools/flutter.sh <dir> <comando…>
set -e
dir="$1"; shift
exec docker run --rm -e PUB_CACHE=/pubcache -v restpos-pubcache:/pubcache -v restpos-gradle:/root/.gradle -v restpos-androidkey:/root/.android -v "$(pwd)":/repo -w "/repo/$dir" \
  ghcr.io/cirruslabs/flutter:stable sh -c "$* ; s=\$?; chown -R $(id -u):$(id -g) /repo/$dir /repo/packages/dart 2>/dev/null; exit \$s"
