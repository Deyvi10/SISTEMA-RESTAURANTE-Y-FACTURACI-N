import { useEffect, useState } from "react";
import { ApiError, uuidv7 } from "../api/client";
import { api, useGuardar, useLocales, usePersonal } from "../api/hooks";
import { useSession } from "../api/session";
import type { Persona, Rol } from "../api/types";
import { useFeedback } from "../components/feedback";
import { AppIcon, Empty, Field, initials, Sheet, Spinner, tintFor, ToggleRow } from "../components/ui";
import { Icon } from "../lib/icons";
import { formatUSD } from "../lib/money";

export const ROLES: { rol: Rol; nombre: string; icono: string; tint: string; texto: string }[] = [
  { rol: "MESERO", nombre: "Mesero", icono: "utensils", tint: "orange", texto: "Toma pedidos en la app con su PIN" },
  { rol: "CAJERO", nombre: "Cajero", icono: "caja", tint: "green", texto: "Cobra y factura en la caja" },
  { rol: "COCINA", nombre: "Cocina", icono: "chef-hat", tint: "red", texto: "Ve y despacha comandas" },
  { rol: "BODEGA", nombre: "Bodega", icono: "tienda", tint: "teal", texto: "Inventario y tomas físicas" },
  { rol: "ADMIN", nombre: "Administrador", icono: "dueno", tint: "indigo", texto: "Configura todo el restaurante" },
];
const rolInfo = (r: Rol) => ROLES.find((x) => x.rol === r)!;

/** Entrada de PIN con puntos y teclado grande, como el desbloqueo del iPhone. */
function PinInput({ value, onChange, error }: { value: string; onChange: (v: string) => void; error?: string | undefined }) {
  const teclas = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "", "0", "⌫"];
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.target instanceof HTMLElement) || e.target.closest("input, textarea, select")) return;
      if (/^\d$/.test(e.key) && value.length < 6) onChange(value + e.key);
      if (e.key === "Backspace") onChange(value.slice(0, -1));
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [value, onChange]);
  return (
    <div className="rp-field" style={{ alignItems: "center" }}>
      <label>PIN de 4 a 6 dígitos</label>
      <div className="pin-dots-lg" aria-label={`${value.length} dígitos escritos`} role="status" data-error={error ? "true" : undefined}>
        {Array.from({ length: Math.max(4, value.length) }, (_, i) => <i key={i} data-filled={i < value.length} />)}
      </div>
      <div className="keypad-sm">
        {teclas.map((t, i) =>
          t === "" ? <span key={i} className="rp-key rp-key--blank" /> : (
            <button key={i} type="button" className={`rp-key ${t === "⌫" ? "rp-key--text" : ""}`} aria-label={t === "⌫" ? "Borrar" : t}
              onClick={() => onChange(t === "⌫" ? value.slice(0, -1) : value.length < 6 ? value + t : value)}>{t}</button>
          ))}
      </div>
      {error ? <span className="rp-field__error" role="alert">{error}</span> : <span className="rp-secondary" style={{ fontSize: 13 }}>Evita 1234, 0000 o tu año de nacimiento.</span>}
    </div>
  );
}

export function Personal() {
  const personal = usePersonal();
  const [edit, setEdit] = useState<Persona | "nueva" | null>(null);
  if (personal.isLoading) return <Spinner />;
  const lista = personal.data ?? [];
  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="rp-t-large-title">Personal</h1>
          <p>Un mesero nuevo trabaja en 10 segundos: nombre, rol y PIN. Nada de correos ni contraseñas largas.</p>
        </div>
        <button className="rp-btn rp-btn--primary" onClick={() => setEdit("nueva")}><Icon name="agregar" size={18} /> Nueva persona</button>
      </header>
      {lista.length <= 1 ? (
        <Empty icono="personal" tint="indigo" titulo="Suma a tu equipo" texto="Crea a tus meseros y cajeros. Entrarán a la app tocando su foto y escribiendo su PIN."
          accion={<button className="rp-btn rp-btn--primary" onClick={() => setEdit("nueva")}>Crear la primera persona</button>} />
      ) : null}
      <div className="equipo">
        {lista.map((p) => {
          const r = rolInfo(p.rol);
          return (
            <button key={p.id} className="persona" data-activo={p.activo} onClick={() => setEdit(p)}>
              <span className="avatar" style={{ ["--tint" as string]: `var(--rp-tint-${tintFor(p.nombreMostrar)})` }}>
                {p.avatarUrl ? <img src={p.avatarUrl} alt="" /> : initials(p.nombreMostrar)}
              </span>
              <b>{p.nombreMostrar}</b>
              <span className="meta">
                <span className="rp-status rp-status--neutral" style={{ gap: 4 }}><Icon name={r.icono} size={12} /> {p.esDueno ? "Dueño" : r.nombre}</span>
                {!p.activo && <span className="rp-status rp-status--danger">Inactivo</span>}
                {p.tienePin && <span className="rp-status rp-status--success">PIN</span>}
                {p.accesoWeb && <span className="rp-status rp-status--neutral">Panel web</span>}
              </span>
            </button>
          );
        })}
      </div>
      {edit && <PersonaSheet persona={edit === "nueva" ? null : edit} onClose={() => setEdit(null)} />}
    </>
  );
}

