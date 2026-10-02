// Asistente de facturación electrónica (F5-06, RF-05-01): seis pasos amables, uno por pantalla,
// sin hablar de XML ni de web services. Icono grande arriba, barra de progreso fina, «Continuar»
// abajo y ✓ al terminar. Se puede dejar la firma para después (aún no la tienes) y volver.
import { useEffect, useState } from "react";
import { ApiError } from "../api/client";
import { api, useGuardar } from "../api/hooks";
import type { ConfigFiscal } from "../api/types";
import { useFeedback } from "../components/feedback";
import { AppIcon, Field, Segmented, ToggleRow } from "../components/ui";
import { Icon } from "../lib/icons";
import { cuerpo, FirmaSeccion, formDe, PuntoFila, REGIMENES, vigencia } from "./Facturacion";

const PASOS = [
  { titulo: "Tu firma electrónica", icono: "seguro", tint: "indigo" },
  { titulo: "Revisamos tu firma", icono: "ok", tint: "green" },
  { titulo: "Datos de tu negocio", icono: "receipt", tint: "pink" },
  { titulo: "Tu régimen tributario", icono: "porcentaje", tint: "orange" },
  { titulo: "Cajas y numeración", icono: "caja", tint: "teal" },
  { titulo: "Tu factura de prueba", icono: "cohete", tint: "purple" },
] as const;

/** El primer paso que falta, para retomar donde se quedó. */
export function pasoInicial(c: ConfigFiscal): number {
  if (!c.certificado) return 0;
  if (!c.guardada) return 2;
  if (c.cajas.some((x) => !x.puntoId)) return 4;
  return 5;
}

