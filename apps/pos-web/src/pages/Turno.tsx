// Jornada (F4-02), turno de caja (F4-03) y movimientos de efectivo (F4-11). La pantalla nunca
// muestra el efectivo esperado: el cierre es ciego (F4-12).
import { formatHour, formatMoney } from "@restpos/ui";
import { ArrowDownToLine, ArrowUpFromLine, CalendarCheck, CalendarX, Inbox, Receipt, Wallet } from "lucide-react";
import { type FormEvent, type ReactNode, useCallback, useEffect, useState } from "react";
import { uuidv7 } from "../api/identidad";
import { ApiError, type ConfigCaja, type EstadoCaja, type Movimiento, nodo, type TipoMovimiento } from "../api/nodo";
import { useSesion } from "../api/sesion";
import { useAtajo } from "../components/atajos";
import { Supervisor } from "../components/Supervisor";
import { montoDe, TecladoMonto } from "../components/TecladoMonto";
import { CierreTurno } from "./CierreTurno";

const CLAVE_CAJA = "restpos.caja";

function cajaGuardada(): string | null {
  try {
    return localStorage.getItem(CLAVE_CAJA);
  } catch {
    return null;
  }
}

function guardarCaja(id: string) {
  try {
    localStorage.setItem(CLAVE_CAJA, id);
  } catch {
    // sin almacenamiento: se vuelve a elegir al recargar
  }
}

/** La caja de esta PC: la que se eligió antes o, si el local tiene una sola, esa. */
export function cajaDeEstaPC(cajas: { id: string }[], guardada: string | null): string | null {
  if (guardada && cajas.some((c) => c.id === guardada)) return guardada;
  return cajas.length === 1 ? cajas[0]!.id : null;
}

export interface Caja {
  config: ConfigCaja | null;
  cajaId: string | null;
  elegir: (id: string) => void;
  estado: EstadoCaja | null;
  error: string | null;
  recargar: () => void;
}

