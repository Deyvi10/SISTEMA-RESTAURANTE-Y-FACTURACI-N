import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Persona, PermisosPersona } from "../api/types";
import { FeedbackProvider } from "../components/feedback";
import { Permisos } from "./Personal";

afterEach(() => vi.restoreAllMocks());

const luis = { id: "u1", nombreMostrar: "Luis P.", rol: "CAJERO", email: null, esDueno: false, tienePin: true, accesoWeb: false, activo: true, avatarKey: null, avatarUrl: null } as Persona;
const permiso = (permiso: string, nombre: string, concedido: boolean, configurable: boolean) => ({ permiso, nombre, descripcion: "d", concedido, configurable, porDefecto: concedido && !configurable });
const datos = (abrirCajon: boolean): PermisosPersona => ({
  usuarioId: "u1",
  rol: "CAJERO",
  descuentoMaximoPct: null,
  permisos: [permiso("ABRIR_CAJON", "Abrir cajón sin venta", abrirCajon, true), permiso("DAR_DESCUENTO", "Dar descuentos y cortesías", false, true),
    permiso("GESTIONAR_TURNO", "Abrir y cerrar turnos", true, false), permiso("CONFIGURAR_SRI", "Configurar facturación SRI", false, false)],
});

describe("permisos de una persona", () => {
  it("muestra interruptores solo para lo ajustable y guarda al instante", async () => {
    const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async (_url, init) =>
      new Response(JSON.stringify(datos(init?.method === "PUT")), { status: 200, headers: { "Content-Type": "application/json" } }),
    );
    render(
      <QueryClientProvider client={new QueryClient()}>
        <FeedbackProvider>
          <Permisos persona={luis} />
        </FeedbackProvider>
      </QueryClientProvider>,
    );
    const seccion = await screen.findByTestId("permisos");
    const interruptores = within(seccion).getAllByRole("switch");
    expect(interruptores).toHaveLength(2); // abrir cajón y descuentos; SRI y turnos no se tocan
    expect(within(seccion).getByText(/Incluidos con su rol/)).toBeInTheDocument();
    expect(within(seccion).getByText("Abrir y cerrar turnos")).toBeInTheDocument();
    expect(within(seccion).queryByText("Configurar facturación SRI")).not.toBeInTheDocument();

    await userEvent.setup().click(within(seccion).getByRole("switch", { name: /Abrir cajón sin venta/ }));
    const put = fetch.mock.calls.find(([, init]) => init?.method === "PUT");
    expect(String(put?.[0])).toContain("/v1/usuarios/u1/permisos");
    expect(JSON.parse(String(put?.[1]?.body))).toEqual({ permiso: "ABRIR_CAJON", concedido: true });
    expect(await within(seccion).findByRole("switch", { name: /Abrir cajón sin venta/ })).toBeChecked();
  });
});
