import { closestCenter, DndContext, type DragEndEvent, KeyboardSensor, PointerSensor, useSensor, useSensors } from "@dnd-kit/core";
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { ApiError, uuidv7 } from "../api/client";
import { api, useCategorias, useEstaciones, useGrupos, useGuardar, useIconos, useLocales, useProductos, useTarifas } from "../api/hooks";
import type { Categoria, Grupo, Producto } from "../api/types";
import { useFeedback } from "../components/feedback";
import { type FotoElegida, normalizar, PhotoPicker } from "../components/PhotoPicker";
import { AppIcon, Empty, Field, Sheet, Spinner, ToggleRow } from "../components/ui";
import { Icon } from "../lib/icons";
import { desglosar, formatUSD } from "../lib/money";

const COLORES = ["orange", "red", "pink", "purple", "indigo", "blue", "teal", "green", "yellow", "gray"];

export function Menu() {
  const categorias = useCategorias();
  const productos = useProductos();
  const [filtro, setFiltro] = useState<string | null>(null);
  const [q, setQ] = useState("");
  const [editCat, setEditCat] = useState<Categoria | "nueva" | null>(null);
  const [editProd, setEditProd] = useState<Producto | "nuevo" | null>(null);
  const [verGrupos, setVerGrupos] = useState(false);

  const visibles = useMemo(() => {
    const n = normalizar(q.trim());
    return (productos.data ?? []).filter((p) =>
      (!filtro || p.categoriaId === filtro) &&
      (!n || normalizar(p.nombre).includes(n) || normalizar(p.alias ?? "") === n || normalizar(p.descripcion).includes(n)));
  }, [productos.data, filtro, q]);

  if (categorias.isLoading || productos.isLoading) return <Spinner />;
  const cats = categorias.data ?? [];
  const catActual = cats.find((c) => c.id === filtro);

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="rp-t-large-title">Menú</h1>
          <p>Tus platos con foto, precio e IVA. Lo que guardas aquí llega solo a la app de meseros y a la caja.</p>
        </div>
        <div className="page-actions">
          <button className="rp-btn rp-btn--gray" onClick={() => setVerGrupos(true)}><Icon name="etiqueta" size={18} /> Modificadores</button>
          <button className="rp-btn rp-btn--primary" onClick={() => setEditProd("nuevo")} disabled={cats.length === 0}><Icon name="agregar" size={18} /> Nuevo plato</button>
        </div>
      </header>

      <div className="menu-layout">
        <aside className="cat-list">
          <div className="rp-group" style={{ margin: 0, padding: 6 }}>
            <button className="cat-item" aria-pressed={filtro === null} onClick={() => setFiltro(null)}>
              <AppIcon icono="utensils-crossed" tint="blue" size={36} />
              <span><b>Todo el menú</b></span>
              <span className="count">{productos.data?.length ?? 0}</span>
            </button>
            <ListaCategorias categorias={cats} filtro={filtro} onFiltro={setFiltro} onEditar={setEditCat} />
          </div>
          <button className="rp-btn rp-btn--tinted rp-btn--block" style={{ marginTop: 12 }} onClick={() => setEditCat("nueva")} aria-label="Nueva categoría">
            <Icon name="agregar" size={18} /> Nueva categoría
          </button>
        </aside>

        <section>
          <div className="toolbar">
            <label className="rp-search">
              <Icon name="buscar" size={18} />
              <input type="search" placeholder="Buscar plato o código rápido (cev, CM…)" value={q} onChange={(e) => setQ(e.target.value)} aria-label="Buscar plato" />
            </label>
            {catActual && <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => setEditCat(catActual)}><Icon name="editar" size={15} /> Editar «{catActual.nombre}»</button>}
          </div>
          {visibles.length === 0 ? (
            q ? <Empty icono="buscar" tint="gray" titulo="Sin resultados" texto={`No hay platos que coincidan con «${q}».`} />
              : <Empty icono={catActual?.icono ?? "utensils"} tint={catActual?.color ?? "orange"} titulo="Agrega tu primer plato"
                texto="Ponle nombre, precio y una foto de la galería. Te toma menos de un minuto."
                accion={<button className="rp-btn rp-btn--primary" onClick={() => setEditProd("nuevo")}>Crear plato</button>} />
          ) : (
            <div className="productos">
              {visibles.map((p) => <TarjetaProducto key={p.id} p={p} onClick={() => setEditProd(p)} />)}
              {!q && (
                <button className="producto producto-nuevo" onClick={() => setEditProd("nuevo")}>
                  <AppIcon icono="agregar" tint="blue" size={44} />
                  Agregar plato
                </button>
              )}
            </div>
          )}
        </section>
      </div>

      {editCat && <CategoriaSheet categoria={editCat === "nueva" ? null : editCat} onClose={() => setEditCat(null)} />}
      {editProd && <ProductoSheet producto={editProd === "nuevo" ? null : editProd} categoriaInicial={filtro ?? cats[0]?.id ?? ""} onClose={() => setEditProd(null)} />}
      {verGrupos && <GruposSheet onClose={() => setVerGrupos(false)} />}
    </>
  );
}

