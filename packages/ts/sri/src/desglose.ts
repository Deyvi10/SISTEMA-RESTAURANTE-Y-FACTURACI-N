// Desglose fiscal (F5-04) para la vista previa: la misma regla que packages/go/sri/desglose.go,
// verificada con los vectores compartidos de packages/testdata/desglose-sri.json. Aritmética
// exacta con BigInt (centavos, micro-unidades): nunca coma flotante.

export interface LineaVenta {
  cantidad: string; // hasta 6 decimales
  bruto: string; // total de la línea antes de descuentos
  final: string; // total de la línea cobrado
  porcentaje: string; // tarifa de IVA, "15"
  codigoTarifa: string; // Tabla 17, "4"
}

export interface Venta {
  lineas: LineaVenta[];
  porTarifa: Record<string, { base: string; iva: string }>;
  propina: string;
  incluyeIva: boolean;
  total: string;
}

export interface LineaFactura {
  cantidad: string;
  precioUnitario: string;
  descuento: string;
  total: string;
  iva: string;
}

export interface Factura {
  lineas: LineaFactura[];
  totalSinImpuestos: string;
  totalDescuento: string;
  impuestos: { porcentaje: string; codigo: string; base: string; valor: string }[];
  propina: string;
  importeTotal: string;
  ajustes: number;
}

export class ErrorDesglose extends Error {}

/** Decimal en texto → entero escalado a `escala` decimales (exacto o error). */
function escalar(s: string, escala: number): bigint {
  const m = /^(-?)(\d+)(?:\.(\d+))?$/.exec(s.trim());
  if (!m) throw new ErrorDesglose(`número inválido: ${s}`);
  const dec = (m[3] ?? "").replace(/0+$/, "");
  if (dec.length > escala) throw new ErrorDesglose(`${s} tiene más de ${escala} decimales`);
  const v = BigInt(m[2]! + dec.padEnd(escala, "0"));
  return m[1] ? -v : v;
}

/** Entero escalado → texto con `escala` decimales fijos. */
function texto(v: bigint, escala: number): string {
  const neg = v < 0n;
  const a = (neg ? -v : v).toString().padStart(escala + 1, "0");
  const s = escala ? `${a.slice(0, -escala)}.${a.slice(-escala)}` : a;
  return neg ? `-${s}` : s;
}

/** Cociente redondeado a mitad lejos de cero (como shopspring/decimal). */
function dividirRedondeando(n: bigint, d: bigint): bigint {
  if (d < 0n) (n = -n), (d = -d);
  const q = n / d;
  const r = n % d;
  if (2n * (r < 0n ? -r : r) >= d) return n < 0n ? q - 1n : q + 1n;
  return q;
}

const centavos = (s: string) => escalar(s, 2);
const dinero = (c: bigint) => texto(c, 2);

/** Mayor residuo; si todos los pesos son cero, todo a la última (como money.Allocate). */
function repartir(total: bigint, pesos: bigint[]): bigint[] {
  let suma = pesos.reduce((a, b) => a + b, 0n);
  if (suma === 0n) {
    pesos = [...pesos];
    pesos[pesos.length - 1] = 1n;
    suma = 1n;
  }
  const signo = total < 0n ? -1n : 1n;
  const t = total < 0n ? -total : total;
  const partes = pesos.map((w, idx) => ({ idx, base: (t * w) / suma, resto: (t * w) % suma }));
  let falta = t - partes.reduce((a, p) => a + p.base, 0n);
  const orden = [...partes].sort((a, b) => (a.resto === b.resto ? 0 : a.resto > b.resto ? -1 : 1));
  for (let i = 0; falta > 0n; i++, falta--) orden[i % orden.length]!.base++;
  return partes.map((p) => signo * p.base);
}