export function Asistente({ cfg, salir }: { cfg: ConfigFiscal; salir: () => void }) {
  const { toast } = useFeedback();
  const [paso, setPaso] = useState(() => pasoInicial(cfg));
  const [f, setF] = useState(() => formDe(cfg));
  const [errores, setErrores] = useState<Record<string, string>>({});
  useEffect(() => setF((x) => ({ ...formDe(cfg), ...x, ambiente: formDe(cfg).ambiente })), [cfg]);
  const guardar = useGuardar((activa: boolean) => api.guardarFacturacion(cuerpo(f, activa)), ["facturacion"]);
  const set = <K extends keyof typeof f>(k: K, v: (typeof f)[K]) => setF((x) => ({ ...x, [k]: v }));
  const p = PASOS[paso]!;
  const cert = cfg.certificado;

  async function guardarYSeguir(activa: boolean, siguiente: number) {
    setErrores({});
    try {
      await guardar.mutateAsync(activa);
      setPaso(siguiente);
    } catch (e) {
      if (e instanceof ApiError) {
        setErrores({ ...e.fields, general: Object.keys(e.fields).length ? "" : e.message });
        // Un dato del negocio inválido se corrige en su paso.
        if (["razonSocial", "direccionMatriz", "nombreComercial"].some((k) => k in e.fields)) setPaso(2);
      } else toast("No se pudo guardar.", "error");
    }
  }

  const sinPunto = cfg.cajas.filter((c) => !c.puntoId);
  const listo = cfg.facturacionActiva && cfg.pruebaAprobada;

  return (
    <div className="asistente" data-testid="asistente">
      <div className="asistente__progreso" role="progressbar" aria-valuemin={1} aria-valuemax={6} aria-valuenow={paso + 1} aria-label={`Paso ${paso + 1} de 6`}>
        <span style={{ width: `${((paso + 1) / PASOS.length) * 100}%` }} />
      </div>
      <div className="asistente__cabeza">
        <AppIcon icono={p.icono} tint={p.tint} size={64} />
        <small>Paso {paso + 1} de 6</small>
        <h1>{p.titulo}</h1>
      </div>

      <div className="asistente__cuerpo">
        {paso === 0 && (
          <>
            <p className="asistente__texto">
              Es el archivo <strong>.p12</strong> que te entregó tu entidad certificadora (Security Data, BCE, ANF…). Con él firmamos cada factura antes de
              enviarla al SRI. Se guarda cifrado y nadie puede descargarlo.
            </p>
            <FirmaSeccion cert={cert} ruc={cfg.ruc} />
          </>
        )}
        {paso === 1 && (
          <ul className="asistente__checks" data-testid="revision-firma">
            {cert ? (
              <>
                <li data-ok="true">
                  <Icon name="ok" size={18} /> Pertenece a <strong>{cert.titular}</strong>
                </li>
                <li data-ok={cert.diasRestantes >= 0 ? "true" : "false"}>
                  <Icon name={cert.diasRestantes >= 0 ? "ok" : "error"} size={18} /> {vigencia(cert).texto}
                </li>
                <li data-ok={cert.ruc ? "true" : "aviso"}>
                  <Icon name={cert.ruc ? "ok" : "info"} size={18} />{" "}
                  {cert.ruc ? `Es del RUC del restaurante (${cert.ruc})` : `Confirma que la firma es del titular del RUC ${cfg.ruc}`}
                </li>
                <li data-ok="true">
                  <Icon name="ok" size={18} /> Sirve para firmar (llave de 2048 bits o más)
                </li>
              </>
            ) : (
              <li data-ok="aviso">
                <Icon name="info" size={18} /> Todavía no subiste tu firma. Puedes seguir y subirla después: la caja factura igual y las facturas esperan en la nube.
              </li>
            )}
          </ul>
        )}
        {paso === 2 && (
          <div className="form">
            <p className="asistente__texto">Tal como constan en tu RUC; van impresos en cada factura.</p>
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
          </div>
        )}
        {paso === 3 && (
          <div className="form">
            <Segmented label="Régimen" value={f.regimen} options={REGIMENES.map((r) => ({ value: r.value, label: r.label }))} onChange={(v) => set("regimen", v)} />
            <p className="rp-secondary">{REGIMENES.find((r) => r.value === f.regimen)?.detalle}</p>
            <div className="rp-group">
              <ToggleRow label="Obligado a llevar contabilidad" checked={f.obligadoContabilidad} onChange={(v) => set("obligadoContabilidad", v)} />
            </div>
            <Field label="Contribuyente especial (N.º de resolución, opcional)" error={errores.contribuyenteEspecial}>
              {(fid) => <input id={fid} value={f.contribuyenteEspecial} maxLength={13} onChange={(e) => set("contribuyenteEspecial", e.target.value)} />}
            </Field>
            <Field label="Agente de retención (N.º de resolución, opcional)" error={errores.agenteRetencion}>
              {(fid) => <input id={fid} value={f.agenteRetencion} inputMode="numeric" maxLength={8} onChange={(e) => set("agenteRetencion", e.target.value.replace(/\D/g, ""))} />}
            </Field>
            <p className="rp-secondary">Revísalo en tu RUC: si no coincide, el SRI rechaza las facturas.</p>
          </div>
        )}
        {paso === 4 && (
          <>
            <p className="asistente__texto">Cada caja factura con su propia serie, por ejemplo 001-001. El primer número es el establecimiento y el segundo el punto de emisión que tienes registrado en el SRI.</p>
            <div className="rp-group">
              {cfg.cajas.map((c) => (
                <PuntoFila key={c.cajaId} p={c} />
              ))}
            </div>
          </>
        )}
        {paso === 5 && (
          <div className="asistente__prueba" data-testid="paso-prueba">
            {listo ? (
              <div className="asistente__hecho" data-testid="asistente-listo">
                <span className="asistente__check" aria-hidden="true">
                  ✓
                </span>
                <p>¡Listo! El SRI autorizó tu factura de prueba. Cuando quieras, pasa a producción desde Facturación SRI.</p>
              </div>
            ) : (
              <ol className="asistente__instrucciones">
                <li data-ok={cfg.facturacionActiva ? "true" : "false"}>
                  {cfg.facturacionActiva ? "Facturación activada en el ambiente de pruebas." : "Activa la facturación en el ambiente de pruebas (no tiene validez tributaria)."}
                </li>
                <li>En la caja, cobra una venta pequeña: sale la factura con la leyenda «AMBIENTE DE PRUEBAS».</li>
                <li data-ok={cfg.pruebaAprobada ? "true" : "false"}>
                  {cert ? "Cuando el SRI la autorice, aquí aparece el ✓ (tarda unos segundos)." : "Sube tu firma para que la factura llegue al SRI."}
                </li>
              </ol>
            )}
            {errores.general && (
              <p className="rp-field__error" role="alert">
                {errores.general}
              </p>
            )}
          </div>
        )}
      </div>

      <div className="asistente__pie">
        {paso > 0 && (
          <button className="rp-btn rp-btn--plain" onClick={() => setPaso(paso - 1)}>
            Atrás
          </button>
        )}
        <button className="rp-btn rp-btn--plain asistente__salir" onClick={salir} data-testid="ver-todo">
          Ver todo en una página
        </button>
        {paso === 0 && !cert && (
          <button className="rp-btn rp-btn--gray" onClick={() => setPaso(2)} data-testid="firma-despues">
            Lo haré después
          </button>
        )}
        {paso < 2 && (
          <button className="rp-btn rp-btn--primary" disabled={paso === 0 && !cert} onClick={() => setPaso(paso + 1)} data-testid="continuar">
            Continuar
          </button>
        )}
        {paso === 2 && (
          <button className="rp-btn rp-btn--primary" disabled={!f.razonSocial.trim() || !f.direccionMatriz.trim()} onClick={() => setPaso(3)} data-testid="continuar">
            Continuar
          </button>
        )}
        {paso === 3 && (
          <button className="rp-btn rp-btn--primary" disabled={guardar.isPending} onClick={() => void guardarYSeguir(cfg.facturacionActiva, 4)} data-testid="continuar">
            {guardar.isPending ? "Guardando…" : "Guardar y continuar"}
          </button>
        )}
        {paso === 4 && (
          <button className="rp-btn rp-btn--primary" disabled={sinPunto.length > 0} onClick={() => setPaso(5)} data-testid="continuar">
            {sinPunto.length ? `Falta la serie de ${sinPunto.length === 1 ? sinPunto[0]!.caja : `${sinPunto.length} cajas`}` : "Continuar"}
          </button>
        )}
        {paso === 5 &&
          (listo ? (
            <button className="rp-btn rp-btn--primary" onClick={salir} data-testid="terminar">
              Terminar
            </button>
          ) : !cfg.facturacionActiva ? (
            <button className="rp-btn rp-btn--primary" disabled={guardar.isPending} onClick={() => void guardarYSeguir(true, 5)} data-testid="activar-pruebas">
              Activar facturación en pruebas
            </button>
          ) : null)}
      </div>
    </div>
  );
}
