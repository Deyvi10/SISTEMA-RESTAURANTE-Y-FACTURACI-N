import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { nodo, type Persona } from "../api/nodo";
import { AtajosProvider } from "../components/atajos";
import { puedenAutorizar, Supervisor } from "../components/Supervisor";

afterEach(() => vi.restoreAllMocks());

const p = (id: string, rol: string, permisos: string[] = []): Persona => ({ id, nombre: id, rol, avatarUrl: null, permisos });

describe("autorización de supervisor", () => {
  it("pueden autorizar el administrador y quien tiene el permiso, nunca uno mismo", () => {
    const gente = [p("Luis", "CAJERO"), p("Sofía", "ADMIN"), p("Marta", "CAJERO", ["ABRIR_CAJON"]), p("Carlos", "MESERO")];
    expect(puedenAutorizar(gente, "ABRIR_CAJON").map((x) => x.id)).toEqual(["Sofía", "Marta"]);
    expect(puedenAutorizar(gente, "ABRIR_CAJON", "Sofía").map((x) => x.id)).toEqual(["Marta"]);
  });

  it("elige al supervisor, escribe su PIN y entrega el token de un solo uso", async () => {
    vi.spyOn(nodo, "personal").mockResolvedValue([p("Luis", "CAJERO"), p("Sofía", "ADMIN")]);
    const autorizar = vi.spyOn(nodo, "autorizar").mockResolvedValue({ token: "tok", expiraAt: "", autorizadoPor: "Sofía" });
    const alAutorizar = vi.fn(async () => null);
    render(
      <AtajosProvider>
        <Supervisor accion="ABRIR_CAJON" referencia="caja-1" excepto="Luis" alAutorizar={alAutorizar} volver={() => {}} />
      </AtajosProvider>,
    );
    fireEvent.click(await screen.findByTestId("supervisor-Sofía"));
    expect(screen.queryByTestId("supervisor-Luis")).not.toBeInTheDocument();
    await act(async () => {
      for (const k of ["5", "1", "7", "3"]) fireEvent.keyDown(window, { key: k });
    });
    expect(autorizar).toHaveBeenCalledWith({ usuarioId: "Sofía", pin: "5173", accion: "ABRIR_CAJON", referencia: "caja-1" });
    expect(alAutorizar).toHaveBeenCalledWith("tok", "Sofía");
  });
});
