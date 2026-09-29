import { defineConfig } from "cypress";

// La caja corre contra un nodo efímero (apps/edge-node/cmd/nodo-e2e): `make e2e` lo arranca.
export default defineConfig({
  expose: { aux: process.env.RESTPOS_E2E_AUX_URL ?? "http://127.0.0.1:7182" },
  e2e: {
    baseUrl: process.env.RESTPOS_E2E_URL ?? "http://127.0.0.1:7181",
    supportFile: "cypress/support/e2e.ts",
    viewportWidth: 1366,
    viewportHeight: 860,
    video: false,
    screenshotOnRunFailure: true,
    defaultCommandTimeout: 8000,
  },
});
