import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { montoDe, TecladoMonto, teclearMonto, verMonto } from "../components/TecladoMonto";
import { cajaDeEstaPC, verFecha } from "../pages/Turno";

const escribir = (teclas: string[]) => teclas.reduce(teclearMonto, "");

describe("teclado de importes", () => {
  it("escribe como la calculadora: un punto, dos decimales y sin ceros a la izquierda", () => {
    expect(escribir(["5", "0"])).toBe("50");
    expect(escribir(["0", "0", "7"])).toBe("7");
    expect(escribir([".", "5"])).toBe("0.5");
    expect(escribir(["1", "2", ",", "3", "4", "9"])).toBe("12.34");
    expect(escribir(["1", ".", ".", "2"])).toBe("1.2");
    expect(escribir(["1", "2", "3", "4", "5", "6", "7"])).toBe("123456");
    expect(escribir(["9", ".", "5", "borrar", "borrar"])).toBe("9");
    expect(escribir(["borrar"])).toBe("");
  });

  it("entrega texto decimal al nodo y lo muestra con separador de miles", () => {
    expect(montoDe("")).toBe("0");
    expect(montoDe("12.")).toBe("12");
    expect(montoDe("0.50")).toBe("0.50");
    expect(verMonto("")).toBe("$0");
    expect(verMonto("1250.5")).toBe("$1,250.5");
    expect(verMonto("123456.")).toBe("$123,456.");
  });

  it("acepta el teclado físico, pero no mientras se escribe el motivo", async () => {
    function Prueba() {
      const [v, setV] = useState("");
      return (
        <>
          <TecladoMonto valor={v} cambiar={setV} etiqueta="Monto" />
          <input aria-label="motivo" />
        </>
      );
    }
    render(<Prueba />);
    await act(async () => {
      for (const k of ["2", "0", ",", "5"]) fireEvent.keyDown(window, { key: k });
    });
    expect(screen.getByTestId("monto")).toHaveTextContent("$20.5");
    fireEvent.keyDown(screen.getByLabelText("motivo"), { key: "9" });
    fireEvent.click(screen.getByRole("button", { name: "Borrar" }));
    expect(screen.getByTestId("monto")).toHaveTextContent("$20.");
  });
});

describe("turno de caja", () => {
  it("recuerda la caja de la PC y elige sola si el local tiene una", () => {
    const cajas = [{ id: "a" }, { id: "b" }];
    expect(cajaDeEstaPC(cajas, "b")).toBe("b");
    expect(cajaDeEstaPC(cajas, "borrada")).toBeNull();
    expect(cajaDeEstaPC(cajas, null)).toBeNull();
    expect(cajaDeEstaPC([{ id: "a" }], null)).toBe("a");
  });

  it("la fecha de negocio se lee sin zona horaria", () => {
    expect(verFecha("2026-09-25")).toBe("viernes, 25 de septiembre");
  });
});
