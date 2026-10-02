// Facturación electrónica SRI (F5-02, F5-06, F5-07): datos del emisor, régimen, puntos de
// emisión por caja, la firma electrónica (.p12, se guarda cifrada y solo la usa el worker
// fiscal) y el interruptor para facturar. Producción se abre con la firma vigente y una
// factura de prueba autorizada por el SRI.
import { useEffect, useRef, useState } from "react";
import { Asistente } from "./AsistenteFiscal";
import { ApiError } from "../api/client";
import { api, useFacturacion, useGuardar } from "../api/hooks";
import type { CambioRegimen, Certificado, ConfigFiscal, PuntoCaja, Regimen } from "../api/types";
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
    ambiente: String(c.ambiente) as "1" | "2",
  };
}
type Form = ReturnType<typeof formDe>;

/** Cuerpo del PUT: los opcionales vacíos viajan como null. */
export function cuerpo(f: Form, activa: boolean) {
  const nulo = (s: string) => (s.trim() === "" ? null : s.trim());
  return { ...f, ambiente: Number(f.ambiente), nombreComercial: nulo(f.nombreComercial), contribuyenteEspecial: nulo(f.contribuyenteEspecial), agenteRetencion: nulo(f.agenteRetencion), facturacionActiva: activa };
}

export function Facturacion() {
  const cfg = useFacturacion();
  // La primera vez, el asistente de 6 pasos (F5-06); después, todo en una página.
  const [asistente, setAsistente] = useState<boolean | null>(null);
  if (cfg.isLoading || !cfg.data) return <Spinner />;
  if (asistente ?? !cfg.data.guardada) return <Asistente cfg={cfg.data} salir={() => setAsistente(false)} />;
  return <FacturacionForm cfg={cfg.data} abrirAsistente={() => setAsistente(true)} />;
}

