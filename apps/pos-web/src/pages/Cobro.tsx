// Cobro «Zero-Click» (F4-05): total gigante, billetes dinámicos y un solo toque para cobrar,
// dar el vuelto, abrir el cajón, emitir el documento y liberar la mesa.
import { formatMoney } from "@restpos/ui";
import { ArrowLeft, Check, CreditCard, Landmark, Smartphone, UserRound, Wallet } from "lucide-react";
import { type Dispatch, type SetStateAction, useCallback, useEffect, useRef, useState } from "react";
import { uuidv7 } from "../api/identidad";
import { ApiError, type CobroOut, type MetodoPago, nodo, type Orden, type Totales } from "../api/nodo";
import { useAtajo } from "../components/atajos";
import { montoDe, TecladoMonto } from "../components/TecladoMonto";
import { opcionesBillete, vueltoCentavos } from "../lib/billetes";
import { centavos, verCentavos } from "../lib/dinero";
import { HojaPagos, type PagoEnvio } from "./PagoMixto";
import type { Caja } from "./Turno";

const esEfectivoM = (m: MetodoPago) => m.tipo === "EFECTIVO";

const ICONO: Record<string, typeof Wallet> = { tarjeta: CreditCard, transferencia: Landmark, billetera: Smartphone };

/** El vuelto «cuenta» hasta su valor, como una caja registradora (≈ 0,6 s). */
function useConteo(objetivo: number, ms = 600): number {
  const [v, setV] = useState(0);
  useEffect(() => {
    if (objetivo <= 0) return setV(objetivo);
    let raf = 0;
    const t0 = performance.now();
    const paso = (t: number) => {
      const k = Math.min(1, (t - t0) / ms);
      setV(Math.round(objetivo * (1 - (1 - k) ** 3)));
      if (k < 1) raf = requestAnimationFrame(paso);
    };
    raf = requestAnimationFrame(paso);
    return () => cancelAnimationFrame(raf);
  }, [objetivo, ms]);
  return v;
}

/** Qué se cobra: la orden abierta de una mesa o una orden sin mesa. */
export interface ACobrar {
  ordenId: string;
  nombre: string;
}

