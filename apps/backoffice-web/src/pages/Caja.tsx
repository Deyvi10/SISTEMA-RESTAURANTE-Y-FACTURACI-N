// Configuración de la caja (F4-03, F4-06, F4-09): cajas del local, métodos de pago con su
// forma de pago del SRI y motivos de descuento. El Nodo Local los recibe al instante.
import { useState } from "react";
import { ApiError, uuidv7 } from "../api/client";
import { api, useCajas, useEstaciones, useGuardar, useLocales, useMetodosPago, useMotivos } from "../api/hooks";
import type { Caja as CajaT, MetodoPago, TipoMetodoPago } from "../api/types";
import { useFeedback } from "../components/feedback";
import { AppIcon, Field, Segmented, Sheet, Spinner, ToggleRow } from "../components/ui";
import { Icon } from "../lib/icons";

export const TIPOS_METODO: { value: TipoMetodoPago; label: string; sri: string }[] = [
  { value: "EFECTIVO", label: "Efectivo", sri: "01 · Sin sistema financiero" },
  { value: "TARJETA_CREDITO", label: "Tarjeta de crédito", sri: "19 · Tarjeta de crédito" },
  { value: "TARJETA_DEBITO", label: "Tarjeta de débito", sri: "16 · Tarjeta de débito" },
  { value: "TRANSFERENCIA", label: "Transferencia", sri: "20 · Otros con sistema financiero" },
  { value: "BILLETERA", label: "Billetera (DeUna, Payphone…)", sri: "20 · Otros con sistema financiero" },
  { value: "OTRO", label: "Otro", sri: "20 · Otros con sistema financiero" },
];

const TINTE_METODO: Record<TipoMetodoPago, string> = {
  EFECTIVO: "green", TARJETA_CREDITO: "blue", TARJETA_DEBITO: "indigo", TRANSFERENCIA: "teal", BILLETERA: "purple", OTRO: "gray",
};

export function Caja() {
  const cajas = useCajas();
  const metodos = useMetodosPago();
  const motivos = useMotivos();
  const estaciones = useEstaciones();
  const [editCaja, setEditCaja] = useState<CajaT | "nueva" | null>(null);
  const [editMetodo, setEditMetodo] = useState<MetodoPago | "nuevo" | null>(null);
  const [nuevoMotivo, setNuevoMotivo] = useState(false);
  if (cajas.isLoading || metodos.isLoading) return <Spinner />;
  const nombreEstacion = (id: string | null) => estaciones.data?.find((e) => e.id === id)?.nombre;

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="rp-t-large-title">Caja</h1>
          <p>Cómo cobra tu restaurante. Los cambios llegan a la caja en segundos y funcionan aunque se caiga el internet.</p>
        </div>
      </header>

      <section className="seccion">
        <div className="seccion__cabecera">
          <h2>Cajas</h2>
          <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => setEditCaja("nueva")}><Icon name="agregar" size={15} /> Caja</button>
        </div>
        <p>Cada caja abre su turno con un fondo y hace su propio cierre. Sus documentos salen en la impresora de su estación, con el cajón.</p>
        <div className="rp-group">
          {(cajas.data ?? []).map((c) => (
            <button key={c.id} className="rp-cell" onClick={() => setEditCaja(c)}>
              <AppIcon icono="caja" tint="green" size={30} />
              <span className="rp-cell__body">
                <span className="rp-cell__title">{c.nombre}{!c.activa && " · desactivada"}</span>
                <span className="rp-cell__subtitle">{nombreEstacion(c.estacionId) ? `Imprime en «${nombreEstacion(c.estacionId)}»` : "Sin estación: elige dónde imprime"}</span>
              </span>
              <Icon name="chevronRight" size={18} />
            </button>
          ))}
        </div>
      </section>

      <section className="seccion">
        <div className="seccion__cabecera">
          <h2>Métodos de pago</h2>
          <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => setEditMetodo("nuevo")}><Icon name="agregar" size={15} /> Método</button>
        </div>
        <p>Cada método va a la factura con su forma de pago del SRI. Una cuenta se puede pagar con varios a la vez.</p>
        <div className="rp-group">
          {(metodos.data ?? []).map((m) => (
            <button key={m.id} className="rp-cell" onClick={() => setEditMetodo(m)}>
              <AppIcon icono={m.icono} tint={TINTE_METODO[m.tipo]} size={30} />
              <span className="rp-cell__body">
                <span className="rp-cell__title">{m.nombre}{!m.activo && " · desactivado"}</span>
                <span className="rp-cell__subtitle">
                  SRI {m.codigoSri}{m.abreCajon ? " · abre el cajón" : ""}{m.pideReferencia ? " · pide referencia" : ""}
                </span>
              </span>
              <Icon name="chevronRight" size={18} />
            </button>
          ))}
        </div>
      </section>

      <section className="seccion">
        <div className="seccion__cabecera">
          <h2>Motivos de descuento</h2>
          <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => setNuevoMotivo(true)}><Icon name="agregar" size={15} /> Motivo</button>
        </div>
        <p>El cajero elige uno al descontar o regalar un plato; queda registrado en la auditoría.</p>
        <Motivos />
      </section>

      {editCaja && <CajaSheet caja={editCaja === "nueva" ? null : editCaja} onClose={() => setEditCaja(null)} />}
      {editMetodo && <MetodoSheet metodo={editMetodo === "nuevo" ? null : editMetodo} onClose={() => setEditMetodo(null)} />}
      {nuevoMotivo && <MotivoSheet onClose={() => setNuevoMotivo(false)} />}
      {motivos.isError && <p className="rp-field__error">No se pudieron cargar los motivos.</p>}
    </>
  );
}

