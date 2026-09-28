import { createContext, type ReactNode, useCallback, useContext, useRef, useState } from "react";
import { Icon } from "../lib/icons";

type Toast = { id: number; texto: string; tipo: "ok" | "error" };
type ConfirmOpts = { titulo: string; mensaje: string; confirmar: string; destructivo?: boolean };

interface Feedback {
  toast: (texto: string, tipo?: Toast["tipo"]) => void;
  confirm: (o: ConfirmOpts) => Promise<boolean>;
}

const Ctx = createContext<Feedback | null>(null);

export function useFeedback(): Feedback {
  const c = useContext(Ctx);
  if (!c) throw new Error("useFeedback fuera de FeedbackProvider");
  return c;
}

/** Notificaciones y confirmaciones dentro de la página (el sistema no usa alert/confirm del navegador). */
export function FeedbackProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [pregunta, setPregunta] = useState<(ConfirmOpts & { resolve: (v: boolean) => void }) | null>(null);
  const seq = useRef(0);
  const toast = useCallback((texto: string, tipo: Toast["tipo"] = "ok") => {
    const id = ++seq.current;
    setToasts((t) => [...t, { id, texto, tipo }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), tipo === "error" ? 6000 : 3200);
  }, []);
  const confirm = useCallback((o: ConfirmOpts) => new Promise<boolean>((resolve) => setPregunta({ ...o, resolve })), []);
  const responder = (v: boolean) => {
    pregunta?.resolve(v);
    setPregunta(null);
  };
  return (
    <Ctx.Provider value={{ toast, confirm }}>
      {children}
      <div className="toasts" aria-live="polite">
        {toasts.map((t) => (
          <div key={t.id} className={`rp-toast ${t.tipo === "error" ? "toast-error" : ""}`} role={t.tipo === "error" ? "alert" : "status"}>
            <span className="rp-toast__icon"><Icon name={t.tipo === "error" ? "error" : "ok"} size={16} strokeWidth={3} /></span>
            <span>{t.texto}</span>
          </div>
        ))}
      </div>
      {pregunta && (
        <div className="alert-stage" onClick={() => responder(false)}>
          <div className="rp-alert" role="alertdialog" aria-labelledby="alert-t" aria-describedby="alert-m" onClick={(e) => e.stopPropagation()}>
            <div className="rp-alert__body">
              <p className="rp-alert__title" id="alert-t">{pregunta.titulo}</p>
              <p className="rp-alert__msg" id="alert-m">{pregunta.mensaje}</p>
            </div>
            <div className="rp-alert__actions">
              <button type="button" autoFocus onClick={() => responder(false)}>Cancelar</button>
              <button type="button" className={`rp-alert__primary ${pregunta.destructivo ? "rp-alert__destructive" : ""}`} onClick={() => responder(true)}>
                {pregunta.confirmar}
              </button>
            </div>
          </div>
        </div>
      )}
    </Ctx.Provider>
  );
}
