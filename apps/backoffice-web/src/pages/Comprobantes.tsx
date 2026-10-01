// Bóveda de comprobantes (F5-12, RF-05-07 y RF-05-04.5): busca cualquier factura por
// comprador, número o clave; filtra por estado y fechas; descarga el XML autorizado y el RIDE;
// reenvía el correo y reintenta lo que espera o lo que el SRI no aceptó. Los errores del SRI
// se muestran en lenguaje claro, con qué hacer.
import { useState } from "react";
import { ApiError, descargar } from "../api/client";
import { api, useComprobante, useComprobantes, useGuardar } from "../api/hooks";
import { useSession } from "../api/session";
import type { FilaComprobante, GrupoComprobante } from "../api/types";
import { useFeedback } from "../components/feedback";
import { Empty, Sheet, Spinner } from "../components/ui";
import { Icon } from "../lib/icons";

export const GRUPOS: { value: GrupoComprobante | ""; label: string; tono: string }[] = [
  { value: "", label: "Todos", tono: "neutral" },
  { value: "AUTORIZADO", label: "Autorizados", tono: "success" },
  { value: "ENVIADO", label: "Enviados", tono: "neutral" },
  { value: "REQUIERE_ATENCION", label: "Requieren atención", tono: "warning" },
  { value: "NO_AUTORIZADO", label: "No autorizados", tono: "danger" },
];

/** Nombre y tono de la píldora de estado (verde autorizado, gris enviado, naranja atención). */
export function pildora(f: Pick<FilaComprobante, "grupo" | "estado">): { texto: string; tono: string } {
  switch (f.grupo) {
    case "AUTORIZADO":
      return { texto: "Autorizado", tono: "success" };
    case "NO_AUTORIZADO":
      return { texto: "No autorizado", tono: "danger" };
    case "REQUIERE_ATENCION":
      return { texto: "Requiere atención", tono: "warning" };
    case "ANULADO":
      return { texto: "Anulado por NC", tono: "neutral" };
    default:
      return { texto: f.estado === "EN_NUBE" ? "En la nube" : "Enviado", tono: "neutral" };
  }
}

/** «1 requiere atención», «3 requieren atención» (solo el texto después del número). */
export function resumenPendiente(g: "REQUIERE_ATENCION" | "NO_AUTORIZADO" | "ENVIADO", n: number): string {
  const uno = n === 1;
  return { REQUIERE_ATENCION: uno ? "requiere atención" : "requieren atención", NO_AUTORIZADO: uno ? "no autorizado" : "no autorizados",
    ENVIADO: "esperando al SRI" }[g];
}

const dinero = (s: string) => `$${s}`;
const fecha = (iso: string) => new Date(`${iso}T12:00:00`).toLocaleDateString("es-EC", { day: "numeric", month: "short", year: "numeric" });
const fechaHora = (iso: string) => new Date(iso).toLocaleString("es-EC", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });

