// Billetes dinámicos (RF-04-03.3): el monto exacto y los tres pagos más probables con los
// billetes de dólar que circulan en Ecuador. Todo en centavos enteros, nunca en coma flotante.
import { centavos, verCentavos } from "./dinero";

/** Múltiplos con los que se paga: $1, $5, $10, $20, $50 y $100. */
const REDONDEOS = [100, 500, 1000, 2000, 5000, 10000];

/** Para "14.50" → ["14.50", "15.00", "20.00", "50.00"]: primero el exacto. */
export function opcionesBillete(total: string, cuantas = 3): string[] {
  const t = centavos(total);
  const mayores = new Set<number>();
  for (const r of REDONDEOS) {
    const arriba = Math.ceil(t / r) * r;
    if (arriba > t) mayores.add(arriba);
  }
  return [verCentavos(t), ...[...mayores].sort((a, b) => a - b).slice(0, cuantas).map(verCentavos)];
}

/** Vuelto de lo recibido sobre el total (negativo si no alcanza). */
export function vueltoCentavos(total: string, recibido: string): number {
  return centavos(recibido) - centavos(total);
}
