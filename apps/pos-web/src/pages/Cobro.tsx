// Cobro «Zero-Click» (F4-05): total gigante, billetes dinámicos y un solo toque para cobrar,
// dar el vuelto, abrir el cajón, emitir el documento y liberar la mesa.
import { formatMoney } from "@restpos/ui";
import { ArrowLeft, Check, CreditCard, Landmark, Smartphone, Wallet } from "lucide-react";
import { type Dispatch, type SetStateAction, useCallback, useEffect, useRef, useState } from "react";
import { uuidv7 } from "../api/identidad";
import { ApiError, type CobroOut, type DescuentoAplicado, type Division as DivisionT, type MetodoPago, nodo, type Orden, type Totales } from "../api/nodo";
import { useAtajo } from "../components/atajos";
import { montoDe, TecladoMonto } from "../components/TecladoMonto";
import { opcionesBillete, vueltoCentavos } from "../lib/billetes";
import { centavos, verCentavos } from "../lib/dinero";
import { HojaDescuento } from "./Descuento";
import { Division } from "./Division";
import { vibrar } from "../lib/dispositivo";
import { Comprador, compradorInicial, compradorParaCobro, esConsumidorFinal, type EstadoComprador, validar } from "./Comprador";
import { HojaPagos, type PagoEnvio } from "./PagoMixto";
import type { Caja } from "./Turno";

const esEfectivoM = (m: MetodoPago) => m.tipo === "EFECTIVO";

/** "10.00" → "10", "12.50" → "12.5" (texto, sin pasar por coma flotante). */
export const porcentaje = (p: string) => (p.includes(".") ? p.replace(/0+$/, "").replace(/\.$/, "") : p);

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

