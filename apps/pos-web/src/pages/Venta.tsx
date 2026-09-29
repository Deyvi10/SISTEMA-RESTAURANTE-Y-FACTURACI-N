// Pedido desde la computadora principal: venta en mostrador (F4-04, para llevar o barra) o para
// una mesa (el Cajero toma pedidos y cobra en la misma PC). Búsqueda predictiva con la misma
// regla que los meseros, modificadores obligatorios, carrito y envío a cocina; luego el cobro en
// el mismo flujo. Todo se maneja con el teclado.
import { formatMoney } from "@restpos/ui";
import { ArrowLeft, Minus, Plus, Search, Trash2 } from "lucide-react";
import { type KeyboardEvent, useEffect, useMemo, useRef, useState } from "react";
import { uuidv7 } from "../api/identidad";
import { ApiError, type Catalogo, type GrupoModificadores, type Modificador, nodo, type Producto, type TipoOrden } from "../api/nodo";
import { useAtajo } from "../components/atajos";
import { IndiceBusqueda } from "../lib/busqueda";
import { centavos, verCentavos } from "../lib/dinero";
import { useDispositivo } from "../lib/dispositivo";

export const TIPOS: { id: Exclude<TipoOrden, "MESA">; nombre: string }[] = [
  { id: "LLEVAR", nombre: "Para llevar" },
  { id: "BARRA", nombre: "Barra" },
];

export interface ItemCarrito {
  id: string; // de la línea en pantalla
  clave: string; // producto + modificadores: el mismo plato se suma en una línea
  producto: Producto;
  mods: Modificador[];
  cantidad: number;
  nota: string; // observaciones para cocina: «sin cebolla», «alérgico al maní»
}

/** Largo máximo de una observación (el nodo admite 140). */
export const MAX_NOTA = 140;

let lineas = 0;

/** Precio de una línea en centavos: (precio + modificadores) × cantidad. */
export function precioLinea(i: ItemCarrito): number {
  return (centavos(i.producto.precio) + i.mods.reduce((t, m) => t + centavos(m.precioAdicional), 0)) * i.cantidad;
}

/**
 * Agrega al carrito sumando la cantidad si ya está el mismo plato con los mismos modificadores y
 * sin observaciones. Uno con observación queda aparte: «sin cebolla» es solo para ese plato.
 */
export function agregar(carrito: ItemCarrito[], producto: Producto, mods: Modificador[]): ItemCarrito[] {
  const clave = producto.id + ":" + mods.map((m) => m.id).sort().join(",");
  const i = carrito.findIndex((x) => x.clave === clave && !x.nota.trim());
  if (i >= 0) return carrito.map((x, j) => (j === i ? { ...x, cantidad: x.cantidad + 1 } : x));
  return [...carrito, { id: `l${++lineas}`, clave, producto, mods, cantidad: 1, nota: "" }];
}

/** Cambia la observación de una línea (recortada al largo que admite el nodo). */
export const anotar = (carrito: ItemCarrito[], id: string, nota: string): ItemCarrito[] =>
  carrito.map((x) => (x.id === id ? { ...x, nota: nota.slice(0, MAX_NOTA) } : x));

/** Mínimo efectivo de un grupo: los obligatorios piden al menos uno. */
export const minimoGrupo = (g: GrupoModificadores) => Math.max(g.min, g.obligatorio ? 1 : 0);

/** ¿La elección cumple los mínimos y máximos de cada grupo? Devuelve el primer problema. */
export function validarMods(grupos: GrupoModificadores[], elegidos: Set<string>): string | null {
  for (const g of grupos) {
    const n = g.modificadores.filter((m) => elegidos.has(m.id)).length;
    if (n < minimoGrupo(g)) return `Elige ${g.nombre.toLowerCase()} (${minimoGrupo(g)} como mínimo).`;
    if (n > g.max) return `En ${g.nombre.toLowerCase()} puedes elegir hasta ${g.max}.`;
  }
  return null;
}

export interface OrdenExistente {
  ordenId: string;
  tipo: TipoOrden;
  mesaId?: string | null;
  etiqueta: string;
  nombre: string;
}

/** Mesa libre para la que se toma un pedido nuevo desde la caja. */
export interface MesaParaPedir {
  id: string;
  nombre: string;
}

