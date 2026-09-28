// Teclado de importes como la calculadora de iOS: monto en 48 pt, teclas grandes y también el
// teclado físico (dígitos, punto o coma, retroceso). El importe es texto decimal; nunca float.
import { Delete } from "lucide-react";
import { type Dispatch, type SetStateAction, useEffect } from "react";

const MAX_ENTEROS = 6;

/** Aplica una tecla ("0"…"9", ".", "borrar") al importe que se está escribiendo. */
export function teclearMonto(actual: string, tecla: string): string {
  if (tecla === "borrar") return actual.slice(0, -1);
  if (tecla === "." || tecla === ",") return actual.includes(".") ? actual : (actual || "0") + ".";
  if (!/^\d$/.test(tecla)) return actual;
  const [enteros, decimales] = actual.split(".");
  if (decimales !== undefined) return decimales.length >= 2 ? actual : actual + tecla;
  if (enteros === "0") return tecla;
  return (enteros ?? "").length >= MAX_ENTEROS ? actual : actual + tecla;
}

/** El importe listo para el nodo: "" → "0", "12." → "12". */
export function montoDe(actual: string): string {
  return actual === "" ? "0" : actual.replace(/\.$/, "");
}

/** Lo que se ve mientras se escribe: $1,250.5 conserva el punto y los decimales tecleados. */
export function verMonto(actual: string): string {
  const [enteros = "0", decimales] = (actual || "0").split(".");
  const miles = enteros.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  return "$" + miles + (decimales !== undefined ? "." + decimales : "");
}

function escribiendoTexto(e: KeyboardEvent) {
  const t = e.target as HTMLElement | null;
  return !!t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
}

export function TecladoMonto({ valor, cambiar, etiqueta }: { valor: string; cambiar: Dispatch<SetStateAction<string>>; etiqueta: string }) {
  useEffect(() => {
    const alTeclear = (e: KeyboardEvent) => {
      if (escribiendoTexto(e) || e.ctrlKey || e.metaKey || e.altKey) return;
      const t = e.key === "Backspace" ? "borrar" : e.key;
      if (/^[\d.,]$/.test(t) || t === "borrar") {
        e.preventDefault();
        cambiar((v) => teclearMonto(v, t));
      }
    };
    window.addEventListener("keydown", alTeclear);
    return () => window.removeEventListener("keydown", alTeclear);
  }, [cambiar]);

  return (
    <div className="teclado-monto">
      <output className="teclado-monto__valor rp-num" aria-label={etiqueta} data-testid="monto">
        {verMonto(valor)}
      </output>
      <div className="rp-keypad">
        {["1", "2", "3", "4", "5", "6", "7", "8", "9", ".", "0"].map((d) => (
          <button key={d} type="button" className="rp-key" onClick={() => cambiar((v) => teclearMonto(v, d))} aria-label={d === "." ? "Punto decimal" : d}>
            {d}
          </button>
        ))}
        <button type="button" className="rp-key rp-key--text" onClick={() => cambiar((v) => teclearMonto(v, "borrar"))} aria-label="Borrar">
          <Delete />
        </button>
      </div>
    </div>
  );
}
