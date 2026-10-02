// Reportes básicos (F5-16): ventas por jornada (fecha de negocio), por método de pago, notas de
// crédito, Cierres Z con su PDF y la exportación a Excel para el contador.
import { useState } from "react";
import { ApiError, descargar } from "../api/client";
import { useCierresZ, useVentas } from "../api/hooks";
import { AlertasFiscales } from "../components/AlertasFiscales";
import { useFeedback } from "../components/feedback";
import { Empty, Spinner } from "../components/ui";
import { Icon } from "../lib/icons";

const dia = (d: Date) => d.toLocaleDateString("en-CA"); // AAAA-MM-DD en la hora local

/** Periodos rápidos: hoy, ayer, últimos 7 días y este mes. */
export function periodo(clave: string, hoy = new Date()): { desde: string; hasta: string } {
  const d = new Date(hoy);
  switch (clave) {
    case "ayer":
      d.setDate(d.getDate() - 1);
      return { desde: dia(d), hasta: dia(d) };
    case "7":
      d.setDate(d.getDate() - 6);
      return { desde: dia(d), hasta: dia(hoy) };
    case "mes":
      return { desde: dia(new Date(hoy.getFullYear(), hoy.getMonth(), 1)), hasta: dia(hoy) };
    default:
      return { desde: dia(hoy), hasta: dia(hoy) };
  }
}

const PERIODOS = [
  { clave: "hoy", nombre: "Hoy" },
  { clave: "ayer", nombre: "Ayer" },
  { clave: "7", nombre: "7 días" },
  { clave: "mes", nombre: "Este mes" },
];

/** «1234.5» → «$1,234.50», desde el texto exacto (sin pasar por coma flotante). */
export function dinero(s: string): string {
  const neg = s.startsWith("-");
  const [ent = "0", dec = ""] = (neg ? s.slice(1) : s).split(".");
  const miles = ent.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  return `${neg ? "−" : ""}$${miles}.${(dec + "00").slice(0, 2)}`;
}
const fechaCorta = (s: string) => new Date(`${s}T12:00:00`).toLocaleDateString("es-EC", { weekday: "short", day: "numeric", month: "short" });

