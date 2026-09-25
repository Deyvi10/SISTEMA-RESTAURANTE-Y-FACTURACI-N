# edge-node

Nodo Local: servidor de la LAN en Go + SQLite WAL. Opera sin internet, asigna secuenciales SRI, imprime y sincroniza con la nube.

```bash
make nodo       # primer plano en http://localhost:7080, datos en .nodo-data/
make nodo-win   # dist/restpos-nodo.exe para Windows x64
```

En la PC del restaurante (como administrador):

```bat
restpos-nodo.exe install   :: registra el servicio «RestPOS · Nodo Local» (arranque automático, reinicio ante fallos)
restpos-nodo.exe start
restpos-nodo.exe status
```

| Variable | Por defecto | Uso |
|---|---|---|
| `RESTPOS_DATA` | `%ProgramData%\RestPOS\Nodo` (Linux: `/var/lib/restpos-nodo`) | Base `nodo.db`, `logs/`, caché de fotos |
| `RESTPOS_HTTP` | `:7080` | API de la LAN, WebSocket y caja web |
| `RESTPOS_NUBE` | `http://localhost:8080` | API de la nube |

## Diseño (docs/03 §4)

- `internal/store`:
  - Una conexión de escritura (todas las escrituras en serie, `BEGIN IMMEDIATE`) y un pool de lectura en `query_only`.
  - Pragmas `WAL`, `synchronous=FULL`, `foreign_keys=ON` y `busy_timeout=5000`.
  - Migraciones en `db/edge/migrations`, aplicadas al arrancar.
  - Una prueba mata el proceso con `kill -9` en plena escritura y verifica que no quedan transacciones a medias.
- `internal/app`:
  - Servidor HTTP con apagado ordenado.
  - Watchdog: si la base no responde en 60 s, el proceso sale y el servicio del sistema operativo lo reinicia.
  - Log JSON en `logs/nodo.log`, rotado a los 20 MB (se guardan 3 archivos).
