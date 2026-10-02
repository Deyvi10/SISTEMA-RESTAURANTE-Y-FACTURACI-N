import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ResumenVentas } from "../api/types";
import { FeedbackProvider } from "../components/feedback";
import { dinero, periodo, Reportes } from "./Reportes";

vi.mock("../api/session", () => ({ useSession: () => ({ puede: () => true }) }));

afterEach(() => vi.restoreAllMocks());

const resumen: ResumenVentas = {
  desde: "2026-09-29", hasta: "2026-09-30", ticketPromedio: "16.88", neto: "64.50",
  total: { documentos: 4, subtotal: "40.00", iva: "6.00", propina: "4.00", descuento: "0.00", total: "67.50" },
  dias: [
    { fecha: "2026-09-29", documentos: 1, subtotal: "10.00", iva: "1.50", propina: "1.00", descuento: "0.00", total: "12.50" },
    { fecha: "2026-09-30", documentos: 3, subtotal: "30.00", iva: "4.50", propina: "3.00", descuento: "0.00", total: "55.00" },
  ],
  porMetodo: [{ metodo: "Tarjeta crédito", monto: "30.00", pagos: 1 }, { metodo: "Efectivo", monto: "37.50", pagos: 3 }],
  notasCredito: { cantidad: 1, valor: "3.00" },
};

function montar() {
  vi.spyOn(globalThis, "fetch").mockImplementation(async (url) => {
    const u = String(url);
    const body = u.includes("/reportes/ventas") ? resumen : u.includes("/reportes/cierres")
      ? [{ id: "z1", numero: 7, caja: "Caja 1", cajero: "Luis P.", fechaNegocio: "2026-09-30", cerradoAt: "2026-10-01T04:00:00Z", resultado: "FALTANTE", hashValido: true }]
      : u.includes("/alertas/fiscales")
        ? [{ clave: "errores-sri", nivel: "CRITICA", titulo: "1 comprobante que el SRI no aceptó.", accion: "Ábrelo.", enlace: "/comprobantes" }]
        : [];
    return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <FeedbackProvider>
          <Reportes />
        </FeedbackProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("reportes", () => {
  it("periodos rápidos en la fecha local", () => {
    const hoy = new Date(2026, 8, 30, 22, 0);
    expect(periodo("hoy", hoy)).toEqual({ desde: "2026-09-30", hasta: "2026-09-30" });
    expect(periodo("ayer", hoy)).toEqual({ desde: "2026-09-29", hasta: "2026-09-29" });
    expect(periodo("7", hoy)).toEqual({ desde: "2026-09-24", hasta: "2026-09-30" });
    expect(periodo("mes", hoy)).toEqual({ desde: "2026-09-01", hasta: "2026-09-30" });
  });

  it("formato de dinero exacto", () => {
    expect(dinero("1234.5")).toBe("$1,234.50");
    expect(dinero("0")).toBe("$0.00");
    expect(dinero("-3.00")).toBe("−$3.00");
    expect(dinero("1234567.89")).toBe("$1,234,567.89");
  });

  it("muestra totales, métodos, jornadas y cierres", async () => {
    montar();
    expect(await screen.findByTestId("kpis")).toHaveTextContent("$67.50");
    expect(screen.getByTestId("kpis")).toHaveTextContent("4 ventas");
    expect(screen.getByTestId("kpis")).toHaveTextContent("Neto $64.50");
    expect(screen.getByTestId("por-metodo")).toHaveTextContent("Tarjeta crédito");
    expect(screen.getByTestId("por-dia")).toHaveTextContent("$55.00");
    expect(await screen.findByTestId("cierres")).toHaveTextContent("Cierre Z 0007 · Caja 1");
    expect(screen.getByTestId("cierres")).toHaveTextContent("Faltante");
    expect(await screen.findByTestId("alertas-fiscales")).toHaveTextContent("el SRI no aceptó");
  });
});