export function Reportes() {
  const { toast } = useFeedback();
  const [sel, setSel] = useState("hoy");
  const [rango, setRango] = useState(() => periodo("hoy"));
  const ventas = useVentas(rango.desde, rango.hasta);
  const cierres = useCierresZ(rango.desde, rango.hasta);
  const v = ventas.data;

  async function bajar(path: string) {
    try {
      await descargar(path);
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "No se pudo descargar.", "error");
    }
  }

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="rp-t-large-title">Reportes</h1>
          <p>Ventas por jornada: lo que se vendió entre la apertura y el cierre del día, aunque pase la medianoche.</p>
        </div>
        <button className="rp-btn rp-btn--gray" onClick={() => void bajar(`/v1/reportes/ventas.xlsx?desde=${rango.desde}&hasta=${rango.hasta}`)} data-testid="excel">
          <Icon name="descargar" size={16} /> Excel
        </button>
      </header>

      <AlertasFiscales />

      <section className="seccion reportes-filtro">
        <div className="rp-chips" role="group" aria-label="Periodo">
          {PERIODOS.map((p) => (
            <button key={p.clave} type="button" className="rp-chip" aria-pressed={sel === p.clave} onClick={() => (setSel(p.clave), setRango(periodo(p.clave)))}>
              {p.nombre}
            </button>
          ))}
        </div>
        <div className="boveda-fechas">
          <label>
            Desde <input type="date" value={rango.desde} max={rango.hasta} onChange={(e) => (setSel(""), setRango({ ...rango, desde: e.target.value }))} />
          </label>
          <label>
            Hasta <input type="date" value={rango.hasta} min={rango.desde} onChange={(e) => (setSel(""), setRango({ ...rango, hasta: e.target.value }))} />
          </label>
        </div>
      </section>

      {ventas.isLoading || !v ? (
        <Spinner />
      ) : (
        <>
          <section className="seccion kpis" data-testid="kpis">
            <div className="kpi kpi--principal">
              <span>Ventas</span>
              <strong>{dinero(v.total.total)}</strong>
              <small>{v.total.documentos} {v.total.documentos === 1 ? "venta" : "ventas"}</small>
            </div>
            <div className="kpi">
              <span>Ticket promedio</span>
              <strong>{dinero(v.ticketPromedio)}</strong>
            </div>
            <div className="kpi">
              <span>IVA</span>
              <strong>{dinero(v.total.iva)}</strong>
            </div>
            <div className="kpi">
              <span>Servicio</span>
              <strong>{dinero(v.total.propina)}</strong>
            </div>
            <div className="kpi">
              <span>Notas de crédito</span>
              <strong>{v.notasCredito.cantidad ? `−${dinero(v.notasCredito.valor)}` : dinero("0")}</strong>
              <small>Neto {dinero(v.neto)}</small>
            </div>
          </section>

          <section className="seccion">
            <h2>Por forma de pago</h2>
            {v.porMetodo.length === 0 ? (
              <p className="rp-secondary">Sin ventas en el periodo.</p>
            ) : (
              <div className="rp-group" data-testid="por-metodo">
                {v.porMetodo.map((m) => (
                  <div key={m.metodo} className="rp-cell">
                    <span className="rp-cell__body">
                      <span className="rp-cell__title">{m.metodo}</span>
                      <span className="rp-cell__subtitle">{m.pagos} {m.pagos === 1 ? "pago" : "pagos"}</span>
                    </span>
                    <span className="comprobante-total">{dinero(m.monto)}</span>
                  </div>
                ))}
              </div>
            )}
          </section>

          {v.dias.length > 1 && (
            <section className="seccion">
              <h2>Por jornada</h2>
              <table className="tabla-reporte" data-testid="por-dia">
                <thead>
                  <tr>
                    <th>Jornada</th>
                    <th>Ventas</th>
                    <th>IVA</th>
                    <th>Servicio</th>
                    <th>Total</th>
                  </tr>
                </thead>
                <tbody>
                  {v.dias.map((d) => (
                    <tr key={d.fecha}>
                      <td>{fechaCorta(d.fecha)}</td>
                      <td>{d.documentos}</td>
                      <td>{dinero(d.iva)}</td>
                      <td>{dinero(d.propina)}</td>
                      <td>{dinero(d.total)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          )}
        </>
      )}

      <section className="seccion">
        <h2>Cierres Z</h2>
        {cierres.isLoading ? (
          <Spinner />
        ) : !cierres.data?.length ? (
          <Empty icono="caja" tint="green" titulo="Sin cierres" texto="Aquí aparece cada cierre de turno de las cajas del periodo." />
        ) : (
          <div className="rp-group" data-testid="cierres">
            {cierres.data.map((c) => (
              <div key={c.id} className="rp-cell">
                <span className="rp-cell__body">
                  <span className="rp-cell__title">
                    Cierre Z {String(c.numero).padStart(4, "0")} · {c.caja}
                  </span>
                  <span className="rp-cell__subtitle">
                    {c.cajero} · jornada del {fechaCorta(c.fechaNegocio)}
                    {!c.hashValido && " · ⚠️ integridad"}
                  </span>
                </span>
                <span className="imp-chip" data-tono={c.resultado === "CUADRADO" ? "success" : c.resultado === "SOBRANTE" ? "warning" : "danger"}>
                  {c.resultado === "CUADRADO" ? "Cuadrado" : c.resultado === "SOBRANTE" ? "Sobrante" : "Faltante"}
                </span>
                <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => void bajar(`/v1/reportes/cierres/${c.id}/pdf`)}>
                  PDF
                </button>
              </div>
            ))}
          </div>
        )}
      </section>
    </>
  );
}
