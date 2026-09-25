import { DndContext, type DragEndEvent, KeyboardSensor, PointerSensor, TouchSensor, useDraggable, useDroppable, useSensor, useSensors } from "@dnd-kit/core";
import { CSS } from "@dnd-kit/utilities";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router";
import { ApiError, uuidv7 } from "../api/client";
import { api, useCategorias, useEstaciones, useGuardar, useImpresoras, useLocales } from "../api/hooks";
import type { Categoria, EstadoImpresora, Estacion, Impresora } from "../api/types";
import { useFeedback } from "../components/feedback";
import { AppIcon, Empty, Field, Segmented, Sheet, Spinner, ToggleRow } from "../components/ui";
import { Icon } from "../lib/icons";

/** Texto y color de cada estado para el dueño (sin jerga técnica). */
export function describirEstado(i: Pick<Impresora, "estado" | "nodoEnLinea" | "activa">): { texto: string; tono: "success" | "warning" | "danger" | "neutral" } {
  if (!i.activa) return { texto: "Desactivada", tono: "neutral" };
  if (!i.nodoEnLinea) return { texto: "Nodo sin conexión", tono: "neutral" };
  const m: Record<EstadoImpresora, { texto: string; tono: "success" | "warning" | "danger" | "neutral" }> = {
    OK: { texto: "Conectada", tono: "success" },
    POCO_PAPEL: { texto: "Poco papel", tono: "warning" },
    SIN_PAPEL: { texto: "Sin papel", tono: "danger" },
    TAPA_ABIERTA: { texto: "Tapa abierta", tono: "danger" },
    SIN_CONEXION: { texto: "Sin conexión", tono: "danger" },
    ERROR: { texto: "Con error", tono: "danger" },
    DESCONOCIDO: { texto: "Buscando…", tono: "neutral" },
  };
  return m[i.estado] ?? m.DESCONOCIDO;
}

/** A qué estación va cada categoría: la suya o la estación por defecto. */
export function estacionDeCategoria(c: Pick<Categoria, "estacionId">, produccion: Pick<Estacion, "id" | "esDefecto">[]): string | undefined {
  if (c.estacionId && produccion.some((e) => e.id === c.estacionId)) return c.estacionId;
  return (produccion.find((e) => e.esDefecto) ?? produccion[0])?.id;
}

const espera = (ms: number) => new Promise((r) => setTimeout(r, ms));

function TarjetaImpresora({ i, onEditar }: { i: Impresora; onEditar: () => void }) {
  const { toast } = useFeedback();
  const [probando, setProbando] = useState(false);
  const est = describirEstado(i);
  async function probar() {
    setProbando(true);
    try {
      const cmd = await api.probarImpresora(i.id);
      for (let n = 0; n < 25; n++) {
        await espera(1000);
        const r = await api.comandoNodo(cmd.id);
        if (r.ejecutadoAt) {
          const ok = r.resultado?.startsWith("OK");
          toast(ok ? `¡Listo! Revisa la hoja de prueba en «${i.nombre}».` : `No se pudo imprimir: ${r.resultado?.replace(/^ERROR: /, "")}`, ok ? "ok" : "error");
          return;
        }
      }
      toast("El nodo no respondió a tiempo. Revisa que la PC de caja tenga internet.", "error");
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo pedir la prueba.", "error");
    } finally {
      setProbando(false);
    }
  }
  return (
    <article className="imp-card" data-tono={est.tono}>
      <header>
        <AppIcon icono="printer" tint={est.tono === "success" ? "green" : est.tono === "danger" ? "red" : est.tono === "warning" ? "orange" : "gray"} size={46} />
        <div className="imp-card__titulo">
          <b>{i.nombre}</b>
          <span className="rp-secondary">{i.conexion === "TCP" ? `${i.host}:${i.puerto}` : "USB"} · {i.anchoPapel} mm{i.modelo ? ` · ${i.modelo}` : ""}</span>
        </div>
        <span className={`rp-status rp-status--${est.tono}`}>{est.texto}</span>
      </header>
      {i.cola > 0 && <p className="imp-card__cola"><Icon name="tiempo" size={15} /> {i.cola} {i.cola === 1 ? "ticket espera" : "tickets esperan"} imprimirse</p>}
      <footer>
        {i.origen === "DETECTADA" && <span className="rp-secondary imp-card__origen"><Icon name="magia" size={14} /> Detectada sola</span>}
        <span style={{ flex: 1 }} />
        <button type="button" className="rp-btn rp-btn--gray rp-btn--sm" onClick={onEditar}><Icon name="editar" size={15} /> Editar</button>
        <button type="button" className="rp-btn rp-btn--tinted rp-btn--sm" disabled={probando || !i.activa} onClick={() => void probar()}>
          {probando ? <><span className="spinner-sm" /> Imprimiendo…</> : <><Icon name="printer" size={15} /> Imprimir prueba</>}
        </button>
      </footer>
    </article>
  );
}