function Motivos() {
  const motivos = useMotivos();
  const { toast, confirm } = useFeedback();
  const borrar = useGuardar((id: string) => api.borrarMotivo(id), ["motivos-descuento"]);
  return (
    <div className="rp-group">
      {(motivos.data ?? []).map((m) => (
        <div key={m.id} className="rp-cell">
          <AppIcon icono={m.tipo === "CORTESIA" ? "magia" : "porcentaje"} tint={m.tipo === "CORTESIA" ? "pink" : "orange"} size={30} />
          <span className="rp-cell__body">
            <span className="rp-cell__title">{m.nombre}</span>
            <span className="rp-cell__subtitle">{m.tipo === "CORTESIA" ? "Cortesía (100 %)" : "Descuento"}</span>
          </span>
          <button className="rp-btn rp-btn--plain rp-btn--sm" aria-label={`Eliminar ${m.nombre}`}
            onClick={async () => {
              if (!(await confirm({ titulo: `¿Eliminar «${m.nombre}»?`, mensaje: "Los descuentos ya hechos con este motivo se conservan.", confirmar: "Eliminar", destructivo: true }))) return;
              try {
                await borrar.mutateAsync(m.id);
              } catch (e) {
                toast(e instanceof ApiError ? e.message : "No se pudo eliminar.", "error");
              }
            }}>
            <Icon name="eliminar" size={16} />
          </button>
        </div>
      ))}
    </div>
  );
}

function CajaSheet({ caja, onClose }: { caja: CajaT | null; onClose: () => void }) {
  const { toast, confirm } = useFeedback();
  const locales = useLocales();
  const estaciones = useEstaciones();
  const deCaja = (estaciones.data ?? []).filter((e) => e.tipo === "CAJA");
  const [id] = useState(() => caja?.id ?? uuidv7());
  const [nombre, setNombre] = useState(caja?.nombre ?? "");
  const [estacionId, setEstacionId] = useState<string>(caja?.estacionId ?? deCaja[0]?.id ?? "");
  const [activa, setActiva] = useState(caja?.activa ?? true);
  const [errores, setErrores] = useState<Record<string, string>>({});
  const guardar = useGuardar(() => {
    const body = { id, localId: caja?.localId ?? locales.data?.[0]?.id, nombre, estacionId: estacionId || null, activa };
    return caja ? api.editarCaja(caja.id, body) : api.crearCaja(body);
  }, ["cajas"]);
  const borrar = useGuardar(() => api.borrarCaja(id), ["cajas"]);
  async function onSave() {
    try {
      await guardar.mutateAsync(undefined);
      toast(caja ? "Caja actualizada" : `Caja «${nombre}» creada`);
      onClose();
    } catch (e) {
      if (e instanceof ApiError) setErrores({ ...e.fields, general: Object.keys(e.fields).length ? "" : e.message });
      else setErrores({ general: "No se pudo guardar." });
    }
  }
  async function onDelete() {
    if (!(await confirm({ titulo: `¿Eliminar ${caja?.nombre}?`, mensaje: "Sus turnos y cierres ya hechos se conservan.", confirmar: "Eliminar", destructivo: true }))) return;
    try {
      await borrar.mutateAsync(undefined);
      toast("Caja eliminada");
      onClose();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo eliminar.", "error");
    }
  }
  return (
    <Sheet open title={caja ? "Editar caja" : "Nueva caja"} onClose={onClose} onSave={() => void onSave()} busy={guardar.isPending}
      destructive={caja ? { label: "Eliminar caja", onClick: () => void onDelete() } : undefined}>
      <div className="form">
        <Field label="Nombre" error={errores.nombre}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Caja 1, Barra, Terraza…" maxLength={40} />}</Field>
        <Field label="Imprime en" error={errores.estacionId} hint="Estación de tipo Caja: documentos, pre-cuentas y el cajón de dinero.">
          {(fid) => (
            <select id={fid} value={estacionId} onChange={(e) => setEstacionId(e.target.value)}>
              <option value="">Sin estación</option>
              {deCaja.map((e) => <option key={e.id} value={e.id}>{e.nombre}</option>)}
            </select>
          )}
        </Field>
        {caja && <div className="rp-group"><ToggleRow label="Caja activa" detalle="Una caja desactivada no aparece para abrir turno." checked={activa} onChange={setActiva} /></div>}
        {errores.general && <p className="rp-field__error" role="alert">{errores.general}</p>}
      </div>
    </Sheet>
  );
}