function TarjetaProducto({ p, onClick }: { p: Producto; onClick: () => void }) {
  return (
    <button className="producto" onClick={onClick} aria-label={`${p.nombre}, ${formatUSD(p.precio)}`}>
      <div className="foto">
        {p.imagenUrl ? <img src={p.imagenUrl} alt="" loading="lazy" /> : <Icon name="camara" size={30} />}
        {p.alias && <span className="alias">{p.alias}</span>}
        {!p.activo && <span className="inactivo">Oculto</span>}
      </div>
      <div className="info">
        <span className="nombre">{p.nombre}</span>
        <span className="precio">{formatUSD(p.precio)}</span>
        {p.descripcion && <span className="desc">{p.descripcion}</span>}
      </div>
    </button>
  );
}

function FilaCategoria({ c, activa, onFiltro, onEditar }: { c: Categoria; activa: boolean; onFiltro: () => void; onEditar: () => void }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: c.id });
  return (
    <div ref={setNodeRef} style={{ transform: CSS.Transform.toString(transform), transition, opacity: isDragging ? 0.6 : 1, display: "flex", alignItems: "center" }}>
      <button className="cat-item" aria-pressed={activa} onClick={onFiltro} onDoubleClick={onEditar}>
        <AppIcon icono={c.icono} tint={c.color} size={36} />
        <span>{c.nombre}{!c.activa && <small className="rp-secondary"> · oculta</small>}</span>
        <span className="count">{c.productos}</span>
      </button>
      <span className="grip" {...attributes} {...listeners} aria-label={`Mover ${c.nombre}`} role="button" tabIndex={0} style={{ padding: 8 }}>
        <Icon name="arrastrar" size={18} />
      </span>
    </div>
  );
}

function ListaCategorias({ categorias, filtro, onFiltro, onEditar }: { categorias: Categoria[]; filtro: string | null; onFiltro: (id: string) => void; onEditar: (c: Categoria) => void }) {
  const qc = useQueryClient();
  const { toast } = useFeedback();
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }), useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }));
  async function onDragEnd(e: DragEndEvent) {
    if (!e.over || e.active.id === e.over.id) return;
    const from = categorias.findIndex((c) => c.id === e.active.id);
    const to = categorias.findIndex((c) => c.id === e.over!.id);
    const nuevo = arrayMove(categorias, from, to);
    qc.setQueryData(["categorias"], nuevo); // se ve al instante
    try {
      await api.ordenarCategorias(nuevo.map((c) => c.id));
    } catch {
      toast("No se pudo guardar el orden. Intenta de nuevo.", "error");
      void qc.invalidateQueries({ queryKey: ["categorias"] });
    }
  }
  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={(e) => void onDragEnd(e)}>
      <SortableContext items={categorias.map((c) => c.id)} strategy={verticalListSortingStrategy}>
        {categorias.map((c) => <FilaCategoria key={c.id} c={c} activa={filtro === c.id} onFiltro={() => onFiltro(c.id)} onEditar={() => onEditar(c)} />)}
      </SortableContext>
    </DndContext>
  );
}