export function Venta({
  existente,
  mesa,
  volver,
  cobrar,
}: {
  existente?: OrdenExistente;
  mesa?: MesaParaPedir;
  volver: () => void;
  cobrar: (ordenId: string, nombre: string) => void;
}) {
  const mesaId = existente?.tipo === "MESA" ? (existente.mesaId ?? undefined) : mesa?.id;
  const [catalogo, setCatalogo] = useState<Catalogo | null>(null);
  const [q, setQ] = useState("");
  const [sel, setSel] = useState(0);
  const [categoria, setCategoria] = useState<string | null>(null);
  const [carrito, setCarrito] = useState<ItemCarrito[]>([]);
  const [tipo, setTipo] = useState<Exclude<TipoOrden, "MESA">>(existente && existente.tipo !== "MESA" ? existente.tipo : "LLEVAR");
  const [etiqueta, setEtiqueta] = useState(existente?.etiqueta ?? "");
  const [pidiendo, setPidiendo] = useState<Producto | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [enviando, setEnviando] = useState(false);
  const [ordenId] = useState(() => existente?.ordenId ?? uuidv7());
  const buscador = useRef<HTMLInputElement>(null);
  const carritoRef = useRef<HTMLElement>(null);
  // En la PC el buscador queda listo para escribir; en el celular no se abre el teclado solo.
  const { tactil } = useDispositivo();
  const enfocarBuscador = () => {
    if (!tactil) buscador.current?.focus();
  };

  useEffect(() => {
    nodo
      .catalogo()
      .then((c) => {
        setCatalogo(c);
        setCategoria([...c.categorias].sort((a, b) => a.orden - b.orden)[0]?.id ?? null);
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo leer el menú."));
    enfocarBuscador();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- solo al abrir
  }, []);

  const indice = useMemo(
    () => new IndiceBusqueda(catalogo?.productos ?? [], { nombre: (p) => p.nombre, alias: (p) => p.alias, vendidos: (p) => p.vendidos }),
    [catalogo],
  );
  const grupos = useMemo(() => new Map((catalogo?.grupos ?? []).map((g) => [g.id, g])), [catalogo]);
  const resultados = q.trim() ? indice.buscar(q, 12) : (catalogo?.productos ?? []).filter((p) => p.categoriaId === categoria).sort((a, b) => a.orden - b.orden);
  useEffect(() => setSel(0), [q, categoria]);

  const elegir = (p: Producto) => {
    setError(null);
    if (p.grupos.some((g) => grupos.has(g))) setPidiendo(p);
    else setCarrito((c) => agregar(c, p, []));
    setQ("");
    enfocarBuscador();
  };

  const total = carrito.reduce((t, i) => t + precioLinea(i), 0);

  const enviar = (luegoCobrar: boolean) => {
    if (enviando || carrito.length === 0) return;
    setEnviando(true);
    setError(null);
    nodo
      .enviarOrden({
        idempotencyKey: uuidv7(),
        ordenId,
        ...(mesaId ? { tipo: "MESA" as const, mesaId } : { tipo }),
        etiqueta: mesaId ? "" : etiqueta.trim(),
        lineas: carrito.map((i) => ({ id: uuidv7(), productoId: i.producto.id, cantidad: String(i.cantidad), modificadores: i.mods.map((m) => m.id), nota: i.nota.trim() })),
      })
      .then((r) => (luegoCobrar ? cobrar(r.orden.id, r.orden.mesa) : volver()))
      .catch((e) => {
        setEnviando(false);
        setError(e instanceof ApiError ? e.message : "No se pudo enviar la orden.");
      });
  };

  const libre = pidiendo === null;
  useAtajo("F2", "Enviar a cocina y cobrar", () => enviar(true), "Venta", libre);
  useAtajo("F3", "Enviar a cocina sin cobrar", () => enviar(false), "Venta", libre);
  useAtajo("/", "Buscar producto", () => buscador.current?.focus(), "Venta", libre);
  useAtajo("Escape", "Volver", volver, "Venta", libre);

  const teclasBuscador = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") setSel((s) => Math.min(s + 1, resultados.length - 1));
    else if (e.key === "ArrowUp") setSel((s) => Math.max(s - 1, 0));
    else if (e.key === "Enter" && resultados[sel]) elegir(resultados[sel]);
    else if (e.key === "Escape" && q) setQ("");
    else if (e.key === "Escape") volver();
    else return;
    e.preventDefault();
    e.stopPropagation();
  };

  return (
    <section className="venta" data-testid="venta">
      <div className="venta__menu">
        <div className="venta__cabecera">
          <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={volver}>
            <ArrowLeft aria-hidden="true" /> Volver <kbd className="tecla">Esc</kbd>
          </button>
          <h1 className="rp-large-title" data-testid="titulo-venta">
            {existente ? `Agregar a ${existente.nombre}` : mesa ? `Pedido para ${mesa.nombre}` : "Nueva venta"}
          </h1>
        </div>
        <label className="rp-search venta__buscador">
          <Search aria-hidden="true" />
          <input
            ref={buscador}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={teclasBuscador}
            placeholder="Buscar producto o alias  ( / )"
            aria-label="Buscar producto"
            aria-controls="resultados"
            data-testid="buscar-producto"
          />
        </label>
        {!q.trim() && (
          <div className="rp-chips" role="radiogroup" aria-label="Categoría">
            {[...(catalogo?.categorias ?? [])]
              .sort((a, b) => a.orden - b.orden)
              .map((c) => (
                <button key={c.id} className="rp-chip" aria-pressed={c.id === categoria} onClick={() => setCategoria(c.id)}>
                  {c.nombre}
                </button>
              ))}
          </div>
        )}
        <div className="productos" id="resultados" role="listbox" aria-label="Productos">
          {resultados.map((p, i) => (
            <button
              key={p.id}
              role="option"
              aria-selected={i === sel}
              className="producto"
              onClick={() => elegir(p)}
              onMouseEnter={() => setSel(i)}
              data-testid={`producto-${p.nombre}`}
            >
              <span className="producto__nombre">{p.nombre}</span>
              <span className="producto__precio rp-num">{formatMoney(verCentavos(centavos(p.precio)))}</span>
              {p.alias && <small className="producto__alias">{p.alias}</small>}
            </button>
          ))}
          {catalogo && q.trim() && resultados.length === 0 && <p className="rp-secondary">Nada coincide con «{q}».</p>}
        </div>
      </div>

      <aside className="venta__carrito" aria-label="Orden" ref={carritoRef}>
        {!mesaId && (
          <>
            <div className="rp-segmented" role="radiogroup" aria-label="Tipo de orden">
              {TIPOS.map((t) => (
                <label key={t.id}>
                  <input type="radio" name="tipo" checked={tipo === t.id} disabled={!!existente} onChange={() => setTipo(t.id)} />
                  <span>{t.nombre}</span>
                </label>
              ))}
            </div>
            <div className="rp-field">
              <label htmlFor="etiqueta">Nombre corto (sale en la comanda)</label>
              <input id="etiqueta" value={etiqueta} maxLength={20} disabled={!!existente} onChange={(e) => setEtiqueta(e.target.value)} placeholder="Ej.: Ana" data-testid="etiqueta" />
            </div>
          </>
        )}
        {carrito.length === 0 ? (
          <p className="rp-secondary venta__vacio">Busca un producto y pulsa Intro para agregarlo.</p>
        ) : (
          <div className="rp-group venta__lineas" data-testid="carrito">
            {carrito.map((i) => (
              <div className="rp-cell venta__linea" key={i.id}>
                <span className="rp-cell__body">
                  <span className="rp-cell__title">{i.producto.nombre}</span>
                  {i.mods.length > 0 && <span className="rp-cell__subtitle">{i.mods.map((m) => m.nombre).join(", ")}</span>}
                  <input
                    className="venta__observacion"
                    value={i.nota}
                    maxLength={MAX_NOTA}
                    onChange={(e) => setCarrito((c) => anotar(c, i.id, e.target.value))}
                    onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), enfocarBuscador())}
                    placeholder="Observaciones: sin cebolla, poco picante…"
                    aria-label={`Observaciones de ${i.producto.nombre}`}
                    data-testid={`nota-${i.producto.nombre}`}
                  />
                </span>
                <span className="rp-stepper" role="group" aria-label={`Cantidad de ${i.producto.nombre}`}>
                  <button
                    type="button"
                    aria-label={i.cantidad === 1 ? "Quitar" : "Menos"}
                    onClick={() => setCarrito((c) => c.flatMap((x) => (x.id !== i.id ? [x] : x.cantidad > 1 ? [{ ...x, cantidad: x.cantidad - 1 }] : [])))}
                  >
                    {i.cantidad === 1 ? <Trash2 aria-hidden="true" /> : <Minus aria-hidden="true" />}
                  </button>
                  <output>{i.cantidad}</output>
                  <button type="button" aria-label="Más" onClick={() => setCarrito((c) => c.map((x) => (x.id === i.id ? { ...x, cantidad: x.cantidad + 1 } : x)))}>
                    <Plus aria-hidden="true" />
                  </button>
                </span>
                <span className="rp-cell__value rp-num venta__precio">{formatMoney(verCentavos(precioLinea(i)))}</span>
              </div>
            ))}
          </div>
        )}
        <p className="venta__total">
          <span>Platos</span>
          <b className="rp-num" data-testid="total-venta">
            {formatMoney(verCentavos(total))}
          </b>
        </p>
        <p className="rp-secondary venta__nota">El servicio y el detalle de impuestos se calculan al cobrar.</p>
        {error && (
          <p className="aviso-error" role="alert">
            {error}
          </p>
        )}
        <button className="rp-btn rp-btn--primary rp-btn--block" disabled={carrito.length === 0 || enviando} onClick={() => enviar(true)} data-testid="enviar-cobrar">
          Enviar y cobrar <kbd className="tecla">F2</kbd>
        </button>
        <button className="rp-btn rp-btn--gray rp-btn--block" disabled={carrito.length === 0 || enviando} onClick={() => enviar(false)} data-testid="enviar">
          Enviar sin cobrar <kbd className="tecla">F3</kbd>
        </button>
      </aside>

      {/* Celular: el pedido queda debajo del menú; esta barra fija lleva el total y lleva a él. */}
      {carrito.length > 0 && (
        <div className="venta__resumen-movil" data-testid="resumen-movil">
          <button type="button" className="rp-btn rp-btn--gray" onClick={() => carritoRef.current?.scrollIntoView({ behavior: "smooth", block: "start" })}>
            Ver pedido · {carrito.reduce((t, i) => t + i.cantidad, 0)}
          </button>
          <button type="button" className="rp-btn rp-btn--primary" disabled={enviando} onClick={() => enviar(true)}>
            Enviar y cobrar {formatMoney(verCentavos(total))}
          </button>
        </div>
      )}

      {pidiendo && (
        <HojaModificadores
          producto={pidiendo}
          grupos={pidiendo.grupos.flatMap((g) => grupos.get(g) ?? [])}
          cerrar={() => {
            setPidiendo(null);
            enfocarBuscador();
          }}
          listo={(mods) => {
            setCarrito((c) => agregar(c, pidiendo, mods));
            setPidiendo(null);
            enfocarBuscador();
          }}
        />
      )}
    </section>
  );
}