export function useCaja(): Caja {
  const { tiempoReal } = useSesion();
  const [config, setConfig] = useState<ConfigCaja | null>(null);
  const [cajaId, setCajaId] = useState<string | null>(null);
  const [estado, setEstado] = useState<EstadoCaja | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    nodo
      .configCaja()
      .then((c) => {
        setConfig(c);
        setCajaId(cajaDeEstaPC(c.cajas, cajaGuardada()));
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo leer la configuración de caja."));
  }, []);

  const recargar = useCallback(() => {
    if (!cajaId) return;
    nodo
      .estadoCaja(cajaId)
      .then((e) => {
        setEstado(e);
        setError(null);
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo leer el estado de la caja."));
  }, [cajaId]);

  useEffect(() => {
    recargar();
    // El primer pedido de un mesero abre la jornada: se vuelve a leer con cada orden.
    const off = tiempoReal.alEvento((e) => e.type === "order.submitted" && recargar());
    const offConexion = tiempoReal.alCambiarConexion((v) => v && recargar());
    return () => {
      off();
      offConexion();
    };
  }, [recargar, tiempoReal]);

  const elegir = useCallback((id: string) => {
    guardarCaja(id);
    setEstado(null);
    setCajaId(id);
  }, []);

  return { config, cajaId, elegir, estado, error, recargar };
}

const fechaLarga = new Intl.DateTimeFormat("es-EC", { weekday: "long", day: "numeric", month: "long", timeZone: "UTC" });

/** "2026-09-25" → "viernes, 25 de septiembre" (la fecha de negocio no tiene hora ni zona). */
export function verFecha(fecha: string): string {
  return fechaLarga.format(new Date(fecha + "T00:00:00Z"));
}

const MOVIMIENTO: Record<TipoMovimiento, { titulo: string; ayuda: string; signo: string }> = {
  RETIRO: { titulo: "Retiro a caja fuerte", ayuda: "Efectivo que sale del cajón hacia la caja fuerte.", signo: "−" },
  INGRESO: { titulo: "Ingreso de efectivo", ayuda: "Dinero que entra y no es una venta (por ejemplo, cambio de monedas).", signo: "+" },
  GASTO: { titulo: "Gasto menor", ayuda: "Compra pagada con el efectivo del cajón (hielo, gas, un repuesto).", signo: "−" },
};

type Hoja = { tipo: "abrir" } | { tipo: "movimiento"; mov: TipoMovimiento } | { tipo: "cerrarJornada" } | { tipo: "cerrarTurno" } | { tipo: "cajon" } | null;

export function Turno({ caja }: { caja: Caja }) {
  const { puede, usuario } = useSesion();
  const { config, cajaId, elegir, estado, error, recargar } = caja;
  const [hoja, setHoja] = useState<Hoja>(null);
  const [movs, setMovs] = useState<Movimiento[]>([]);
  const [aviso, setAviso] = useState<string | null>(null);
  const autorizado = puede("GESTIONAR_TURNO");
  const turno = estado?.turno ?? null;

  useEffect(() => {
    if (!turno) return setMovs([]);
    nodo
      .movimientos(turno.id)
      .then(setMovs)
      .catch(() => setMovs([]));
  }, [turno]);

  const hecho = (msg: string) => {
    setHoja(null);
    setAviso(msg);
    recargar();
  };

  const libre = hoja === null && autorizado;
  useAtajo("F3", "Abrir el turno de caja", () => setHoja({ tipo: "abrir" }), "Turno", libre && !!estado && !turno);
  useAtajo("F6", "Retiro a caja fuerte", () => setHoja({ tipo: "movimiento", mov: "RETIRO" }), "Turno", libre && !!turno);
  useAtajo("F7", "Ingreso de efectivo", () => setHoja({ tipo: "movimiento", mov: "INGRESO" }), "Turno", libre && !!turno);
  useAtajo("F8", "Gasto menor", () => setHoja({ tipo: "movimiento", mov: "GASTO" }), "Turno", libre && !!turno);
  useAtajo("F4", "Cerrar el turno (cierre ciego)", () => setHoja({ tipo: "cerrarTurno" }), "Turno", libre && !!turno);
  useAtajo("k", "Abrir el cajón sin venta", () => setHoja({ tipo: "cajon" }), "Turno", hoja === null && !!cajaId);

  if (config && !cajaId) {
    return (
      <section className="turno">
        <h1 className="rp-large-title">¿Qué caja es esta PC?</h1>
        <p className="rp-secondary">Se recuerda en esta computadora. Las cajas se crean en el panel web, en Caja.</p>
        {config.cajas.length === 0 && <p className="aviso-error">El local no tiene cajas activas.</p>}
        <div className="gente">
          {config.cajas.map((c) => (
            <button key={c.id} className="persona" onClick={() => elegir(c.id)} data-testid={`elegir-${c.nombre}`}>
              <Wallet aria-hidden="true" />
              <b>{c.nombre}</b>
            </button>
          ))}
        </div>
      </section>
    );
  }

  return (
    <section className="turno">
      <div className="turno__cabecera">
        <h1 className="rp-large-title">{estado?.caja.nombre ?? "Caja"}</h1>
        <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => setHoja({ tipo: "cajon" })} disabled={!cajaId} data-testid="abrir-cajon">
          <Inbox aria-hidden="true" /> Abrir cajón <kbd className="tecla">K</kbd>
        </button>
        {config && config.cajas.length > 1 && (
          <div className="rp-segmented" role="radiogroup" aria-label="Caja de esta PC">
            {config.cajas.map((c) => (
              <label key={c.id}>
                <input type="radio" name="caja" checked={cajaId === c.id} onChange={() => elegir(c.id)} />
                <span>{c.nombre}</span>
              </label>
            ))}
          </div>
        )}
      </div>
      {error && <p className="aviso-error">{error}</p>}
      {aviso && (
        <p className="rp-secondary" role="status">
          {aviso}
        </p>
      )}
      {!autorizado && <p className="rp-secondary">Tu usuario no abre turnos ni mueve efectivo. Pide a quien esté a cargo de la caja.</p>}

      <div className="turno__tarjetas">
        <article className="tarjeta" data-testid="jornada">
          <header className="tarjeta__cabecera">
            {estado?.jornada ? <CalendarCheck aria-hidden="true" /> : <CalendarX aria-hidden="true" />}
            <h2 className="rp-t-headline">Jornada</h2>
            <span className={`rp-status ${estado?.jornada ? "rp-status--success" : "rp-status--neutral"}`}>{estado?.jornada ? "Abierta" : "Sin abrir"}</span>
          </header>
          {estado?.jornada ? (
            <>
              <p className="tarjeta__dato">{verFecha(estado.jornada.fechaNegocio)}</p>
              <p className="rp-secondary">Desde las {formatHour(new Date(estado.jornada.abiertaAt))}. Si pasa la medianoche, las ventas siguen en este día.</p>
              <button className="rp-btn rp-btn--gray" disabled={!autorizado} onClick={() => setHoja({ tipo: "cerrarJornada" })}>
                Cerrar jornada
              </button>
            </>
          ) : (
            <>
              <p className="rp-secondary">Se abre sola con el primer pedido o al abrir un turno.</p>
              <button
                className="rp-btn rp-btn--tinted"
                disabled={!autorizado}
                onClick={() =>
                  nodo
                    .abrirJornada()
                    .then(() => hecho("Jornada abierta."))
                    .catch((e) => setAviso(e instanceof ApiError ? e.message : "No se pudo abrir la jornada."))
                }
              >
                Abrir jornada
              </button>
            </>
          )}
        </article>

        <article className="tarjeta" data-testid="turno">
          <header className="tarjeta__cabecera">
            <Wallet aria-hidden="true" />
            <h2 className="rp-t-headline">Turno de caja</h2>
            <span className={`rp-status ${turno ? "rp-status--success" : "rp-status--warning"}`}>{turno ? "Abierto" : "Cerrado"}</span>
          </header>
          {turno ? (
            <>
              <p className="tarjeta__dato">{turno.cajeroNombre}</p>
              <p className="rp-secondary">
                Desde las {formatHour(new Date(turno.abiertoAt))} · fondo inicial <span className="rp-num">{formatMoney(turno.fondoInicial)}</span>
              </p>
              <div className="turno__acciones">
                {(Object.keys(MOVIMIENTO) as TipoMovimiento[]).map((t, i) => (
                  <button key={t} className="rp-btn rp-btn--gray" disabled={!autorizado} onClick={() => setHoja({ tipo: "movimiento", mov: t })} data-testid={`mov-${t}`}>
                    {t === "INGRESO" ? <ArrowDownToLine aria-hidden="true" /> : t === "RETIRO" ? <ArrowUpFromLine aria-hidden="true" /> : <Receipt aria-hidden="true" />}
                    {t === "RETIRO" ? "Retiro" : t === "INGRESO" ? "Ingreso" : "Gasto"}
                    <kbd className="tecla">F{6 + i}</kbd>
                  </button>
                ))}
              </div>
              <button className="rp-btn rp-btn--tinted" disabled={!autorizado} onClick={() => setHoja({ tipo: "cerrarTurno" })} data-testid="cerrar-turno">
                Cerrar turno <kbd className="tecla">F4</kbd>
              </button>
            </>
          ) : (
            <>
              <p className="rp-secondary">Sin turno abierto no se cobra en esta caja.</p>
              <button className="rp-btn rp-btn--primary" disabled={!autorizado || !estado} onClick={() => setHoja({ tipo: "abrir" })} data-testid="abrir-turno">
                Abrir turno <kbd className="tecla">F3</kbd>
              </button>
            </>
          )}
        </article>
      </div>

      {turno && (
        <>
          <h3 className="rp-section-header">Movimientos del turno</h3>
          {movs.length === 0 ? (
            <p className="rp-secondary">Todavía no hay retiros, ingresos ni gastos.</p>
          ) : (
            <div className="rp-group movimientos" data-testid="movimientos">
              {movs.map((m) => (
                <div className="rp-cell" key={m.id}>
                  <span className="rp-cell__body">
                    <span className="rp-cell__title">{m.motivo}</span>
                    <span className="rp-cell__subtitle">
                      {MOVIMIENTO[m.tipo].titulo} · {m.usuarioNombre} · {formatHour(new Date(m.createdAt))}
                    </span>
                  </span>
                  <span className="rp-cell__value rp-num">
                    {MOVIMIENTO[m.tipo].signo}
                    {formatMoney(m.monto)}
                  </span>
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {hoja?.tipo === "abrir" && cajaId && (
        <HojaAbrirTurno cajaId={cajaId} nombre={estado?.caja.nombre ?? ""} cerrar={() => setHoja(null)} listo={() => hecho("Turno abierto. Ya puedes cobrar en esta caja.")} />
      )}
      {hoja?.tipo === "movimiento" && cajaId && (
        <HojaMovimiento cajaId={cajaId} tipo={hoja.mov} cerrar={() => setHoja(null)} listo={(m) => hecho(`${MOVIMIENTO[m.tipo].titulo} de ${formatMoney(m.monto)} registrado.`)} />
      )}
      {hoja?.tipo === "cerrarTurno" && cajaId && config && <CierreTurno cajaId={cajaId} config={config} cerrar={() => setHoja(null)} listo={(r) => {
            setAviso(`Turno cerrado con el Cierre Z ${String(r.cierre.numero).padStart(4, "0")}. Abre un turno nuevo para volver a cobrar.`);
            recargar();
          }} />}
      {hoja?.tipo === "cajon" && cajaId && (
        <HojaCajon
          cajaId={cajaId}
          conPermiso={puede("ABRIR_CAJON")}
          usuarioId={usuario?.id}
          cerrar={() => setHoja(null)}
          listo={(r) => hecho(`Cajón abierto por «${r.impresora}»${r.autorizadoPor ? `, autorizó ${r.autorizadoPor}` : ""}. Quedó registrado.${r.aviso ? ` ${r.aviso}` : ""}`)}
        />
      )}
      {hoja?.tipo === "cerrarJornada" && <AlertaCerrarJornada cerrar={() => setHoja(null)} listo={(msg) => hecho(msg)} />}
    </section>
  );
}

function Hoja({ titulo, cerrar, children }: { titulo: string; cerrar: () => void; children: ReactNode }) {
  useAtajo("Escape", "Cerrar la hoja", cerrar, "Turno");
  return (
    <div className="rp-scrim" data-open="true" onClick={cerrar}>
      <div className="rp-sheet hoja-caja" role="dialog" aria-modal="true" aria-label={titulo} onClick={(e) => e.stopPropagation()}>
        <div className="rp-sheet__grabber" />
        <div className="rp-sheet__header">
          <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={cerrar}>
            Cancelar
          </button>
          <h2 className="rp-sheet__title">{titulo}</h2>
          <span />
        </div>
        <div className="rp-sheet__body">{children}</div>
      </div>
    </div>
  );
}

function HojaAbrirTurno({ cajaId, nombre, cerrar, listo }: { cajaId: string; nombre: string; cerrar: () => void; listo: () => void }) {
  const [fondo, setFondo] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);
  const abrir = () => {
    if (ocupado) return;
    setOcupado(true);
    nodo
      .abrirTurno(cajaId, montoDe(fondo))
      .then(listo)
      .catch((e) => {
        setOcupado(false);
        setError(e instanceof ApiError ? e.message : "No se pudo abrir el turno.");
      });
  };
  useAtajo("Enter", "Confirmar", abrir, "Turno");
  return (
    <Hoja titulo={`Abrir turno · ${nombre}`} cerrar={cerrar}>
      <p className="hoja-caja__ayuda">Cuenta el efectivo con el que empieza el cajón.</p>
      <TecladoMonto valor={fondo} cambiar={setFondo} etiqueta="Fondo inicial" />
      {error && (
        <p className="aviso-error hoja-caja__ayuda" role="alert">
          {error}
        </p>
      )}
      <div className="hoja-caja__pie">
        <button className="rp-btn rp-btn--primary rp-btn--block" onClick={abrir} disabled={ocupado} data-testid="confirmar-turno">
          Abrir con {fondo ? "este fondo" : "$0.00"}
        </button>
      </div>
    </Hoja>
  );
}

function HojaMovimiento({ cajaId, tipo, cerrar, listo }: { cajaId: string; tipo: TipoMovimiento; cerrar: () => void; listo: (m: Movimiento) => void }) {
  const [monto, setMonto] = useState("");
  const [motivo, setMotivo] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);
  // Una clave por hoja: si la red falla y se reintenta, el nodo no lo registra dos veces.
  const [clave] = useState(() => uuidv7());
  const guardar = (e?: FormEvent) => {
    e?.preventDefault();
    if (ocupado) return;
    setOcupado(true);
    nodo
      .movimiento({ cajaId, tipo, monto: montoDe(monto), motivo: motivo.trim(), idempotencyKey: clave })
      .then(listo)
      .catch((err) => {
        setOcupado(false);
        setError(err instanceof ApiError ? err.message : "No se pudo registrar el movimiento.");
      });
  };
  const m = MOVIMIENTO[tipo];
  return (
    <Hoja titulo={m.titulo} cerrar={cerrar}>
      <p className="hoja-caja__ayuda">{m.ayuda}</p>
      <TecladoMonto valor={monto} cambiar={setMonto} etiqueta="Monto" />
      <form className="hoja-caja__pie" onSubmit={guardar}>
        <div className="rp-field">
          <label htmlFor="motivo">Motivo</label>
          <input id="motivo" value={motivo} onChange={(e) => setMotivo(e.target.value)} maxLength={200} placeholder="Ej.: compra de hielo" data-testid="motivo" />
        </div>
        {error && (
          <p className="aviso-error" role="alert">
            {error}
          </p>
        )}
        <button className="rp-btn rp-btn--primary rp-btn--block" disabled={ocupado || !monto || motivo.trim().length < 3} data-testid="guardar-movimiento">
          Registrar
        </button>
      </form>
    </Hoja>
  );
}

function AlertaCerrarJornada({ cerrar, listo }: { cerrar: () => void; listo: (msg: string) => void }) {
  const [error, setError] = useState<ApiError | null>(null);
  const [ocupado, setOcupado] = useState(false);
  const transferir = error?.code === "ORDENES_ABIERTAS";
  const confirmar = () => {
    setOcupado(true);
    nodo
      .cerrarJornada(transferir)
      .then(() => listo(transferir ? "Jornada cerrada. Las órdenes abiertas pasan a la próxima jornada." : "Jornada cerrada. Los meseros deben volver a entrar con su PIN."))
      .catch((e) => {
        setOcupado(false);
        setError(e instanceof ApiError ? e : new ApiError(0, "ERROR", "No se pudo cerrar la jornada."));
      });
  };
  useAtajo("Escape", "Cancelar", cerrar, "Turno");
  return (
    <div className="rp-scrim centrado-fijo" data-open="true">
      <div className="rp-alert" role="alertdialog" aria-modal="true" aria-labelledby="cerrar-jornada">
        <div className="rp-alert__body">
          <p className="rp-alert__title" id="cerrar-jornada">
            {transferir ? "Hay órdenes abiertas" : error ? "No se puede cerrar la jornada" : "¿Cerrar la jornada?"}
          </p>
          <p className="rp-alert__msg">
            {error
              ? transferir
                ? "Puedes pasarlas a la próxima jornada y cerrar igual, o volver a cobrarlas."
                : error.message
              : "Termina el día de negocio y cierra la sesión de los meseros. No se puede volver a abrir hoy."}
          </p>
        </div>
        <div className="rp-alert__actions">
          <button onClick={cerrar}>Cancelar</button>
          {(!error || transferir) && (
            <button className="rp-alert__primary rp-alert__destructive" onClick={confirmar} disabled={ocupado} data-testid="confirmar-cierre-jornada">
              {transferir ? "Transferir y cerrar" : "Cerrar"}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

/** Abrir el cajón sin venta (F4-13): motivo y, si hace falta, el PIN de un supervisor. */
function HojaCajon({
  cajaId,
  conPermiso,
  usuarioId,
  cerrar,
  listo,
}: {
  cajaId: string;
  conPermiso: boolean;
  usuarioId?: string;
  cerrar: () => void;
  listo: (r: { impresora: string; autorizadoPor?: string; aviso?: string }) => void;
}) {
  const [motivo, setMotivo] = useState("");
  const [paso, setPaso] = useState<"motivo" | "supervisor">("motivo");
  const [error, setError] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);
  const abrir = async (autorizacion = ""): Promise<string | null> => {
    try {
      listo(await nodo.abrirCajon({ cajaId, motivo: motivo.trim(), autorizacion }));
      return null;
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : "No se pudo abrir el cajón.";
      if (e instanceof ApiError && e.code === "REQUIERE_SUPERVISOR") setPaso("supervisor");
      else setError(msg);
      return msg;
    }
  };
  const continuar = (e?: FormEvent) => {
    e?.preventDefault();
    if (motivo.trim().length < 3 || ocupado) return;
    if (!conPermiso) return setPaso("supervisor");
    setOcupado(true);
    void abrir().finally(() => setOcupado(false));
  };
  return (
    <Hoja titulo="Abrir cajón sin venta" cerrar={cerrar}>
      {paso === "motivo" ? (
        <form className="hoja-caja__pie" onSubmit={continuar}>
          <p className="rp-secondary">El pulso sale por la impresora de la caja con un comprobante. Queda registrado con tu usuario.</p>
          <div className="rp-field">
            <label htmlFor="motivo-cajon">Motivo</label>
            <input id="motivo-cajon" value={motivo} onChange={(e) => setMotivo(e.target.value)} maxLength={200} placeholder="Ej.: cambio de monedas" autoFocus data-testid="motivo-cajon" />
          </div>
          {error && (
            <p className="aviso-error" role="alert">
              {error}
            </p>
          )}
          <button className="rp-btn rp-btn--primary rp-btn--block" disabled={motivo.trim().length < 3 || ocupado} data-testid="continuar-cajon">
            {conPermiso ? "Abrir cajón" : "Pedir autorización"}
          </button>
        </form>
      ) : (
        <Supervisor accion="ABRIR_CAJON" referencia={cajaId} excepto={usuarioId} alAutorizar={(token) => abrir(token)} volver={() => setPaso("motivo")} />
      )}
    </Hoja>
  );
}
