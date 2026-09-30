import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ConfigFiscal } from "../api/types";
import { FeedbackProvider } from "../components/feedback";
import { cuerpo, Facturacion, formDe } from "./Facturacion";

afterEach(() => vi.restoreAllMocks());

const base: ConfigFiscal = {
  guardada: false, ambiente: 1, ruc: "1790011674001", razonSocial: "Distribuidora del Pacífico S.A.", nombreComercial: "Don Pepe",
  direccionMatriz: "Quito", obligadoContabilidad: false, contribuyenteEspecial: null, agenteRetencion: null, regimen: "GENERAL",
  facturacionActiva: false, pendientes: ["Confirma los datos del emisor y guárdalos.", "Asigna un punto de emisión a «Caja 1»."],
  cajas: [{ cajaId: "c1", caja: "Caja 1", localId: "l1", puntoId: null, establecimiento: null, puntoEmision: null, conNodo: false }],
};

function montar(cfg: ConfigFiscal) {
  const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async () =>
    new Response(JSON.stringify(cfg), { status: 200, headers: { "Content-Type": "application/json" } }),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <FeedbackProvider>
        <Facturacion />
      </FeedbackProvider>
    </QueryClientProvider>,
  );
  return fetch;
}

describe("facturación SRI", () => {
  it("los opcionales vacíos viajan como null y el ambiente es de pruebas", () => {
    const f = { ...formDe(base), nombreComercial: "  ", contribuyenteEspecial: " 5368 " };
    expect(cuerpo(f, true)).toEqual(expect.objectContaining({ ambiente: 1, nombreComercial: null, contribuyenteEspecial: "5368", agenteRetencion: null, facturacionActiva: true }));
  });

  it("muestra lo pendiente y no deja activar hasta completarlo", async () => {
    const fetch = montar(base);
    expect(await screen.findByTestId("pendientes")).toHaveTextContent("Asigna un punto de emisión a «Caja 1»");
    await userEvent.click(screen.getByRole("switch", { name: /Facturar electrónicamente/ }));
    expect(fetch.mock.calls.filter(([, init]) => init?.method === "PUT")).toHaveLength(0);
  });

  it("asigna la serie de una caja con tres dígitos", async () => {
    const fetch = montar(base);
    const punto = await screen.findByLabelText("Punto de emisión de Caja 1");
    await userEvent.type(punto, "0a02");
    expect(punto).toHaveValue("002");
    await userEvent.click(screen.getByRole("button", { name: "Guardar" }));
    const put = fetch.mock.calls.find(([url, init]) => String(url).includes("/punto-emision") && init?.method === "PUT");
    expect(JSON.parse(String(put?.[1]?.body))).toEqual({ establecimiento: "001", puntoEmision: "002" });
  });
});