export function Comprobantes() {
  const [grupo, setGrupo] = useState<GrupoComprobante | "">("");
  const [texto, setTexto] = useState("");
  const [buscar, setBuscar] = useState("");
  const [desde, setDesde] = useState("");
  const [hasta, setHasta] = useState("");
  const [abierto, setAbierto] = useState<string | null>(null);
  const q = useComprobantes({ estado: grupo, q: buscar, desde, hasta });
  const primera = q.data?.pages[0];
  const filas = q.data?.pages.flatMap((p) => p.filas) ?? [];
  const pend = primera?.pendientes ?? {};

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="rp-t-large-title">Comprobantes</h1>
          <p>Todas tus facturas electrónicas: encuéntralas, descárgalas y revisa las que esperan al SRI.</p>
        </div>
      </header>

      {(pend.REQUIERE_ATENCION ?? 0) + (pend.NO_AUTORIZADO ?? 0) + (pend.ENVIADO ?? 0) > 0 && (
        <section className="pendientes-fiscales" data-testid="pendientes-fiscales">
          {(["REQUIERE_ATENCION", "NO_AUTORIZADO", "ENVIADO"] as const).map((g) =>
            pend[g] ? (
              <button key={g} type="button" className="pendiente-fiscal" data-tono={GRUPOS.find((x) => x.value === g)?.tono} onClick={() => setGrupo(g)}>
                <strong>{pend[g]}</strong>
                <span>{resumenPendiente(g, pend[g]!)}</span>
              </button>
            ) : null,
          )}
        </section>
      )}

      <section className="seccion boveda-filtros">
        <form
          className="rp-search"
          role="search"
          onSubmit={(e) => {
            e.preventDefault();
            setBuscar(texto.trim());
          }}
        >
          <Icon name="buscar" size={18} />
          <input
            aria-label="Buscar comprobantes"
            placeholder="Cliente, cédula, número o clave de acceso"
            value={texto}
            onChange={(e) => {
              setTexto(e.target.value);
              if (e.target.value === "") setBuscar("");
            }}
          />
        </form>
        <div className="rp-chips" role="group" aria-label="Estado">
          {GRUPOS.map((g) => (
            <button key={g.value} type="button" className="rp-chip" aria-pressed={grupo === g.value} onClick={() => setGrupo(g.value)}>
              {g.label}
            </button>
          ))}
        </div>
        <div className="boveda-fechas">
          <label>
            Desde <input type="date" value={desde} max={hasta || undefined} onChange={(e) => setDesde(e.target.value)} />
          </label>
          <label>
            Hasta <input type="date" value={hasta} min={desde || undefined} onChange={(e) => setHasta(e.target.value)} />
          </label>
        </div>
      </section>

      <section className="seccion">
        {q.isLoading ? (
          <Spinner />
        ) : filas.length === 0 ? (
          <Empty icono="receipt" tint="pink" titulo="Sin comprobantes" texto={buscar || grupo || desde || hasta ? "Nada coincide con la búsqueda." : "Cuando la caja facture, aquí aparecerá cada comprobante."} />
        ) : (
          <div className="rp-group" data-testid="comprobantes">
            {filas.map((f) => (
              <Fila key={f.id} f={f} onClick={() => setAbierto(f.id)} />
            ))}
          </div>
        )}
        {q.hasNextPage && (
          <button type="button" className="rp-btn rp-btn--gray rp-btn--block cargar-mas" disabled={q.isFetchingNextPage} onClick={() => void q.fetchNextPage()}>
            {q.isFetchingNextPage ? "Cargando…" : "Ver más"}
          </button>
        )}
      </section>

      {abierto && <DetalleSheet id={abierto} onClose={() => setAbierto(null)} />}
    </>
  );
}

function Fila({ f, onClick }: { f: FilaComprobante; onClick: () => void }) {
  const p = pildora(f);
  return (
    <button type="button" className="rp-cell rp-cell--button comprobante-fila" onClick={onClick} data-testid={`comprobante-${f.numero}`}>
      <span className="rp-cell__body">
        <span className="rp-cell__title">
          {f.tipo === "04" ? "Nota de crédito" : "Factura"} {f.numero}
          {f.ambiente === 1 && <small className="marca-pruebas">pruebas</small>}
        </span>
        <span className="rp-cell__subtitle">
          <span className="imp-chip pildora-movil" data-tono={p.tono}>
            {p.texto}
          </span>
          {f.comprador ?? "Consumidor final"} · {fecha(f.fechaEmision)}
          {f.explicacion && f.grupo !== "AUTORIZADO" && <span className="comprobante-por-que"> · {f.explicacion.que}</span>}
        </span>
      </span>
      <span className="comprobante-total">{dinero(f.importeTotal)}</span>
      <span className="imp-chip pildora-escritorio" data-tono={p.tono}>
        {p.texto}
      </span>
      <Icon name="chevronRight" size={18} />
    </button>
  );
}