function CategoriaSheet({ categoria, onClose }: { categoria: Categoria | null; onClose: () => void }) {
  const iconos = useIconos();
  const estaciones = useEstaciones();
  const { toast, confirm } = useFeedback();
  const [nombre, setNombre] = useState(categoria?.nombre ?? "");
  const [icono, setIcono] = useState(categoria?.icono ?? "utensils");
  const [color, setColor] = useState(categoria?.color ?? "orange");
  const [estacionId, setEstacionId] = useState(categoria?.estacionId ?? "");
  const [notas, setNotas] = useState<string[]>(categoria?.notasRapidas ?? []);
  const [nota, setNota] = useState("");
  const [errores, setErrores] = useState<Record<string, string>>({});
  const [id] = useState(() => categoria?.id ?? uuidv7());
  const guardar = useGuardar(() => {
    const body = { id, nombre, icono, color, estacionId: estacionId || null, notasRapidas: notas };
    return categoria ? api.editarCategoria(categoria.id, body) : api.crearCategoria(body);
  }, ["categorias"]);
  const borrar = useGuardar(() => api.borrarCategoria(id), ["categorias"]);

  async function onSave() {
    setErrores({});
    try {
      await guardar.mutateAsync(undefined);
      toast(categoria ? "Categoría actualizada" : `Categoría «${nombre}» creada`);
      onClose();
    } catch (e) {
      if (e instanceof ApiError) setErrores(e.fields), toast(e.message, "error");
    }
  }
  async function onDelete() {
    if (!(await confirm({ titulo: `¿Eliminar «${categoria?.nombre}»?`, mensaje: "Si tiene platos, primero muévelos a otra categoría.", confirmar: "Eliminar", destructivo: true }))) return;
    try {
      await borrar.mutateAsync(undefined);
      toast("Categoría eliminada");
      onClose();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo eliminar.", "error");
    }
  }
  const agregarNota = () => {
    const t = nota.trim();
    if (t && !notas.includes(t) && notas.length < 12) setNotas([...notas, t]);
    setNota("");
  };

  return (
    <Sheet open title={categoria ? "Editar categoría" : "Nueva categoría"} onClose={onClose} onSave={() => void onSave()} busy={guardar.isPending}
      destructive={categoria ? { label: "Eliminar categoría", onClick: () => void onDelete() } : undefined}>
      <div className="form">
        <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
          <AppIcon icono={icono} tint={color} size={64} />
          <div style={{ flex: 1 }}>
            <Field label="Nombre" error={errores.nombre}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Ceviches, Bebidas, Postres…" maxLength={40} />}</Field>
          </div>
        </div>
        <div className="rp-field">
          <label>Icono</label>
          <div className="icon-picker">
            {(iconos.data ?? []).map((i) => (
              <button key={i.icono} type="button" className="icon-opt" aria-pressed={icono === i.icono} onClick={() => setIcono(i.icono)}>
                <AppIcon icono={i.icono} tint={color} size={36} />{i.etiqueta}
              </button>
            ))}
          </div>
        </div>
        <div className="rp-field">
          <label>Color</label>
          <div className="color-picker">
            {COLORES.map((c) => <button key={c} type="button" className="color-opt" aria-label={c} aria-pressed={color === c} style={{ background: `var(--rp-tint-${c})` }} onClick={() => setColor(c)} />)}
          </div>
        </div>
        <Field label="¿Dónde se prepara?" hint="Los platos de esta categoría se imprimen en esa estación.">
          {(fid) => (
            <select id={fid} value={estacionId} onChange={(e) => setEstacionId(e.target.value)}>
              <option value="">Estación por defecto</option>
              {(estaciones.data ?? []).filter((s) => s.tipo === "PRODUCCION").map((s) => <option key={s.id} value={s.id}>{s.nombre}</option>)}
            </select>
          )}
        </Field>
        <div className="rp-field">
          <label htmlFor="nota-nueva">Notas rápidas para el mesero</label>
          <div className="chips-edit">
            {notas.map((n) => <span key={n} className="chip-x">{n}<button type="button" aria-label={`Quitar ${n}`} onClick={() => setNotas(notas.filter((x) => x !== n))}><Icon name="cerrar" size={12} /></button></span>)}
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <input id="nota-nueva" value={nota} onChange={(e) => setNota(e.target.value)} onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), agregarNota())} placeholder="Sin sal, Extra ají…" maxLength={30} style={{ flex: 1 }} />
            <button type="button" className="rp-btn rp-btn--gray" onClick={agregarNota}>Agregar</button>
          </div>
          {errores.notasRapidas && <span className="rp-field__error">{errores.notasRapidas}</span>}
        </div>
      </div>
    </Sheet>
  );
}