function FacturacionForm({ cfg, abrirAsistente }: { cfg: ConfigFiscal; abrirAsistente: () => void }) {
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
      setConfirmar(false);
      toast(mensaje);
    } catch (e) {
      if (e instanceof ApiError) {
        setErrores({ ...e.fields, general: Object.keys(e.fields).length ? "" : e.message });
        if (!Object.keys(e.fields).length) toast(e.message, "error");
      } else toast("No se pudo guardar.", "error");
    }
  }
  const puedeActivar = cfg.guardada && cfg.pendientes.length === 0;
  const produccion = cfg.ambiente === 2;
  // Paso a producción con confirmación dentro de la página (F5-15).
  const [confirmar, setConfirmar] = useState(false);
  const puedeProduccion = produccion || (!!cfg.certificado && cfg.certificado.diasRestantes >= 0 && cfg.pruebaAprobada);

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="rp-t-large-title">Facturación SRI</h1>
          <p>Con la facturación activa, cada cobro emite su comprobante electrónico con clave de acceso, aunque no haya internet.</p>
        </div>
        <button className="rp-btn rp-btn--gray" onClick={abrirAsistente} data-testid="abrir-asistente">
          Asistente paso a paso
        </button>
      </header>

      <section className="seccion">
        <div className="rp-group">
          <div className="rp-cell">
            <AppIcon icono="receipt" tint={cfg.facturacionActiva ? "green" : "pink"} size={30} />
            <span className="rp-cell__body">
              <span className="rp-cell__title">{!cfg.facturacionActiva ? "Facturación apagada" : produccion ? "Facturando en producción" : "Facturando en ambiente de pruebas"}</span>
              <span className="rp-cell__subtitle">
                {!cfg.facturacionActiva
                  ? "La caja emite documentos internos sin valor tributario."
                  : produccion
                    ? "Cada cobro emite una factura con validez tributaria."
                    : "Los comprobantes salen con la leyenda «AMBIENTE DE PRUEBAS – SIN VALIDEZ TRIBUTARIA»."}
              </span>
            </span>
          </div>
          <ToggleRow
            label="Facturar electrónicamente"
            detalle={puedeActivar || cfg.facturacionActiva ? "Se aplica en la caja en segundos." : "Completa lo pendiente para activarla."}
            checked={cfg.facturacionActiva}
            onChange={(v) => {
              if (v && !puedeActivar) return;
              void onGuardar(v, v ? (produccion ? "Facturación activada" : "Facturación activada (pruebas)") : "Facturación apagada");
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
        {cfg.avisos.length > 0 && (
          <ul className="pendientes avisos" data-testid="avisos">
            {cfg.avisos.map((p) => (
              <li key={p}>
                <Icon name="info" size={15} /> {p}
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
          <Segmented
            label="Ambiente"
            value={f.ambiente}
            options={
              produccion
                ? [{ value: "2", label: "Producción" }]
                : puedeProduccion
                  ? [{ value: "1", label: "Pruebas" }, { value: "2", label: "Producción" }]
                  : [{ value: "1", label: "Pruebas" }]
            }
            onChange={(v) => set("ambiente", v)}
          />
          <p className="rp-secondary">
            {produccion
              ? "Facturas en producción: cada comprobante tiene validez tributaria."
              : puedeProduccion
                ? "Producción: las facturas tienen validez tributaria. Pruebas: el SRI las revisa pero no valen."
                : "Producción se habilita con tu firma electrónica vigente y una factura de prueba autorizada por el SRI."}
          </p>
          {confirmar && (
            <div className="confirmar-produccion" role="alertdialog" aria-label="Pasar a producción" data-testid="confirmar-produccion">
              <p>
                <strong>¿Pasar a producción?</strong>
              </p>
              <p>Desde ahora, cada cobro emitirá una factura con validez tributaria ante el SRI, con una numeración nueva. No se puede volver al ambiente de pruebas desde el panel.</p>
              <div className="acciones">
                <button className="rp-btn rp-btn--primary" disabled={guardar.isPending} onClick={() => void onGuardar(cfg.facturacionActiva, "Ya facturas en producción")} data-testid="si-produccion">
                  Sí, pasar a producción
                </button>
                <button className="rp-btn rp-btn--plain" onClick={() => (setConfirmar(false), set("ambiente", "1"))}>
                  Seguir en pruebas
                </button>
              </div>
            </div>
          )}
          {errores.general && (
            <p className="rp-field__error" role="alert">
              {errores.general}
            </p>
          )}
          <button
            className="rp-btn rp-btn--primary"
            disabled={guardar.isPending || confirmar}
            onClick={() => (f.ambiente === "2" && !produccion ? setConfirmar(true) : void onGuardar(cfg.facturacionActiva, "Datos del emisor guardados"))}
            data-testid="guardar-emisor"
          >
            {cfg.guardada ? "Guardar cambios" : "Confirmar datos del emisor"}
          </button>
        </div>
      </section>

      {cfg.guardada && <CambioSeccion actual={cfg.cambioProgramado} />}

      <section className="seccion">
        <h2>Puntos de emisión</h2>
        <p>Cada caja emite con su propia serie, por ejemplo 001-001. Cambiar la serie empieza una numeración nueva.</p>
        <div className="rp-group">
          {cfg.cajas.map((c) => (
            <PuntoFila key={c.cajaId} p={c} />
          ))}
        </div>
      </section>

      <FirmaSeccion cert={cfg.certificado} ruc={cfg.ruc} />
    </>
  );
}

export function PuntoFila({ p }: { p: PuntoCaja }) {
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

/** Días que faltan, en palabras. */
export function vigencia(c: Certificado): { texto: string; tint: string } {
  const fecha = new Date(c.validoHasta).toLocaleDateString("es-EC", { day: "numeric", month: "long", year: "numeric" });
  if (c.diasRestantes < 0) return { texto: `Venció el ${fecha}`, tint: "red" };
  if (c.diasRestantes === 0) return { texto: `Vence hoy (${fecha})`, tint: "red" };
  const dias = c.diasRestantes === 1 ? "1 día" : `${c.diasRestantes} días`;
  return { texto: `Vigente hasta el ${fecha} · quedan ${dias}`, tint: c.diasRestantes <= 30 ? "orange" : "green" };
}

// Paso 1 y 2 de F5-06: subir el .p12 con su contraseña. La nube lo valida (vigencia, llave,
// RUC del restaurante) y lo guarda cifrado; nunca se vuelve a mostrar ni descargar.
export function FirmaSeccion({ cert, ruc }: { cert: Certificado | null; ruc: string }) {
  const { toast } = useFeedback();
  const [abierto, setAbierto] = useState(!cert);
  const [archivo, setArchivo] = useState<File | null>(null);
  const [clave, setClave] = useState("");
  const [error, setError] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const subir = useGuardar(() => api.subirCertificado(archivo as File, clave), ["facturacion"]);
  useEffect(() => setAbierto(!cert), [cert]);

  async function onSubir() {
    setError("");
    try {
      const c = await subir.mutateAsync(undefined);
      setClave("");
      setArchivo(null);
      if (input.current) input.current.value = "";
      toast(c.aviso ?? "Firma electrónica guardada");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo subir la firma.");
    }
  }
  const v = cert && vigencia(cert);

  return (
    <section className="seccion" data-testid="firma">
      <h2>Firma electrónica</h2>
      <p>Tu certificado .p12 firma cada factura antes de enviarla al SRI. Se guarda cifrado y no se puede descargar ni ver otra vez.</p>
      {cert && v && (
        <div className="rp-group">
          <div className="rp-cell" data-testid="certificado">
            <AppIcon icono="seguro" tint={v.tint} size={30} />
            <span className="rp-cell__body">
              <span className="rp-cell__title">{cert.titular}</span>
              <span className="rp-cell__subtitle">
                {v.texto}
                {cert.ruc ? ` · RUC ${cert.ruc}` : ""}
              </span>
            </span>
            {!abierto && (
              <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => setAbierto(true)}>
                Reemplazar
              </button>
            )}
          </div>
        </div>
      )}
      {abierto && (
        <div className="form">
          <Field label="Archivo .p12" hint={`El de la firma del titular del RUC ${ruc}. Lo entrega tu entidad certificadora.`} error={error || undefined}>
            {(fid, desc) => (
              <input
                id={fid}
                ref={input}
                type="file"
                accept=".p12,.pfx,application/x-pkcs12"
                aria-describedby={desc}
                onChange={(e) => {
                  setArchivo(e.target.files?.[0] ?? null);
                  setError("");
                }}
              />
            )}
          </Field>
          <Field label="Contraseña de la firma">
            {(fid) => <input id={fid} type="password" autoComplete="off" value={clave} onChange={(e) => setClave(e.target.value)} />}
          </Field>
          <div className="acciones">
            <button className="rp-btn rp-btn--primary" disabled={!archivo || !clave || subir.isPending} onClick={() => void onSubir()} data-testid="subir-firma">
              {subir.isPending ? "Verificando…" : cert ? "Reemplazar firma" : "Subir firma"}
            </button>
            {cert && (
              <button className="rp-btn rp-btn--plain" onClick={() => setAbierto(false)}>
                Cancelar
              </button>
            )}
          </div>
        </div>
      )}
    </section>
  );
}

const manana = () => {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  return d.toLocaleDateString("en-CA"); // AAAA-MM-DD en la hora local
};

// Cambio de régimen con fecha de vigencia (F5-14, RF-05-05): rige para los comprobantes desde
// esa fecha; lo ya emitido no cambia.
function CambioSeccion({ actual }: { actual: CambioRegimen | null }) {
  const { toast } = useFeedback();
  const [abierto, setAbierto] = useState(false);
  const [c, setC] = useState<CambioRegimen>({ desde: manana(), regimen: "RIMPE_EMPRENDEDOR", obligadoContabilidad: false, contribuyenteEspecial: null, agenteRetencion: null });
  const [errores, setErrores] = useState<Record<string, string>>({});
  const programar = useGuardar(() => api.programarCambio(c), ["facturacion"]);
  const cancelar = useGuardar(() => api.cancelarCambio(), ["facturacion"]);
  const nombre = (r: Regimen) => REGIMENES.find((x) => x.value === r)?.label ?? r;

  async function onProgramar() {
    setErrores({});
    try {
      await programar.mutateAsync(undefined);
      setAbierto(false);
      toast("Cambio programado");
    } catch (e) {
      if (e instanceof ApiError) setErrores({ ...e.fields, general: Object.keys(e.fields).length ? "" : e.message });
    }
  }
  return (
    <section className="seccion" data-testid="cambio-regimen">
      <h2>Cambio de régimen</h2>
      <p>Si el SRI te cambia de régimen o de calificación desde una fecha, prográmalo: rige para lo que se emita desde ese día y lo ya emitido no cambia.</p>
      {actual ? (
        <div className="rp-group">
          <div className="rp-cell" data-testid="cambio-actual">
            <AppIcon icono="tiempo" tint="orange" size={30} />
            <span className="rp-cell__body">
              <span className="rp-cell__title">
                Desde el {new Date(`${actual.desde}T12:00:00`).toLocaleDateString("es-EC", { day: "numeric", month: "long", year: "numeric" })}: {nombre(actual.regimen)}
              </span>
              <span className="rp-cell__subtitle">
                {actual.obligadoContabilidad ? "Obligado a llevar contabilidad" : "No obligado a llevar contabilidad"}
                {actual.contribuyenteEspecial ? ` · contribuyente especial ${actual.contribuyenteEspecial}` : ""}
                {actual.agenteRetencion ? ` · agente de retención ${actual.agenteRetencion}` : ""}
              </span>
            </span>
            <button className="rp-btn rp-btn--gray rp-btn--sm" disabled={cancelar.isPending} onClick={() => void cancelar.mutateAsync(undefined).then(() => toast("Cambio cancelado"))}>
              Cancelar cambio
            </button>
          </div>
        </div>
      ) : !abierto ? (
        <button className="rp-btn rp-btn--gray" onClick={() => setAbierto(true)} data-testid="programar-cambio">
          Programar un cambio
        </button>
      ) : (
        <div className="form">
          <Field label="Desde" error={errores.desde}>
            {(fid) => <input id={fid} type="date" min={manana()} value={c.desde} onChange={(e) => setC({ ...c, desde: e.target.value })} />}
          </Field>
          <Segmented label="Régimen desde esa fecha" value={c.regimen} options={REGIMENES.map((r) => ({ value: r.value, label: r.label }))} onChange={(v) => setC({ ...c, regimen: v })} />
          <div className="rp-group">
            <ToggleRow label="Obligado a llevar contabilidad" checked={c.obligadoContabilidad} onChange={(v) => setC({ ...c, obligadoContabilidad: v })} />
          </div>
          <Field label="Contribuyente especial (opcional)" error={errores.contribuyenteEspecial}>
            {(fid) => <input id={fid} value={c.contribuyenteEspecial ?? ""} maxLength={13} onChange={(e) => setC({ ...c, contribuyenteEspecial: e.target.value || null })} />}
          </Field>
          <Field label="Agente de retención (opcional)" error={errores.agenteRetencion}>
            {(fid) => <input id={fid} value={c.agenteRetencion ?? ""} inputMode="numeric" maxLength={8} onChange={(e) => setC({ ...c, agenteRetencion: e.target.value.replace(/\D/g, "") || null })} />}
          </Field>
          {errores.general && (
            <p className="rp-field__error" role="alert">
              {errores.general}
            </p>
          )}
          <div className="acciones">
            <button className="rp-btn rp-btn--primary" disabled={programar.isPending} onClick={() => void onProgramar()} data-testid="guardar-cambio">
              Programar
            </button>
            <button className="rp-btn rp-btn--plain" onClick={() => setAbierto(false)}>
              Cancelar
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
