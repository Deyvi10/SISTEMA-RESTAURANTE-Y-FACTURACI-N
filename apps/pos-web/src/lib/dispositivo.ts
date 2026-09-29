// La caja se adapta al equipo: en la computadora principal trabaja con teclado (atajos, foco
// automático, dos columnas); en un celular o tableta, con el dedo (barra de pestañas abajo, una
// columna, sin pistas de teclas, sin abrir el teclado del sistema por sorpresa, vibración al cobrar).
import { useSyncExternalStore } from "react";

export type TipoDispositivo = "celular" | "tablet" | "computadora";

export interface Dispositivo {
  tipo: TipoDispositivo;
  /** Pantalla táctil sin ratón: sin atajos a la vista ni foco automático en buscadores. */
  tactil: boolean;
}

// Anchos en px CSS: un celular en vertical no pasa de 767; una tableta de 1099.
export const CONSULTAS = {
  celular: "(max-width: 767px)",
  tablet: "(max-width: 1099px)",
  tactil: "(hover: none) and (pointer: coarse)",
} as const;

type Coincide = (consulta: string) => boolean;

const coincideAqui: Coincide = (q) => typeof window !== "undefined" && typeof window.matchMedia === "function" && window.matchMedia(q).matches;

/** Decide el tipo de equipo con las consultas de medios (se puede probar sin navegador). */
export function clasificar(coincide: Coincide = coincideAqui): Dispositivo {
  const tactil = coincide(CONSULTAS.tactil);
  if (coincide(CONSULTAS.celular)) return { tipo: "celular", tactil };
  // Una ventana angosta en la PC sigue siendo «computadora»: la tableta es táctil.
  if (coincide(CONSULTAS.tablet) && tactil) return { tipo: "tablet", tactil };
  return { tipo: "computadora", tactil };
}

let actual = clasificar();
const oyentes = new Set<() => void>();

function recalcular() {
  const nuevo = clasificar();
  if (nuevo.tipo === actual.tipo && nuevo.tactil === actual.tactil) return;
  actual = nuevo;
  marcar();
  oyentes.forEach((f) => f());
}

/** Deja el tipo en <html data-dispositivo data-tactil> para que el CSS también lo use. */
function marcar() {
  if (typeof document === "undefined") return;
  document.documentElement.dataset.dispositivo = actual.tipo;
  document.documentElement.dataset.tactil = String(actual.tactil);
}

let iniciado = false;
/** Se llama una vez al arrancar: marca el documento y sigue los cambios (girar el equipo, redimensionar). */
export function iniciarDispositivo() {
  if (iniciado || typeof window === "undefined" || typeof window.matchMedia !== "function") return;
  iniciado = true;
  actual = clasificar();
  marcar();
  for (const q of Object.values(CONSULTAS)) window.matchMedia(q).addEventListener("change", recalcular);
}

const suscribir = (f: () => void) => {
  oyentes.add(f);
  return () => oyentes.delete(f);
};

/** El equipo actual; se actualiza solo si cambia (por ejemplo, al girar una tableta). */
export function useDispositivo(): Dispositivo {
  return useSyncExternalStore(suscribir, () => actual);
}

/** Vibración corta de confirmación en celulares (no hace nada donde no existe). */
export function vibrar(ms = 40) {
  if (actual.tactil && typeof navigator !== "undefined" && typeof navigator.vibrate === "function") navigator.vibrate(ms);
}
