import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SesionProvider } from "../api/sesion";
import { AtajosProvider } from "../components/atajos";
import { Personal } from "../pages/Personal";

describe("¿Quién cobra?", () => {
  it("la tecla 1 elige a la primera persona y muestra su teclado", async () => {
    vi.stubGlobal("WebSocket", class { close() {} } as unknown as typeof WebSocket);
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) =>
        new Response(JSON.stringify(url.includes("personal") ? [{ id: "u1", nombre: "Luis P.", rol: "CAJERO", avatarUrl: null, permisos: [] }] : { indicador: "VERDE", nube: "EN_LINEA" }), { status: 200 }),
      ),
    );
    render(
      <SesionProvider pcDelNodo>
        <AtajosProvider>
          <Personal />
        </AtajosProvider>
      </SesionProvider>,
    );
    await screen.findByText("Luis P.");
    await act(async () => {
      fireEvent.keyDown(window, { key: "1" });
    });
    expect(await screen.findByText("Hola, Luis")).toBeInTheDocument();
  });
});
