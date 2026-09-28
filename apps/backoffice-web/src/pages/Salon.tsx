import { useEffect, useState } from "react";
import { ApiError, uuidv7 } from "../api/client";
import { api, useEstaciones, useGuardar, useLocales, useMesas, useZonas } from "../api/hooks";
import type { Estacion, Mesa } from "../api/types";
import { useFeedback } from "../components/feedback";
import { AppIcon, Empty, Field, Segmented, Sheet, Spinner } from "../components/ui";
import { Icon } from "../lib/icons";

const ICONOS_ESTACION = [
  { icono: "cocina", etiqueta: "Cocina caliente" }, { icono: "cocinaFria", etiqueta: "Cocina fría" }, { icono: "bar", etiqueta: "Bar" },
  { icono: "kds", etiqueta: "Pantalla" }, { icono: "factura", etiqueta: "Caja" }, { icono: "impresora", etiqueta: "Impresora" },
];
const COLORES = ["red", "orange", "yellow", "green", "teal", "blue", "indigo", "purple", "pink", "gray"];

function Stepper({ value, onChange, min, max, label }: { value: number; onChange: (n: number) => void; min: number; max: number; label: string }) {
  return (
    <span className="rp-stepper" role="group" aria-label={label}>
      <button type="button" aria-label="Menos" onClick={() => onChange(Math.max(min, value - 1))}>−</button>
      <output aria-live="polite">{value}</output>
      <button type="button" aria-label="Más" onClick={() => onChange(Math.min(max, value + 1))}>+</button>
    </span>
  );
}

