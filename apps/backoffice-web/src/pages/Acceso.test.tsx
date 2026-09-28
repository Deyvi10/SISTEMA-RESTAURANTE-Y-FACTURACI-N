import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SessionProvider } from "../api/session";
import { Login } from "./Acceso";

afterEach(() => vi.restoreAllMocks());

function renderLogin() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter><SessionProvider><Login /></SessionProvider></MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("inicio de sesión", () => {
  it("muestra el mensaje de la API cuando las credenciales no coinciden", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (url) =>
      String(url).includes("/login")
        ? new Response(JSON.stringify({ code: "CREDENCIALES_INVALIDAS", detail: "El correo o la contraseña no coinciden. Revisa e intenta de nuevo." }), { status: 401 })
        : new Response(null, { status: 401 }),
    );
    renderLogin();
    const user = userEvent.setup();
    const entrar = screen.getByRole("button", { name: "Entrar" });
    expect(entrar).toBeDisabled();
    await user.type(screen.getByLabelText("Correo o RUC"), "pepe@donpepe.ec");
    await user.type(screen.getByLabelText("Contraseña"), "incorrecta1");
    await user.click(entrar);
    expect(await screen.findByRole("alert")).toHaveTextContent("no coinciden");
  });

  it("permite ver la contraseña escrita", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(null, { status: 401 }));
    renderLogin();
    const user = userEvent.setup();
    const campo = screen.getByLabelText("Contraseña");
    expect(campo).toHaveAttribute("type", "password");
    await user.click(screen.getByRole("button", { name: "Mostrar contraseña" }));
    expect(campo).toHaveAttribute("type", "text");
  });
});