function MetodoSheet({ metodo, onClose }: { metodo: MetodoPago | null; onClose: () => void }) {
  const { toast, confirm } = useFeedback();
  const [id] = useState(() => metodo?.id ?? uuidv7());
  const [nombre, setNombre] = useState(metodo?.nombre ?? "");
  const [tipo, setTipo] = useState<TipoMetodoPago>(metodo?.tipo ?? "TARJETA_CREDITO");
  const [pideReferencia, setPideReferencia] = useState(metodo?.pideReferencia ?? true);
  const [abreCajon, setAbreCajon] = useState(metodo?.abreCajon ?? false);
  const [activo, setActivo] = useState(metodo?.activo ?? true);
  const [errores, setErrores] = useState<Record<string, string>>({});
  const efectivo = metodo?.tipo === "EFECTIVO";
  const guardar = useGuardar(() => {
    const body = { id, nombre, tipo, pideReferencia, abreCajon, activo };
    return metodo ? api.editarMetodo(metodo.id, body) : api.crearMetodo(body);
  }, ["metodos-pago"]);
  const borrar = useGuardar(() => api.borrarMetodo(id), ["metodos-pago"]);
  const sri = TIPOS_METODO.find((t) => t.value === tipo)?.sri;
  async function onSave() {
    try {
      await guardar.mutateAsync(undefined);
      toast(metodo ? "Método actualizado" : `«${nombre}» agregado`);
      onClose();
    } catch (e) {
      if (e instanceof ApiError) setErrores({ ...e.fields, general: Object.keys(e.fields).length ? "" : e.message });
      else setErrores({ general: "No se pudo guardar." });
    }
  }
  async function onDelete() {
    if (!(await confirm({ titulo: `¿Eliminar ${metodo?.nombre}?`, mensaje: "Los pagos ya registrados se conservan.", confirmar: "Eliminar", destructivo: true }))) return;
    try {
      await borrar.mutateAsync(undefined);
      toast("Método eliminado");
      onClose();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo eliminar.", "error");
    }
  }
  return (
    <Sheet open title={metodo ? "Editar método" : "Nuevo método de pago"} onClose={onClose} onSave={() => void onSave()} busy={guardar.isPending}
      destructive={metodo && !efectivo ? { label: "Eliminar método", onClick: () => void onDelete() } : undefined}>
      <div className="form">
        <Field label="Nombre" error={errores.nombre}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Tarjeta, DeUna, Transferencia…" maxLength={30} />}</Field>
        <Field label="Tipo" error={errores.tipo} hint={sri ? `Forma de pago SRI: ${sri}` : undefined}>
          {(fid) => (
            <select id={fid} value={tipo} onChange={(e) => setTipo(e.target.value as TipoMetodoPago)} disabled={efectivo}>
              {TIPOS_METODO.filter((t) => efectivo || t.value !== "EFECTIVO").map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
          )}
        </Field>
        <div className="rp-group">
          {!efectivo && <ToggleRow label="Pedir referencia" detalle="Lote, referencia y últimos 4 dígitos (opcionales) para cuadrar los vouchers." checked={pideReferencia} onChange={setPideReferencia} />}
          {!efectivo && <ToggleRow label="Abrir el cajón" detalle="Solo si con este método también entra dinero al cajón." checked={abreCajon} onChange={setAbreCajon} />}
          {metodo && !efectivo && <ToggleRow label="Disponible en la caja" checked={activo} onChange={setActivo} />}
        </div>
        {efectivo && <p className="rp-secondary">El efectivo siempre está disponible y abre el cajón: es con lo que se cuadra la caja.</p>}
        {errores.general && <p className="rp-field__error" role="alert">{errores.general}</p>}
      </div>
    </Sheet>
  );
}

function MotivoSheet({ onClose }: { onClose: () => void }) {
  const { toast } = useFeedback();
  const [nombre, setNombre] = useState("");
  const [tipo, setTipo] = useState<"DESCUENTO" | "CORTESIA">("DESCUENTO");
  const [error, setError] = useState<string>();
  const guardar = useGuardar(() => api.crearMotivo({ id: uuidv7(), nombre, tipo }), ["motivos-descuento"]);
  async function onSave() {
    try {
      await guardar.mutateAsync(undefined);
      toast(`Motivo «${nombre}» agregado`);
      onClose();
    } catch (e) {
      setError(e instanceof ApiError ? (e.fields.nombre ?? e.message) : "No se pudo guardar.");
    }
  }
  return (
    <Sheet open title="Nuevo motivo" onClose={onClose} onSave={() => void onSave()} busy={guardar.isPending}>
      <div className="form">
        <Segmented label="Tipo" value={tipo} onChange={setTipo} options={[{ value: "DESCUENTO", label: "Descuento" }, { value: "CORTESIA", label: "Cortesía" }]} />
        <Field label="Motivo" error={error}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder={tipo === "CORTESIA" ? "Invitación de la casa" : "Cliente frecuente"} maxLength={40} />}</Field>
      </div>
    </Sheet>
  );
}
