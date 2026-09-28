/// <reference types="vitest/config" />
import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// La caja se compila dentro del binario del nodo (go:embed) y se sirve en /pos/.
// En desarrollo el nodo corre en :7080 (`make nodo`); el proxy comparte origen como en producción.
const nodo = process.env.NODO_URL ?? "http://127.0.0.1:7080";
const salida = fileURLToPath(new URL("../edge-node/internal/web/pos", import.meta.url));

export default defineConfig({
  base: "/pos/",
  plugins: [
    react(),
    {
      // vite vacía la carpeta al compilar: se repone el .gitkeep que permite compilar el nodo sin la caja.
      name: "gitkeep",
      closeBundle: () => writeFileSync(`${salida}/.gitkeep`, ""),
    },
  ],
  build: { outDir: salida, emptyOutDir: true },
  server: {
    port: 5174,
    proxy: { "/v1": { target: nodo, ws: true }, "/media": nodo },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    css: false,
  },
});
