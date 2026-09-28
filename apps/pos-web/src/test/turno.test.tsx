import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { montoDe, TecladoMonto, teclearMonto, verMonto } from "../components/TecladoMonto";
import { AtajosProvider } from "../components/atajos";
import { centavos } from "../lib/dinero";
import { CierreTurno, montoValido, totalContado } from "../pages/CierreTurno";
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

describe("cierre de turno", () => {
  const dens = [
    { clave: "B20", valor: "20", moneda: false, etiqueta: "$20" },
    { clave: "B1", valor: "1", moneda: false, etiqueta: "$1" },
    { clave: "M0.25", valor: "0.25", moneda: true, etiqueta: "25¢" },
    { clave: "M0.01", valor: "0.01", moneda: true, etiqueta: "1¢" },
  ];

  it("suma el conteo en centavos exactos", () => {
    expect(centavos("0.25")).toBe(25);
    expect(centavos("100")).toBe(10000);
    expect(centavos("0.1")).toBe(10);
    expect(totalContado(dens, {})).toBe("0.00");
    expect(totalContado(dens, { B20: 3, B1: 2, "M0.25": 3, "M0.01": 7 })).toBe("62.82");
    // 0.1 + 0.2 en coma flotante no da 0.3; en centavos sí.
    expect(totalContado([{ clave: "a", valor: "0.10", moneda: true, etiqueta: "" }, { clave: "b", valor: "0.20", moneda: true, etiqueta: "" }], { a: 1, b: 1 })).toBe("0.30");
  });

  it("valida los montos declarados", () => {
    expect(["15.40", "0", "12", " 3.5 "].every(montoValido)).toBe(true);
    expect(["15.405", "-2", "abc", "", "1,5"].some(montoValido)).toBe(false);
  });

  it("el contador responde a +, −, flechas y números", () => {
    render(
      <AtajosProvider>
        <CierreTurno cajaId="c" config={{ cajas: [], metodos: [], consumidorFinalMaximo: "50.00", denominaciones: dens }} cerrar={() => {}} listo={() => {}} />
      </AtajosProvider>,
    );
    const fila = screen.getByTestId("den-B20");
    fireEvent.keyDown(fila, { key: "+" });
    fireEvent.keyDown(fila, { key: "ArrowRight" });
    fireEvent.keyDown(fila, { key: "-" });
    expect(fila).toHaveAccessibleName("$20 billete: 1");
    fireEvent.keyDown(fila, { key: "Backspace" });
    fireEvent.keyDown(fila, { key: "1" });
    fireEvent.keyDown(fila, { key: "2" });
    expect(fila).toHaveAccessibleName("$20 billete: 12");
    expect(screen.getByText("$240.00", { selector: "b" })).toBeInTheDocument();
    // Sin métodos distintos del efectivo, el asistente salta ese paso.
    expect(screen.getByText("1 de 3")).toBeInTheDocument();
  });
});
