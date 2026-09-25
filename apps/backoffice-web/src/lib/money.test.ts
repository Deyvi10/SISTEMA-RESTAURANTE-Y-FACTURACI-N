import { describe, expect, it } from "vitest";
import { desglosar, formatUSD, parseDecimal } from "./money";

describe("dinero exacto", () => {
  it("coincide con el cálculo de la API en Go", () => {
    // Mismos casos que la prueba de la API: 15.00 con IVA 15 % incluido.
    expect(desglosar("15.00", "15.00", true)).toEqual({ base: "13.04", iva: "1.96", total: "15.00" });
    expect(desglosar("12.50", "15.00", true)).toEqual({ base: "10.87", iva: "1.63", total: "12.50" });
    expect(desglosar("11", "15", true)).toEqual({ base: "9.57", iva: "1.43", total: "11.00" });
    expect(desglosar("10.00", "15", false)).toEqual({ base: "10.00", iva: "1.50", total: "11.50" });
    expect(desglosar("3.00", "0", true)).toEqual({ base: "3.00", iva: "0.00", total: "3.00" });
  });

  it("base + IVA siempre suma el total", () => {
    for (let c = 1; c <= 5000; c += 7) {
      const precio = `${Math.floor(c / 100)}.${String(c % 100).padStart(2, "0")}`;
      const d = desglosar(precio, "15.00", true)!;
      const suma = parseDecimal(d.base)! + parseDecimal(d.iva)!;
      expect(suma).toBe(parseDecimal(d.total));
      expect(d.total).toBe(formatUSD(precio).replace("$", "").replace(/,/g, ""));
    }
  });

  it("rechaza formatos que no son decimales con punto", () => {
    for (const s of ["12,50", "abc", "", "-1", "1.1234567", "1e3"]) expect(parseDecimal(s)).toBeNull();
    expect(desglosar("12,50", "15", true)).toBeNull();
  });

  it("formatea en dólares", () => {
    expect(formatUSD("1250.5")).toBe("$1,250.50");
    expect(formatUSD("0.005")).toBe("$0.01");
    expect(formatUSD("2.675")).toBe("$2.68");
  });
});
