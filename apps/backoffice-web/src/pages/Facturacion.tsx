// Facturación electrónica SRI (F5-02, pasos 3 a 5 de F5-06): datos del emisor, régimen, puntos
// de emisión por caja y el interruptor para facturar. La firma electrónica (.p12) se carga en
// F5-07; mientras tanto solo existe el ambiente de pruebas.
import { useEffect, useState } from "react";
import { ApiError } from "../api/client";
import { api, useFacturacion, useGuardar } from "../api/hooks";
import type { ConfigFiscal, PuntoCaja, Regimen } from "../api/types";
import { useFeedback } from "../components/feedback";
import { AppIcon, Field, Segmented, Spinner, ToggleRow } from "../components/ui";
import { Icon } from "../lib/icons";

export const REGIMENES: { value: Regimen; label: string; detalle: string }[] = [
  { value: "GENERAL", label: "General", detalle: "Facturas con IVA según la actividad." },
  { value: "RIMPE_EMPRENDEDOR", label: "RIMPE Emprendedor", detalle: "La factura lleva la leyenda «CONTRIBUYENTE RÉGIMEN RIMPE»." },
  { value: "RIMPE_NEGOCIO_POPULAR", label: "RIMPE Negocio popular", detalle: "Emite notas de venta: esa modalidad todavía no está disponible." },
];

/** El formulario a partir de lo que devuelve la nube (los opcionales vacíos van como ""). */
export function formDe(c: ConfigFiscal) {
  return {
    razonSocial: c.razonSocial,
    nombreComercial: c.nombreComercial ?? "",
    direccionMatriz: c.direccionMatriz,
    obligadoContabilidad: c.obligadoContabilidad,
    contribuyenteEspecial: c.contribuyenteEspecial ?? "",
    agenteRetencion: c.agenteRetencion ?? "",
    regimen: c.regimen,
  };
}
type Form = ReturnType<typeof formDe>;

/** Cuerpo del PUT: los opcionales vacíos viajan como null. */
export function cuerpo(f: Form, activa: boolean) {
  const nulo = (s: string) => (s.trim() === "" ? null : s.trim());
  return { ...f, ambiente: 1, nombreComercial: nulo(f.nombreComercial), contribuyenteEspecial: nulo(f.contribuyenteEspecial), agenteRetencion: nulo(f.agenteRetencion), facturacionActiva: activa };
}

export function Facturacion() {
  const cfg = useFacturacion();
  if (cfg.isLoading || !cfg.data) return <Spinner />;
  return <FacturacionForm cfg={cfg.data} />;
}

