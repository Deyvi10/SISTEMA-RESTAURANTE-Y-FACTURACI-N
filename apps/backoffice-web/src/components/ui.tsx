import { type ReactNode, useEffect, useId, useRef } from "react";
import { Icon } from "../lib/icons";

/** Icono de app de iOS: cuadrado redondeado con degradado del tinte. */
export function AppIcon({ icono, tint = "blue", size = 32 }: { icono: string; tint?: string; size?: number }) {
  return (
    <span className="rp-app-icon" style={{ ["--tint" as string]: `var(--rp-tint-${tint})`, width: size, height: size }}>
      <Icon name={icono} size={Math.round(size * 0.58)} strokeWidth={2.1} />
    </span>
  );
}

export function Spinner({ label = "Cargando…" }: { label?: string }) {
  return (
    <div className="cargando" role="status">
      <div className="spinner" />
      <span className="sr-only">{label}</span>
    </div>
  );
}

/** Campo con etiqueta, error y pista. El error se anuncia a lectores de pantalla. */
export function Field({ label, error, hint, children, id }: { label: string; error?: string | undefined; hint?: string; children: (id: string, describedBy?: string) => ReactNode; id?: string }) {
  const auto = useId();
  const fid = id ?? auto;
  const desc = error ? `${fid}-err` : hint ? `${fid}-hint` : undefined;
  return (
    <div className="rp-field" data-invalid={error ? "true" : undefined}>
      <label htmlFor={fid}>{label}</label>
      {children(fid, desc)}
      {error ? (
        <span className="rp-field__error" id={`${fid}-err`} role="alert">
          {error}
        </span>
      ) : hint ? (
        <span className="rp-field__hint" id={`${fid}-hint`} style={{ color: "var(--rp-color-label-secondary)" }}>
          {hint}
        </span>
      ) : null}
    </div>
  );
}

export function ToggleRow({ label, detalle, checked, onChange }: { label: string; detalle?: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="rp-cell">
      <span className="rp-cell__body">
        <span className="rp-cell__title">{label}</span>
        {detalle && <span className="rp-cell__subtitle">{detalle}</span>}
      </span>
      <input className="rp-toggle" type="checkbox" role="switch" checked={checked} onChange={(e) => onChange(e.target.checked)} />
    </label>
  );
}

export function Segmented<T extends string>({ value, options, onChange, label }: { value: T; options: { value: T; label: string }[]; onChange: (v: T) => void; label: string }) {
  const name = useId();
  return (
    <div className="rp-segmented" role="radiogroup" aria-label={label}>
      {options.map((o) => (
        <label key={o.value}>
          <input type="radio" name={name} checked={value === o.value} onChange={() => onChange(o.value)} />
          <span>{o.label}</span>
        </label>
      ))}
    </div>
  );
}

/** Hoja modal estilo iOS. Escape cierra; el foco entra al abrir y vuelve al cerrar. */
export function Sheet({
  open, title, onClose, onSave, saveLabel = "Guardar", busy, children, destructive,
}: {
  open: boolean; title: string; onClose: () => void; onSave?: () => void; saveLabel?: string; busy?: boolean; children: ReactNode;
  destructive?: { label: string; onClick: () => void };
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const previo = document.activeElement as HTMLElement | null;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    requestAnimationFrame(() => ref.current?.querySelector<HTMLElement>("input, select, textarea, button")?.focus());
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
      previo?.focus();
    };
  }, [open, onClose]);
  if (!open) return null;
  return (
    <>
      <div className="rp-scrim" data-open="true" onClick={onClose} />
      <div className="rp-sheet" data-open="true" role="dialog" aria-modal="true" aria-label={title} ref={ref}>
        <div className="rp-sheet__grabber" />
        <div className="rp-sheet__header">
          <button type="button" className="rp-btn rp-btn--plain" onClick={onClose}>Cancelar</button>
          <span className="rp-sheet__title">{title}</span>
          {onSave ? (
            <button type="button" className="rp-btn rp-btn--plain" style={{ fontWeight: 600 }} onClick={onSave} disabled={busy}>
              {busy ? "Guardando…" : saveLabel}
            </button>
          ) : <span />}
        </div>
        <div className="rp-sheet__body">
          {children}
          {destructive && (
            <div style={{ padding: "8px 16px 0" }}>
              <button type="button" className="rp-btn rp-btn--gray rp-btn--block" style={{ color: "var(--rp-color-danger-text)" }} onClick={destructive.onClick}>
                {destructive.label}
              </button>
            </div>
          )}
        </div>
      </div>
    </>
  );
}

export function Empty({ icono, tint = "orange", titulo, texto, accion }: { icono: string; tint?: string; titulo: string; texto: string; accion?: ReactNode }) {
  return (
    <div className="rp-empty">
      <AppIcon icono={icono} tint={tint} size={60} />
      <p className="rp-empty__title">{titulo}</p>
      <p>{texto}</p>
      {accion}
    </div>
  );
}

export function initials(nombre: string): string {
  return nombre.split(/\s+/).filter(Boolean).slice(0, 2).map((p) => p[0]!.toUpperCase()).join("");
}

const TINTS = ["orange", "pink", "purple", "indigo", "blue", "teal", "green", "red"];
/** Color estable por nombre, para avatares. */
export function tintFor(s: string): string {
  let h = 0;
  for (const c of s) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return TINTS[h % TINTS.length]!;
}