function PersonaSheet({ persona, onClose }: { persona: Persona | null; onClose: () => void }) {
  const { me } = useSession();
  const { toast, confirm } = useFeedback();
  const [id] = useState(() => persona?.id ?? uuidv7());
  const [nombre, setNombre] = useState(persona?.nombreMostrar ?? "");
  const [rol, setRol] = useState<Rol>(persona?.rol ?? "MESERO");
  const [pin, setPin] = useState("");
  const [email, setEmail] = useState(persona?.email ?? "");
  const [cambiarPin, setCambiarPin] = useState(!persona);
  const [errores, setErrores] = useState<Record<string, string>>({});
  const conEmail = rol === "ADMIN" || rol === "CAJERO";
  const guardar = useGuardar(async () => {
    const body = { id, nombreMostrar: nombre, rol, email: conEmail && email ? email : null, pin: cambiarPin ? pin : "" };
    return persona ? api.editarPersona(persona.id, body) : api.crearPersona(body);
  }, ["personal"]);
  const estado = useGuardar((activo: boolean) => api.cambiarEstado(id, activo), ["personal"]);

  async function onSave() {
    setErrores({});
    try {
      await guardar.mutateAsync(undefined);
      toast(persona ? "Cambios guardados" : conEmail && email ? `${nombre} creado. Le enviamos su invitación por correo.` : `${nombre} ya puede entrar con su PIN`);
      onClose();
    } catch (e) {
      if (e instanceof ApiError) setErrores(e.fields), toast(e.message, "error");
    }
  }
  async function onToggleActivo() {
    if (!persona) return;
    const desactivar = persona.activo;
    if (desactivar && !(await confirm({ titulo: `¿Desactivar a ${persona.nombreMostrar}?`, mensaje: "Sale de todas las pantallas al instante y ya no podrá entrar. Su historial se conserva.", confirmar: "Desactivar", destructivo: true }))) return;
    try {
      await estado.mutateAsync(!desactivar);
      toast(desactivar ? `${persona.nombreMostrar} desactivado` : `${persona.nombreMostrar} activado`);
      onClose();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo cambiar.", "error");
    }
  }
  const puedeDesactivar = persona && !persona.esDueno && persona.id !== me?.id;

  return (
    <Sheet open title={persona ? persona.nombreMostrar : "Nueva persona"} onClose={onClose} onSave={() => void onSave()} busy={guardar.isPending}
      destructive={puedeDesactivar ? { label: persona.activo ? "Desactivar" : "Activar de nuevo", onClick: () => void onToggleActivo() } : undefined}>
      <div className="form">
        <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 8 }}>
          <span className="avatar" style={{ ["--tint" as string]: `var(--rp-tint-${tintFor(nombre || "?")})`, width: 84, height: 84, fontSize: 30 }}>{initials(nombre || "?")}</span>
        </div>
        <Field label="Nombre como aparece en la pantalla" error={errores.nombreMostrar}>{(fid) => <input id={fid} value={nombre} onChange={(e) => setNombre(e.target.value)} placeholder="Carlos M." maxLength={40} />}</Field>
        <div className="rp-field">
          <label>Rol</label>
          <div className="rol-picker">
            {ROLES.filter((r) => !persona?.esDueno || r.rol === "ADMIN").map((r) => (
              <button key={r.rol} type="button" className="icon-opt" aria-pressed={rol === r.rol} onClick={() => setRol(r.rol)} title={r.texto}>
                <AppIcon icono={r.icono} tint={r.tint} size={40} />{r.nombre}
              </button>
            ))}
          </div>
          <span className="rp-secondary" style={{ fontSize: 13 }}>{rolInfo(rol).texto}.</span>
          {errores.rol && <span className="rp-field__error">{errores.rol}</span>}
        </div>
        {conEmail && (
          <Field label={rol === "ADMIN" ? "Correo (obligatorio)" : "Correo para entrar al panel (opcional)"} error={errores.email}
            hint="Le enviaremos una invitación con su contraseña temporal.">
            {(fid, d) => <input id={fid} type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="nombre@restaurante.ec" aria-describedby={d} />}
          </Field>
        )}
        {persona && !cambiarPin ? (
          <div className="rp-group">
            <ToggleRow label="Asignar un PIN nuevo" detalle={persona.tienePin ? "El PIN actual deja de servir." : "Esta persona aún no tiene PIN."} checked={cambiarPin} onChange={setCambiarPin} />
          </div>
        ) : rol !== "ADMIN" || cambiarPin ? (
          <PinInput value={pin} onChange={setPin} error={errores.pin} />
        ) : null}
      </div>
    </Sheet>
  );
}