function FacturacionForm({ cfg }: { cfg: ConfigFiscal }) {
  const { toast } = useFeedback();
  const [f, setF] = useState<Form>(() => formDe(cfg));
  const [errores, setErrores] = useState<Record<string, string>>({});
  useEffect(() => setF(formDe(cfg)), [cfg]);
  const guardar = useGuardar((activa: boolean) => api.guardarFacturacion(cuerpo(f, activa)), ["facturacion"]);
  const set = <K extends keyof Form>(k: K, v: Form[K]) => setF((x) => ({ ...x, [k]: v }));

  async function onGuardar(activa: boolean, mensaje: string) {
    setErrores({});
    try {
      await guardar.mutateAsync(activa);
      toast(mensaje);
    } catch (e) {
      if (e instanceof ApiError) {
        setErrores({ ...e.fields, general: Object.keys(e.fields).length ? "" : e.message });
        if (!Object.keys(e.fields).length) toast(e.message, "error");
      } else toast("No se pudo guardar.", "error");
    }
  }
  const puedeActivar = cfg.guardada && cfg.pendientes.length === 0;

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="rp-t-large-title">Facturación SRI</h1>
          <p>Con la facturación activa, cada cobro emite su comprobante electrónico con clave de acceso, aunque no haya internet.</p>
        </div>
      </header>

      <section className="seccion">
        <div className="rp-group">
          <div className="rp-cell">
            <AppIcon icono="receipt" tint={cfg.facturacionActiva ? "green" : "pink"} size={30} />
            <span className="rp-cell__body">
              <span className="rp-cell__title">{cfg.facturacionActiva ? "Facturando en ambiente de pruebas" : "Facturación apagada"}</span>
              <span className="rp-cell__subtitle">
                {cfg.facturacionActiva ? "Los comprobantes salen con la leyenda «AMBIENTE DE PRUEBAS – SIN VALIDEZ TRIBUTARIA»." : "La caja emite documentos internos sin valor tributario."}
              </span>
            </span>
          </div>
          <ToggleRow
            label="Facturar electrónicamente"
            detalle={puedeActivar || cfg.facturacionActiva ? "Se aplica en la caja en segundos." : "Completa lo pendiente para activarla."}
            checked={cfg.facturacionActiva}
            onChange={(v) => {
              if (v && !puedeActivar) return;
              void onGuardar(v, v ? "Facturación activada (pruebas)" : "Facturación apagada");
            }}
          />
        </div>
        {cfg.pendientes.length > 0 && (
          <ul className="pendientes" data-testid="pendientes">
            {cfg.pendientes.map((p) => (
              <li key={p}>
                <Icon name="error" size={15} /> {p}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="seccion">
        <h2>Datos del emisor</h2>
        <p>Tal como constan en tu RUC: van en cada comprobante.</p>
        <div className="form">
          <Field label="RUC" hint="Es el RUC con el que registraste el restaurante.">
            {(fid) => <input id={fid} value={cfg.ruc} readOnly />}
          </Field>
          <Field label="Razón social" error={errores.razonSocial}>
            {(fid) => <input id={fid} value={f.razonSocial} maxLength={300} onChange={(e) => set("razonSocial", e.target.value)} />}
          </Field>
          <Field label="Nombre comercial (opcional)" error={errores.nombreComercial}>
            {(fid) => <input id={fid} value={f.nombreComercial} maxLength={300} onChange={(e) => set("nombreComercial", e.target.value)} />}
          </Field>
          <Field label="Dirección de la matriz" error={errores.direccionMatriz}>
            {(fid) => <input id={fid} value={f.direccionMatriz} maxLength={300} onChange={(e) => set("direccionMatriz", e.target.value)} />}
          </Field>
          <div className="rp-group">
            <ToggleRow label="Obligado a llevar contabilidad" checked={f.obligadoContabilidad} onChange={(v) => set("obligadoContabilidad", v)} />
          </div>
          <Field label="Contribuyente especial (N.º de resolución, opcional)" error={errores.contribuyenteEspecial}>
            {(fid) => <input id={fid} value={f.contribuyenteEspecial} maxLength={13} onChange={(e) => set("contribuyenteEspecial", e.target.value)} />}
          </Field>
          <Field label="Agente de retención (N.º de resolución, opcional)" error={errores.agenteRetencion}>
            {(fid) => <input id={fid} value={f.agenteRetencion} inputMode="numeric" maxLength={8} onChange={(e) => set("agenteRetencion", e.target.value.replace(/\D/g, ""))} />}
          </Field>
          <Segmented label="Régimen tributario" value={f.regimen} options={REGIMENES.map((r) => ({ value: r.value, label: r.label }))} onChange={(v) => set("regimen", v)} />
          <p className="rp-secondary">{REGIMENES.find((r) => r.value === f.regimen)?.detalle}</p>
          <Segmented label="Ambiente" value="1" options={[{ value: "1", label: "Pruebas" }]} onChange={() => {}} />
          <p className="rp-secondary">Producción se habilita cuando cargues tu firma electrónica (.p12) y se apruebe una factura de prueba.</p>
          {errores.general && (
            <p className="rp-field__error" role="alert">
              {errores.general}
            </p>
          )}
          <button className="rp-btn rp-btn--primary" disabled={guardar.isPending} onClick={() => void onGuardar(cfg.facturacionActiva, "Datos del emisor guardados")} data-testid="guardar-emisor">
            {cfg.guardada ? "Guardar cambios" : "Confirmar datos del emisor"}
          </button>
        </div>
      </section>

      <section className="seccion">
        <h2>Puntos de emisión</h2>
        <p>Cada caja emite con su propia serie, por ejemplo 001-001. Cambiar la serie empieza una numeración nueva.</p>
        <div className="rp-group">
          {cfg.cajas.map((c) => (
            <PuntoFila key={c.cajaId} p={c} />
          ))}
        </div>
      </section>

      <section className="seccion">
        <h2>Firma electrónica</h2>
        <div className="rp-group">
          <div className="rp-cell">
            <AppIcon icono="seguro" tint="gray" size={30} />
            <span className="rp-cell__body">
              <span className="rp-cell__title">Certificado .p12</span>
              <span className="rp-cell__subtitle">Pronto podrás subirlo aquí; se guarda cifrado y solo se usa para firmar en la nube.</span>
            </span>
          </div>
        </div>
      </section>
    </>
  );
}

function PuntoFila({ p }: { p: PuntoCaja }) {
  const { toast } = useFeedback();
  const [est, setEst] = useState(p.establecimiento ?? "001");
  const [pto, setPto] = useState(p.puntoEmision ?? "");
  const [error, setError] = useState("");
  const asignar = useGuardar(() => api.asignarPunto(p.cajaId, { establecimiento: est, puntoEmision: pto }), ["facturacion"]);
  const cambio = est !== (p.establecimiento ?? "") || pto !== (p.puntoEmision ?? "");
  const tres = (s: string) => s.replace(/\D/g, "").slice(0, 3);
  async function guardar() {
    setError("");
    try {
      await asignar.mutateAsync(undefined);
      toast(`«${p.caja}» emite con la serie ${est}-${pto}`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo guardar.");
    }
  }
  return (
    <div className="rp-cell punto-fila" data-testid={`punto-${p.caja}`}>
      <AppIcon icono="caja" tint={p.puntoId ? "green" : "orange"} size={30} />
      <span className="rp-cell__body">
        <span className="rp-cell__title">{p.caja}</span>
        <span className="rp-cell__subtitle">{error || (p.puntoId ? (p.conNodo ? "Serie lista" : "Serie lista; se numerará al activar el Nodo Local") : "Sin serie")}</span>
      </span>
      <input aria-label={`Establecimiento de ${p.caja}`} className="serie" value={est} inputMode="numeric" onChange={(e) => setEst(tres(e.target.value))} placeholder="001" />
      <span>-</span>
      <input aria-label={`Punto de emisión de ${p.caja}`} className="serie" value={pto} inputMode="numeric" onChange={(e) => setPto(tres(e.target.value))} placeholder="001" />
      <button className="rp-btn rp-btn--gray rp-btn--sm" disabled={!cambio || est.length !== 3 || pto.length !== 3 || asignar.isPending} onClick={() => void guardar()}>
        Guardar
      </button>
    </div>
  );
}
