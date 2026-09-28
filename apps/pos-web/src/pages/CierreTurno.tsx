// Cierre de turno ciego (F4-12): el cajero cuenta billetes y monedas y declara los vouchers sin
// ver lo esperado; el nodo calcula las diferencias, genera el Cierre Z y lo imprime.
import { formatMoney } from "@restpos/ui";
import { CircleCheck, CircleX, Minus, Plus, TriangleAlert } from "lucide-react";
import { type KeyboardEvent, useEffect, useMemo, useRef, useState } from "react";
import { uuidv7 } from "../api/identidad";
import { ApiError, type CierreOut, type ConfigCaja, type Denominacion, type MetodoPago, nodo, type ResultadoCierre } from "../api/nodo";
import { useAtajo } from "../components/atajos";
import { centavos, verCentavos } from "../lib/dinero";

/** Total del efectivo contado, en texto decimal. */
export function totalContado(dens: Denominacion[], conteo: Record<string, number>): string {
  return verCentavos(dens.reduce((t, d) => t + centavos(d.valor) * (conteo[d.clave] ?? 0), 0));
}

export const montoValido = (s: string) => /^\d{1,6}(\.\d{1,2})?$/.test(s.trim());

const RESULTADO: Record<ResultadoCierre, { titulo: string; tono: string; Icono: typeof CircleCheck }> = {
  CUADRADO: { titulo: "Cuadrado", tono: "verde", Icono: CircleCheck },
  SOBRANTE: { titulo: "Sobrante", tono: "amarillo", Icono: TriangleAlert },
  FALTANTE: { titulo: "Faltante", tono: "rojo", Icono: CircleX },
};

type Paso = "billetes" | "monedas" | "otros" | "confirmar";
const PASOS: { id: Paso; titulo: string }[] = [
  { id: "billetes", titulo: "Billetes" },
  { id: "monedas", titulo: "Monedas" },
  { id: "otros", titulo: "Tarjetas y transferencias" },
  { id: "confirmar", titulo: "Confirmar" },
];

function Contador({ d, n, cambiar }: { d: Denominacion; n: number; cambiar: (n: number) => void }) {
  const teclear = (e: KeyboardEvent<HTMLDivElement>) => {
    const fila = e.currentTarget;
    const mover = (el: Element | null) => (el as HTMLElement | null)?.focus();
    if (e.key === "ArrowDown") mover(fila.nextElementSibling);
    else if (e.key === "ArrowUp") mover(fila.previousElementSibling);
    else if (e.key === "+" || e.key === "ArrowRight") cambiar(n + 1);
    else if (e.key === "-" || e.key === "ArrowLeft") cambiar(Math.max(0, n - 1));
    else if (e.key === "Backspace" || e.key === "Delete") cambiar(0);
    else if (/^\d$/.test(e.key)) cambiar(Math.min(9999, n * 10 + Number(e.key)));
    else return;
    e.preventDefault();
    e.stopPropagation();
  };
  return (
    <div className="rp-cell contador" tabIndex={0} onKeyDown={teclear} data-testid={`den-${d.clave}`} aria-label={`${d.etiqueta} ${d.moneda ? "moneda" : "billete"}: ${n}`}>
      <span className="rp-cell__body">
        <span className="rp-cell__title contador__etiqueta">{d.etiqueta}</span>
      </span>
      <span className="rp-cell__value rp-num contador__subtotal">{n > 0 ? formatMoney(verCentavos(centavos(d.valor) * n)) : ""}</span>
      <span className="rp-stepper" role="group" aria-label={`Cantidad de ${d.etiqueta}`}>
        <button type="button" tabIndex={-1} aria-label="Menos" onClick={() => cambiar(Math.max(0, n - 1))} disabled={n === 0}>
          <Minus aria-hidden="true" />
        </button>
        <output>{n}</output>
        <button type="button" tabIndex={-1} aria-label="Más" onClick={() => cambiar(n + 1)}>
          <Plus aria-hidden="true" />
        </button>
      </span>
    </div>
  );
}

