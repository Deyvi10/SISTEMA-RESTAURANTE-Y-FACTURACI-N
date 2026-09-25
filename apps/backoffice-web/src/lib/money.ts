// Dinero exacto en el navegador: aritmética con BigInt escalado, nunca float (regla del proyecto).

const SCALE = 1_000_000n; // 6 decimales, como precio unitario del SRI

/** Convierte "12.50" en 12500000n (6 decimales). Devuelve null si no es un decimal válido. */
export function parseDecimal(s: string): bigint | null {
  const t = s.trim();
  if (!/^\d{1,9}(\.\d{1,6})?$/.test(t)) return null;
  const [ent = "0", frac = ""] = t.split(".");
  return BigInt(ent) * SCALE + BigInt((frac + "000000").slice(0, 6));
}

/** División entera con redondeo half-up para valores no negativos. */
function divRound(a: bigint, b: bigint): bigint {
  return (a * 2n + b) / (2n * b);
}

function centsToString(c: bigint): string {
  return `${c / 100n}.${(c % 100n).toString().padStart(2, "0")}`;
}

export interface Desglose {
  base: string;
  iva: string;
  total: string;
}

/**
 * Mismo cálculo que catalogo.Desglosar en Go: con IVA incluido, base = precio / (1 + t)
 * a 6 decimales y luego a centavos; el IVA es la diferencia, así base + IVA = total exacto.
 */
export function desglosar(precio: string, porcentaje: string, incluyeIva: boolean): Desglose | null {
  const p = parseDecimal(precio);
  const t = parseDecimal(porcentaje);
  if (p === null || t === null) return null;
  const cien = 100n * SCALE;
  let base6: bigint, total6: bigint;
  if (incluyeIva) {
    total6 = p;
    base6 = divRound(p * cien, cien + t);
  } else {
    base6 = p;
    total6 = p + divRound(p * t, cien);
  }
  const base = divRound(base6, 10_000n);
  const total = divRound(total6, 10_000n);
  return { base: centsToString(base), iva: centsToString(total - base), total: centsToString(total) };
}

const fmt = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" });

/** "1250.5" → "$1,250.50" (formato provisional DP-11). Solo presentación. */
export function formatUSD(decimal: string): string {
  const p = parseDecimal(decimal);
  if (p === null) return decimal;
  const cents = divRound(p, 10_000n);
  const entero = fmt.format(Number(cents / 100n)).replace(/\.00$/, "");
  return `${entero}.${(cents % 100n).toString().padStart(2, "0")}`;
}
