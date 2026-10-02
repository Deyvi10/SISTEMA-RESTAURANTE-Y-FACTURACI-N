// Comprobantes de la caja (F5-13, F5-18): busca facturas y notas de crédito del local por
// número, cliente o clave (funciona sin internet), muestra su estado ante el SRI y emite la
// nota de crédito total o parcial: qué platos se devuelven, el motivo, el cliente (la NC no
// admite consumidor final), cómo se devuelve el dinero y, si hace falta, el PIN de un supervisor.
import { formatMoney } from "@restpos/ui";
import { ArrowLeft, FileMinus, Printer, Search } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { uuidv7 } from "../api/identidad";
import { ApiError, type ComprobanteCaja, type DetalleComprobanteCaja, nodo, type NotaCreditoOut } from "../api/nodo";
import { useAtajo } from "../components/atajos";
import { Supervisor } from "../components/Supervisor";
import { Comprador, compradorInicial, compradorParaCobro, type EstadoComprador } from "./Comprador";
import type { Caja } from "./Turno";

/** Píldora del estado ante el SRI. */
export function estadoComprobante(c: Pick<ComprobanteCaja, "estado">): { texto: string; tono: string } {
  switch (c.estado) {
    case "AUTORIZADO":
      return { texto: "Autorizado", tono: "success" };
    case "ANULADO":
      return { texto: "Anulado por NC", tono: "neutral" };
    case "NO_AUTORIZADO":
      return { texto: "No autorizado", tono: "danger" };
    case "REQUIERE_ATENCION":
      return { texto: "Requiere atención", tono: "warning" };
    case "ENVIADO":
      return { texto: "Enviado al SRI", tono: "neutral" };
    default:
      return { texto: "Emitido en el local", tono: "neutral" };
  }
}

/** Cantidad con hasta 3 decimales sin ceros sobrantes ("2.000" → "2"). */
const cant = (s: string) => String(Number(s));

