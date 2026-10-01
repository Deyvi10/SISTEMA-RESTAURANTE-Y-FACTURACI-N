import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { FilaComprobante, ListaComprobantes } from "../api/types";
import { FeedbackProvider } from "../components/feedback";
import { Comprobantes, PASOS, pildora, resumenPendiente } from "./Comprobantes";

afterEach(() => vi.restoreAllMocks());

const fila = (x: Partial<FilaComprobante>): FilaComprobante => ({
  id: "c1", tipo: "01", numero: "001-001-000000002", claveAcceso: "1".repeat(49), fechaEmision: "2026-09-30", comprador: "María Pérez",
  identificacion: "1710034065", importeTotal: "18.47", ambiente: 1, estado: "AUTORIZADO", grupo: "AUTORIZADO", serie: "001001",
  tieneCorreo: true, correoEnviado: true, fechaAutorizacion: "2026-09-30T21:57:41Z", ...x,
});

function montar(lista: ListaComprobantes) {
  const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async () =>
    new Response(JSON.stringify(lista), { status: 200, headers: { "Content-Type": "application/json" } }),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <FeedbackProvider>
        <Comprobantes />
      </FeedbackProvider>
    </QueryClientProvider>,
  );
  return fetch;
}

describe("bóveda de comprobantes", () => {
  it("píldoras: verde autorizado, gris enviado, naranja requiere atención", () => {
    expect(pildora({ grupo: "AUTORIZADO", estado: "AUTORIZADO" })).toEqual({ texto: "Autorizado", tono: "success" });
    expect(pildora({ grupo: "ENVIADO", estado: "RECIBIDO" })).toEqual({ texto: "Enviado", tono: "neutral" });
    expect(pildora({ grupo: "ENVIADO", estado: "EN_NUBE" }).texto).toBe("En la nube");
    expect(pildora({ grupo: "REQUIERE_ATENCION", estado: "DEVUELTO" })).toEqual({ texto: "Requiere atención", tono: "warning" });
    expect(pildora({ grupo: "ANULADO", estado: "ANULADO" }).texto).toBe("Anulado por NC");
    expect(PASOS.RECIBIDO).toBe("Recibido por el SRI");
    expect(resumenPendiente("REQUIERE_ATENCION", 1)).toBe("requiere atención");
    expect(resumenPendiente("REQUIERE_ATENCION", 3)).toBe("requieren atención");
  });

  it("muestra lo pendiente, el porqué en lenguaje claro y filtra al tocarlo", async () => {
    const fetch = montar({
      filas: [
        fila({}),
        fila({ id: "c2", numero: "001-001-000000901", estado: "DEVUELTO", grupo: "REQUIERE_ATENCION", fechaAutorizacion: null,
          explicacion: { que: "El XML no pasó la validación de esquema del SRI.", accion: "Contacta al soporte.", soporte: true } }),
      ],
      siguiente: null,
      pendientes: { REQUIERE_ATENCION: 1 },
    });
    expect(await screen.findByTestId("comprobante-001-001-000000901")).toHaveTextContent("validación de esquema");
    expect(screen.getByTestId("comprobante-001-001-000000002")).toHaveTextContent("$18.47");
    await userEvent.click(screen.getByTestId("pendientes-fiscales").querySelector("button")!);
    expect(fetch.mock.calls.some(([url]) => String(url).includes("estado=REQUIERE_ATENCION"))).toBe(true);
  });

  it("busca por cliente al pulsar Intro", async () => {
    const fetch = montar({ filas: [fila({})], siguiente: null, pendientes: {} });
    await userEvent.type(await screen.findByLabelText("Buscar comprobantes"), "María{Enter}");
    expect(fetch.mock.calls.some(([url]) => String(url).includes("q=Mar%C3%ADa"))).toBe(true);
  });

  it("sin comprobantes explica dónde aparecerán", async () => {
    montar({ filas: [], siguiente: null, pendientes: {} });
    expect(await screen.findByText(/Cuando la caja facture/)).toBeInTheDocument();
  });
});
