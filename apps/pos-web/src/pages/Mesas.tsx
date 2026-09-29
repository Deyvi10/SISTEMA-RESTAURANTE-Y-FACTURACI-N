// Mesas y órdenes abiertas (F4-04 empieza aquí): en vivo por WebSocket, búsqueda por número o
// nombre con «/», flechas para moverse e Intro para abrir.
import { formatMoney } from "@restpos/ui";
import { Clock, Coffee, Lock, Plus, Receipt, Search, ShoppingBag, Users } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ApiError, type Mesa, nodo, type OrdenSinMesa, type Salon } from "../api/nodo";
import { useSesion } from "../api/sesion";
import { useAtajo } from "../components/atajos";
import { useDispositivo } from "../lib/dispositivo";

const ESTADO: Record<Mesa["estado"], string> = { LIBRE: "libre", OCUPADA: "ocupada", POR_PAGAR: "porPagar", DEMORADA: "demorada" };

/** Filtra por número de orden exacto o por nombre de mesa/mesero. */
export function filtrarMesas(mesas: Mesa[], q: string): Mesa[] {
  const t = q.trim().toLowerCase();
  if (!t) return mesas;
  return mesas.filter((m) => String(m.numeroOrden ?? "") === t.replace(/^#/, "") || m.nombre.toLowerCase().includes(t) || (m.meseroNombre ?? "").toLowerCase().includes(t));
}

/** Órdenes sin mesa que coinciden con la búsqueda (número o nombre corto). */
export function filtrarSinMesa(ordenes: OrdenSinMesa[], q: string): OrdenSinMesa[] {
  const t = q.trim().toLowerCase().replace(/^#/, "");
  if (!t) return ordenes;
  return ordenes.filter((o) => String(o.numero) === t || o.nombre.toLowerCase().includes(t));
}

const natural = new Intl.Collator("es", { numeric: true, sensitivity: "base" });

/** Orden del salón: por zona y luego por nombre natural (Mesa 2 antes que Mesa 10). */
export function ordenarMesas(mesas: Mesa[], zonas: { id: string; orden: number }[]): Mesa[] {
  const ordenZona = new Map(zonas.map((z) => [z.id, z.orden]));
  return [...mesas].sort((a, b) => (ordenZona.get(a.zonaId) ?? 0) - (ordenZona.get(b.zonaId) ?? 0) || natural.compare(a.nombre, b.nombre));
}

export function minutosDesde(iso: string | null, ahora = Date.now()): number | null {
  return iso ? Math.max(0, Math.floor((ahora - Date.parse(iso)) / 60_000)) : null;
}

const ICONO_TIPO: Record<string, typeof Users> = { LLEVAR: ShoppingBag, BARRA: Coffee, MESA: Users };
/** Icono del tipo de orden; uno que ya no existe (una orden vieja de delivery) no tumba la caja. */
export const iconoTipo = (tipo: string) => ICONO_TIPO[tipo] ?? ShoppingBag;

export function Mesas({ abrir, abrirOrden, nuevaVenta }: { abrir: (m: Mesa) => void; abrirOrden: (o: OrdenSinMesa) => void; nuevaVenta: () => void }) {
  const { tiempoReal } = useSesion();
  const { tactil } = useDispositivo();
  const [salon, setSalon] = useState<Salon | null>(null);
  const [sinMesa, setSinMesa] = useState<OrdenSinMesa[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [zona, setZona] = useState<string | "todas">("todas");
  const [q, setQ] = useState("");
  const [foco, setFoco] = useState(0);
  const buscador = useRef<HTMLInputElement>(null);
  const rejilla = useRef<HTMLDivElement>(null);

  const cargar = useCallback(() => {
    nodo
      .ordenesSinMesa()
      .then(setSinMesa)
      .catch(() => {});
    nodo
      .salon()
      .then((s) => {
        setSalon(s);
        setError(null);
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo leer el salón."));
  }, []);

  useEffect(() => {
    cargar();
    const off = tiempoReal.alEvento((e) => {
      if (e.type.startsWith("table.") || e.type === "order.submitted" || e.type === "order.line_voided" || e.type === "catalog.updated") cargar();
    });
    const offConexion = tiempoReal.alCambiarConexion((v) => v && cargar());
    return () => {
      off();
      offConexion();
    };
  }, [cargar, tiempoReal]);

  const visibles = useMemo(() => {
    const todas = ordenarMesas(salon?.mesas ?? [], salon?.zonas ?? []);
    return filtrarMesas(zona === "todas" ? todas : todas.filter((m) => m.zonaId === zona), q);
  }, [salon, zona, q]);

  useEffect(() => setFoco((f) => Math.min(f, Math.max(0, visibles.length - 1))), [visibles.length]);

  const mover = (d: number) => {
    const cols = rejilla.current ? getComputedStyle(rejilla.current).gridTemplateColumns.split(" ").length : 6;
    setFoco((f) => Math.min(visibles.length - 1, Math.max(0, f + (Math.abs(d) === 1 ? d : Math.sign(d) * cols))));
  };
  useAtajo("/", "Buscar mesa u orden", () => buscador.current?.focus(), "Mesas");
  useAtajo("n", "Nueva venta en mostrador", nuevaVenta, "Mesas");
  useAtajo("ArrowRight", "Mesa siguiente", () => mover(1), "Mesas");
  useAtajo("ArrowLeft", "Mesa anterior", () => mover(-1), "Mesas");
  useAtajo("ArrowDown", "Mesa de abajo", () => mover(2), "Mesas");
  useAtajo("ArrowUp", "Mesa de arriba", () => mover(-2), "Mesas");
  useAtajo("Enter", "Abrir la mesa elegida", () => visibles[foco] && abrir(visibles[foco]), "Mesas");

  // Con teclado, la mesa elegida toma el foco; con el dedo no (no hay flechas y el aro estorba).
  useEffect(() => {
    if (!tactil) (rejilla.current?.children[foco] as HTMLElement | undefined)?.focus({ preventScroll: false });
  }, [foco, tactil]);

  const cuenta = (e: Mesa["estado"]) => (salon?.mesas ?? []).filter((m) => m.estado === e).length;

  return (
    <section className="mesas">
      <div className="mesas__barra">
        <label className="rp-search buscador">
          <Search aria-hidden="true" />
          <input
            ref={buscador}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && visibles[0]) abrir(visibles[0]);
              if (e.key === "Escape") {
                setQ("");
                e.currentTarget.blur();
              }
            }}
            placeholder="Buscar mesa, orden o mesero  ( / )"
            aria-label="Buscar mesa, orden o mesero"
            data-testid="buscar-mesa"
          />
        </label>
        <div className="rp-segmented zonas" role="radiogroup" aria-label="Zona">
          {[{ id: "todas", nombre: "Todas" }, ...(salon?.zonas ?? [])].map((z) => (
            <label key={z.id}>
              <input type="radio" name="zona" checked={zona === z.id} onChange={() => setZona(z.id)} />
              <span>{z.nombre}</span>
            </label>
          ))}
        </div>
      </div>
      <div className="sin-mesa">
        <button className="rp-btn rp-btn--primary nueva-venta" onClick={nuevaVenta} data-testid="nueva-venta">
          <Plus aria-hidden="true" /> Nueva venta <kbd className="tecla">N</kbd>
        </button>
        {filtrarSinMesa(sinMesa, q).map((o) => {
          const Icono = iconoTipo(o.tipo);
          const min = minutosDesde(o.abiertaAt);
          return (
            <button key={o.id} className="orden-sin-mesa" onClick={() => abrirOrden(o)} data-testid={`orden-${o.nombre}`}>
              <Icono aria-hidden="true" />
              <span className="orden-sin-mesa__nombre">{o.nombre}</span>
              <span className="rp-num">{formatMoney(o.total)}</span>
              <small>
                {o.platos} {o.platos === 1 ? "plato" : "platos"}
                {min != null && ` · ${min} min`}
              </small>
            </button>
          );
        })}
      </div>
      <p className="leyenda">
        <span data-estado="libre">{cuenta("LIBRE")} libres</span>
        <span data-estado="ocupada">{cuenta("OCUPADA") + cuenta("DEMORADA")} ocupadas</span>
        <span data-estado="porPagar">{cuenta("POR_PAGAR")} por pagar</span>
      </p>
      {error && <p className="aviso-error">{error}</p>}
      <div className="rejilla-mesas" ref={rejilla} role="grid" aria-label="Mesas">
        {visibles.map((m, i) => {
          const min = minutosDesde(m.abiertaAt);
          return (
            <button
              key={m.id}
              className="rp-mesa mesa"
              data-estado={m.bloqueo ? "bloqueada" : ESTADO[m.estado]}
              tabIndex={i === foco ? 0 : -1}
              onFocus={() => setFoco(i)}
              onClick={() => abrir(m)}
              data-testid={`mesa-${m.nombre}`}
            >
              <span className="mesa__nombre">{m.nombre}</span>
              {m.estado === "LIBRE" ? (
                <small>
                  <Users aria-hidden="true" />
                  {m.capacidad} pers.
                </small>
              ) : (
                <>
                  <span className="mesa__total rp-num">{formatMoney(m.total)}</span>
                  <small>
                    {m.estado === "POR_PAGAR" ? <Receipt aria-hidden="true" /> : <Clock aria-hidden="true" />}
                    {m.numeroOrden != null && `#${m.numeroOrden} · `}
                    {min != null && `${min} min`}
                    {m.meseroNombre && ` · ${m.meseroNombre}`}
                  </small>
                </>
              )}
              {m.bloqueo && (
                <span className="mesa__bloqueo">
                  <Lock aria-hidden="true" />
                  {m.bloqueo.usuarioNombre}
                </span>
              )}
            </button>
          );
        })}
        {salon && visibles.length === 0 && <p className="rp-secondary">Ninguna mesa coincide con «{q}».</p>}
      </div>
    </section>
  );
}