export function Salon() {
  const zonas = useZonas();
  const mesas = useMesas();
  const estaciones = useEstaciones();
  const locales = useLocales();
  const [zonaId, setZonaId] = useState<string>("");
  const [agregar, setAgregar] = useState(false);
  const [editMesa, setEditMesa] = useState<Mesa | null>(null);
  const [zonaSheet, setZonaSheet] = useState<"nueva" | "editar" | null>(null);
  const [editEst, setEditEst] = useState<Estacion | "nueva" | null>(null);

  useEffect(() => {
    if (!zonaId && zonas.data?.[0]) setZonaId(zonas.data[0].id);
  }, [zonas.data, zonaId]);

  if (zonas.isLoading || mesas.isLoading) return <Spinner />;
  const zona = zonas.data?.find((z) => z.id === zonaId);
  const mesasZona = (mesas.data ?? []).filter((m) => m.zonaId === zonaId);
  const filas = Math.max(1, ...mesasZona.map((m) => m.posY + 1));

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="rp-t-large-title">Salón</h1>
          <p>Así verán tus meseros el salón en la app: cada mesa cambia de color cuando está ocupada, por pagar o la está atendiendo otro.</p>
        </div>
        <div className="page-actions">
          <button className="rp-btn rp-btn--gray" onClick={() => setZonaSheet("nueva")}><Icon name="agregar" size={18} /> Zona</button>
          <button className="rp-btn rp-btn--primary" onClick={() => setAgregar(true)} disabled={!zona}><Icon name="mesa" size={18} /> Agregar mesas</button>
        </div>
      </header>

      {(zonas.data ?? []).length > 0 && (
        <div className="toolbar">
          <div style={{ minWidth: 280, maxWidth: 560, flex: 1 }}>
            <Segmented label="Zona" value={zonaId} onChange={setZonaId} options={(zonas.data ?? []).map((z) => ({ value: z.id, label: `${z.nombre} · ${z.mesas}` }))} />
          </div>
          {zona && <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => setZonaSheet("editar")}><Icon name="editar" size={15} /> Editar zona</button>}
        </div>
      )}

      {mesasZona.length === 0 ? (
        <Empty icono="mesa" tint="green" titulo="Esta zona no tiene mesas" texto="Agrégalas de una vez: dinos cuántas y las acomodamos por ti."
          accion={<button className="rp-btn rp-btn--primary" onClick={() => setAgregar(true)}>Agregar mesas</button>} />
      ) : (
        <div className="plano" style={{ gridTemplateRows: `repeat(${filas}, auto)` }}>
          {mesasZona.map((m) => (
            <button key={m.id} className={`rp-mesa ${m.forma === "REDONDA" ? "rp-mesa--redonda" : ""}`} data-estado={m.activa ? "libre" : "bloqueada"}
              style={{ gridColumn: (m.posX % 6) + 1, gridRow: m.posY + 1 }} onClick={() => setEditMesa(m)} aria-label={`${m.nombre}, ${m.capacidad} personas`}>
              {m.nombre.replace(/^Mesa\s+/i, "")}
              <small><Icon name="personal" size={12} /> {m.capacidad} pers.</small>
            </button>
          ))}
        </div>
      )}

      <section className="seccion">
        <h2>Estaciones de preparación</h2>
        <p>Cada plato se imprime en la estación de su categoría: bebidas en el bar, platos en la cocina. El mesero no elige nada.</p>
        <div className="estaciones">
          {(estaciones.data ?? []).map((s) => (
            <button key={s.id} className="estacion" onClick={() => setEditEst(s)}>
              <AppIcon icono={s.icono} tint={s.color} size={48} />
              <span><b>{s.nombre}</b><small>{s.tipo === "CAJA" ? "Pre-cuentas y facturas" : "Comandas"}{s.esDefecto ? " · por defecto" : ""}</small></span>
            </button>
          ))}
          <button className="estacion" style={{ border: "2px dashed var(--rp-color-separator)", boxShadow: "none", background: "transparent", color: "var(--rp-color-accent)" }} onClick={() => setEditEst("nueva")}>
            <AppIcon icono="agregar" tint="blue" size={48} /><b>Nueva estación</b>
          </button>
        </div>
      </section>

      {agregar && zona && <AgregarMesas zonaId={zona.id} zonaNombre={zona.nombre} onClose={() => setAgregar(false)} />}
      {editMesa && <MesaSheet mesa={editMesa} onClose={() => setEditMesa(null)} />}
      {zonaSheet && <ZonaSheet zona={zonaSheet === "editar" ? zona ?? null : null} localId={locales.data?.[0]?.id ?? ""} onClose={(nueva) => { setZonaSheet(null); if (nueva) setZonaId(nueva); }} />}
      {editEst && <EstacionSheet estacion={editEst === "nueva" ? null : editEst} localId={locales.data?.[0]?.id ?? ""} onClose={() => setEditEst(null)} />}
    </>
  );
}

function AgregarMesas({ zonaId, zonaNombre, onClose }: { zonaId: string; zonaNombre: string; onClose: () => void }) {
  const { toast } = useFeedback();
  const [cantidad, setCantidad] = useState(6);
  const [capacidad, setCapacidad] = useState(4);
  const crear = useGuardar(() => api.crearMesas({ zonaId, cantidad, capacidad }), ["mesas", "zonas"]);
  async function onSave() {
    try {
      const creadas = await crear.mutateAsync(undefined);
      toast(`${creadas.length} mesas creadas en ${zonaNombre}`);
      onClose();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudieron crear.", "error");
    }
  }
  return (
    <Sheet open title={`Agregar mesas a ${zonaNombre}`} onClose={onClose} onSave={() => void onSave()} saveLabel="Crear" busy={crear.isPending}>
      <div className="form" style={{ alignItems: "center", textAlign: "center", gap: 22 }}>
        <AppIcon icono="mesa" tint="green" size={72} />
        <div>
          <p className="rp-secondary" style={{ margin: "0 0 10px" }}>¿Cuántas mesas?</p>
          <div className="rp-t-amount" style={{ fontSize: 64, lineHeight: 1 }}>{cantidad}</div>
          <div style={{ marginTop: 12 }}><Stepper value={cantidad} onChange={setCantidad} min={1} max={60} label="Cantidad de mesas" /></div>
        </div>
        <div className="rp-group" style={{ width: "100%" }}>
          <div className="rp-cell"><span className="rp-cell__body">Personas por mesa</span><Stepper value={capacidad} onChange={setCapacidad} min={1} max={50} label="Capacidad" /></div>
        </div>
        <p className="rp-secondary" style={{ margin: 0 }}>Las numeramos solas (Mesa 1, Mesa 2…) sin repetir nombres. Luego puedes renombrarlas.</p>
      </div>
    </Sheet>
  );
}