function HojaModificadores({ producto, grupos, cerrar, listo }: { producto: Producto; grupos: GrupoModificadores[]; cerrar: () => void; listo: (m: Modificador[]) => void }) {
  const [elegidos, setElegidos] = useState<Set<string>>(new Set());
  const [intento, setIntento] = useState(false);
  const problema = validarMods(grupos, elegidos);
  const confirmar = () => {
    setIntento(true);
    if (!problema) listo(grupos.flatMap((g) => g.modificadores.filter((m) => elegidos.has(m.id))));
  };
  useAtajo("Escape", "Cancelar", cerrar, "Modificadores");
  useAtajo("Enter", "Agregar", confirmar, "Modificadores");
  const alternar = (g: GrupoModificadores, m: Modificador) =>
    setElegidos((prev) => {
      const s = new Set(prev);
      if (s.has(m.id)) s.delete(m.id);
      else {
        // Grupo de una sola opción: elegir otra reemplaza la anterior.
        if (g.max === 1) g.modificadores.forEach((x) => s.delete(x.id));
        s.add(m.id);
      }
      return s;
    });
  return (
    <div className="rp-scrim" data-open="true" onClick={cerrar}>
      <div className="rp-sheet hoja-caja hoja-mods" role="dialog" aria-modal="true" aria-label={producto.nombre} onClick={(e) => e.stopPropagation()}>
        <div className="rp-sheet__header">
          <button className="rp-btn rp-btn--plain rp-btn--sm" onClick={cerrar}>
            Cancelar
          </button>
          <h2 className="rp-sheet__title">{producto.nombre}</h2>
          <span />
        </div>
        <div className="rp-sheet__body">
          {grupos.map((g, gi) => (
            <fieldset key={g.id} className="mods__grupo">
              <legend className="rp-section-header">
                {g.nombre} {minimoGrupo(g) > 0 ? "· obligatorio" : "· opcional"}
                {g.max > 1 ? ` · hasta ${g.max}` : ""}
              </legend>
              <div className="rp-group">
                {g.modificadores.map((m) => (
                  <label className="rp-cell" key={m.id}>
                    <input
                      type={g.max === 1 ? "radio" : "checkbox"}
                      name={g.id}
                      checked={elegidos.has(m.id)}
                      onChange={() => alternar(g, m)}
                      onClick={(e) => g.max === 1 && elegidos.has(m.id) && (e.preventDefault(), alternar(g, m))}
                      autoFocus={gi === 0 && m === g.modificadores[0]}
                    />
                    <span className="rp-cell__body">{m.nombre}</span>
                    {centavos(m.precioAdicional) > 0 && <span className="rp-cell__value rp-num">+{formatMoney(verCentavos(centavos(m.precioAdicional)))}</span>}
                  </label>
                ))}
              </div>
            </fieldset>
          ))}
          {intento && problema && (
            <p className="aviso-error hoja-caja__ayuda" role="alert">
              {problema}
            </p>
          )}
          <div className="hoja-caja__pie">
            <button className="rp-btn rp-btn--primary rp-btn--block" onClick={confirmar} data-testid="agregar-con-mods">
              Agregar <kbd className="tecla">Intro</kbd>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
