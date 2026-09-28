// Descuentos y cortesías (F4-09, RF-04-07): de la cuenta o de un plato, en % o en monto, con
// motivo de la lista; la cortesía es el 100 %. Si pasa el límite de la persona, o es una
// cortesía, un supervisor autoriza con su PIN.
import { formatMoney } from "@restpos/ui";
import { useState } from "react";
import { ApiError, type LineaOrden, type MotivoDescuento, nodo, type Totales } from "../api/nodo";
import { useAtajo } from "../components/atajos";
import { Supervisor } from "../components/Supervisor";
import { montoDe, TecladoMonto } from "../components/TecladoMonto";
import { centavos, verCentavos } from "../lib/dinero";

type Tipo = "PORCENTAJE" | "MONTO" | "CORTESIA";

/** Cuánto descontaría sobre un importe (misma regla que el nodo; solo para mostrar). */
export function vistaPrevia(tipo: Tipo, valor: string, sobre: string): number {
  const base = centavos(sobre);
  if (tipo === "CORTESIA") return base;
  const v = montoDe(valor);
  if (tipo === "MONTO") return Math.min(centavos(v), base);
  // % en centésimas de punto: 12.5 % → 1250; redondeo a centavos half-up como el nodo.
  const pct = centavos(v);
  return Math.min(base, Math.floor((base * pct + 5000) / 10000));
}

export function HojaDescuento({
  ordenId,
  lineas,
  totalPlatos,
  motivos,
  usuarioId,
  cerrar,
  listo,
}: {
  ordenId: string;
  lineas: LineaOrden[];
  totalPlatos: string; // suma de platos con los descuentos de línea ya aplicados
  motivos: MotivoDescuento[];
  usuarioId?: string;
  cerrar: () => void;
  listo: (t: Totales) => void;
}) {
  const [linea, setLinea] = useState<string | null>(null);
  const [tipo, setTipo] = useState<Tipo>("PORCENTAJE");
  const [valor, setValor] = useState("");
  const [motivo, setMotivo] = useState<string | null>(null);
  const [supervisor, setSupervisor] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);
  useAtajo("Escape", "Cancelar", () => (supervisor ? setSupervisor(false) : cerrar()), "Descuento");

  const vivas = lineas.filter((l) => l.estado !== "ANULADA");
  const sobre = linea ? (vivas.find((l) => l.id === linea)?.total ?? "0") : totalPlatos;
  const cortesia = tipo === "CORTESIA";
  const lista = motivos.filter((m) => m.tipo === (cortesia ? "CORTESIA" : "DESCUENTO"));
  const previa = vistaPrevia(tipo, valor, sobre);
  const listoParaAplicar = !!motivo && (cortesia || centavos(montoDe(valor)) > 0) && !(tipo === "PORCENTAJE" && centavos(montoDe(valor)) > 10000);

  const aplicar = async (autorizacion = ""): Promise<string | null> => {
    if (!motivo || ocupado) return "Elige el motivo.";
    setOcupado(true);
    setError(null);
    try {
      listo(await nodo.descontar(ordenId, { lineaId: linea, tipo: cortesia ? "PORCENTAJE" : tipo, valor: cortesia ? "100" : montoDe(valor), cortesia, motivoId: motivo, autorizacion }));
      return null;
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : "No se pudo aplicar el descuento.";
      if (e instanceof ApiError && e.code === "REQUIERE_SUPERVISOR") setSupervisor(true);
      else setError(msg);
      return msg;
    } finally {
      setOcupado(false);
    }
  };
  useAtajo("Enter", "Aplicar el descuento", () => listoParaAplicar && void aplicar(), "Descuento", !supervisor);

  return (
    <div className="rp-scrim" data-open="true" onClick={cerrar}>
      <div className="rp-sheet hoja-caja hoja-descuento" role="dialog" aria-modal="true" aria-label="Descuento" onClick={(e) => e.stopPropagation()}>
        <div className="rp-sheet__header">
          <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={supervisor ? () => setSupervisor(false) : cerrar}>
            {supervisor ? "Atrás" : "Cancelar"}
          </button>
          <h2 className="rp-sheet__title">{cortesia ? "Cortesía" : "Descuento"}</h2>
          <span />
        </div>
        <div className="rp-sheet__body">
          {supervisor ? (
            <Supervisor accion="DAR_DESCUENTO" referencia={ordenId} excepto={usuarioId} alAutorizar={(token) => aplicar(token)} volver={() => setSupervisor(false)} />
          ) : (
            <>
              <div className="descuento__fila">
                <label className="rp-field">
                  <span>A qué</span>
                  <select value={linea ?? ""} onChange={(e) => setLinea(e.target.value || null)} data-testid="alcance">
                    <option value="">Toda la cuenta ({formatMoney(totalPlatos)})</option>
                    {vivas.map((l) => (
                      <option key={l.id} value={l.id}>
                        {l.cantidad} {l.producto} ({formatMoney(l.total)})
                      </option>
                    ))}
                  </select>
                </label>
                <div className="rp-segmented" role="radiogroup" aria-label="Tipo de descuento">
                  {(
                    [
                      ["PORCENTAJE", "%"],
                      ["MONTO", "$"],
                      ["CORTESIA", "Cortesía"],
                    ] as const
                  ).map(([t, n]) => (
                    <label key={t}>
                      <input
                        type="radio"
                        name="tipo-descuento"
                        checked={tipo === t}
                        onChange={() => {
                          setTipo(t);
                          setMotivo(null);
                        }}
                      />
                      <span>{n}</span>
                    </label>
                  ))}
                </div>
              </div>
              {!cortesia && <TecladoMonto valor={valor} cambiar={setValor} etiqueta={tipo === "PORCENTAJE" ? "Porcentaje" : "Monto"} unidad={tipo === "PORCENTAJE" ? "%" : "$"} />}
              <p className="hoja-caja__ayuda rp-num" data-testid="previa">
                {tipo === "PORCENTAJE" && !cortesia && valor ? `${montoDe(valor)} % · ` : ""}
                Descuenta {formatMoney(verCentavos(previa))} de {formatMoney(sobre)}
                {cortesia ? " (invitación)" : ""}
              </p>
              <p className="rp-section-header descuento__motivos-titulo">Motivo</p>
              <div className="rp-chips descuento__motivos" role="radiogroup" aria-label="Motivo">
                {lista.map((m) => (
                  <button key={m.id} type="button" className="rp-chip" aria-pressed={motivo === m.id} onClick={() => setMotivo(m.id)}>
                    {m.nombre}
                  </button>
                ))}
                {lista.length === 0 && <span className="rp-secondary">No hay motivos de {cortesia ? "cortesía" : "descuento"}: el dueño los crea en el panel web, en Caja.</span>}
              </div>
              {error && (
                <p className="aviso-error hoja-caja__ayuda" role="alert">
                  {error}
                </p>
              )}
              <div className="hoja-caja__pie">
                <button className="rp-btn rp-btn--primary rp-btn--block" disabled={!listoParaAplicar || ocupado} onClick={() => void aplicar()} data-testid="aplicar-descuento">
                  Aplicar <kbd className="tecla">Intro</kbd>
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
