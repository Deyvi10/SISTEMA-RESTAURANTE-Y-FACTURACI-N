# Caja web (POS)

React + TypeScript + Vite con el mismo sistema de diseño que el backoffice (`@restpos/ui`).
Se compila **dentro del binario del Nodo Local** (`go:embed`) y el nodo la sirve en `/pos/`:
funciona sin internet y se actualiza junto con el nodo (F4-01).

```bash
make pos       # tipos + pruebas + build en apps/edge-node/internal/web/pos (luego compila el nodo)
make pos-dev   # recarga en caliente en http://localhost:5174 contra `make nodo` en :7080
```

- **PC del nodo:** entra directo (el nodo confía en loopback) y pide el PIN.
- **Otra PC de la red:** se empareja con el código de la PC del nodo (Estado › Emparejar). La llave
  Ed25519 se crea con WebCrypto, no se puede exportar y vive en IndexedDB. Los navegadores solo
  permiten WebCrypto en orígenes seguros: hace falta el TLS del nodo (F2-07, DP-10); mientras tanto
  la pantalla lo explica.
- Los tokens de dispositivo y de usuario viven solo en memoria: al recargar se vuelve a pedir el PIN.
- Atajos de teclado: la tecla `?` los muestra; cada pantalla registra los suyos con `useAtajo`.
- Semáforo: 🟢 nodo y nube · 🟡 sin internet (se sigue cobrando) · 🔴 sin conexión con el nodo.
