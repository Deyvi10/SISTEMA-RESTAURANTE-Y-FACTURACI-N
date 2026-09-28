import { describe, expect, it } from "vitest";
import vectores from "../../../../packages/testdata/reparto-centavos.json";
import { centavos, repartirCentavos, verCentavos } from "../lib/dinero";

describe("reparto de centavos (mismos casos que el nodo)", () => {
  it.each(vectores.casos.map((c) => [c.total, c.pesos, c.esperado] as const))("%s entre %j", (total, pesos, esperado) => {
    expect(repartirCentavos(centavos(total), [...pesos]).map(verCentavos)).toEqual(esperado);
  });
});
