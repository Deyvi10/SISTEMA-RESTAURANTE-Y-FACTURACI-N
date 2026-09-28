// Pago mixto (F4-06): una cuenta con varios métodos ($20 en efectivo + $15.40 con tarjeta) y,
// en tarjetas, lote, referencia y últimos 4 opcionales para el cuadre de vouchers.
import { formatMoney } from "@restpos/ui";
import { Plus, Trash2 } from "lucide-react";
import { type FormEvent, useState } from "react";
import type { MetodoPago } from "../api/nodo";
import { useAtajo } from "../components/atajos";
import { centavos, montoValido, verCentavos } from "../lib/dinero";

export interface FilaPago {
  clave: string;
  metodo: MetodoPago;
  monto: string;
  recibido: string; // solo efectivo
  referencia: string;
  lote: string;
  ultimos4: string;
}

/** Lo que el nodo recibe por cada pago. */
export interface PagoEnvio {
  metodoId: string;
  monto: string;
  recibido: string;
  referencia: string;
  lote: string;
  ultimos4: string;
}

const esEfectivo = (m: MetodoPago) => m.tipo === "EFECTIVO";

/** Cuánto falta por asignar (negativo si se pasaron). Los montos inválidos cuentan como 0. */
export function restante(total: string, filas: FilaPago[]): number {
  return centavos(total) - filas.reduce((t, f) => t + (montoValido(f.monto) ? centavos(f.monto.trim()) : 0), 0);
}

/** El primer problema de las filas o null si se puede cobrar. */
export function problemaPagos(total: string, filas: FilaPago[]): string | null {
  if (filas.length === 0) return "Agrega al menos un método.";
  if (filas.filter((f) => esEfectivo(f.metodo)).length > 1) return "Registra el efectivo en un solo pago.";
  for (const f of filas) {
    if (!montoValido(f.monto) || centavos(f.monto.trim()) === 0) return `Escribe el monto de ${f.metodo.nombre}.`;
    if (esEfectivo(f.metodo) && f.recibido.trim() !== "") {
      if (!montoValido(f.recibido)) return "Lo recibido en efectivo no es un monto válido.";
      if (centavos(f.recibido.trim()) < centavos(f.monto.trim())) return "Lo recibido en efectivo no alcanza para su parte.";
    }
    if (f.ultimos4.trim() !== "" && !/^\d{4}$/.test(f.ultimos4.trim())) return "Los últimos 4 dígitos son 4 números.";
  }
  const r = restante(total, filas);
  if (r > 0) return `Faltan ${formatMoney(verCentavos(r))} por asignar.`;
  if (r < 0) return `Los pagos se pasan por ${formatMoney(verCentavos(-r))}.`;
  return null;
}

export function aEnvio(filas: FilaPago[]): PagoEnvio[] {
  return filas.map((f) => ({
    metodoId: f.metodo.id,
    monto: f.monto.trim(),
    recibido: esEfectivo(f.metodo) ? f.recibido.trim() : "",
    referencia: f.referencia.trim(),
    lote: f.lote.trim(),
    ultimos4: f.ultimos4.trim(),
  }));
}

let secuencia = 0;
const nuevaFila = (metodo: MetodoPago, monto: string): FilaPago => ({ clave: String(++secuencia), metodo, monto, recibido: "", referencia: "", lote: "", ultimos4: "" });

