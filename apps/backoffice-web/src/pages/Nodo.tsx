import { useEffect, useState } from "react";
import { ApiError } from "../api/client";
import { api, useGuardar, useLocales, useNodos } from "../api/hooks";
import type { CodigoNodo, NodoLocal } from "../api/types";
import { useFeedback } from "../components/feedback";
import { AppIcon, Spinner } from "../components/ui";
import { Icon } from "../lib/icons";

/** «hace 2 min», «hace 3 h»… para el último contacto del nodo. */
export function haceCuanto(iso: string | null, ahora = Date.now()): string {
  if (!iso) return "nunca";
  const s = Math.max(0, Math.round((ahora - new Date(iso).getTime()) / 1000));
  if (s < 60) return "hace un momento";
  if (s < 3600) return `hace ${Math.round(s / 60)} min`;
  if (s < 86400) return `hace ${Math.round(s / 3600)} h`;
  return `hace ${Math.round(s / 86400)} d`;
}

function useCuentaRegresiva(hasta: string | undefined) {
  const [ahora, setAhora] = useState(Date.now());
  useEffect(() => {
    if (!hasta) return;
    const t = setInterval(() => setAhora(Date.now()), 1000);
    return () => clearInterval(t);
  }, [hasta]);
  if (!hasta) return null;
  const s = Math.max(0, Math.round((new Date(hasta).getTime() - ahora) / 1000));
  return { vencido: s === 0, texto: `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}` };
}

function CodigoGrande({ codigo, onNuevo }: { codigo: CodigoNodo; onNuevo: () => void }) {
  const { toast } = useFeedback();
  const cuenta = useCuentaRegresiva(codigo.expiraAt);
  const [a, b] = codigo.codigo.split("-");
  return (
    <div className="codigo-nodo" data-vencido={cuenta?.vencido}>
      <div className="codigo-nodo__cajas" aria-label={`Código ${codigo.codigo.split("").join(" ")}`}>
        {[...(a ?? "")].map((c, i) => <span key={`a${i}`}>{c}</span>)}
        <i aria-hidden="true" />
        {[...(b ?? "")].map((c, i) => <span key={`b${i}`}>{c}</span>)}
      </div>
      {cuenta?.vencido ? (
        <p className="codigo-nodo__pie">Este código venció. <button type="button" className="rp-btn rp-btn--plain rp-btn--sm" onClick={onNuevo}>Generar otro</button></p>
      ) : (
        <p className="codigo-nodo__pie">
          <Icon name="tiempo" size={15} /> Vence en <b className="rp-num">{cuenta?.texto}</b> · se usa una sola vez
          <button type="button" className="rp-btn rp-btn--plain rp-btn--sm" onClick={() => void navigator.clipboard?.writeText(codigo.codigo).then(() => toast("Código copiado"))}>
            <Icon name="copiar" size={15} /> Copiar
          </button>
        </p>
      )}
    </div>
  );
}

function EstadoNodo({ n }: { n: NodoLocal }) {
  if (n.estado === "REVOCADO") return <span className="rp-status rp-status--neutral">Revocado</span>;
  return n.enLinea
    ? <span className="rp-status rp-status--success en-linea">En línea</span>
    : <span className="rp-status rp-status--warning">Sin conexión</span>;
}

function TarjetaNodo({ n, onRevocar }: { n: NodoLocal; onRevocar: () => void }) {
  const s = n.salud ?? {};
  const activo = n.estado === "ACTIVO";
  return (
    <article className="nodo-card" data-activo={activo}>
      <header>
        <AppIcon icono="servidor" tint={activo ? (n.enLinea ? "green" : "orange") : "gray"} size={48} />
        <div className="nodo-card__titulo">
          <b>{n.nombreEquipo || "PC de caja"}</b>
          <span className="rp-secondary">{n.localNombre} · versión {n.version || "—"}</span>
        </div>
        <EstadoNodo n={n} />
      </header>
      {activo && (
        <dl className="nodo-card__datos">
          <div><dt><Icon name="actividad" size={15} /> Último contacto</dt><dd>{haceCuanto(n.ultimoHeartbeatAt)}</dd></div>
          <div><dt><Icon name="recargar" size={15} /> Por sincronizar</dt><dd className="rp-num">{s.outboxPendientes ?? 0} cambios</dd></div>
          <div><dt><Icon name="disco" size={15} /> Disco libre</dt><dd className="rp-num">{s.discoLibreMb != null && s.discoLibreMb >= 0 ? `${(s.discoLibreMb / 1024).toFixed(1)} GB` : "—"}</dd></div>
          <div><dt><Icon name="tiempo" size={15} /> Reloj de la PC</dt><dd>{s.alertaReloj ? <span className="texto-alerta">Desfasado {Math.abs(s.derivaSegundos ?? 0)} s</span> : "Correcto"}</dd></div>
        </dl>
      )}
      {activo && s.alertaReloj && (
        <p className="nodo-card__aviso"><Icon name="error" size={16} /> La hora de esta PC no coincide con la real. Corrígela en Windows (Configuración › Hora e idioma › Sincronizar ahora): la fecha va dentro de cada factura.</p>
      )}
      <footer>
        <span className="rp-secondary">{activo ? `Activado ${haceCuanto(n.activadoAt)}` : `Revocado ${haceCuanto(n.revocadoAt)}`}</span>
        {activo && <button type="button" className="rp-btn rp-btn--gray rp-btn--sm texto-peligro" onClick={onRevocar}>Revocar</button>}
      </footer>
    </article>
  );
}