export function Cobro({ orden: aCobrar, caja, volver, irATurno, agregar }: { orden: ACobrar; caja: Caja; volver: () => void; irATurno: () => void; agregar?: (o: Orden) => void }) {
  const [datos, setDatos] = useState<{ orden: Orden; totales: Totales } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);
  const [hecho, setHecho] = useState<CobroOut | null>(null);
  const [otroMonto, setOtroMonto] = useState<string | null>(null);
  const [hojaPago, setHojaPago] = useState<{ inicial?: MetodoPago } | null>(null);
  // Una clave por intento de cobro: un doble toque o un reintento no cobra dos veces.
  const clave = useRef(uuidv7());

  useEffect(() => {
    nodo
      .orden(aCobrar.ordenId)
      .then(setDatos)
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo leer la orden."));
  }, [aCobrar.ordenId]);

  const config = caja.config;
  const turno = caja.estado?.turno ?? null;
  const total = datos?.totales.total ?? "0.00";
  const maximo = config?.consumidorFinalMaximo ?? "50.00";
  const excedeCF = centavos(total) > centavos(maximo);
  const efectivo = config?.metodos.find((m) => m.tipo === "EFECTIVO");
  const otros = config?.metodos.filter((m) => m.tipo !== "EFECTIVO") ?? [];
  const puedeCobrar = !!turno && !!datos && !excedeCF && centavos(total) > 0 && !ocupado && !hecho;

  const enviarCobro = useCallback(
    (pago: { metodoId: string; recibido: string } | { pagos: PagoEnvio[] }) => {
      if (!caja.cajaId || !datos || ocupado) return;
      setOcupado(true);
      setError(null);
      nodo
        .cobrar(datos.orden.id, { cajaId: caja.cajaId, consumidorFinal: true, idempotencyKey: clave.current, ...pago })
        .then(setHecho)
        .catch((e) => {
          setOcupado(false);
          clave.current = uuidv7();
          setError(e instanceof ApiError ? e.message : "No se pudo cobrar. Intenta de nuevo.");
        });
    },
    [caja.cajaId, datos, ocupado],
  );
  const cobrar = useCallback(
    (metodo: MetodoPago | undefined, recibido: string) => {
      if (!metodo) return;
      // Los métodos con voucher abren el detalle para anotar lote y últimos 4 (Intro cobra).
      if (!esEfectivoM(metodo) && metodo.pideReferencia && !recibido) setHojaPago({ inicial: metodo });
      else enviarCobro({ metodoId: metodo.id, recibido });
    },
    [enviarCobro],
  );

  const billetes = opcionesBillete(total);
  const libre = puedeCobrar && otroMonto === null && hojaPago === null;
  useAtajo("1", "Cobrar el monto exacto en efectivo", () => cobrar(efectivo, billetes[0]!), "Cobro", libre);
  useAtajo("2", "Cobrar con el segundo billete", () => billetes[1] && cobrar(efectivo, billetes[1]), "Cobro", libre && billetes.length > 1);
  useAtajo("3", "Cobrar con el tercer billete", () => billetes[2] && cobrar(efectivo, billetes[2]), "Cobro", libre && billetes.length > 2);
  useAtajo("4", "Cobrar con el cuarto billete", () => billetes[3] && cobrar(efectivo, billetes[3]), "Cobro", libre && billetes.length > 3);
  useAtajo("o", "Otro monto en efectivo", () => setOtroMonto(""), "Cobro", libre);
  useAtajo("m", "Pago mixto (varios métodos)", () => setHojaPago({}), "Cobro", libre);
  useAtajo("a", "Agregar platos a la orden", () => datos && agregar?.(datos.orden), "Cobro", !!datos && datos.orden.tipo !== "MESA" && !!agregar && !hecho && otroMonto === null);
  useAtajo("Escape", hecho ? "Siguiente cliente" : "Volver a las mesas", volver, "Cobro", otroMonto === null && hojaPago === null);
  useAtajo("Enter", "Siguiente cliente", volver, "Cobro", !!hecho);

  if (hecho) return <Listo out={hecho} conMesa={datos?.orden.tipo === "MESA"} volver={volver} />;

  return (
    <section className="cobro" data-testid="cobro">
      <div className="cobro__detalle">
        <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={volver}>
          <ArrowLeft aria-hidden="true" /> Volver <kbd className="tecla">Esc</kbd>
        </button>
        <h1 className="rp-large-title">{datos?.orden.mesa ?? aCobrar.nombre}</h1>
        <p className="rp-secondary">
          {datos ? `Orden #${datos.orden.numero} · ${datos.orden.meseroNombre}` : error ? "" : "Cargando…"}
        </p>
        {datos && datos.orden.tipo !== "MESA" && agregar && !hecho && (
          <button className="rp-btn rp-btn--gray rp-btn--sm cobro__agregar" onClick={() => agregar(datos.orden)} data-testid="agregar-platos">
            Agregar platos <kbd className="tecla">A</kbd>
          </button>
        )}
        {datos && (
          <div className="rp-group cobro__lineas">
            {datos.orden.lineas
              .filter((l) => l.estado !== "ANULADA")
              .map((l) => (
                <div className="rp-cell" key={l.id}>
                  <span className="rp-num cobro__cant">{l.cantidad}</span>
                  <span className="rp-cell__body">
                    <span className="rp-cell__title">{l.producto}</span>
                    {l.modificadores.length > 0 && <span className="rp-cell__subtitle">{l.modificadores.map((m) => m.nombre).join(", ")}</span>}
                  </span>
                  <span className="rp-cell__value rp-num">{formatMoney(l.total)}</span>
                </div>
              ))}
          </div>
        )}
        {datos && (
          <dl className="cobro__totales">
            <dt>Subtotal</dt>
            <dd className="rp-num">{formatMoney(datos.totales.subtotal)}</dd>
            <dt>IVA</dt>
            <dd className="rp-num">{formatMoney(datos.totales.iva)}</dd>
            {centavos(datos.totales.propina) > 0 && (
              <>
                <dt>Servicio</dt>
                <dd className="rp-num">{formatMoney(datos.totales.propina)}</dd>
              </>
            )}
          </dl>
        )}
      </div>

      <div className="cobro__panel">
        <p className="cobro__rotulo">Total</p>
        <p className="cobro__total rp-num" data-testid="total">
          {formatMoney(total)}
        </p>
        <button className="cobro__cf" aria-pressed={!excedeCF} disabled={excedeCF} data-testid="consumidor-final">
          <UserRound aria-hidden="true" />
          Consumidor final
        </button>
        {excedeCF && (
          <p className="aviso-error">
            Supera {formatMoney(maximo)}, el máximo para consumidor final. Hacen falta los datos del comprador (llegan con la facturación).
          </p>
        )}
        {!turno && (
          <p className="cobro__aviso">
            Esta caja no tiene turno abierto.{" "}
            <button className="rp-btn rp-btn--tinted rp-btn--sm" onClick={irATurno}>
              Abrir turno
            </button>
          </p>
        )}
        {error && (
          <p className="aviso-error" role="alert">
            {error}
          </p>
        )}

        <p className="cobro__rotulo">Efectivo</p>
        <div className="billetes" role="group" aria-label="Cobrar en efectivo">
          {billetes.map((b, i) => (
            <button key={b} className="billete" disabled={!puedeCobrar || !efectivo} onClick={() => cobrar(efectivo, b)} data-testid={`billete-${b}`}>
              <span className="billete__monto rp-num">{formatMoney(b)}</span>
              <small>{i === 0 ? "Exacto" : `Vuelto ${formatMoney(verCentavos(vueltoCentavos(total, b)))}`}</small>
              <kbd className="tecla tecla--esquina">{i + 1}</kbd>
            </button>
          ))}
          <button className="billete billete--otro" disabled={!puedeCobrar || !efectivo} onClick={() => setOtroMonto("")} data-testid="otro-monto">
            <span className="billete__monto">Otro</span>
            <small>monto</small>
            <kbd className="tecla tecla--esquina">O</kbd>
          </button>
        </div>

        {otros.length > 0 && (
          <>
            <p className="cobro__rotulo">Otros métodos · total exacto</p>
            <div className="metodos">
              {otros.map((m) => {
                const Icono = ICONO[m.icono] ?? Wallet;
                return (
                  <button key={m.id} className="rp-btn rp-btn--gray metodo" disabled={!puedeCobrar} onClick={() => cobrar(m, "")} data-testid={`metodo-${m.nombre}`}>
                    <Icono aria-hidden="true" /> {m.nombre}
                  </button>
                );
              })}
              <button className="rp-btn rp-btn--tinted metodo" disabled={!puedeCobrar} onClick={() => setHojaPago({})} data-testid="pago-mixto">
                Pago mixto <kbd className="tecla">M</kbd>
              </button>
            </div>
          </>
        )}
      </div>

      {hojaPago && config && (
        <HojaPagos
          total={total}
          metodos={config.metodos}
          inicial={hojaPago.inicial}
          ocupado={ocupado}
          error={error}
          cerrar={() => {
            setHojaPago(null);
            setError(null);
          }}
          cobrar={(pagos) => enviarCobro({ pagos })}
        />
      )}
      {otroMonto !== null && (
        <OtroMonto
          total={total}
          valor={otroMonto}
          cambiar={setOtroMonto}
          cerrar={() => setOtroMonto(null)}
          cobrar={(r) => {
            setOtroMonto(null);
            cobrar(efectivo, r);
          }}
        />
      )}
    </section>
  );
}

