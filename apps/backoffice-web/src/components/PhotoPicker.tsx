import { type DragEvent, useMemo, useRef, useState } from "react";
import { ApiError } from "../api/client";
import { api, useGaleria } from "../api/hooks";
import type { FotoGaleria } from "../api/types";
import { Icon } from "../lib/icons";
import { Segmented } from "./ui";

export const normalizar = (s: string) => s.normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase();

/** Ordena la galería por parecido con el nombre del plato: «Ceviche mixto» sugiere los ceviches. */
export function sugerir(fotos: FotoGaleria[], nombre: string): { foto: FotoGaleria; puntos: number }[] {
  const palabras = normalizar(nombre).split(/[^a-z0-9ñ]+/).filter((w) => w.length >= 3);
  return fotos
    .map((foto) => {
      const hay = normalizar(`${foto.nombre} ${foto.palabras}`);
      const puntos = palabras.reduce((n, w) => n + (hay.includes(w) ? 2 : hay.includes(w.slice(0, 4)) ? 1 : 0), 0);
      return { foto, puntos };
    })
    .sort((a, b) => b.puntos - a.puntos);
}

export interface FotoElegida {
  key: string;
  url: string;
}

export function PhotoPicker({ value, onChange, nombrePlato }: { value: FotoElegida | null; onChange: (f: FotoElegida | null) => void; nombrePlato: string }) {
  const galeria = useGaleria();
  const [modo, setModo] = useState<"galeria" | "subir">("galeria");
  const [over, setOver] = useState(false);
  const [subiendo, setSubiendo] = useState(false);
  const [error, setError] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const ordenadas = useMemo(() => sugerir(galeria.data ?? [], nombrePlato), [galeria.data, nombrePlato]);

  async function subir(f: File | undefined) {
    if (!f) return;
    setError("");
    if (f.size > 10 * 1024 * 1024) {
      setError("La foto pesa más de 10 MB. Usa una más liviana.");
      return;
    }
    setSubiendo(true);
    try {
      const img = await api.subirFoto(f);
      onChange({ key: img.key, url: img.urls.md });
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo subir la foto.");
    } finally {
      setSubiendo(false);
    }
  }
  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setOver(false);
    void subir(e.dataTransfer.files[0]);
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <div className="foto-actual">
        {value ? <img src={value.url} alt="Foto del plato" /> : (
          <div style={{ textAlign: "center", color: "var(--rp-color-label-secondary)" }}>
            <Icon name="camara" size={36} />
            <p style={{ margin: "6px 0 0" }}>Los platos con foto se venden más</p>
          </div>
        )}
        {value && (
          <div className="acciones">
            <button type="button" className="rp-btn rp-btn--sm rp-btn--gray" style={{ background: "rgba(0,0,0,.55)", color: "#fff" }} onClick={() => onChange(null)}>
              <Icon name="eliminar" size={15} /> Quitar
            </button>
          </div>
        )}
      </div>
      <Segmented label="Origen de la foto" value={modo} onChange={setModo} options={[{ value: "galeria", label: "Galería de platos" }, { value: "subir", label: "Subir mi foto" }]} />
      {modo === "subir" ? (
        <div className="dropzone" data-over={over} role="button" tabIndex={0}
          onClick={() => input.current?.click()} onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && input.current?.click()}
          onDragOver={(e) => { e.preventDefault(); setOver(true); }} onDragLeave={() => setOver(false)} onDrop={onDrop}>
          <Icon name={subiendo ? "subir" : "fotoNueva"} size={32} />
          <b>{subiendo ? "Optimizando tu foto…" : "Arrastra la foto aquí o toca para elegirla"}</b>
          <span style={{ fontSize: 13 }}>JPG, PNG o WebP de hasta 10 MB. La recortamos y optimizamos por ti.</span>
          <input ref={input} type="file" accept="image/jpeg,image/png,image/webp" hidden onChange={(e) => void subir(e.target.files?.[0])} />
        </div>
      ) : (
        <div className="galeria" role="listbox" aria-label="Galería de fotos de platos">
          {ordenadas.map(({ foto, puntos }) => (
            <button key={foto.slug} type="button" role="option" aria-selected={value?.key === foto.key} aria-pressed={value?.key === foto.key}
              onClick={() => onChange({ key: foto.key, url: foto.urls.md })} title={`${foto.nombre} · ${foto.licencia}`}>
              <img src={foto.urls.sm} alt="" loading="lazy" />
              {puntos > 0 && <em className="sugerida">Sugerida</em>}
              <span>{foto.nombre}</span>
            </button>
          ))}
        </div>
      )}
      {error && <p className="error-inline" role="alert">{error}</p>}
    </div>
  );
}
