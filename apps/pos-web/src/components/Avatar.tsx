// Foto de la persona o sus iniciales sobre un degradado estable por nombre.
const TINTES = ["blue", "green", "orange", "pink", "purple", "red", "teal", "indigo"];

export function iniciales(nombre: string): string {
  const p = nombre.split(/\s+/).filter(Boolean);
  return ((p[0]?.[0] ?? "?") + (p[1]?.[0] ?? "")).toUpperCase();
}

export function tintePorNombre(nombre: string): string {
  let h = 0;
  for (const c of nombre) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return TINTES[h % TINTES.length]!;
}

export function Avatar({ nombre, url, tamano = 44 }: { nombre: string; url?: string | null; tamano?: number }) {
  return url ? (
    <img className="avatar" src={url} alt="" width={tamano} height={tamano} />
  ) : (
    <span className="avatar" aria-hidden="true" style={{ width: tamano, height: tamano, fontSize: tamano * 0.38, ["--tint" as string]: `var(--rp-tint-${tintePorNombre(nombre)})` }}>
      {iniciales(nombre)}
    </span>
  );
}