function ProductoSheet({ producto, categoriaInicial, onClose }: { producto: Producto | null; categoriaInicial: string; onClose: () => void }) {
  const categorias = useCategorias();
  const tarifas = useTarifas();
  const grupos = useGrupos();
  const locales = useLocales();
  const { toast, confirm } = useFeedback();
  const [id] = useState(() => producto?.id ?? uuidv7());
  const [nombre, setNombre] = useState(producto?.nombre ?? "");
  const [precio, setPrecio] = useState(producto?.precio ?? "");
  const [categoriaId, setCategoriaId] = useState(producto?.categoriaId ?? categoriaInicial);
  const [alias, setAlias] = useState(producto?.alias ?? "");
  const [descripcion, setDescripcion] = useState(producto?.descripcion ?? "");
  const [tarifaId, setTarifaId] = useState(producto?.tarifaIvaId ?? "");
  const [foto, setFoto] = useState<FotoElegida | null>(producto?.imagenKey ? { key: producto.imagenKey, url: producto.imagenUrl ?? "" } : null);
  const [gruposSel, setGruposSel] = useState<string[]>(producto?.gruposModificadores ?? []);
  const [activo, setActivo] = useState(producto?.activo ?? true);
  const [qr, setQr] = useState(producto?.visibleMenuQr ?? true);
  const [errores, setErrores] = useState<Record<string, string>>({});

  const tarifa = tarifas.data?.find((t) => t.id === (tarifaId || tarifas.data?.[0]?.id));
  const incluye = locales.data?.[0]?.preciosIncluyenIva ?? true;
  const desglose = tarifa ? desglosar(precio.replace(",", "."), tarifa.porcentaje, incluye) : null;

  const guardar = useGuardar(() => {
    const body = {
      id, categoriaId, nombre, precio: precio.replace(",", "."), alias: alias || null, descripcion, tarifaIvaId: tarifa?.id ?? "",
      imagenKey: foto?.key ?? null, gruposModificadores: gruposSel, activo, visibleMenuQr: qr,
    };
    return producto ? api.editarProducto(producto.id, body) : api.crearProducto(body);
  }, ["productos", "categorias"]);
  const borrar = useGuardar(() => api.borrarProducto(id), ["productos", "categorias"]);

  async function onSave() {
    setErrores({});
    try {
      await guardar.mutateAsync(undefined);
      toast(producto ? "Plato actualizado" : `«${nombre}» ya está en tu menú`);
      onClose();
    } catch (e) {
      if (e instanceof ApiError) setErrores(e.fields), toast(e.message, "error");
    }
  }
  async function onDelete() {
    if (!(await confirm({ titulo: `¿Eliminar «${producto?.nombre}»?`, mensaje: "Desaparece del menú. Las ventas pasadas conservan su nombre y precio.", confirmar: "Eliminar", destructivo: true }))) return;
    await borrar.mutateAsync(undefined);
    toast("Plato eliminado");
    onClose();
  }

  return (
    <Sheet open title={producto ? "Editar plato" : "Nuevo plato"} onClose={onClose} onSave={() => void onSave()} busy={guardar.isPending}
      destructive={producto ? { label: "Eliminar plato", onClick: () => void onDelete() } : undefined}>
      <div className="form">
        <PhotoPicker value={foto} onChange={setFoto} nombrePlato={nombre} />
        <Field label="Nombre del plato" error={errores.nombre}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Ceviche mixto" maxLength={80} />}</Field>
        <div className="form-row">
          <Field label="Precio" error={errores.precio}>
            {(fid, d) => (
              <div className="precio-input">
                <span>$</span>
                <input id={fid} inputMode="decimal" value={precio} onChange={(e) => setPrecio(e.target.value)} placeholder="12.50" aria-describedby={d} />
              </div>
            )}
          </Field>
          <Field label="Categoría" error={errores.categoriaId}>
            {(fid) => (
              <select id={fid} value={categoriaId} onChange={(e) => setCategoriaId(e.target.value)}>
                {(categorias.data ?? []).map((c) => <option key={c.id} value={c.id}>{c.nombre}</option>)}
              </select>
            )}
          </Field>
        </div>
        {desglose && (
          <p className="desglose" aria-live="polite">
            <span>Base <b>${desglose.base}</b></span><span>IVA {tarifa?.porcentaje.replace(".00", "")} % <b>${desglose.iva}</b></span><span>El cliente paga <b>${desglose.total}</b></span>
          </p>
        )}
        <div className="form-row">
          <Field label="Código rápido" error={errores.alias} hint="El mesero lo escribe para encontrarlo al toque.">
            {(fid, d) => <input id={fid} value={alias} onChange={(e) => setAlias(e.target.value.toUpperCase().replace(/\s/g, ""))} placeholder="CM" maxLength={12} aria-describedby={d} />}
          </Field>
          <Field label="IVA" error={errores.tarifaIvaId}>
            {(fid) => (
              <select id={fid} value={tarifa?.id ?? ""} onChange={(e) => setTarifaId(e.target.value)}>
                {(tarifas.data ?? []).map((t) => <option key={t.id} value={t.id}>{t.descripcion}</option>)}
              </select>
            )}
          </Field>
        </div>
        <Field label="Descripción (menú QR)" error={errores.descripcion}>{(fid) => <textarea id={fid} value={descripcion} onChange={(e) => setDescripcion(e.target.value)} maxLength={300} placeholder="Pescado fresco con limón, cebolla colorada y culantro." />}</Field>
        {(grupos.data ?? []).length > 0 && (
          <div className="rp-field">
            <label>Modificadores</label>
            <div className="rp-group">
              {(grupos.data ?? []).map((g) => (
                <ToggleRow key={g.id} label={g.nombre} detalle={`${g.obligatorio ? "Obligatorio" : "Opcional"} · ${g.modificadores.map((m) => m.nombre).join(", ")}`}
                  checked={gruposSel.includes(g.id)} onChange={(v) => setGruposSel(v ? [...gruposSel, g.id] : gruposSel.filter((x) => x !== g.id))} />
              ))}
            </div>
          </div>
        )}
        <div className="rp-group">
          <ToggleRow label="Disponible para vender" detalle="Si lo apagas, desaparece de la app y la caja." checked={activo} onChange={setActivo} />
          <ToggleRow label="Mostrar en el menú QR" checked={qr} onChange={setQr} />
        </div>
      </div>
    </Sheet>
  );
}

