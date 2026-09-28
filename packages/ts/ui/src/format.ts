/**
 * Formato de presentación para Ecuador (RNF-32). El dinero llega como texto decimal
 * exacto desde la API ("1250.5"); nunca se convierte a number para calcular, solo para mostrar.
 * Separadores según DP-11 (provisional: $1,250.50, confirmar con el piloto).
 */
const moneyFmt = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", minimumFractionDigits: 2 });

/** "1250.5" → "$1,250.50". Lanza si el texto no es un decimal válido. */
export function formatMoney(decimal: string): string {
  if (!/^-?\d+(\.\d+)?$/.test(decimal)) {
    throw new Error(`Importe inválido: ${decimal}`);
  }
  const [int = "0", frac = ""] = decimal.replace("-", "").split(".");
  // Redondeo half-up a 2 decimales sobre el texto, sin pasar por float.
  let cents = BigInt(int) * 100n + BigInt((frac + "00").slice(0, 2));
  if ((frac[2] ?? "0") >= "5") cents += 1n;
  const negative = decimal.startsWith("-") && cents !== 0n;
  const whole = Number(cents / 100n); // la parte entera de un importe de restaurante cabe en number
  const formatted = moneyFmt.format(whole).replace(/\.00$/, "") + "." + (cents % 100n).toString().padStart(2, "0");
  return negative ? "-" + formatted : formatted;
}

const hourFmt = new Intl.DateTimeFormat("es-EC", { hour: "2-digit", minute: "2-digit", hourCycle: "h23", timeZone: "America/Guayaquil" });

/** Hora local del restaurante en 24 h (igual que los tickets impresos), sin importar la zona del dispositivo. */
export function formatHour(date: Date): string {
  return hourFmt.format(date);
}