export function HojaPagos({
  total,
  metodos,
  inicial,
  ocupado,
  error,
  cerrar,
  cobrar,
}: {
  total: string;
  metodos: MetodoPago[];
  inicial?: MetodoPago;
  ocupado: boolean;
  error: string | null;
  cerrar: () => void;
  cobrar: (pagos: PagoEnvio[]) => void;
}) {
  const [filas, setFilas] = useState<FilaPago[]>(() => (inicial ? [nuevaFila(inicial, total)] : []));
  const problema = problemaPagos(total, filas);
  const falta = restante(total, filas);
  const cambiar = (clave: string, campo: keyof FilaPago, v: string) => setFilas((fs) => fs.map((f) => (f.clave === clave ? { ...f, [campo]: v.replace(",", ".") } : f)));
  const confirmar = (e?: FormEvent) => {
    e?.preventDefault();
    if (!problema && !ocupado) cobrar(aEnvio(filas));
  };
  useAtajo("Escape", "Cancelar", cerrar, "Pago");

  return (
    <div className="rp-scrim" data-open="true" onClick={cerrar}>
      <form className="rp-sheet hoja-caja hoja-pagos" role="dialog" aria-modal="true" aria-label="Detalle del pago" onClick={(e) => e.stopPropagation()} onSubmit={confirmar}>
        <div className="rp-sheet__header">
          <button type="button" className="rp-btn rp-btn--plain rp-btn--sm" onClick={cerrar}>
            Cancelar
          </button>
          <h2 className="rp-sheet__title">{inicial && filas.length === 1 ? inicial.nombre : "Pago mixto"}</h2>
          <span />
        </div>
        <div className="rp-sheet__body">
          <p className="pagos__total">
            Total <b className="rp-num">{formatMoney(total)}</b>
            <span className={`rp-num pagos__falta${falta === 0 ? " pagos__falta--ok" : ""}`} data-testid="restante">
              {falta === 0 ? "Completo" : falta > 0 ? `Faltan ${formatMoney(verCentavos(falta))}` : `Sobran ${formatMoney(verCentavos(-falta))}`}
            </span>
          </p>
          {filas.map((f, i) => (
            <fieldset key={f.clave} className="pago" data-testid={`pago-${f.metodo.nombre}`}>
              <legend className="pago__metodo">
                {f.metodo.nombre}
                <button type="button" className="rp-btn rp-btn--plain rp-btn--sm" aria-label={`Quitar ${f.metodo.nombre}`} onClick={() => setFilas((fs) => fs.filter((x) => x.clave !== f.clave))}>
                  <Trash2 aria-hidden="true" />
                </button>
              </legend>
              <label className="rp-field">
                <span>Monto</span>
                <input inputMode="decimal" className="rp-num" value={f.monto} onChange={(e) => cambiar(f.clave, "monto", e.target.value)} autoFocus={i === filas.length - 1} aria-label={`Monto de ${f.metodo.nombre}`} />
              </label>
              {esEfectivo(f.metodo) ? (
                <label className="rp-field">
                  <span>Recibido (opcional)</span>
                  <input inputMode="decimal" className="rp-num" value={f.recibido} placeholder={f.monto} onChange={(e) => cambiar(f.clave, "recibido", e.target.value)} aria-label="Efectivo recibido" />
                  {montoValido(f.recibido) && montoValido(f.monto) && centavos(f.recibido.trim()) > centavos(f.monto.trim()) && (
                    <small className="pago__vuelto rp-num">Vuelto {formatMoney(verCentavos(centavos(f.recibido.trim()) - centavos(f.monto.trim())))}</small>
                  )}
                </label>
              ) : (
                f.metodo.pideReferencia && (
                  <div className="pago__voucher">
                    <label className="rp-field">
                      <span>Lote</span>
                      <input value={f.lote} maxLength={20} onChange={(e) => cambiar(f.clave, "lote", e.target.value)} />
                    </label>
                    <label className="rp-field">
                      <span>Referencia</span>
                      <input value={f.referencia} maxLength={40} onChange={(e) => cambiar(f.clave, "referencia", e.target.value)} />
                    </label>
                    <label className="rp-field">
                      <span>Últimos 4</span>
                      <input inputMode="numeric" value={f.ultimos4} maxLength={4} onChange={(e) => cambiar(f.clave, "ultimos4", e.target.value.replace(/\D/g, ""))} />
                    </label>
                  </div>
                )
              )}
            </fieldset>
          ))}
          <div className="pagos__agregar">
            {metodos
              .filter((m) => !(esEfectivo(m) && filas.some((f) => esEfectivo(f.metodo))))
              .map((m) => (
                <button key={m.id} type="button" className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => setFilas((fs) => [...fs, nuevaFila(m, verCentavos(Math.max(0, falta)))])} data-testid={`agregar-${m.nombre}`}>
                  <Plus aria-hidden="true" /> {m.nombre}
                </button>
              ))}
          </div>
          {(error || (problema && filas.length > 0)) && (
            <p className="aviso-error hoja-caja__ayuda" role="alert">
              {error ?? problema}
            </p>
          )}
          <div className="hoja-caja__pie">
            <button type="submit" className="rp-btn rp-btn--primary rp-btn--block" disabled={!!problema || ocupado} data-testid="cobrar-mixto">
              {ocupado ? "Cobrando…" : `Cobrar ${formatMoney(total)}`} <kbd className="tecla">Intro</kbd>
            </button>
          </div>
        </div>
      </form>
    </div>
  );
}