export function Nodo() {
  const nodos = useNodos();
  const locales = useLocales();
  const { toast, confirm } = useFeedback();
  const [codigo, setCodigo] = useState<CodigoNodo | null>(null);
  const generar = useGuardar((localId: string) => api.generarCodigoNodo(localId), []);
  const revocar = useGuardar((id: string) => api.revocarNodo(id), ["nodos"]);
  const local = locales.data?.[0];
  if (nodos.isLoading || locales.isLoading) return <Spinner />;
  const lista = nodos.data ?? [];
  const activo = lista.find((n) => n.estado === "ACTIVO");

  async function nuevoCodigo() {
    if (!local) return;
    try {
      setCodigo(await generar.mutateAsync(local.id));
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo generar el código.", "error");
    }
  }
  async function onRevocar(n: NodoLocal) {
    const ok = await confirm({
      titulo: "¿Revocar este Nodo Local?",
      mensaje: `«${n.nombreEquipo || "PC de caja"}» dejará de sincronizar con la nube al instante. Hazlo si cambiaste de PC o si el equipo se perdió.`,
      confirmar: "Revocar", destructivo: true,
    });
    if (!ok) return;
    try {
      await revocar.mutateAsync(n.id);
      toast("Nodo revocado");
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo revocar.", "error");
    }
  }

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="rp-t-large-title">Nodo Local</h1>
          <p>La PC de caja que mantiene tu restaurante funcionando aunque se caiga el internet.</p>
        </div>
      </header>

      <section className="nodo-hero">
        <div className="nodo-hero__texto">
          <AppIcon icono="servidor" tint="orange" size={64} />
          <h2 className="rp-t-title2">{activo ? "Instala un nodo nuevo" : "Conecta la PC de caja"}</h2>
          <p className="rp-secondary">
            {activo
              ? "Si cambias de computadora, genera un código y actívala. El nodo anterior se revoca solo: nunca habrá dos emitiendo facturas."
              : "Tres pasos y listo: comandas, impresión y cobros seguirán funcionando sin internet."}
          </p>
          <ol className="nodo-pasos">
            <li><span>1</span><div><b>Instala el Nodo Local</b> en la PC de caja con Windows 10 u 11.</div></li>
            <li><span>2</span><div>En esa PC abre <code>http://localhost:7080</code> en el navegador.</div></li>
            <li><span>3</span><div>Escribe el <b>código de activación</b> que generas aquí.</div></li>
          </ol>
        </div>
        <div className="nodo-hero__codigo">
          {codigo ? <CodigoGrande codigo={codigo} onNuevo={() => void nuevoCodigo()} /> : (
            <div className="codigo-nodo codigo-nodo--vacio">
              <Icon name="pin" size={34} />
              <p>El código vale 30 minutos y se usa una sola vez.</p>
            </div>
          )}
          <button type="button" className="rp-btn rp-btn--primary" disabled={generar.isPending || !local} onClick={() => void nuevoCodigo()}>
            {generar.isPending ? "Generando…" : codigo ? "Generar otro código" : "Generar código de activación"}
          </button>
        </div>
      </section>

      <section className="seccion">
        <h2 className="rp-t-title3" style={{ marginBottom: 12 }}>Tus nodos</h2>
        {lista.length === 0 ? (
          <p className="rp-secondary">Todavía no activas ningún nodo. Cuando lo hagas, aquí verás si está en línea, cuánto le falta por sincronizar y si su reloj está bien.</p>
        ) : (
          <div className="nodos">{lista.map((n) => <TarjetaNodo key={n.id} n={n} onRevocar={() => void onRevocar(n)} />)}</div>
        )}
      </section>
    </>
  );
}
