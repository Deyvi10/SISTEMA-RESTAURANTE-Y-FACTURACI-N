// Importes en centavos enteros: se suman y comparan sin coma flotante.

/** "0.25" → 25. Los importes se suman en centavos enteros, nunca en coma flotante. */
export function centavos(valor: string): number {
  const [ent = "0", dec = ""] = valor.split(".");
  return Number(ent) * 100 + Number((dec + "00").slice(0, 2));
}

export function verCentavos(c: number): string {
  return `${Math.floor(c / 100)}.${String(c % 100).padStart(2, "0")}`;
}

/** Importe escrito por el cajero: hasta 6 enteros y 2 decimales, sin signo. */
export const montoValido = (s: string) => /^\d{1,6}(\.\d{1,2})?$/.test(s.trim());

/**
 * Reparte centavos por pesos con el método del mayor residuo; los empates van a las últimas
 * partes («ajuste en la última cuenta»). Es money.AllocateUltimo del nodo: mismos casos en
 * packages/testdata/reparto-centavos.json. Todo en enteros.
 */
export function repartirCentavos(total: number, pesos: number[]): number[] {
  const sumaPesos = pesos.reduce((a, b) => a + b, 0);
  if (pesos.length === 0 || sumaPesos <= 0) throw new Error("pesos inválidos");
  // Cuota exacta = total × peso / Σpesos, como fracción entera (cociente y resto).
  const partes = pesos.map((p, i) => ({ i, base: Math.floor((total * p) / sumaPesos), resto: (total * p) % sumaPesos }));
  let sobra = total - partes.reduce((t, x) => t + x.base, 0);
  const orden = [...partes].sort((a, b) => b.resto - a.resto || b.i - a.i);
  for (let k = 0; sobra > 0; k = (k + 1) % orden.length, sobra--) orden[k]!.base++;
  return partes.map((x) => x.base);
}