export function Cobro({
  orden: aCobrar,
  caja,
  volver,
  irATurno,
  agregar,
  usuarioId,
}: {
  orden: ACobrar;
  caja: Caja;
  volver: () => void;
  irATurno: () => void;
  agregar?: (o: Orden) => void;
  usuarioId?: string; // quien cobra: no se autoriza a sí mismo como supervisor
}) {
  const [datos, setDatos] = useState<{ orden: Orden; totales: Totales } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);
  const [hecho, setHecho] = useState<CobroOut | null>(null);
  const [otroMonto, setOtroMonto] = useState<string | null>(null);
  const [hojaPago, setHojaPago] = useState<{ inicial?: MetodoPago } | null>(null);
  const [comprador, setComprador] = useState<EstadoComprador>(compradorInicial);
  const [quitandoServicio, setQuitandoServicio] = useState(false);
  const [hojaDescuento, setHojaDescuento] = useState(false);
  const campoId = useRef<HTMLInputElement>(null);
  // Una clave por intento de cobro: un doble toque o un reintento no cobra dos veces.
  const clave = useRef(uuidv7());

  const [division, setDivision] = useState<DivisionT | null>(null);
  const [dividiendo, setDividiendo] = useState(false);
  const [cuentaSel, setCuentaSel] = useState<string | null>(null);
  const recargar = useCallback(() => {
    nodo
      .orden(aCobrar.ordenId)
      .then(setDatos)
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo leer la orden."));
    nodo
      .cuentas(aCobrar.ordenId)
      .then((d) => {
        setDivision(d.cuentas.length > 0 ? d : null);
        setCuentaSel((sel) => (d.cuentas.some((c) => c.id === sel && c.estado === "ABIERTA") ? sel : (d.cuentas.find((c) => c.estado === "ABIERTA")?.id ?? null)));
      })
      .catch(() => setDivision(null));
  }, [aCobrar.ordenId]);
  useEffect(recargar, [recargar]);
  const cuenta = division?.cuentas.find((c) => c.id === cuentaSel) ?? null;
  /** Parte de un plato compartido que lleva la cuenta elegida («1/3»), o null si lo lleva entero. */
  const parteDe = (linea: string) => {
    if (!cuenta || !division) return null;
    const peso = (c: { asignaciones: { lineaId: string; peso: number }[] }) => c.asignaciones.find((a) => a.lineaId === linea)?.peso ?? 0;
    const todas = division.cuentas.reduce((t, c) => t + peso(c), 0);
    const mia = peso(cuenta);
    return mia > 0 && mia < todas ? `${mia}/${todas}` : null;
  };
  // Lo que se muestra en los totales: la cuenta elegida o toda la orden.
  const vistaTotales = cuenta ? { subtotal: cuenta.subtotal, iva: cuenta.iva, propina: cuenta.propina } : datos?.totales;

  const config = caja.config;
  const turno = caja.estado?.turno ?? null;
  // Con la cuenta dividida se cobra la cuenta elegida.
  const total = cuenta ? cuenta.total : (datos?.totales.total ?? "0.00");
  const maximo = config?.consumidorFinalMaximo ?? "50.00";
  const excedeCF = centavos(total) > centavos(maximo);
  const cf = esConsumidorFinal(comprador);
  const { envio: datosComprador, problema: faltaComprador } = compradorParaCobro(comprador);
  const compradorListo = cf ? !excedeCF : datosComprador !== null;
  const efectivo = config?.metodos.find((m) => m.tipo === "EFECTIVO");
  const otros = config?.metodos.filter((m) => m.tipo !== "EFECTIVO") ?? [];
  // Toda la cuenta invitada: total en cero por cortesías, se cierra sin cobro.
  const invitada = !!datos && centavos(total) === 0 && centavos(datos.totales.descuento ?? "0") > 0;
  const puedeCobrar = !!turno && !!datos && compradorListo && (centavos(total) > 0 || invitada) && !ocupado && !hecho;

  const enviarCobro = useCallback(
    (pago: { metodoId: string; recibido: string } | { pagos: PagoEnvio[] }) => {
      if (!caja.cajaId || !datos || ocupado) return;
      setOcupado(true);
      setError(null);
      nodo
        .cobrar(datos.orden.id, {
          cajaId: caja.cajaId,
          consumidorFinal: cf,
          ...(datosComprador ? { comprador: datosComprador } : {}),
          ...(cuenta ? { cuentaId: cuenta.id } : {}),
          idempotencyKey: clave.current,
          ...pago,
        })
        .then(setHecho)
        .catch((e) => {
          setOcupado(false);
          clave.current = uuidv7();
          setError(e instanceof ApiError ? e.message : "No se pudo cobrar. Intenta de nuevo.");
        });
    },
    [caja.cajaId, datos, ocupado, cf, datosComprador, cuenta],
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
  const libre = puedeCobrar && !dividiendo && otroMonto === null && hojaPago === null && !quitandoServicio && !hojaDescuento;
  const cambiarServicio = (retirar: boolean, motivo = "") => {
    if (!datos) return;
    nodo
      .propina(datos.orden.id, retirar, motivo)
      .then((t) => {
        setDatos((d) => (d ? { ...d, totales: t } : d));
        setQuitandoServicio(false);
        if (division) recargar(); // las cuentas se recalculan en el nodo
        clave.current = uuidv7(); // el total cambió: es otro cobro
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo cambiar el servicio."));
  };
  const servicio = datos?.totales.propinaActiva && !hecho;
  const descuentosDe = (linea: string | null) => (datos?.totales.descuentos ?? []).filter((d) => d.lineaId === linea);
  /** Precio de la línea con solo su propio descuento (el de la cuenta se muestra en los totales). */
  const precioLinea = (l: { id: string; total: string }) => centavos(l.total) - descuentosDe(l.id).reduce((t, d) => t + centavos(d.monto), 0);
  const alCambiarTotales = (t: Totales) => {
    setDatos((d) => (d ? { ...d, totales: t } : d));
    if (division) recargar();
    clave.current = uuidv7(); // el total cambió: es otro cobro
  };
  const quitarDescuento = (id: string) => {
    if (!datos) return;
    nodo
      .quitarDescuento(datos.orden.id, id)
      .then(alCambiarTotales)
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo quitar el descuento."));
  };
  useAtajo("1", "Cobrar el monto exacto en efectivo", () => cobrar(efectivo, billetes[0]!), "Cobro", libre);
  useAtajo("2", "Cobrar con el segundo billete", () => billetes[1] && cobrar(efectivo, billetes[1]), "Cobro", libre && billetes.length > 1);
  useAtajo("3", "Cobrar con el tercer billete", () => billetes[2] && cobrar(efectivo, billetes[2]), "Cobro", libre && billetes.length > 2);
  useAtajo("4", "Cobrar con el cuarto billete", () => billetes[3] && cobrar(efectivo, billetes[3]), "Cobro", libre && billetes.length > 3);
  useAtajo("o", "Otro monto en efectivo", () => setOtroMonto(""), "Cobro", libre);
  useAtajo("m", "Pago mixto (varios métodos)", () => setHojaPago({}), "Cobro", libre);
  useAtajo("d", "Descuento o cortesía", () => setHojaDescuento(true), "Cobro", !!datos && !hecho && !dividiendo && otroMonto === null && hojaPago === null && !quitandoServicio && !hojaDescuento);
  useAtajo("s", "Quitar o reponer el servicio (propina)", () => (datos?.totales.propinaRetirada ? cambiarServicio(false) : setQuitandoServicio(true)), "Cobro", !!servicio && !dividiendo && otroMonto === null && hojaPago === null && !quitandoServicio);
  useAtajo("c", "Datos del comprador", () => {
    setComprador((x) => ({ ...x, modo: "ID" }));
    requestAnimationFrame(() => campoId.current?.focus());
  }, "Cobro", !hecho && !dividiendo && otroMonto === null && hojaPago === null);
  useAtajo("a", "Agregar platos a la orden", () => datos && agregar?.(datos.orden), "Cobro", !!datos && !division && !!agregar && !hecho && !dividiendo && otroMonto === null);
  useAtajo("Escape", hecho ? "Siguiente cliente" : "Volver a las mesas", volver, "Cobro", !dividiendo && otroMonto === null && hojaPago === null);
  useAtajo("Enter", hecho?.cerrada === false ? "Cobrar la siguiente cuenta" : "Siguiente cliente", () => (hecho?.cerrada === false ? siguienteCuenta() : volver()), "Cobro", !!hecho);

  const siguienteCuenta = () => {
    setHecho(null);
    setOcupado(false);
    setComprador(compradorInicial);
    clave.current = uuidv7();
    recargar();
  };
  useAtajo("v", "Dividir la cuenta", () => setDividiendo(true), "Cobro", !!datos && !hecho && !dividiendo && otroMonto === null && hojaPago === null && !hojaDescuento && !quitandoServicio);

  if (dividiendo && datos)
    return (
      <Division
        ordenId={datos.orden.id}
        lineas={datos.orden.lineas}
        totales={datos.totales}
        volver={() => setDividiendo(false)}
        listo={() => {
          setDividiendo(false);
          clave.current = uuidv7();
          recargar();
        }}
      />
    );
  if (hecho) return <Listo out={hecho} conMesa={datos?.orden.tipo === "MESA"} volver={volver} siguiente={hecho.cerrada === false ? siguienteCuenta : undefined} />;

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
        {datos && !division && agregar && !hecho && (
          <button className="rp-btn rp-btn--gray rp-btn--sm cobro__agregar" onClick={() => agregar(datos.orden)} data-testid="agregar-platos">
            Agregar platos <kbd className="tecla">A</kbd>
          </button>
        )}
        {datos && (
          <div className="rp-group cobro__lineas">
            {datos.orden.lineas
              .filter((l) => l.estado !== "ANULADA" && (!cuenta || cuenta.lineas[l.id] !== undefined))
              .map((l) => (
                <div className="rp-cell" key={l.id}>
                  <span className="rp-num cobro__cant">{l.cantidad}</span>
                  <span className="rp-cell__body">
                    <span className="rp-cell__title">{l.producto}</span>
                    {parteDe(l.id) && <span className="rp-cell__subtitle">Compartido · {parteDe(l.id)}</span>}
                    {l.modificadores.length > 0 && <span className="rp-cell__subtitle">{l.modificadores.map((m) => m.nombre).join(", ")}</span>}
                    {l.nota && <span className="rp-cell__subtitle cobro__nota">«{l.nota}»</span>}
                    {descuentosDe(l.id).map((d) => (
                      <ChipDescuento key={d.id} d={d} quitar={hecho ? undefined : () => quitarDescuento(d.id)} />
                    ))}
                  </span>
                  <span className="rp-cell__value rp-num">
                    {!cuenta && descuentosDe(l.id).length > 0 && <s className="cobro__antes">{formatMoney(l.total)}</s>}
                    {formatMoney(cuenta ? (cuenta.lineas[l.id] ?? "0") : verCentavos(precioLinea(l)))}
                  </span>
                </div>
              ))}
          </div>
        )}
        {datos && descuentosDe(null).map((d) => (
          <p className="cobro__descuento-cuenta" key={d.id}>
            <ChipDescuento d={d} quitar={hecho ? undefined : () => quitarDescuento(d.id)} />
          </p>
        ))}
        {datos && !hecho && (
          <div className="cobro__herramientas">
            <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => setHojaDescuento(true)} data-testid="descuento">
              Descuento o cortesía <kbd className="tecla">D</kbd>
            </button>
            <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => setDividiendo(true)} data-testid="dividir">
              {division ? "Cambiar la división" : "Dividir la cuenta"} <kbd className="tecla">V</kbd>
            </button>
          </div>
        )}
        {datos && (
          <dl className="cobro__totales">
            {!cuenta && centavos(datos.totales.descuento ?? "0") > 0 && (
              <>
                <dt>Descuentos</dt>
                <dd className="rp-num" data-testid="total-descuento">
                  −{formatMoney(datos.totales.descuento ?? "0")}
                </dd>
              </>
            )}
            <dt>Subtotal</dt>
            <dd className="rp-num">{formatMoney(vistaTotales?.subtotal ?? "0")}</dd>
            <dt>IVA</dt>
            <dd className="rp-num">{formatMoney(vistaTotales?.iva ?? "0")}</dd>
            {datos.totales.propinaActiva && (
              <>
                <dt className="cobro__servicio">
                  Servicio {porcentaje(datos.totales.propinaPorcentaje ?? "10")} %
                  {!hecho && (
                    <button
                      className="rp-btn rp-btn--plain rp-btn--sm"
                      onClick={() => (datos.totales.propinaRetirada ? cambiarServicio(false) : setQuitandoServicio(true))}
                      data-testid="servicio"
                    >
                      {datos.totales.propinaRetirada ? "Reponer" : "Quitar"} <kbd className="tecla">S</kbd>
                    </button>
                  )}
                </dt>
                <dd className="rp-num">{datos.totales.propinaRetirada ? <s>{formatMoney("0")}</s> : formatMoney(vistaTotales?.propina ?? "0")}</dd>
              </>
            )}
          </dl>
        )}
      </div>

      <div className="cobro__panel">
        {division && (
          <div className="rp-segmented cobro__cuentas" role="radiogroup" aria-label="Cuenta que se cobra">
            {division.cuentas.map((c) => (
              <label key={c.id}>
                <input type="radio" name="cuenta" checked={c.id === cuentaSel} disabled={c.estado === "PAGADA"} onChange={() => (setCuentaSel(c.id), setComprador(compradorInicial), (clave.current = uuidv7()))} />
                <span data-testid={`elegir-cuenta-${c.numero}`}>
                  Cuenta {c.numero} {c.estado === "PAGADA" ? "✓" : formatMoney(c.total)}
                </span>
              </label>
            ))}
          </div>
        )}
        <p className="cobro__rotulo">{cuenta ? `Total de la cuenta ${cuenta.numero}` : "Total"}</p>
        <p className="cobro__total rp-num" data-testid="total">
          {formatMoney(total)}
        </p>
        <Comprador
          ref={campoId}
          estado={comprador}
          cambiar={setComprador}
          cfPermitido={!excedeCF}
          alConfirmar={() => puedeCobrar && cobrar(efectivo, billetes[0]!)}
        />
        {cf && excedeCF && (
          <p className="aviso-error">
            Supera {formatMoney(maximo)}, el máximo para consumidor final: pulsa <kbd className="tecla">C</kbd> y escribe los datos del comprador.
          </p>
        )}
        {/* El campo ya muestra los problemas de la identificación; aquí solo lo que falta del formulario. */}
        {!cf && faltaComprador && datos && validar(comprador).valida && comprador.buscado === comprador.identificacion.trim().toUpperCase() && (
          <p className="cobro__aviso">{faltaComprador}</p>
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

        {invitada ? (
          <button className="rp-btn rp-btn--primary rp-btn--block cobro__invitada" disabled={!puedeCobrar || !efectivo} onClick={() => efectivo && enviarCobro({ metodoId: efectivo.id, recibido: "" })} data-testid="cerrar-cortesia">
            Cerrar como cortesía (sin cobro) <kbd className="tecla">1</kbd>
          </button>
        ) : (
          <>
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
          </>
        )}
      </div>

      {hojaDescuento && datos && (
        <HojaDescuento
          ordenId={datos.orden.id}
          lineas={datos.orden.lineas}
          totalPlatos={verCentavos(datos.orden.lineas.filter((l) => l.estado !== "ANULADA").reduce((t, l) => t + precioLinea(l), 0))}
          motivos={config?.motivos ?? []}
          usuarioId={usuarioId}
          cerrar={() => setHojaDescuento(false)}
          listo={(t) => {
            alCambiarTotales(t);
            setHojaDescuento(false);
          }}
        />
      )}
      {quitandoServicio && <AlertaServicio monto={datos?.totales.propina ?? "0"} cerrar={() => setQuitandoServicio(false)} quitar={(m) => cambiarServicio(true, m)} />}
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

function Listo({ out, conMesa, volver, siguiente }: { out: CobroOut; conMesa: boolean; volver: () => void; siguiente?: () => void }) {
  const d = out.documento;
  const vuelto = useConteo(centavos(d.vuelto));
  useEffect(() => vibrar(), []); // en el celular se siente que cobró
  // Caja libre para el siguiente cliente: vuelve sola a las mesas (con cuentas pendientes, no).
  useEffect(() => {
    if (siguiente) return;
    const t = setTimeout(volver, 6000);
    return () => clearTimeout(t);
  }, [volver, siguiente]);
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
      {d.compradorTipo !== "07" && (
        <p className="rp-secondary" data-testid="comprador-documento">
          {d.comprador} · {d.compradorIdentificacion}
          {d.clienteGuardado ? " · guardado para la próxima" : ""}
        </p>
      )}
      <p className="rp-secondary">
        {d.metodo} · {d.codigo} · {d.cuenta ? `cuenta ${d.cuenta} de ${d.mesa}` : d.mesa} {siguiente ? "cobrada" : conMesa ? "libre" : "cobrada"}
        {d.abreCajon ? " · cajón abierto" : ""}
      </p>
      {d.tipo === "FACTURA" && (
        <p className="rp-secondary cobro-listo__factura" data-testid="factura-emitida">
          Factura {d.codigo} emitida{d.ambiente === 1 ? " en ambiente de pruebas" : ""}; se autoriza con el SRI en segundo plano.
        </p>
      )}
      {out.aviso && <p className="cobro__aviso">{out.aviso}</p>}
      {siguiente ? (
        <button className="rp-btn rp-btn--primary" onClick={siguiente} autoFocus data-testid="siguiente-cuenta">
          Cobrar la siguiente cuenta <kbd className="tecla">Intro</kbd>
        </button>
      ) : (
        <button className="rp-btn rp-btn--primary" onClick={volver} autoFocus data-testid="siguiente-cliente">
          Siguiente cliente <kbd className="tecla">Intro</kbd>
        </button>
      )}
    </section>
  );
}

/** El cliente rechaza el servicio: se quita con un motivo opcional (queda en la auditoría). */
function AlertaServicio({ monto, cerrar, quitar }: { monto: string; cerrar: () => void; quitar: (motivo: string) => void }) {
  const [motivo, setMotivo] = useState("");
  useAtajo("Escape", "Cancelar", cerrar, "Servicio");
  return (
    <div className="rp-scrim centrado-fijo" data-open="true">
      <form
        className="rp-alert"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="quitar-servicio"
        onSubmit={(e) => {
          e.preventDefault();
          quitar(motivo.trim());
        }}
      >
        <div className="rp-alert__body">
          <p className="rp-alert__title" id="quitar-servicio">
            ¿Quitar el servicio de {formatMoney(monto)}?
          </p>
          <p className="rp-alert__msg">El cliente puede rechazarlo. Queda registrado con tu usuario.</p>
          <input className="alerta__campo" value={motivo} onChange={(e) => setMotivo(e.target.value)} maxLength={200} placeholder="Motivo (opcional)" autoFocus data-testid="motivo-servicio" />
        </div>
        <div className="rp-alert__actions">
          <button type="button" onClick={cerrar}>
            Cancelar
          </button>
          <button type="submit" className="rp-alert__primary rp-alert__destructive" data-testid="quitar-servicio">
            Quitar
          </button>
        </div>
      </form>
    </div>
  );
}

function ChipDescuento({ d, quitar }: { d: DescuentoAplicado; quitar?: () => void }) {
  const que = d.cortesia ? "Cortesía" : d.tipo === "PORCENTAJE" ? `${porcentaje(d.valor)} %` : "Descuento";
  return (
    <span className="chip-descuento" data-testid="chip-descuento">
      {que} −{formatMoney(d.monto)} · {d.motivo}
      {d.lineaId === null && " (toda la cuenta)"}
      {quitar && (
        <button type="button" className="chip-descuento__quitar" onClick={quitar} aria-label={`Quitar ${que.toLowerCase()} de ${d.motivo}`}>
          ×
        </button>
      )}
    </span>
  );
}