function MesaSheet({ mesa, onClose }: { mesa: Mesa; onClose: () => void }) {
  const zonas = useZonas();
  const { toast, confirm } = useFeedback();
  const [nombre, setNombre] = useState(mesa.nombre);
  const [capacidad, setCapacidad] = useState(mesa.capacidad);
  const [forma, setForma] = useState(mesa.forma);
  const [zonaId, setZonaId] = useState(mesa.zonaId);
  const [errores, setErrores] = useState<Record<string, string>>({});
  const guardar = useGuardar(() => api.editarMesa(mesa.id, { zonaId, nombre, capacidad, forma, posX: mesa.posX, posY: mesa.posY }), ["mesas", "zonas"]);
  const borrar = useGuardar(() => api.borrarMesa(mesa.id), ["mesas", "zonas"]);
  async function onSave() {
    setErrores({});
    try {
      await guardar.mutateAsync(undefined);
      toast("Mesa actualizada");
      onClose();
    } catch (e) {
      if (e instanceof ApiError) setErrores(e.fields), toast(e.message, "error");
    }
  }
  async function onDelete() {
    if (!(await confirm({ titulo: `¿Eliminar ${mesa.nombre}?`, mensaje: "Deja de aparecer en la app de meseros.", confirmar: "Eliminar", destructivo: true }))) return;
    await borrar.mutateAsync(undefined);
    toast("Mesa eliminada");
    onClose();
  }
  return (
    <Sheet open title={mesa.nombre} onClose={onClose} onSave={() => void onSave()} busy={guardar.isPending} destructive={{ label: "Eliminar mesa", onClick: () => void onDelete() }}>
      <div className="form">
        <Field label="Nombre" error={errores.nombre}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} maxLength={20} />}</Field>
        <Field label="Zona">{(fid) => <select id={fid} value={zonaId} onChange={(e) => setZonaId(e.target.value)}>{(zonas.data ?? []).map((z) => <option key={z.id} value={z.id}>{z.nombre}</option>)}</select>}</Field>
        <div className="rp-group"><div className="rp-cell"><span className="rp-cell__body">Personas</span><Stepper value={capacidad} onChange={setCapacidad} min={1} max={50} label="Capacidad" /></div></div>
        <Segmented label="Forma" value={forma} onChange={setForma} options={[{ value: "CUADRADA", label: "Cuadrada" }, { value: "REDONDA", label: "Redonda" }, { value: "RECTANGULAR", label: "Rectangular" }]} />
      </div>
    </Sheet>
  );
}

function ZonaSheet({ zona, localId, onClose }: { zona: { id: string; nombre: string; mesas: number } | null; localId: string; onClose: (nueva?: string) => void }) {
  const { toast, confirm } = useFeedback();
  const [nombre, setNombre] = useState(zona?.nombre ?? "");
  const [id] = useState(() => zona?.id ?? uuidv7());
  const [error, setError] = useState<string>();
  const guardar = useGuardar(() => (zona ? api.renombrarZona(zona.id, nombre) : api.crearZona({ id, localId, nombre })), ["zonas"]);
  const borrar = useGuardar(() => api.borrarZona(id), ["zonas"]);
  async function onSave() {
    try {
      await guardar.mutateAsync(undefined);
      toast(zona ? "Zona actualizada" : `Zona «${nombre}» creada`);
      onClose(zona ? undefined : id);
    } catch (e) {
      setError(e instanceof ApiError ? (e.fields.nombre ?? e.message) : "No se pudo guardar.");
    }
  }
  async function onDelete() {
    if (!(await confirm({ titulo: `¿Eliminar ${zona?.nombre}?`, mensaje: "Solo se puede si no tiene mesas.", confirmar: "Eliminar", destructivo: true }))) return;
    try {
      await borrar.mutateAsync(undefined);
      toast("Zona eliminada");
      onClose();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo eliminar.", "error");
    }
  }
  return (
    <Sheet open title={zona ? "Editar zona" : "Nueva zona"} onClose={() => onClose()} onSave={() => void onSave()} busy={guardar.isPending}
      destructive={zona ? { label: "Eliminar zona", onClick: () => void onDelete() } : undefined}>
      <div className="form">
        <Field label="Nombre" error={error}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Terraza, Segundo piso, Barra…" maxLength={40} />}</Field>
      </div>
    </Sheet>
  );
}