function GruposSheet({ onClose }: { onClose: () => void }) {
  const grupos = useGrupos();
  const [edit, setEdit] = useState<Grupo | "nuevo" | null>(null);
  if (edit) return <GrupoSheet grupo={edit === "nuevo" ? null : edit} onClose={() => setEdit(null)} />;
  return (
    <Sheet open title="Modificadores" onClose={onClose}>
      <div className="form">
        <p className="rp-secondary" style={{ margin: 0 }}>Opciones que el mesero elige al tomar el pedido: término de la carne, acompañado, extras con precio…</p>
        {(grupos.data ?? []).length === 0 ? (
          <Empty icono="etiqueta" tint="purple" titulo="Sin modificadores" texto="Crea el primero, por ejemplo «Término de la carne»." />
        ) : (
          <div className="rp-group">
            {(grupos.data ?? []).map((g) => (
              <button key={g.id} className="rp-cell" onClick={() => setEdit(g)}>
                <AppIcon icono="etiqueta" tint="purple" />
                <span className="rp-cell__body">
                  <span className="rp-cell__title">{g.nombre}</span>
                  <span className="rp-cell__subtitle">{g.obligatorio ? `Obligatorio · elige ${g.min === g.max ? g.min : `${g.min} a ${g.max}`}` : `Opcional · hasta ${g.max}`} · en {g.productos} platos</span>
                </span>
                <Icon name="chevronRight" size={16} className="rp-cell__chevron" />
              </button>
            ))}
          </div>
        )}
        <button className="rp-btn rp-btn--tinted rp-btn--block" onClick={() => setEdit("nuevo")}><Icon name="agregar" size={18} /> Nuevo grupo</button>
      </div>
    </Sheet>
  );
}

