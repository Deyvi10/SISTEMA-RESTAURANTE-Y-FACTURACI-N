/// <reference types="vitest/config" />
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// En desarrollo la API corre en :8080 (`go run ./apps/cloud-api/cmd/cloud-api`).
// El proxy hace que backoffice y API compartan origen: la cookie de refresh funciona igual que en producción.
const api = process.env.API_URL ?? "http://localhost:8080";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { "/v1": api, "/media": api },
  },
  test: {
    environment: "jsdom",
    globals: true, // permite a Testing Library limpiar el DOM entre pruebas
    setupFiles: ["./src/test/setup.ts"],
    css: false,
  },
});