function ChipCategoria({ c, estaciones, onMover }: { c: Categoria; estaciones: Estacion[]; onMover: (estacionId: string) => void }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: c.id });
  const [menu, setMenu] = useState(false);
  return (
    <div className="cat-chip-wrap">
      <button type="button" ref={setNodeRef} className="cat-chip" data-arrastrando={isDragging}
        style={{ transform: CSS.Translate.toString(transform), ["--tint" as string]: `var(--rp-tint-${c.color})` }}
        {...listeners} {...attributes} aria-label={`${c.nombre}: arrastra a otra estación o toca para elegir`}
        onClick={() => setMenu((m) => !m)}>
        <Icon name={c.icono} size={16} /> {c.nombre}
      </button>
      {menu && (
        <div className="cat-menu" role="menu">
          <span>Mover «{c.nombre}» a</span>
          {estaciones.map((e) => (
            <button key={e.id} type="button" role="menuitem" onClick={() => { setMenu(false); onMover(e.id); }}>
              <AppIcon icono={e.icono} tint={e.color} size={22} /> {e.nombre}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function ColumnaEstacion({ e, categorias, impresoras, todas, estaciones, onMover, onAsignar }: {
  e: Estacion; categorias: Categoria[]; impresoras: Impresora[]; todas: Impresora[]; estaciones: Estacion[];
  onMover: (cat: string, est: string) => void; onAsignar: (ids: string[]) => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: e.id });
  const [eligiendo, setEligiendo] = useState(false);
  const asignadas = new Set(impresoras.map((i) => i.id));
  return (
    <section ref={setNodeRef} className="estacion-col" data-sobre={isOver} aria-label={`Estación ${e.nombre}`}>
      <header>
        <AppIcon icono={e.icono} tint={e.color} size={38} />
        <div><b>{e.nombre}</b>{e.esDefecto && <small className="rp-secondary">Por defecto</small>}</div>
      </header>
      <div className="estacion-col__imps">
        {impresoras.length === 0
          ? <p className="estacion-col__aviso"><Icon name="error" size={15} /> Sin impresora: sus comandas no saldrán en papel.</p>
          : impresoras.map((i) => <span key={i.id} className="imp-chip" data-tono={describirEstado(i).tono}><Icon name="printer" size={14} /> {i.nombre}</span>)}
        <button type="button" className="rp-btn rp-btn--plain rp-btn--sm" onClick={() => setEligiendo((x) => !x)} disabled={todas.length === 0}>
          <Icon name="agregar" size={15} /> {impresoras.length ? "Cambiar" : "Asignar impresora"}
        </button>
        {eligiendo && (
          <div className="imp-elegir">
            {todas.map((i) => (
              <label key={i.id}>
                <input type="checkbox" checked={asignadas.has(i.id)} onChange={(ev) => {
                  const nuevas = ev.target.checked ? [...asignadas, i.id] : [...asignadas].filter((x) => x !== i.id);
                  onAsignar(nuevas);
                }} /> {i.nombre}
              </label>
            ))}
          </div>
        )}
      </div>
      <div className="estacion-col__cats">
        {categorias.length === 0 && <p className="rp-secondary estacion-col__vacia">Arrastra categorías aquí</p>}
        {categorias.map((c) => <ChipCategoria key={c.id} c={c} estaciones={estaciones.filter((x) => x.id !== e.id)} onMover={(est) => onMover(c.id, est)} />)}
      </div>
    </section>
  );
}

function ImpresoraSheet({ imp, localId, onClose }: { imp: Impresora | null; localId: string; onClose: () => void }) {
  const { toast, confirm } = useFeedback();
  const [id] = useState(() => imp?.id ?? uuidv7());
  const [nombre, setNombre] = useState(imp?.nombre ?? "");
  const [host, setHost] = useState(imp?.host ?? "");
  const [puerto, setPuerto] = useState(String(imp?.puerto ?? 9100));
  const [ancho, setAncho] = useState<"58" | "80">(String(imp?.anchoPapel ?? 80) as "58" | "80");
  const [activa, setActiva] = useState(imp?.activa ?? true);
  const [errores, setErrores] = useState<Record<string, string>>({});
  const guardar = useGuardar(() => {
    const body = { id, localId, nombre, host, puerto: Number(puerto) || 0, anchoPapel: Number(ancho), activa };
    return imp ? api.editarImpresora(imp.id, body) : api.crearImpresora(body);
  }, ["impresoras"]);
  const borrar = useGuardar(() => api.borrarImpresora(id), ["impresoras"]);
  async function onSave() {
    setErrores({});
    try {
      await guardar.mutateAsync(undefined);
      toast(imp ? "Impresora actualizada" : `«${nombre}» agregada`);
      onClose();
    } catch (e) {
      if (e instanceof ApiError) setErrores(e.fields), toast(e.message, "error");
    }
  }
  async function onDelete() {
    if (!(await confirm({ titulo: `¿Quitar ${imp?.nombre}?`, mensaje: "Sus estaciones dejarán de imprimir aquí. Lo pendiente en el nodo no se pierde.", confirmar: "Quitar", destructivo: true }))) return;
    await borrar.mutateAsync(undefined);
    toast("Impresora quitada");
    onClose();
  }
  return (
    <Sheet open title={imp ? "Editar impresora" : "Agregar impresora de red"} onClose={onClose} onSave={() => void onSave()} busy={guardar.isPending}>
      <div className="form">
        <Field label="Nombre" error={errores.nombre}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Cocina caliente" autoFocus />}</Field>
        {(!imp || imp.conexion === "TCP") && (
          <div className="form-row">
            <Field label="Dirección IP" error={errores.host} hint="Imprime la hoja de configuración de la impresora para verla.">{(fid, d) => <input id={fid} aria-describedby={d} value={host} onChange={(e) => setHost(e.target.value)} placeholder="192.168.1.50" inputMode="decimal" />}</Field>
            <Field label="Puerto" error={errores.puerto}>{(fid) => <input id={fid} value={puerto} onChange={(e) => setPuerto(e.target.value.replace(/\D/g, ""))} inputMode="numeric" />}</Field>
          </div>
        )}
        <Segmented label="Ancho del papel" value={ancho} onChange={setAncho} options={[{ value: "80", label: "80 mm (estándar)" }, { value: "58", label: "58 mm (pequeña)" }]} />
        {imp && <div className="rp-group" style={{ margin: 0 }}><ToggleRow label="Activa" detalle="Si la apagas, el nodo deja de enviarle tickets." checked={activa} onChange={setActiva} /></div>}
        <p className="rp-secondary" style={{ margin: 0, fontSize: 13 }}>
          <Icon name="info" size={14} /> Consejo: reserva esta IP en tu router (DHCP) para que no cambie. Si cambia igual, el nodo la vuelve a encontrar por su dirección MAC.
        </p>
        {imp && <button type="button" className="rp-btn rp-btn--gray texto-peligro" onClick={() => void onDelete()}><Icon name="eliminar" size={16} /> Quitar impresora</button>}
      </div>
    </Sheet>
  );
}

export function Impresoras() {
  const impresoras = useImpresoras();
  const estaciones = useEstaciones();
  const categorias = useCategorias();
  const locales = useLocales();
  const qc = useQueryClient();
  const { toast } = useFeedback();
  const [editar, setEditar] = useState<Impresora | "nueva" | null>(null);
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }), useSensor(TouchSensor, { activationConstraint: { delay: 180, tolerance: 6 } }), useSensor(KeyboardSensor));

  if (impresoras.isLoading || estaciones.isLoading || categorias.isLoading) return <Spinner />;
  const imps = impresoras.data ?? [];
  const produccion = (estaciones.data ?? []).filter((e) => e.tipo === "PRODUCCION");
  const cats = categorias.data ?? [];
  const nodoEnLinea = imps.some((i) => i.nodoEnLinea);

  async function mover(catId: string, estId: string) {
    const previo = qc.getQueryData<Categoria[]>(["categorias"]);
    qc.setQueryData<Categoria[]>(["categorias"], (xs) => xs?.map((c) => (c.id === catId ? { ...c, estacionId: estId } : c)));
    try {
      await api.rutearCategoria(catId, estId);
      const c = cats.find((x) => x.id === catId), e = produccion.find((x) => x.id === estId);
      toast(`«${c?.nombre}» ahora sale en ${e?.nombre}`);
    } catch (e) {
      qc.setQueryData(["categorias"], previo);
      toast(e instanceof ApiError ? e.message : "No se pudo mover la categoría.", "error");
    }
  }
  async function asignar(estId: string, lista: string[]) {
    try {
      await api.asignarImpresoras(estId, lista);
      void qc.invalidateQueries({ queryKey: ["impresoras"] });
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo asignar.", "error");
    }
  }
  function onDragEnd(ev: DragEndEvent) {
    const cat = String(ev.active.id), est = ev.over ? String(ev.over.id) : null;
    const c = cats.find((x) => x.id === cat);
    if (!est || !c || estacionDeCategoria(c, produccion) === est) return;
    void mover(cat, est);
  }

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="rp-t-large-title">Impresoras</h1>
          <p>Arrastra cada categoría a su estación: el plato sale solo en la impresora correcta.</p>
        </div>
        <div className="page-actions">
          <button type="button" className="rp-btn rp-btn--primary" onClick={() => setEditar("nueva")}><Icon name="agregar" size={18} /> Agregar impresora</button>
        </div>
      </header>

      {!nodoEnLinea && (
        <div className="aviso-nodo">
          <AppIcon icono="servidor" tint="orange" size={40} />
          <div>
            <b>Conecta tu Nodo Local para ver el estado real</b>
            <p>El nodo detecta las impresoras de tu red y te avisa si alguna se queda sin papel. <Link to="/nodo">Ir a Nodo Local</Link></p>
          </div>
        </div>
      )}

      <section className="seccion" style={{ marginTop: 8 }}>
        <h2 className="rp-t-title3" style={{ marginBottom: 12 }}>Tus impresoras</h2>
        {imps.length === 0 ? (
          <Empty icono="printer" tint="gray" titulo="Aún no tienes impresoras"
            texto="Cuando actives el Nodo Local aparecerán solas. También puedes agregar una de red con su IP."
            accion={<button type="button" className="rp-btn rp-btn--primary" onClick={() => setEditar("nueva")}>Agregar impresora</button>} />
        ) : (
          <div className="imps">{imps.map((i) => <TarjetaImpresora key={i.id} i={i} onEditar={() => setEditar(i)} />)}</div>
        )}
      </section>

      <section className="seccion">
        <h2 className="rp-t-title3" style={{ marginBottom: 4 }}>¿Qué sale en cada estación?</h2>
        <p className="rp-secondary" style={{ margin: "0 0 14px" }}>Lo que no asignes sale en la estación por defecto: ninguna comanda se pierde.</p>
        {produccion.length === 0 ? (
          <Empty icono="cocina" tint="red" titulo="Crea tus estaciones" texto="Cocina, Bar, Parrilla… Las creas en Salón › Estaciones." accion={<Link to="/salon" className="rp-btn rp-btn--primary">Ir a Salón</Link>} />
        ) : (
          <DndContext sensors={sensors} onDragEnd={onDragEnd}>
            <div className="estaciones-tablero">
              {produccion.map((e) => (
                <ColumnaEstacion key={e.id} e={e} estaciones={produccion}
                  categorias={cats.filter((c) => estacionDeCategoria(c, produccion) === e.id)}
                  impresoras={imps.filter((i) => i.estaciones.includes(e.id))} todas={imps}
                  onMover={(c, est) => void mover(c, est)} onAsignar={(l) => void asignar(e.id, l)} />
              ))}
            </div>
          </DndContext>
        )}
      </section>

      {editar && <ImpresoraSheet imp={editar === "nueva" ? null : editar} localId={locales.data?.[0]?.id ?? ""} onClose={() => setEditar(null)} />}
    </>
  );
}
