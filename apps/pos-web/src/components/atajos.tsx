// Marco de atajos de teclado (F4-01): cada pantalla registra los suyos y la tecla «?» los muestra.
// Mientras se escribe en un campo solo responden Escape y las teclas de función.
import { createContext, type ReactNode, useContext, useEffect, useMemo, useRef, useState } from "react";

export interface Atajo {
  tecla: string; // «F2», «Enter», «/», «?», «Escape», «1»…
  descripcion: string;
  grupo: string;
  accion: () => void;
}

interface Registro {
  registrar: (a: Atajo) => () => void;
  lista: () => Atajo[];
}

const Ctx = createContext<Registro | null>(null);

/** El nombre visible de una tecla, como en los teclados en español. */
export function nombreTecla(t: string): string {
  return ({ Enter: "Intro", Escape: "Esc", ArrowUp: "↑", ArrowDown: "↓", ArrowLeft: "←", ArrowRight: "→", " ": "Espacio" } as Record<string, string>)[t] ?? t;
}

function escribiendo(e: KeyboardEvent): boolean {
  const el = e.target as HTMLElement | null;
  return !!el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT" || el.isContentEditable);
}

export function AtajosProvider({ children }: { children: ReactNode }) {
  const atajos = useRef(new Set<Atajo>());
  const [ayuda, setAyuda] = useState(false);

  useEffect(() => {
    const alTeclear = (e: KeyboardEvent) => {
      if (e.ctrlKey || e.metaKey || e.altKey || e.repeat) return;
      const funcion = /^F\d{1,2}$/.test(e.key);
      if (escribiendo(e) && !funcion && e.key !== "Escape") return;
      if (e.key === "?") {
        e.preventDefault();
        setAyuda((v) => !v);
        return;
      }
      if (e.key === "Escape" && ayuda) {
        setAyuda(false);
        return;
      }
      // El último registrado gana: una hoja abierta tapa los atajos de la pantalla de atrás.
      const coincide = [...atajos.current].reverse().find((a) => a.tecla.toLowerCase() === e.key.toLowerCase());
      if (coincide) {
        e.preventDefault();
        coincide.accion();
      }
    };
    window.addEventListener("keydown", alTeclear);
    return () => window.removeEventListener("keydown", alTeclear);
  }, [ayuda]);

  const registro = useMemo<Registro>(
    () => ({
      registrar: (a) => {
        atajos.current.add(a);
        return () => void atajos.current.delete(a);
      },
      lista: () => [...atajos.current],
    }),
    [],
  );

  return (
    <Ctx.Provider value={registro}>
      {children}
      {ayuda && <AyudaAtajos atajos={registro.lista()} cerrar={() => setAyuda(false)} />}
    </Ctx.Provider>
  );
}

/** Registra un atajo mientras el componente está montado. */
export function useAtajo(tecla: string, descripcion: string, accion: () => void, grupo = "General", activo = true) {
  const r = useContext(Ctx);
  const fn = useRef(accion);
  fn.current = accion;
  useEffect(() => {
    if (!r || !activo) return;
    return r.registrar({ tecla, descripcion, grupo, accion: () => fn.current() });
  }, [r, tecla, descripcion, grupo, activo]);
}

function AyudaAtajos({ atajos, cerrar }: { atajos: Atajo[]; cerrar: () => void }) {
  const grupos = new Map<string, Atajo[]>();
  for (const a of atajos) grupos.set(a.grupo, [...(grupos.get(a.grupo) ?? []), a]);
  grupos.set("General", [...(grupos.get("General") ?? []), { tecla: "?", descripcion: "Mostrar u ocultar esta ayuda", grupo: "General", accion: () => {} }]);
  return (
    <div className="rp-scrim" onClick={cerrar}>
      <div className="rp-sheet ayuda-atajos" role="dialog" aria-modal="true" aria-labelledby="ayuda-titulo" onClick={(e) => e.stopPropagation()}>
        <div className="rp-sheet__header">
          <h2 className="rp-sheet__title" id="ayuda-titulo">
            Atajos de teclado
          </h2>
          <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={cerrar}>
            Listo
          </button>
        </div>
        <div className="rp-sheet__body">
          {[...grupos].map(([g, lista]) => (
            <section key={g}>
              <h3 className="rp-section-header">{g}</h3>
              <div className="rp-group">
                {lista.map((a) => (
                  <div className="rp-cell" key={a.grupo + a.tecla}>
                    <span className="rp-cell__body">
                      <span className="rp-cell__title">{a.descripcion}</span>
                    </span>
                    <kbd className="tecla">{nombreTecla(a.tecla)}</kbd>
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      </div>
    </div>
  );
}