function EstacionSheet({ estacion, localId, onClose }: { estacion: Estacion | null; localId: string; onClose: () => void }) {
  const { toast, confirm } = useFeedback();
  const [id] = useState(() => estacion?.id ?? uuidv7());
  const [nombre, setNombre] = useState(estacion?.nombre ?? "");
  const [icono, setIcono] = useState(estacion?.icono ?? "cocina");
  const [color, setColor] = useState(estacion?.color ?? "red");
  const [tipo, setTipo] = useState<Estacion["tipo"]>(estacion?.tipo ?? "PRODUCCION");
  const [error, setError] = useState<string>();
  const guardar = useGuardar(() => {
    const body = { id, localId, nombre, icono, color, tipo };
    return estacion ? api.editarEstacion(estacion.id, body) : api.crearEstacion(body);
  }, ["estaciones"]);
  const borrar = useGuardar(() => api.borrarEstacion(id), ["estaciones", "categorias"]);
  async function onSave() {
    try {
      await guardar.mutateAsync(undefined);
      toast(estacion ? "Estación actualizada" : `Estación «${nombre}» creada`);
      onClose();
    } catch (e) {
      setError(e instanceof ApiError ? (e.fields.nombre ?? e.message) : "No se pudo guardar.");
    }
  }
  async function onDelete() {
    if (!(await confirm({ titulo: `¿Eliminar ${estacion?.nombre}?`, mensaje: "Sus categorías pasan a la estación por defecto.", confirmar: "Eliminar", destructivo: true }))) return;
    try {
      await borrar.mutateAsync(undefined);
      toast("Estación eliminada");
      onClose();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo eliminar.", "error");
    }
  }
  return (
    <Sheet open title={estacion ? "Editar estación" : "Nueva estación"} onClose={onClose} onSave={() => void onSave()} busy={guardar.isPending}
      destructive={estacion && !estacion.esDefecto ? { label: "Eliminar estación", onClick: () => void onDelete() } : undefined}>
      <div className="form">
        <div style={{ display: "flex", gap: 14, alignItems: "center" }}>
          <AppIcon icono={icono} tint={color} size={64} />
          <div style={{ flex: 1 }}><Field label="Nombre" error={error}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Cocina caliente, Bar, Sushi…" maxLength={40} />}</Field></div>
        </div>
        <Segmented label="Tipo" value={tipo} onChange={setTipo} options={[{ value: "PRODUCCION", label: "Prepara platos" }, { value: "CAJA", label: "Caja" }]} />
        <div className="icon-picker">
          {ICONOS_ESTACION.map((i) => <button key={i.icono} type="button" className="icon-opt" aria-pressed={icono === i.icono} onClick={() => setIcono(i.icono)}><AppIcon icono={i.icono} tint={color} size={36} />{i.etiqueta}</button>)}
        </div>
        <div className="color-picker">
          {COLORES.map((c) => <button key={c} type="button" className="color-opt" aria-label={c} aria-pressed={color === c} style={{ background: `var(--rp-tint-${c})` }} onClick={() => setColor(c)} />)}
        </div>
      </div>
    </Sheet>
  );
}
