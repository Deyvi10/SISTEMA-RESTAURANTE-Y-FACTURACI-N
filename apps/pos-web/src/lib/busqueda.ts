// Búsqueda predictiva del menú (RF-03-05), la misma regla que la app de meseros
// (apps/waiter-app/lib/core/busqueda.dart): insensible a tildes y mayúsculas, por prefijos de
// palabras («sec pol» → Seco de Pollo), alias exacto primero y, a igual relevancia, lo más
// vendido. Casos compartidos en packages/testdata/busqueda-productos.json.

const TILDES: Record<string, string> = {
  á: "a", à: "a", ä: "a", â: "a", é: "e", è: "e", ë: "e", ê: "e", í: "i", ì: "i", ï: "i", î: "i",
  ó: "o", ò: "o", ö: "o", ô: "o", ú: "u", ù: "u", ü: "u", û: "u", ñ: "n", ç: "c",
};

/** Minúsculas sin tildes ni signos (lo que no es letra o número pasa a espacio). */
export function normalizar(s: string): string {
  let out = "";
  for (const ch of s.toLowerCase()) {
    const m = TILDES[ch] ?? ch;
    out += /^[a-z0-9]$/.test(m) ? m : " ";
  }
  return out;
}

const palabras = (s: string) => s.split(" ").filter((w) => w !== "");

interface Entrada<T> {
  item: T;
  palabras: string[];
  alias: string;
  vendidos: number;
}

/** 0 = no coincide. Cada término debe ser prefijo de alguna palabra (o el alias exacto). */
function puntaje(e: Entrada<unknown>, q: string[]): number {
  const a = e.alias.trim();
  if (q.length === 1 && a !== "" && a === q[0]) return 1000;
  let total = 0;
  for (let i = 0; i < q.length; i++) {
    const t = q[i]!;
    let mejor = 0;
    for (let j = 0; j < e.palabras.length; j++) {
      const w = e.palabras[j]!;
      if (w === t) mejor = Math.max(mejor, 30);
      else if (w.startsWith(t)) mejor = Math.max(mejor, 20 - Math.min(j, 10) + (i === j ? 5 : 0));
      else if (t.length >= 3 && w.includes(t)) mejor = Math.max(mejor, 5);
    }
    if (mejor === 0) return 0;
    total += mejor;
  }
  return total;
}

/** Índice armado una vez al cargar el catálogo; cada pulsación recorre la lista (O(n)). */
export class IndiceBusqueda<T> {
  private readonly entradas: Entrada<T>[];

  constructor(items: Iterable<T>, campos: { nombre: (t: T) => string; alias?: (t: T) => string; vendidos?: (t: T) => number }) {
    this.entradas = [...items].map((item) => ({
      item,
      palabras: palabras(normalizar(campos.nombre(item))),
      alias: normalizar(campos.alias?.(item) ?? ""),
      vendidos: campos.vendidos?.(item) ?? 0,
    }));
  }

  buscar(consulta: string, limite = 40): T[] {
    const q = palabras(normalizar(consulta));
    if (q.length === 0) return [];
    const puntuados: [number, number, T][] = [];
    for (const e of this.entradas) {
      const p = puntaje(e, q);
      if (p > 0) puntuados.push([p, e.vendidos, e.item]);
    }
    puntuados.sort((a, b) => b[0] - a[0] || b[1] - a[1]);
    return puntuados.slice(0, limite).map((x) => x[2]);
  }
}