export function CierreTurno({ cajaId, config, cerrar, listo }: { cajaId: string; config: ConfigCaja; cerrar: () => void; listo: (r: CierreOut) => void }) {
  const [paso, setPaso] = useState<Paso>("billetes");
  const [conteo, setConteo] = useState<Record<string, number>>({});
  const [otros, setOtros] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);
  const [hecho, setHecho] = useState<CierreOut | null>(null);
  // Una clave por cierre: si la red se corta y se reintenta, el nodo devuelve el mismo Z.
  const [clave] = useState(() => uuidv7());
  const cuerpo = useRef<HTMLDivElement>(null);

  // Al abrir, el foco va al primer billete: se cuenta sin tocar el mouse.
  useEffect(() => (cuerpo.current?.querySelector("[tabindex='0']") as HTMLElement | null)?.focus(), []);

  const noEfectivo: MetodoPago[] = useMemo(() => config.metodos.filter((m) => m.tipo !== "EFECTIVO"), [config.metodos]);
  const billetes = config.denominaciones.filter((d) => !d.moneda);
  const monedas = config.denominaciones.filter((d) => d.moneda);
  const contado = totalContado(config.denominaciones, conteo);
  const pasos = PASOS.filter((p) => p.id !== "otros" || noEfectivo.length > 0);
  const i = pasos.findIndex((p) => p.id === paso);
  const otrosValidos = Object.values(otros).every((v) => v.trim() === "" || montoValido(v));

  const ir = (d: number) => {
    const sig = pasos[i + d];
    if (!sig) return;
    setPaso(sig.id);
    requestAnimationFrame(() => (cuerpo.current?.querySelector("[tabindex='0'], input") as HTMLElement | null)?.focus());
  };

  const confirmar = () => {
    if (ocupado) return;
    setOcupado(true);
    setError(null);
    nodo
      .cerrarTurno({
        cajaId,
        conteo: config.denominaciones.map((d) => ({ clave: d.clave, cantidad: conteo[d.clave] ?? 0 })),
        declarado: noEfectivo.map((m) => ({ metodoId: m.id, monto: (otros[m.id] ?? "").trim() || "0" })),
        idempotencyKey: clave,
      })
      .then((r) => {
        setHecho(r);
        listo(r);
      })
      .catch((e) => {
        setOcupado(false);
        setError(e instanceof ApiError ? e.message : "No se pudo cerrar el turno. Intenta de nuevo: no se duplicará.");
      });
  };

  useAtajo("Escape", "Cancelar el cierre", () => (hecho ? cerrar() : i > 0 ? ir(-1) : cerrar()), "Cierre");
  useAtajo("Enter", paso === "confirmar" ? "Cerrar el turno" : "Siguiente paso", () => (hecho ? cerrar() : paso === "confirmar" ? confirmar() : ir(1)), "Cierre");

  const titulo = hecho ? `Cierre Z ${String(hecho.cierre.numero).padStart(4, "0")}` : "Cerrar turno";
  return (
    <div className="rp-scrim" data-open="true">
      <div className="rp-sheet hoja-caja hoja-cierre" role="dialog" aria-modal="true" aria-label={titulo}>
        <div className="rp-sheet__header">
          {hecho ? (
            <span />
          ) : (
            <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={i > 0 ? () => ir(-1) : cerrar}>
              {i > 0 ? "Atrás" : "Cancelar"}
            </button>
          )}
          <h2 className="rp-sheet__title">{titulo}</h2>
          {hecho ? (
            <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={cerrar} data-testid="cierre-listo">
              Listo
            </button>
          ) : (
            <span className="rp-secondary pasos">
              {i + 1} de {pasos.length}
            </span>
          )}
        </div>
        <div className="rp-sheet__body" ref={cuerpo}>
          {hecho ? (
            <Resultado out={hecho} />
          ) : (
            <>
              <h3 className="rp-section-header">{pasos[i]?.titulo}</h3>
              {(paso === "billetes" || paso === "monedas") && (
                <>
                  <p className="hoja-caja__ayuda">Cuenta el cajón y marca cuántos hay de cada uno. Flechas para moverte, + y − o el número directo.</p>
                  <div className="rp-group">
                    {(paso === "billetes" ? billetes : monedas).map((d) => (
                      <Contador key={d.clave} d={d} n={conteo[d.clave] ?? 0} cambiar={(n) => setConteo((c) => ({ ...c, [d.clave]: n }))} />
                    ))}
                  </div>
                </>
              )}
              {paso === "otros" && (
                <>
                  <p className="hoja-caja__ayuda">Suma los vouchers del datáfono y los comprobantes de transferencia. Déjalo vacío si no hubo.</p>
                  <div className="rp-group">
                    {noEfectivo.map((m) => (
                      <label className="rp-cell" key={m.id}>
                        <span className="rp-cell__body">{m.nombre}</span>
                        <span>$</span>
                        <input
                          className="monto-voucher rp-num"
                          inputMode="decimal"
                          value={otros[m.id] ?? ""}
                          placeholder="0.00"
                          aria-invalid={!!otros[m.id] && !montoValido(otros[m.id]!)}
                          onChange={(e) => setOtros((o) => ({ ...o, [m.id]: e.target.value.replace(",", ".") }))}
                          onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), otrosValidos && ir(1))}
                          data-testid={`voucher-${m.nombre}`}
                        />
                      </label>
                    ))}
                  </div>
                  {!otrosValidos && <p className="aviso-error hoja-caja__ayuda">Escribe montos como 15.40 (hasta dos decimales).</p>}
                </>
              )}
              {paso === "confirmar" && (
                <>
                  <p className="hoja-caja__ayuda">Revisa lo que declaras. Al cerrar se genera el Cierre Z y ya no se puede cambiar.</p>
                  <div className="rp-group">
                    <div className="rp-cell">
                      <span className="rp-cell__body">Efectivo contado</span>
                      <span className="rp-cell__value rp-num">{formatMoney(contado)}</span>
                    </div>
                    {noEfectivo.map((m) => (
                      <div className="rp-cell" key={m.id}>
                        <span className="rp-cell__body">{m.nombre}</span>
                        <span className="rp-cell__value rp-num">{formatMoney((otros[m.id] ?? "").trim() || "0")}</span>
                      </div>
                    ))}
                  </div>
                  {error && (
                    <p className="aviso-error hoja-caja__ayuda" role="alert">
                      {error}
                    </p>
                  )}
                </>
              )}
              <div className="hoja-caja__pie">
                <p className="cierre__contado rp-num">
                  Efectivo contado <b>{formatMoney(contado)}</b>
                </p>
                {paso === "confirmar" ? (
                  <button className="rp-btn rp-btn--destructive rp-btn--block" onClick={confirmar} disabled={ocupado} data-testid="confirmar-cierre">
                    {ocupado ? "Cerrando…" : "Cerrar turno y generar Cierre Z"}
                  </button>
                ) : (
                  <button className="rp-btn rp-btn--primary rp-btn--block" onClick={() => ir(1)} disabled={paso === "otros" && !otrosValidos} data-testid="siguiente">
                    Siguiente
                  </button>
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function Resultado({ out }: { out: CierreOut }) {
  const r = RESULTADO[out.cierre.resultado];
  return (
    <div className="cierre-resultado" data-tono={r.tono} data-testid="resultado-cierre">
      <r.Icono className="cierre-resultado__icono" aria-hidden="true" />
      <p className="rp-t-title cierre-resultado__titulo">{r.titulo}</p>
      <div className="rp-group">
        {out.cierre.lineas.map((l) => (
          <div className="rp-cell" key={l.metodoId + l.metodo} data-tono={RESULTADO[l.resultado].tono}>
            <span className="rp-cell__body">
              <span className="rp-cell__title">{l.metodo}</span>
              <span className="rp-cell__subtitle">{RESULTADO[l.resultado].titulo}</span>
            </span>
            <span className="rp-cell__value rp-num">{l.resultado === "CUADRADO" ? "✓" : formatMoney(l.diferencia.replace("-", ""))}</span>
          </div>
        ))}
      </div>
      <p className="hoja-caja__ayuda">
        {out.impresoras.length > 0 ? `Impreso en ${out.impresoras.join(", ")}. ` : ""}
        {out.aviso ? `${out.aviso} ` : ""}
        La caja queda cerrada hasta abrir un turno nuevo. El dueño recibe el Cierre Z por correo.
      </p>
    </div>
  );
}