function DetalleSheet({ id, onClose }: { id: string; onClose: () => void }) {
  const { puede } = useSession();
  const { toast } = useFeedback();
  const d = useComprobante(id);
  const [correo, setCorreo] = useState("");
  const [error, setError] = useState("");
  const reenviar = useGuardar(() => api.reenviarComprobante(id, correo.trim()), ["comprobante"]);
  const reintentar = useGuardar(() => api.reintentarComprobante(id), ["comprobantes", "comprobante"]);

  async function bajar(formato: "xml" | "pdf") {
    try {
      await descargar(`/v1/comprobantes/${id}/${formato}`);
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo descargar.", "error");
    }
  }
  async function onReenviar() {
    setError("");
    try {
      await reenviar.mutateAsync(undefined);
      toast(`Correo enviado a ${correo.trim() || d.data?.correo}`);
      setCorreo("");
    } catch (e) {
      setError(e instanceof ApiError ? (e.fields.correo ?? e.message) : "No se pudo enviar.");
    }
  }
  async function onReintentar() {
    try {
      await reintentar.mutateAsync(undefined);
      toast("Se volvió a enviar al SRI");
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo reintentar.", "error");
    }
  }

  const c = d.data;
  const p = c && pildora(c);
  const autorizado = c?.grupo === "AUTORIZADO";
  const reintentable = c && !autorizado && c.grupo !== "ANULADO" && c.estado !== "REQUIERE_ATENCION";
  return (
    <Sheet open title={c ? `${c.tipo === "04" ? "Nota de crédito" : "Factura"} ${c.numero}` : "Comprobante"} onClose={onClose}>
      {!c || !p ? (
        <Spinner />
      ) : (
        <div className="comprobante-detalle" data-testid="detalle-comprobante">
          <div className="comprobante-resumen">
            <span className="imp-chip" data-tono={p.tono}>
              {p.texto}
            </span>
            <strong className="comprobante-importe">{dinero(c.importeTotal)}</strong>
            <span>
              {c.comprador ?? "Consumidor final"}
              {c.identificacion && c.identificacion !== "9999999999999" ? ` · ${c.identificacion}` : ""}
            </span>
            <span className="rp-secondary">
              Emitida el {fecha(c.fechaEmision)}
              {c.fechaAutorizacion ? ` · autorizada el ${fechaHora(c.fechaAutorizacion)}` : ""}
              {c.ambiente === 1 ? " · ambiente de pruebas" : ""}
            </span>
            <code className="clave">{c.claveAcceso}</code>
          </div>

          {c.explicacion && !autorizado && (
            <div className="explicacion-sri" data-tono={c.explicacion.soporte ? "danger" : "warning"} data-testid="explicacion">
              <p>
                <strong>{c.explicacion.que}</strong>
              </p>
              <p>{c.explicacion.accion}</p>
              {c.mensajes.length > 0 && (
                <details>
                  <summary>Respuesta del SRI</summary>
                  {c.mensajes.map((m, i) => (
                    <p key={i} className="rp-secondary">
                      {m.Identificador} {m.Mensaje} {m.InformacionAdicional}
                    </p>
                  ))}
                </details>
              )}
            </div>
          )}

          <div className="acciones-comprobante">
            <button type="button" className="rp-btn rp-btn--gray" onClick={() => void bajar("pdf")} data-testid="bajar-pdf" disabled={c.estado === "REQUIERE_ATENCION"}>
              <Icon name="descargar" size={16} /> RIDE (PDF)
            </button>
            <button type="button" className="rp-btn rp-btn--gray" onClick={() => void bajar("xml")} data-testid="bajar-xml" disabled={!autorizado}>
              <Icon name="descargar" size={16} /> XML autorizado
            </button>
            {reintentable && puede("CONFIGURAR_SRI") && (
              <button type="button" className="rp-btn rp-btn--primary" onClick={() => void onReintentar()} disabled={reintentar.isPending} data-testid="reintentar">
                <Icon name="recargar" size={16} /> {reintentar.isPending ? "Reintentando…" : "Reintentar"}
              </button>
            )}
          </div>

          {autorizado && (
            <div className="reenviar">
              <label className="rp-field">
                <span className="rp-field__label">Reenviar por correo</span>
                <input type="email" inputMode="email" placeholder={c.correo ?? "correo@ejemplo.com"} value={correo} onChange={(e) => setCorreo(e.target.value)} aria-label="Correo para reenviar" />
              </label>
              {error && (
                <p className="rp-field__error" role="alert">
                  {error}
                </p>
              )}
              <button type="button" className="rp-btn rp-btn--gray" disabled={reenviar.isPending || (!correo.trim() && !c.correo)} onClick={() => void onReenviar()} data-testid="reenviar">
                <Icon name="correo" size={16} /> {reenviar.isPending ? "Enviando…" : correo.trim() ? "Enviar" : "Reenviar al cliente"}
              </button>
              {c.correos.length > 0 && (
                <p className="rp-secondary">
                  {c.correos[0]!.ok ? "Último envío" : "Último intento fallido"}: {c.correos[0]!.destino}, {fechaHora(c.correos[0]!.fecha)}
                </p>
              )}
            </div>
          )}

          <h3 className="historia-titulo">Historia</h3>
          <ol className="historia" data-testid="historia">
            {c.eventos.map((e, i) => (
              <li key={i}>
                <span>{PASOS[e.estado] ?? e.estado}</span>
                {e.detalle.reintentoManual ? <small> (reintento manual)</small> : null}
                <time>{fechaHora(e.fecha)}</time>
              </li>
            ))}
          </ol>
        </div>
      )}
    </Sheet>
  );
}

/** Cada paso de la historia de un comprobante, en palabras. */
export const PASOS: Record<string, string> = {
  EN_NUBE: "Llegó a la nube",
  FIRMADO: "Firmado",
  RECIBIDO: "Recibido por el SRI",
  AUTORIZADO: "Autorizado por el SRI",
  NO_AUTORIZADO: "No autorizado por el SRI",
  DEVUELTO: "Devuelto por el SRI",
  REQUIERE_ATENCION: "Requiere atención",
  ANULADO: "Anulado por nota de crédito",
};
