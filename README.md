# Sistema Restaurante y Facturación

Punto de venta SaaS para restaurantes en Ecuador: comandas en tiempo real, caja, **facturación electrónica SRI**, inventario por recetas y analítica. La arquitectura **Cloud-Edge** mantiene el salón operando aunque se caiga el internet.

- 📚 Especificación completa: [`documentacion-proyecto/docs/`](documentacion-proyecto/docs/README.md)
- 🎫 Backlog por fases: [`docs/12-backlog-tickets.md`](documentacion-proyecto/docs/12-backlog-tickets.md)

## Empezar

Requisitos: Go 1.27+, Docker con Compose, GNU Make.

```bash
make hooks   # una vez: activa los ganchos de Git
make dev     # levanta Postgres, S3, Mailpit, toxiproxy, impresoras simuladas y stub del SRI
make test    # pruebas con detector de carreras
make help    # todos los comandos
```

| Servicio local | Dirección |
|---|---|
| Impresoras simuladas (panel) | http://localhost:8090 · TCP 9100 cocina, 9101 bar, 9102 caja |
| Stub del SRI | http://localhost:8091 |
| Correo (Mailpit) | http://localhost:8025 |
| PostgreSQL | `localhost:5442` (cambia con `PG_PORT` en `.env`) |
| S3 con Object Lock (VersityGW) | http://localhost:7171 (cambia con `S3_PORT`) |

### Ver el panel del restaurante (Fase 1)

```bash
make dev    # terminal 1: servicios locales
make demo   # una vez: crea «Cevichería Don Pepe» con 22 platos fotografiados, 18 mesas y equipo
make api    # terminal 2: API en :8080
make bo     # terminal 3: panel en http://localhost:5173  →  demo@donpepe.ec / DonPepe2026
```

Prueba rápida de impresión, sin código: `printf 'Hola cocina\n\x1dVA\x03' | nc localhost 9100` y mira el panel.

## Estructura

```
apps/         cloud-api · edge-node · waiter-app · pos-web · backoffice-web · kds-web · menu-web
packages/go/  money · ids · clock · sri · escpos   (dominio puro, probado a fondo)
packages/     ts · dart · testdata (vectores compartidos entre lenguajes)
contracts/    openapi · events
db/           cloud/migrations · edge/migrations
tools/        printer-sim · sri-stub · githooks
deploy/       docker · local
```

## Estado

**Fase 1 en curso** (rama `feat/F0-fundaciones`). Ver el avance en la sección *Estado* de [`CLAUDE.md`](CLAUDE.md).
