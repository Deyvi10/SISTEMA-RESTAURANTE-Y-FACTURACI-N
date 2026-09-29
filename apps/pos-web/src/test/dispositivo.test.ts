import { describe, expect, it } from "vitest";
import { CONSULTAS, clasificar } from "../lib/dispositivo";

const con = (...activas: string[]) => (q: string) => activas.includes(q);

describe("detección del equipo", () => {
  it("celular: pantalla angosta (táctil o no)", () => {
    expect(clasificar(con(CONSULTAS.celular, CONSULTAS.tablet, CONSULTAS.tactil))).toEqual({ tipo: "celular", tactil: true });
  });
  it("tableta: mediana y táctil", () => {
    expect(clasificar(con(CONSULTAS.tablet, CONSULTAS.tactil))).toEqual({ tipo: "tablet", tactil: true });
  });
  it("una ventana mediana en la PC con ratón sigue siendo computadora", () => {
    expect(clasificar(con(CONSULTAS.tablet))).toEqual({ tipo: "computadora", tactil: false });
  });
  it("computadora: ancha y con ratón", () => {
    expect(clasificar(con())).toEqual({ tipo: "computadora", tactil: false });
  });
});
