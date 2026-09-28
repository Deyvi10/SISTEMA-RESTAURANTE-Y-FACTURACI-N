import { describe, expect, it } from "vitest";
import vectores from "../../../../packages/testdata/busqueda-productos.json";
import { IndiceBusqueda, normalizar } from "../lib/busqueda";

type Item = { nombre: string; alias: string; vendidos: number };

describe("búsqueda predictiva (vectores compartidos con la app de meseros)", () => {
  const idx = new IndiceBusqueda<Item>(vectores.menu, { nombre: (p) => p.nombre, alias: (p) => p.alias, vendidos: (p) => p.vendidos });

  it.each(vectores.casos.map((c) => [c.consulta, c.resultado] as const))("«%s»", (consulta, resultado) => {
    expect(idx.buscar(consulta).map((p) => p.nombre)).toEqual(resultado);
  });

  it("normaliza tildes, eñes y signos", () => {
    expect(normalizar("Ñoquis, AL pésto!")).toBe("noquis  al pesto ");
  });

  it("p95 ≤ 50 ms por pulsación con 500 productos", () => {
    const grande = Array.from({ length: 500 }, (_, i) => ({ nombre: `Plato número ${i} de la casa ${i % 7 === 0 ? "ceviche" : "seco"}`, alias: `P${i}`, vendidos: i }));
    const g = new IndiceBusqueda<Item>(grande, { nombre: (p) => p.nombre, alias: (p) => p.alias, vendidos: (p) => p.vendidos });
    const tiempos: number[] = [];
    for (const q of ["c", "ce", "cev", "cevi", "ceviche", "s", "se", "sec", "plato 4", "casa sec", "p12"]) {
      for (let r = 0; r < 20; r++) {
        const t0 = performance.now();
        g.buscar(q);
        tiempos.push(performance.now() - t0);
      }
    }
    tiempos.sort((a, b) => a - b);
    expect(tiempos[Math.floor(tiempos.length * 0.95)]).toBeLessThan(50);
  });
});
