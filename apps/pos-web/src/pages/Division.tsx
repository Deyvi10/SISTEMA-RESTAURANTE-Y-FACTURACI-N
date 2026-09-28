// Dividir la cuenta (F4-08, RF-04-06): platos a la izquierda, cuentas a la derecha con
// «+ Nueva cuenta»; se asigna tocando o con el teclado, sin llamar al nodo hasta confirmar.
// Un plato se comparte entre varias cuentas (la pizza entre tres) y las pagadas no se tocan.
import { formatMoney } from "@restpos/ui";
import { ArrowLeft, Lock, Plus, Share2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { ApiError, type Asignacion, type Cuenta, type LineaOrden, nodo, type Totales } from "../api/nodo";
import { useAtajo } from "../components/atajos";
import { centavos, repartirCentavos, verCentavos } from "../lib/dinero";

/** Cuenta en edición: pesos por plato. */
export interface CuentaEdicion {
  clave: string;
  pesos: Record<string, number>;
}

let secuencia = 0;
export const nuevaCuenta = (pesos: Record<string, number> = {}): CuentaEdicion => ({ clave: `c${++secuencia}`, pesos });

/** Lo que falta por pagar de cada plato (en centavos): su importe menos lo pagado en cuentas cerradas. */
export function pendientes(lineas: LineaOrden[], finales: Record<string, string> | undefined, pagadas: Cuenta[]): Record<string, number> {
  const out: Record<string, number> = {};
  for (const l of lineas) {
    if (l.estado === "ANULADA") continue;
    out[l.id] = centavos(finales?.[l.id] ?? l.total) - pagadas.reduce((t, c) => t + centavos(c.lineas[l.id] ?? "0"), 0);
  }
  return out;
}

/**
 * Consumo de cada cuenta abierta (vista previa, misma regla de centavos que el nodo): los platos
 * que van a las mismas cuentas con los mismos pesos se reparten juntos, con el residuo en la última.
 */
export function consumo(pendiente: Record<string, number>, cuentas: CuentaEdicion[]): number[] {
  const total = cuentas.map(() => 0);
  const grupos = new Map<string, { idx: (readonly [number, number])[]; cents: number }>();
  for (const [linea, cents] of Object.entries(pendiente)) {
    const idx = cuentas.map((c, i) => [i, c.pesos[linea] ?? 0] as const).filter(([, p]) => p > 0);
    if (idx.length === 0) continue;
    const firma = idx.map(([i, p]) => `${i}:${p}`).join(",");
    const g = grupos.get(firma) ?? { idx, cents: 0 };
    g.cents += cents;
    grupos.set(firma, g);
  }
  for (const { idx, cents } of grupos.values()) {
    const partes = repartirCentavos(cents, idx.map(([, p]) => p));
    idx.forEach(([i], k) => (total[i]! += partes[k]!));
  }
  return total;
}

/** Platos con saldo que no están en ninguna cuenta abierta. */
export const sinCuenta = (pendiente: Record<string, number>, cuentas: CuentaEdicion[]) =>
  Object.entries(pendiente)
    .filter(([l, c]) => c > 0 && !cuentas.some((x) => (x.pesos[l] ?? 0) > 0))
    .map(([l]) => l);

/** Partes iguales: N cuentas y cada plato en todas (el residuo de centavos a la última). */
export const partesIguales = (n: number, pendiente: Record<string, number>): CuentaEdicion[] =>
  Array.from({ length: n }, () => nuevaCuenta(Object.fromEntries(Object.keys(pendiente).map((l) => [l, 1]))));

/** Pasar un plato a una sola cuenta (lo quita de las demás). */
export const mover = (cuentas: CuentaEdicion[], linea: string, destino: number) =>
  cuentas.map((c, i) => {
    const pesos = { ...c.pesos };
    if (i === destino) pesos[linea] = 1;
    else delete pesos[linea];
    return { ...c, pesos };
  });

/** Compartir un plato por igual entre las cuentas elegidas. */
export const compartir = (cuentas: CuentaEdicion[], linea: string, entre: number[]) =>
  cuentas.map((c, i) => {
    const pesos = { ...c.pesos };
    if (entre.includes(i)) pesos[linea] = 1;
    else delete pesos[linea];
    return { ...c, pesos };
  });

export function Division({ ordenId, lineas, totales, volver, listo }: { ordenId: string; lineas: LineaOrden[]; totales: Totales; volver: () => void; listo: () => void }) {
  const [pagadas, setPagadas] = useState<Cuenta[]>([]);
  const [cuentas, setCuentas] = useState<CuentaEdicion[] | null>(null);
  const [sel, setSel] = useState(0);
  const [foco, setFoco] = useState(0);
  const [compartiendo, setCompartiendo] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);

  useEffect(() => {
    nodo
      .cuentas(ordenId)
      .then((d) => {
        setPagadas(d.cuentas.filter((c) => c.estado === "PAGADA"));
        const abiertas = d.cuentas.filter((c) => c.estado === "ABIERTA").map((c) => nuevaCuenta(Object.fromEntries(c.asignaciones.map((a) => [a.lineaId, a.peso]))));
        setCuentas(abiertas.length > 0 ? abiertas : [nuevaCuenta(), nuevaCuenta()]);
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo leer la división."));
  }, [ordenId]);

  const vivas = lineas.filter((l) => l.estado !== "ANULADA");
  const pend = useMemo(() => pendientes(lineas, totales.lineas, pagadas), [lineas, totales.lineas, pagadas]);
  const conSaldo = vivas.filter((l) => (pend[l.id] ?? 0) > 0);
  const lista = cuentas ?? [];
  const previa = consumo(pend, lista);
  const faltan = sinCuenta(pend, lista);

  const confirmar = () => {
    if (ocupado || faltan.length > 0) return;
    setOcupado(true);
    setError(null);
    const body = lista.filter((c) => Object.keys(c.pesos).length > 0).map((c) => ({ asignaciones: Object.entries(c.pesos).map(([lineaId, peso]): Asignacion => ({ lineaId, peso })) }));
    nodo
      .dividir(ordenId, body)
      .then(listo)
      .catch((e) => {
        setOcupado(false);
        setError(e instanceof ApiError ? e.message : "No se pudo dividir la cuenta.");
      });
  };
  const deshacer = () => {
    setOcupado(true);
    nodo
      .dividir(ordenId, [])
      .then(listo)
      .catch((e) => {
        setOcupado(false);
        setError(e instanceof ApiError ? e.message : "No se pudo deshacer la división.");
      });
  };
  const agregarCuenta = () => {
    setCuentas((c) => [...(c ?? []), nuevaCuenta()]);
    setSel(lista.length);
  };
  const asignarFoco = () => {
    const l = conSaldo[foco];
    if (l) setCuentas((c) => mover(c ?? [], l.id, sel));
  };

  const libre = compartiendo === null;
  useAtajo("Escape", "Volver al cobro", () => (compartiendo ? setCompartiendo(null) : volver()), "División");
  useAtajo("F2", "Confirmar la división", confirmar, "División", libre);
  useAtajo("n", "Nueva cuenta", agregarCuenta, "División", libre);
  useAtajo("ArrowDown", "Plato siguiente", () => setFoco((f) => Math.min(f + 1, conSaldo.length - 1)), "División", libre);
  useAtajo("ArrowUp", "Plato anterior", () => setFoco((f) => Math.max(f - 1, 0)), "División", libre);
  useAtajo("Enter", "Pasar el plato a la cuenta elegida", asignarFoco, "División", libre);
  for (const k of ["1", "2", "3", "4", "5", "6", "7", "8", "9"]) {
    // eslint-disable-next-line react-hooks/rules-of-hooks -- lista fija de 9 teclas
    useAtajo(k, `Elegir la cuenta ${k}`, () => Number(k) <= lista.length && setSel(Number(k) - 1), "División", libre);
  }

  const numeroDe = (i: number) => pagadas.length + i + 1;
  const partesDe = (linea: string) => lista.map((c, i) => [i, c.pesos[linea] ?? 0] as const).filter(([, p]) => p > 0);

  return (
    <section className="division" data-testid="division">
      <div className="division__platos">
        <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={volver}>
          <ArrowLeft aria-hidden="true" /> Volver <kbd className="tecla">Esc</kbd>
        </button>
        <h1 className="rp-large-title">Dividir la cuenta</h1>
        <p className="rp-secondary">Elige una cuenta y toca los platos que paga. Para repartir un plato entre varias, usa «Compartir».</p>
        <div className="rp-group division__lista" role="listbox" aria-label="Platos">
          {conSaldo.map((l, i) => {
            const partes = partesDe(l.id);
            return (
              <div key={l.id} className="rp-cell division__plato" aria-selected={i === foco} role="option" data-testid={`plato-${l.producto}`}>
                <button className="division__asignar" onClick={() => (setFoco(i), setCuentas((c) => mover(c ?? [], l.id, sel)))}>
                  <span className="rp-num cobro__cant">{l.cantidad}</span>
                  <span className="rp-cell__body">
                    <span className="rp-cell__title">{l.producto}</span>
                    <span className="rp-cell__subtitle">
                      {partes.length === 0 ? "Sin cuenta" : partes.length === 1 ? `Cuenta ${numeroDe(partes[0]![0])}` : `Entre ${partes.map(([k]) => numeroDe(k)).join(", ")}`}
                    </span>
                  </span>
                  <span className="rp-cell__value rp-num">{formatMoney(verCentavos(pend[l.id] ?? 0))}</span>
                </button>
                <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={() => setCompartiendo(l.id)} aria-label={`Compartir ${l.producto}`}>
                  <Share2 aria-hidden="true" />
                </button>
              </div>
            );
          })}
        </div>
      </div>

      <aside className="division__cuentas" aria-label="Cuentas">
        <div className="division__acciones">
          <span className="rp-secondary">Partes iguales:</span>
          {[2, 3, 4, 5, 6].map((n) => (
            <button key={n} className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => (setCuentas(partesIguales(n, pend)), setSel(0))} data-testid={`iguales-${n}`}>
              {n}
            </button>
          ))}
        </div>
        {pagadas.map((c) => (
          <div key={c.id} className="division__cuenta division__cuenta--pagada">
            <Lock aria-hidden="true" /> Cuenta {c.numero} · pagada {c.documento ? `(${c.documento})` : ""} <b className="rp-num">{formatMoney(c.total)}</b>
          </div>
        ))}
        {lista.map((c, i) => (
          <button key={c.clave} className="division__cuenta" aria-pressed={i === sel} onClick={() => setSel(i)} data-testid={`cuenta-${numeroDe(i)}`}>
            <span className="division__cuenta-titulo">
              Cuenta {numeroDe(i)} <kbd className="tecla">{i + 1}</kbd>
            </span>
            <b className="rp-num division__cuenta-total">{formatMoney(verCentavos(previa[i] ?? 0))}</b>
            <small>
              {Object.keys(c.pesos).length} {Object.keys(c.pesos).length === 1 ? "plato" : "platos"}
            </small>
          </button>
        ))}
        <button className="rp-btn rp-btn--gray" onClick={agregarCuenta} data-testid="nueva-cuenta">
          <Plus aria-hidden="true" /> Nueva cuenta <kbd className="tecla">N</kbd>
        </button>
        <p className="rp-secondary division__nota">El consumo incluye el IVA de los platos; el servicio y el detalle se calculan al confirmar.</p>
        {faltan.length > 0 && <p className="cobro__aviso">Faltan {faltan.length === 1 ? "1 plato" : `${faltan.length} platos`} por asignar.</p>}
        {error && (
          <p className="aviso-error" role="alert">
            {error}
          </p>
        )}
        <button className="rp-btn rp-btn--primary rp-btn--block" disabled={ocupado || faltan.length > 0} onClick={confirmar} data-testid="confirmar-division">
          Confirmar división <kbd className="tecla">F2</kbd>
        </button>
        {pagadas.length === 0 && (
          <button className="rp-btn rp-btn--plain rp-btn--block" disabled={ocupado} onClick={deshacer} data-testid="deshacer-division">
            Cobrar todo junto (sin dividir)
          </button>
        )}
      </aside>

      {compartiendo && (
        <HojaCompartir
          plato={vivas.find((l) => l.id === compartiendo)?.producto ?? ""}
          cuentas={lista.map((_, i) => numeroDe(i))}
          marcadas={partesDe(compartiendo).map(([i]) => i)}
          cerrar={() => setCompartiendo(null)}
          listo={(entre) => {
            setCuentas((c) => compartir(c ?? [], compartiendo, entre));
            setCompartiendo(null);
          }}
        />
      )}
    </section>
  );
}

function HojaCompartir({ plato, cuentas, marcadas, cerrar, listo }: { plato: string; cuentas: number[]; marcadas: number[]; cerrar: () => void; listo: (entre: number[]) => void }) {
  const [entre, setEntre] = useState<number[]>(marcadas.length > 1 ? marcadas : cuentas.map((_, i) => i));
  useAtajo("Enter", "Compartir", () => entre.length > 0 && listo(entre), "Compartir");
  return (
    <div className="rp-scrim" data-open="true" onClick={cerrar}>
      <div className="rp-sheet hoja-caja" role="dialog" aria-modal="true" aria-label={`Compartir ${plato}`} onClick={(e) => e.stopPropagation()}>
        <div className="rp-sheet__header">
          <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={cerrar}>
            Cancelar
          </button>
          <h2 className="rp-sheet__title">Compartir {plato}</h2>
          <span />
        </div>
        <div className="rp-sheet__body">
          <p className="hoja-caja__ayuda">Se reparte por igual; el centavo que sobre va a la última cuenta.</p>
          <div className="rp-group" style={{ margin: "0 20px" }}>
            {cuentas.map((n, i) => (
              <label key={n} className="rp-cell">
                <input type="checkbox" checked={entre.includes(i)} onChange={(e) => setEntre((x) => (e.target.checked ? [...x, i].sort((a, b) => a - b) : x.filter((y) => y !== i)))} />
                <span className="rp-cell__body">Cuenta {n}</span>
              </label>
            ))}
          </div>
          <div className="hoja-caja__pie">
            <button className="rp-btn rp-btn--primary rp-btn--block" disabled={entre.length === 0} onClick={() => listo(entre)} data-testid="confirmar-compartir">
              Compartir entre {entre.length} <kbd className="tecla">Intro</kbd>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