function OtroMonto({
  total,
  valor,
  cambiar,
  cerrar,
  cobrar,
}: {
  total: string;
  valor: string;
  cambiar: Dispatch<SetStateAction<string | null>>;
  cerrar: () => void;
  cobrar: (recibido: string) => void;
}) {
  const recibido = montoDe(valor);
  const vuelto = vueltoCentavos(total, recibido);
  const alcanza = vuelto >= 0;
  useAtajo("Escape", "Cancelar", cerrar, "Cobro");
  useAtajo("Enter", "Cobrar", () => alcanza && cobrar(recibido), "Cobro");
  const set = useCallback((f: string | ((v: string) => string)) => cambiar((v) => (typeof f === "function" ? f(v ?? "") : f)), [cambiar]);
  return (
    <div className="rp-scrim" data-open="true" onClick={cerrar}>
      <div className="rp-sheet hoja-caja" role="dialog" aria-modal="true" aria-label="Otro monto" onClick={(e) => e.stopPropagation()}>
        <div className="rp-sheet__header">
          <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={cerrar}>
            Cancelar
          </button>
          <h2 className="rp-sheet__title">Recibido</h2>
          <span />
        </div>
        <div className="rp-sheet__body">
          <p className="hoja-caja__ayuda">Total {formatMoney(total)}</p>
          <TecladoMonto valor={valor} cambiar={set} etiqueta="Recibido" />
          <p className="hoja-caja__ayuda rp-num" data-testid="vuelto-previo">
            {alcanza ? `Vuelto ${formatMoney(verCentavos(vuelto))}` : `Faltan ${formatMoney(verCentavos(-vuelto))}`}
          </p>
          <div className="hoja-caja__pie">
            <button className="rp-btn rp-btn--primary rp-btn--block" disabled={!alcanza} onClick={() => cobrar(recibido)} data-testid="cobrar-otro">
              Cobrar
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function Listo({ out, conMesa, volver }: { out: CobroOut; conMesa: boolean; volver: () => void }) {
  const d = out.documento;
  const vuelto = useConteo(centavos(d.vuelto));
  // Caja libre para el siguiente cliente: vuelve sola a las mesas.
  useEffect(() => {
    const t = setTimeout(volver, 6000);
    return () => clearTimeout(t);
  }, [volver]);
  return (
    <section className="cobro-listo" data-testid="cobro-listo">
      <span className="cobro-listo__check">
        <Check aria-hidden="true" />
      </span>
      {centavos(d.vuelto) > 0 ? (
        <>
          <p className="cobro__rotulo">Vuelto</p>
          <p className="cobro__total rp-num" aria-label={`Vuelto ${formatMoney(d.vuelto)}`} data-testid="vuelto">
            {formatMoney(verCentavos(vuelto))}
          </p>
          <p className="rp-secondary">
            {d.pagos.length > 1
              ? `Recibido ${formatMoney(d.recibido)} para ${formatMoney(d.pagos.find((p) => p.tipo === "EFECTIVO")?.monto ?? d.totales.total)} en efectivo`
              : `Recibido ${formatMoney(d.recibido)} · total ${formatMoney(d.totales.total)}`}
          </p>
        </>
      ) : (
        <p className="cobro__total rp-num">{formatMoney(d.totales.total)}</p>
      )}
      {d.pagos.length > 1 && (
        <p className="rp-secondary rp-num" data-testid="detalle-pagos">
          {d.pagos.map((p) => `${p.metodo} ${formatMoney(p.monto)}`).join(" · ")}
        </p>
      )}
      <p className="rp-secondary">
        {d.metodo} · {d.codigo} · {d.mesa} {conMesa ? "libre" : "cobrada"}
        {d.abreCajon ? " · cajón abierto" : ""}
      </p>
      {out.aviso && <p className="cobro__aviso">{out.aviso}</p>}
      <button className="rp-btn rp-btn--primary" onClick={volver} autoFocus data-testid="siguiente-cliente">
        Siguiente cliente <kbd className="tecla">Intro</kbd>
      </button>
    </section>
  );
}
