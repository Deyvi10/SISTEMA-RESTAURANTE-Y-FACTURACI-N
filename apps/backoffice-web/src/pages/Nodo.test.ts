import { describe, expect, it } from "vitest";
import { haceCuanto } from "./Nodo";

describe("haceCuanto", () => {
  const ahora = Date.parse("2026-09-25T12:00:00Z");
  it("expresa el último contacto en palabras", () => {
    expect(haceCuanto(null, ahora)).toBe("nunca");
    expect(haceCuanto("2026-09-25T11:59:30Z", ahora)).toBe("hace un momento");
    expect(haceCuanto("2026-09-25T11:55:00Z", ahora)).toBe("hace 5 min");
    expect(haceCuanto("2026-09-25T09:00:00Z", ahora)).toBe("hace 3 h");
    expect(haceCuanto("2026-09-23T12:00:00Z", ahora)).toBe("hace 2 d");
  });
  it("no muestra tiempos negativos si el reloj del navegador va atrasado", () => {
    expect(haceCuanto("2026-09-25T12:05:00Z", ahora)).toBe("hace un momento");
  });
});