export function Comprobantes({ caja, usuarioId }: { caja: Caja; usuarioId?: string }) {
  const [texto, setTexto] = useState("");
  const [fecha, setFecha] = useState("");
  const [lista, setLista] = useState<ComprobanteCaja[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [abierto, setAbierto] = useState<string | null>(null);
  const buscar = useRef<HTMLInputElement>(null);

  const cargar = useCallback((q: string, dia = "") => {
    nodo
      .comprobantes(q, dia)
      .then((l) => {
        setLista(l);
        setError(null);
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudieron leer los comprobantes."));
  }, []);
  useEffect(() => cargar(""), [cargar]);
  useAtajo("/", "Buscar un comprobante", () => buscar.current?.focus(), "Comprobantes", !abierto);

  return (
    <section className="comprobantes" aria-label="Comprobantes">
      <form
        className="rp-search comprobantes__buscar"
        role="search"
        onSubmit={(e) => {
          e.preventDefault();
          cargar(texto.trim(), fecha);
        }}
      >
        <Search aria-hidden="true" />
        <input ref={buscar} aria-label="Buscar comprobantes" placeholder="Número, cliente o clave de acceso" value={texto} onChange={(e) => setTexto(e.target.value)} data-testid="buscar-comprobante" />
        <input
          type="date"
          aria-label="Fecha de emisión"
          className="comprobantes__fecha"
          value={fecha}
          onChange={(e) => {
            setFecha(e.target.value);
            cargar(texto.trim(), e.target.value);
          }}
          data-testid="fecha-comprobante"
        />
      </form>
      {error && (
        <p className="rp-field__error" role="alert">
          {error}
        </p>
      )}
      {!lista ? (
        <div className="spinner" role="status" aria-label="Cargando comprobantes" />
      ) : lista.length === 0 ? (
        <p className="rp-secondary comprobantes__vacio">{texto ? "Nada coincide con la búsqueda." : "Todavía no hay facturas en este local."}</p>
      ) : (
        <ul className="rp-group comprobantes__lista" data-testid="lista-comprobantes">
          {lista.map((c) => {
            const e = estadoComprobante(c);
            return (
              <li key={c.id}>
                <button className="rp-cell comprobantes__fila" onClick={() => setAbierto(c.id)} data-testid={`${c.tipo === "04" ? "nc" : "factura"}-${c.numero}`}>
                  <span className="rp-cell__body">
                    <span className="rp-cell__title">
                      {c.tipo === "04" ? "Nota de crédito" : "Factura"} {c.numero}
                    </span>
                    <span className="rp-cell__subtitle">
                      {c.comprador} · {new Date(c.emitido).toLocaleString("es-EC", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" })}
                      {c.sustento ? ` · modifica ${c.sustento}` : ""}
                    </span>
                  </span>
                  <span className="comprobantes__total">{c.tipo === "04" ? "−" : ""}{formatMoney(c.total)}</span>
                  <span className={`rp-status rp-status--${e.tono}`}>{e.texto}</span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
      {abierto && (
        <HojaComprobante
          id={abierto}
          caja={caja}
          usuarioId={usuarioId}
          cerrar={() => setAbierto(null)}
          alEmitir={() => cargar(texto.trim(), fecha)}
        />
      )}
    </section>
  );
}

type Paso = "detalle" | "nota" | "supervisor";

function HojaComprobante({ id, caja, usuarioId, cerrar, alEmitir }: { id: string; caja: Caja; usuarioId?: string; cerrar: () => void; alEmitir: () => void }) {
  const [d, setD] = useState<DetalleComprobanteCaja | null>(null);
  const [paso, setPaso] = useState<Paso>("detalle");
  const [devolver, setDevolver] = useState<Record<number, string>>({});
  const [todo, setTodo] = useState(true);
  const [motivo, setMotivo] = useState("");
  const [comprador, setComprador] = useState<EstadoComprador>({ ...compradorInicial, modo: "ID" });
  // null: todavía sin elegir (se propone efectivo); "": no se devuelve dinero.
  const [metodo, setMetodo] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);
  const [hecha, setHecha] = useState<NotaCreditoOut | null>(null);
  const [aviso, setAviso] = useState<string | null>(null);
  const clave = useRef(uuidv7());

  useEffect(() => {
    nodo
      .comprobante(id)
      .then(setD)
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo leer el comprobante."));
  }, [id]);
  const efectivo = caja.config?.metodos.find((m) => m.tipo === "EFECTIVO");
  useEffect(() => {
    if (efectivo && metodo === null) setMetodo(efectivo.id);
  }, [efectivo, metodo]);
  useAtajo("Escape", "Volver", () => (paso === "supervisor" ? setPaso("nota") : paso === "nota" ? setPaso("detalle") : cerrar()), "Comprobante");

  const disponibles = d?.lineas.filter((l) => Number(l.disponible) > 0) ?? [];
  const seleccion = Object.entries(devolver)
    .filter(([, c]) => Number(c) > 0)
    .map(([i, c]) => ({ indice: Number(i), cantidad: c }));
  const { envio: datosCliente, problema: faltaCliente } = compradorParaCobro(comprador);
  const necesitaCliente = d?.consumidorFinal ?? false;
  const listo = motivo.trim().length >= 3 && (todo || seleccion.length > 0) && (!necesitaCliente || datosCliente !== null);

  const emitir = async (autorizacion = ""): Promise<string | null> => {
    if (!d || !caja.cajaId) return "Elige la caja.";
    setOcupado(true);
    setError(null);
    try {
      const out = await nodo.notaCredito(d.id, {
        cajaId: caja.cajaId,
        lineas: todo ? [] : seleccion,
        todo,
        motivo: motivo.trim(),
        ...(necesitaCliente && datosCliente ? { comprador: datosCliente } : {}),
        ...(metodo ? { devolucion: { metodoId: metodo } } : {}),
        autorizacion,
        idempotencyKey: clave.current,
      });
      setHecha(out);
      alEmitir();
      return null;
    } catch (e) {
      if (e instanceof ApiError && e.code === "REQUIERE_SUPERVISOR") {
        setPaso("supervisor");
        return null;
      }
      const m = e instanceof ApiError ? e.message : "No se pudo emitir la nota de crédito.";
      setError(m);
      return m;
    } finally {
      setOcupado(false);
    }
  };

  const reimprimir = async () => {
    if (!d || !caja.cajaId) return;
    setOcupado(true);
    try {
      const r = await nodo.reimprimirComprobante(d.id, caja.cajaId);
      setAviso(`Se reimprimió en ${r.impresoras.join(", ")}.`);
    } catch (e) {
      setAviso(e instanceof ApiError ? e.message : "No se pudo reimprimir.");
    } finally {
      setOcupado(false);
    }
  };

  const titulo = hecha ? "Nota de crédito emitida" : paso === "nota" || paso === "supervisor" ? "Nota de crédito" : d ? `${d.tipo === "04" ? "Nota de crédito" : "Factura"} ${d.numero}` : "Comprobante";
  return (
    <div className="rp-scrim" data-open="true" onClick={cerrar}>
      <div className="rp-sheet hoja-caja hoja-comprobante" role="dialog" aria-modal="true" aria-label={titulo} onClick={(e) => e.stopPropagation()}>
        <div className="rp-sheet__header">
          <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={hecha || paso === "detalle" ? cerrar : () => setPaso(paso === "supervisor" ? "nota" : "detalle")}>
            {hecha || paso === "detalle" ? "Cerrar" : (
              <>
                <ArrowLeft aria-hidden="true" /> Atrás
              </>
            )}
          </button>
          <h2 className="rp-sheet__title">{titulo}</h2>
          <span />
        </div>
        <div className="rp-sheet__body">
          {!d ? (
            error ? <p className="rp-field__error">{error}</p> : <div className="spinner" role="status" aria-label="Cargando" />
          ) : hecha ? (
            <div className="nc-hecha" data-testid="nc-hecha">
              <p className="nc-hecha__numero">
                Nota de crédito <strong>{hecha.numero}</strong> por <strong>{formatMoney(hecha.valor)}</strong>
              </p>
              <p>{hecha.revierteTodo ? `La factura ${d.numero} quedó revertida por completo.` : `Queda saldo de la factura ${d.numero}.`}</p>
              {hecha.devuelto && (
                <p className="nc-hecha__devuelto" data-testid="nc-devuelto">
                  Devuelve {formatMoney(hecha.devuelto)} en {hecha.metodo?.toLowerCase()}.
                </p>
              )}
              {hecha.autorizadoPor && <p className="rp-secondary">Autorizó {hecha.autorizadoPor}.</p>}
              <p className="rp-secondary">
                {hecha.impresoras.length ? `Se imprimió en ${hecha.impresoras.join(", ")}.` : hecha.aviso} La nube la envía al SRI en cuanto su factura esté autorizada.
              </p>
              <button className="rp-btn rp-btn--primary rp-btn--block" onClick={cerrar} autoFocus>
                Listo
              </button>
            </div>
          ) : paso === "supervisor" ? (
            <Supervisor accion="EMITIR_NC" referencia={d.id} excepto={usuarioId} alAutorizar={(token) => emitir(token)} volver={() => setPaso("nota")} />
          ) : paso === "nota" ? (
            <form
              className="nc"
              onSubmit={(e) => {
                e.preventDefault();
                if (listo && !ocupado) void emitir();
              }}
            >
              <fieldset className="nc__alcance">
                <legend>Qué se revierte</legend>
                <label className="nc__opcion">
                  <input type="radio" checked={todo} onChange={() => setTodo(true)} data-testid="nc-todo" /> Toda la factura ({formatMoney(d.saldo ?? d.total)})
                </label>
                <label className="nc__opcion">
                  <input type="radio" checked={!todo} onChange={() => setTodo(false)} data-testid="nc-parcial" /> Solo algunos platos
                </label>
                {!todo && (
                  <ul className="nc__lineas">
                    {disponibles.map((l) => (
                      <li key={l.indice}>
                        <span>{l.descripcion}</span>
                        <input
                          aria-label={`Cantidad a devolver de ${l.descripcion}`}
                          type="number"
                          min={0}
                          max={Number(l.disponible)}
                          step="any"
                          inputMode="decimal"
                          value={devolver[l.indice] ?? ""}
                          placeholder="0"
                          onChange={(e) => setDevolver((x) => ({ ...x, [l.indice]: e.target.value }))}
                          data-testid={`nc-cantidad-${l.indice}`}
                        />
                        <small>de {cant(l.disponible)}</small>
                      </li>
                    ))}
                  </ul>
                )}
              </fieldset>
              <label className="rp-field">
                <span>Motivo</span>
                <input value={motivo} maxLength={300} onChange={(e) => setMotivo(e.target.value)} placeholder="Ej.: el plato llegó frío" data-testid="nc-motivo" autoFocus />
              </label>
              {necesitaCliente && (
                <div className="nc__cliente">
                  <p className="rp-secondary">La factura fue a consumidor final, pero la nota de crédito exige identificar al cliente (SRI).</p>
                  <Comprador estado={comprador} cambiar={setComprador} cfPermitido={false} alConfirmar={() => listo && void emitir()} />
                  {faltaCliente && comprador.identificacion && <p className="cobro__aviso">{faltaCliente}</p>}
                </div>
              )}
              <label className="rp-field">
                <span>Devolver el dinero</span>
                <select value={metodo ?? ""} onChange={(e) => setMetodo(e.target.value)} data-testid="nc-devolucion">
                  {caja.config?.metodos.map((m) => (
                    <option key={m.id} value={m.id}>
                      En {m.nombre.toLowerCase()}
                    </option>
                  ))}
                  <option value="">No se devuelve dinero</option>
                </select>
              </label>
              {metodo && !caja.estado?.turno && <p className="cobro__aviso">Para devolver dinero, abre el turno de la caja.</p>}
              {error && (
                <p className="rp-field__error" role="alert">
                  {error}
                </p>
              )}
              <button className="rp-btn rp-btn--primary rp-btn--block" type="submit" disabled={!listo || ocupado} data-testid="nc-emitir">
                {ocupado ? "Emitiendo…" : "Emitir nota de crédito"}
              </button>
            </form>
          ) : (
            <div className="comprobante-caja" data-testid="detalle-comprobante">
              <p className="comprobante-caja__total">{formatMoney(d.total)}</p>
              <p>
                {d.comprador} · {d.identificacion}
              </p>
              <p className="rp-secondary">
                {estadoComprobante(d).texto}
                {d.mensaje && d.estado !== "AUTORIZADO" ? ` · ${d.mensaje}` : ""}
                {d.ambiente === 1 ? " · ambiente de pruebas" : ""}
              </p>
              <code className="comprobante-caja__clave">{d.claveAcceso}</code>
              {d.fechaAutorizacion && (
                <p className="rp-secondary" data-testid="autorizado-el">
                  Autorizado el {new Date(d.fechaAutorizacion).toLocaleString("es-EC", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" })}
                </p>
              )}
              <button className="rp-btn rp-btn--gray rp-btn--block" disabled={ocupado} onClick={() => void reimprimir()} data-testid="reimprimir">
                <Printer aria-hidden="true" /> Reimprimir el RIDE
              </button>
              {aviso && (
                <p className="rp-secondary" role="status">
                  {aviso}
                </p>
              )}
              {d.tipo === "01" && (
                <>
                  <ul className="comprobante-caja__lineas">
                    {d.lineas.map((l) => (
                      <li key={l.indice}>
                        <span>
                          {cant(l.cantidad)} × {l.descripcion}
                        </span>
                        <span>{formatMoney(l.total)}</span>
                        {Number(l.disponible) < Number(l.cantidad) && <small> (devuelto {cant(String(Number(l.cantidad) - Number(l.disponible)))})</small>}
                      </li>
                    ))}
                  </ul>
                  {d.notasCredito.length > 0 && (
                    <p className="rp-secondary">
                      Notas de crédito: {d.notasCredito.map((n) => `${n.numero} (${formatMoney(n.total)})`).join(", ")}
                    </p>
                  )}
                  {Number(d.saldo ?? "0") > 0 ? (
                    <button className="rp-btn rp-btn--gray rp-btn--block" onClick={() => setPaso("nota")} data-testid="abrir-nc">
                      <FileMinus aria-hidden="true" /> Nota de crédito
                    </button>
                  ) : (
                    <p className="rp-secondary">Esta factura ya está revertida por completo.</p>
                  )}
                </>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