/** Desglosar reparte los totales cobrados entre las líneas (ver desglose.go). */
export function desglosar(v: Venta): Factura {
  const grupos = new Map<string, number[]>();
  v.lineas.forEach((l, i) => {
    if (escalar(l.cantidad, 6) <= 0n) throw new ErrorDesglose(`la línea ${i + 1} no tiene cantidad`);
    const fin = centavos(l.final);
    if (fin < 0n || fin > centavos(l.bruto)) throw new ErrorDesglose(`la línea ${i + 1} vale más tras descuentos`);
    grupos.set(l.porcentaje, [...(grupos.get(l.porcentaje) ?? []), i]);
  });
  const pcts = [...grupos.keys()].sort();
  if (pcts.length !== Object.keys(v.porTarifa).length) throw new ErrorDesglose("las tarifas de las líneas y de los totales no coinciden");
  const lineas: LineaFactura[] = new Array(v.lineas.length);
  const impuestos: Factura["impuestos"] = [];
  let totalSin = 0n,
    totalDesc = 0n,
    ajustes = 0;
  for (const pct of pcts) {
    const tot = v.porTarifa[pct];
    if (!tot) throw new ErrorDesglose(`falta el total de la tarifa ${pct} %`);
    const pctC = escalar(pct, 2); // tasa = pctC / 10000
    const idx = grupos.get(pct)!;
    const bases = repartir(centavos(tot.base), idx.map((i) => centavos(v.lineas[i]!.final)));
    const ivas = ivasDeLineas(v, idx, bases, centavos(tot.iva), pctC);
    let baseT = 0n,
      ivaT = 0n;
    idx.forEach((i, k) => {
      const l = v.lineas[i]!;
      const base = bases[k]!;
      const iva = ivas[k]!;
      // Bruta: sin descuento sale de la base; con descuento, del bruto (sin IVA si lo incluye).
      let bruta = base;
      if (centavos(l.final) < centavos(l.bruto)) {
        bruta = v.incluyeIva ? dividirRedondeando(centavos(l.bruto) * 10000n, 10000n + pctC) : centavos(l.bruto);
        if (bruta < base) bruta = base;
      }
      const q = escalar(l.cantidad, 6);
      const pu = dividirRedondeando(bruta * 10n ** 10n, q); // micro-unidades
      if (dividirRedondeando(q * pu, 10n ** 10n) !== bruta) throw new ErrorDesglose(`línea ${i + 1}: el precio unitario no reproduce la base`);
      if (iva !== dividirRedondeando(base * pctC, 10000n)) ajustes++;
      lineas[i] = { cantidad: texto(q, 6), precioUnitario: texto(pu, 6), descuento: dinero(bruta - base), total: dinero(base), iva: dinero(iva) };
      baseT += base;
      ivaT += iva;
      totalSin += base;
      totalDesc += bruta - base;
    });
    if (baseT !== centavos(tot.base) || ivaT !== centavos(tot.iva)) throw new ErrorDesglose(`la tarifa ${pct} % no cuadra`);
    impuestos.push({ porcentaje: pct, codigo: v.lineas[idx[0]!]!.codigoTarifa, base: dinero(baseT), valor: dinero(ivaT) });
  }
  const propina = centavos(v.propina);
  if (propina < 0n || propina * 100n > totalSin * 10n) throw new ErrorDesglose("la propina supera el 10 % del subtotal");
  const importe = totalSin + propina + impuestos.reduce((a, i) => a + centavos(i.valor), 0n);
  if (importe !== centavos(v.total)) throw new ErrorDesglose(`el comprobante suma ${dinero(importe)} y se cobró ${v.total}`);
  return { lineas, totalSinImpuestos: dinero(totalSin), totalDescuento: dinero(totalDesc), impuestos, propina: dinero(propina), importeTotal: dinero(importe), ajustes };
}

function ivasDeLineas(v: Venta, idx: number[], bases: bigint[], ivaTarifa: bigint, pctC: bigint): bigint[] {
  if (v.incluyeIva) {
    const ivas = idx.map((i, k) => centavos(v.lineas[i]!.final) - bases[k]!);
    if (ivas.some((x) => x < 0n)) throw new ErrorDesglose("una línea quedaría con IVA negativo");
    if (ivas.reduce((a, b) => a + b, 0n) !== ivaTarifa) throw new ErrorDesglose("el IVA de las líneas no suma el de la tarifa");
    return ivas;
  }
  // exacto_k = base_k × pctC / 10000 centavos; se comparan restos en diezmilésimas de centavo.
  const exactos = bases.map((b) => b * pctC); // en 1/10000 de centavo
  const ivas = exactos.map((e) => dividirRedondeando(e, 10000n));
  let faltan = ivaTarifa - ivas.reduce((a, b) => a + b, 0n);
  while (faltan !== 0n) {
    let mejor = -1;
    let dist = 0n;
    idx.forEach((_, k) => {
      let diff = exactos[k]! - ivas[k]! * 10000n; // > 0: se redondeó hacia abajo
      if (faltan < 0n) {
        diff = -diff;
        if (ivas[k] === 0n) return;
      }
      if (mejor < 0 || diff > dist) (mejor = k), (dist = diff);
    });
    if (mejor < 0) throw new ErrorDesglose("no se puede ajustar el IVA de la tarifa");
    if (faltan > 0n) (ivas[mejor] = ivas[mejor]! + 1n), faltan--;
    else (ivas[mejor] = ivas[mejor]! - 1n), faltan++;
  }
  return ivas;
}
