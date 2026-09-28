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