export function Ajustes() {
  const locales = useLocales();
  const { toast } = useFeedback();
  const local = locales.data?.[0];
  const [form, setForm] = useState(local);
  const [errores, setErrores] = useState<Record<string, string>>({});
  useEffect(() => setForm(local), [local]);
  const guardar = useGuardar(() => api.editarLocal(local!.id, {
    nombre: form?.nombre, direccion: form?.direccion, propinaLegalActiva: form?.propinaLegalActiva,
    propinaPorcentaje: form?.propinaPorcentaje, preciosIncluyenIva: form?.preciosIncluyenIva,
  }), ["locales", "productos"]);
  if (!form) return <Spinner />;
  const set = <K extends keyof typeof form>(k: K, v: (typeof form)[K]) => setForm({ ...form, [k]: v });
  async function onSave() {
    setErrores({});
    try {
      await guardar.mutateAsync(undefined);
      toast("Ajustes guardados");
    } catch (e) {
      if (e instanceof ApiError) setErrores(e.fields), toast(e.message, "error");
    }
  }
  return (
    <>
      <header className="page-head">
        <div><h1 className="rp-t-large-title">Ajustes</h1><p>Datos de tu local, propina de servicio e IVA.</p></div>
        <button className="rp-btn rp-btn--primary" onClick={() => void onSave()} disabled={guardar.isPending}>{guardar.isPending ? "Guardando…" : "Guardar cambios"}</button>
      </header>
      <div style={{ maxWidth: 640, display: "flex", flexDirection: "column", gap: 18 }}>
        <div className="rp-section-header" style={{ margin: "0 4px -10px" }}>Local</div>
        <div className="rp-group" style={{ margin: 0, padding: 16, display: "grid", gap: 14 }}>
          <Field label="Nombre del local" error={errores.nombre}>{(fid) => <input id={fid} value={form.nombre} onChange={(e) => set("nombre", e.target.value)} />}</Field>
          <Field label="Dirección">{(fid) => <input id={fid} value={form.direccion} onChange={(e) => set("direccion", e.target.value)} placeholder="Av. 9 de Octubre y Malecón, Guayaquil" />}</Field>
          <p className="rp-secondary" style={{ margin: 0, fontSize: 13 }}>Establecimiento SRI {form.codigoEstablecimiento} · Zona horaria {form.zonaHoraria}</p>
        </div>
        <div className="rp-section-header" style={{ margin: "0 4px -10px" }}>Cobro</div>
        <div className="rp-group" style={{ margin: 0 }}>
          <ToggleRow label="Cobrar servicio (propina legal)" detalle="Se calcula sobre el subtotal sin IVA y va aparte en la factura." checked={form.propinaLegalActiva} onChange={(v) => set("propinaLegalActiva", v)} />
          {form.propinaLegalActiva && (
            <div className="rp-cell">
              <span className="rp-cell__body">Porcentaje</span>
              <input aria-label="Porcentaje de servicio" inputMode="decimal" value={form.propinaPorcentaje} onChange={(e) => set("propinaPorcentaje", e.target.value)} style={{ width: 90, minHeight: 40, textAlign: "right" }} />
              <span>%</span>
            </div>
          )}
          <ToggleRow label="Mis precios incluyen IVA" detalle={`Recomendado: lo que ve el cliente es lo que paga. Ej.: ${formatUSD("15.00")} ya con IVA.`} checked={form.preciosIncluyenIva} onChange={(v) => set("preciosIncluyenIva", v)} />
        </div>
        {errores.propinaPorcentaje && <p className="error-inline">{errores.propinaPorcentaje}</p>}
      </div>
    </>
  );
}