function GrupoSheet({ grupo, onClose }: { grupo: Grupo | null; onClose: () => void }) {
  const { toast, confirm } = useFeedback();
  const [id] = useState(() => grupo?.id ?? uuidv7());
  const [nombre, setNombre] = useState(grupo?.nombre ?? "");
  const [obligatorio, setObligatorio] = useState(grupo?.obligatorio ?? false);
  const [max, setMax] = useState(grupo?.max ?? 1);
  const [opciones, setOpciones] = useState(grupo?.modificadores.map((m) => ({ id: m.id, nombre: m.nombre, precioAdicional: m.precioAdicional })) ?? [{ id: uuidv7(), nombre: "", precioAdicional: "0" }]);
  const [errores, setErrores] = useState<Record<string, string>>({});
  const guardar = useGuardar(() => {
    const body = { id, nombre, obligatorio, min: obligatorio ? 1 : 0, max, modificadores: opciones.filter((o) => o.nombre.trim()) };
    return grupo ? api.editarGrupo(grupo.id, body) : api.crearGrupo(body);
  }, ["grupos"]);
  const borrar = useGuardar(() => api.borrarGrupo(id), ["grupos", "productos"]);
  async function onDelete() {
    if (!(await confirm({ titulo: "¿Eliminar este grupo?", mensaje: "Se quitará de todos los platos que lo usan.", confirmar: "Eliminar", destructivo: true }))) return;
    await borrar.mutateAsync(undefined);
    toast("Grupo eliminado");
    onClose();
  }
  async function onSave() {
    setErrores({});
    try {
      await guardar.mutateAsync(undefined);
      toast("Modificadores guardados");
      onClose();
    } catch (e) {
      if (e instanceof ApiError) setErrores(e.fields), toast(e.message, "error");
    }
  }
  return (
    <Sheet open title={grupo ? "Editar grupo" : "Nuevo grupo"} onClose={onClose} onSave={() => void onSave()} busy={guardar.isPending}
      destructive={grupo ? { label: "Eliminar grupo", onClick: () => void onDelete() } : undefined}>
      <div className="form">
        <Field label="Nombre del grupo" error={errores.nombre}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Término de la carne" maxLength={40} />}</Field>
        <div className="rp-group">
          <ToggleRow label="Obligatorio" detalle="El mesero no puede enviar el pedido sin elegir." checked={obligatorio} onChange={setObligatorio} />
          <div className="rp-cell">
            <span className="rp-cell__body">Se puede elegir hasta</span>
            <span className="rp-stepper">
              <button type="button" aria-label="Menos" onClick={() => setMax(Math.max(1, max - 1))}>−</button>
              <output>{max}</output>
              <button type="button" aria-label="Más" onClick={() => setMax(Math.min(20, max + 1))}>+</button>
            </span>
          </div>
        </div>
        <div className="rp-field">
          <label>Opciones</label>
          <div className="rp-group">
            {opciones.map((o, i) => (
              <div key={o.id} className="rp-cell" style={{ gap: 8 }}>
                <input aria-label={`Opción ${i + 1}`} value={o.nombre} placeholder="Tocino" maxLength={40} style={{ flex: 1, minHeight: 40 }}
                  onChange={(e) => setOpciones(opciones.map((x) => (x.id === o.id ? { ...x, nombre: e.target.value } : x)))} />
                <div className="precio-input" style={{ width: 110 }}>
                  <span>+$</span>
                  <input aria-label={`Precio adicional de la opción ${i + 1}`} inputMode="decimal" value={o.precioAdicional} style={{ minHeight: 40, paddingLeft: 40, fontSize: 16 }}
                    onChange={(e) => setOpciones(opciones.map((x) => (x.id === o.id ? { ...x, precioAdicional: e.target.value } : x)))} />
                </div>
                <button type="button" className="rp-btn rp-btn--plain" aria-label="Quitar opción" onClick={() => setOpciones(opciones.filter((x) => x.id !== o.id))}><Icon name="eliminar" size={18} /></button>
              </div>
            ))}
          </div>
          {errores.modificadores && <span className="rp-field__error">{errores.modificadores}</span>}
          <button type="button" className="rp-btn rp-btn--gray rp-btn--sm" style={{ alignSelf: "flex-start" }} onClick={() => setOpciones([...opciones, { id: uuidv7(), nombre: "", precioAdicional: "0" }])}>
            <Icon name="agregar" size={15} /> Agregar opción
          </button>
        </div>
      </div>
    </Sheet>
  );
}
