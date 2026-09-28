// Datos del comprador (F4-07, RF-04-05): campo único «Cédula / RUC / Pasaporte» validado en la
// caja, autocompletado en cascada (nodo → nube) y formulario mínimo con consentimiento (LOPDP).
import { CONSUMIDOR_FINAL, validarIdentificacion, validarPasaporte, type Validacion } from "@restpos/sri";
import { UserRound } from "lucide-react";
import { type ChangeEvent, forwardRef, type KeyboardEvent, useEffect, useRef } from "react";
import { type BusquedaCliente, nodo } from "../api/nodo";

export interface EstadoComprador {
  modo: "CF" | "ID";
  pasaporte: boolean;
  identificacion: string;
  buscado: string; // la identificación de la última búsqueda terminada
  origen: "NODO" | "NUBE" | null; // de dónde vino el autocompletado
  razonSocial: string;
  email: string;
  direccion: string;
  telefono: string;
  consentimiento: boolean;
}

export const compradorInicial: EstadoComprador = {
  modo: "CF", pasaporte: false, identificacion: "", buscado: "", origen: null,
  razonSocial: "", email: "", direccion: "", telefono: "", consentimiento: false,
};

/** Lo que el nodo recibe como comprador. */
export interface CompradorEnvio {
  tipoIdentificacion: string;
  identificacion: string;
  razonSocial: string;
  email: string;
  direccion: string;
  telefono: string;
  consentimiento: boolean;
}

export function validar(e: EstadoComprador): Validacion {
  const id = e.identificacion.trim().toUpperCase();
  return e.pasaporte ? validarPasaporte(id) : validarIdentificacion(id);
}

const correoValido = (s: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s.trim());

/** El comprador listo para cobrar, o el motivo por el que falta algo. */
export function compradorParaCobro(e: EstadoComprador): { envio: CompradorEnvio | null; problema: string | null } {
  if (e.modo === "CF") return { envio: null, problema: null };
  const v = validar(e);
  if (!e.identificacion.trim()) return { envio: null, problema: "Escribe la cédula, el RUC o el pasaporte del comprador." };
  if (!v.valida) return { envio: null, problema: v.motivo ?? "La identificación no es válida." };
  if (v.tipo === "07") return { envio: null, problema: null };
  if (e.buscado !== e.identificacion.trim().toUpperCase()) return { envio: null, problema: "Buscando al comprador…" };
  if (e.razonSocial.trim().length < 2) return { envio: null, problema: "Escribe el nombre o la razón social del comprador." };
  if (!correoValido(e.email)) return { envio: null, problema: "Escribe un correo válido: ahí le llega su comprobante." };
  return {
    envio: {
      tipoIdentificacion: e.pasaporte ? "06" : "",
      identificacion: e.identificacion.trim().toUpperCase(),
      razonSocial: e.razonSocial.trim(),
      email: e.email.trim(),
      direccion: e.direccion.trim(),
      telefono: e.telefono.trim(),
      consentimiento: e.consentimiento,
    },
    problema: null,
  };
}

/** ¿La venta queda a consumidor final (sin datos, o con el 9999999999999)? */
export const esConsumidorFinal = (e: EstadoComprador) => e.modo === "CF" || e.identificacion.trim() === CONSUMIDOR_FINAL;

interface Props {
  estado: EstadoComprador;
  cambiar: (f: (e: EstadoComprador) => EstadoComprador) => void;
  cfPermitido: boolean;
  alConfirmar: () => void; // Intro en el formulario: cobrar
}

export const Comprador = forwardRef<HTMLInputElement, Props>(function Comprador({ estado: e, cambiar, cfPermitido, alConfirmar }, ref) {
  const v = validar(e);
  const id = e.identificacion.trim().toUpperCase();
  const buscando = useRef(0);

  // Búsqueda en cascada en cuanto la identificación es válida (el nodo responde ≤ 300 ms).
  useEffect(() => {
    if (e.modo !== "ID" || !v.valida || v.tipo === "07" || e.buscado === id) return;
    const n = ++buscando.current;
    const t = setTimeout(() => {
      nodo
        .buscarCliente(id, e.pasaporte ? "06" : "")
        .then((b: BusquedaCliente) => {
          if (n !== buscando.current) return;
          cambiar((x) =>
            b.cliente
              ? { ...x, buscado: id, origen: b.origen ?? "NODO", razonSocial: b.cliente.razonSocial, email: b.cliente.email, direccion: b.cliente.direccion, telefono: b.cliente.telefono, consentimiento: true }
              : { ...x, buscado: id, origen: null, consentimiento: false },
          );
        })
        .catch(() => n === buscando.current && cambiar((x) => ({ ...x, buscado: id, origen: null })));
    }, 120);
    return () => clearTimeout(t);
  }, [e.modo, e.pasaporte, e.buscado, id, v.valida, v.tipo, cambiar]);

  const campo = (k: "razonSocial" | "email" | "direccion" | "telefono") => ({
    value: e[k],
    onChange: (ev: ChangeEvent<HTMLInputElement>) => {
      const valor = ev.target.value;
      cambiar((x) => ({ ...x, [k]: valor }));
    },
    onKeyDown: (ev: KeyboardEvent<HTMLInputElement>) => ev.key === "Enter" && (ev.preventDefault(), alConfirmar()),
  });
  const mostrarFormulario = e.modo === "ID" && v.valida && v.tipo !== "07" && e.buscado === id;

  return (
    <div className="comprador" data-testid="comprador">
      <div className="comprador__modo">
        <button
          type="button"
          className="cobro__cf"
          aria-pressed={e.modo === "CF"}
          disabled={!cfPermitido}
          onClick={() => cambiar(() => compradorInicial)}
          data-testid="consumidor-final"
        >
          <UserRound aria-hidden="true" />
          Consumidor final
        </button>
        <button type="button" className="rp-btn rp-btn--plain rp-btn--sm" aria-pressed={e.modo === "ID"} onClick={() => cambiar((x) => ({ ...x, modo: "ID" }))} data-testid="con-datos">
          Con datos <kbd className="tecla">C</kbd>
        </button>
      </div>
      {e.modo === "ID" && (
        <div className="comprador__form">
          <label className="rp-field comprador__id" data-invalid={id.length >= 10 && !v.valida ? "true" : undefined}>
            <span>{e.pasaporte ? "Pasaporte" : "Cédula / RUC / Pasaporte"}</span>
            <input
              ref={ref}
              value={e.identificacion}
              inputMode={e.pasaporte ? "text" : "numeric"}
              autoComplete="off"
              maxLength={20}
              onChange={(ev) => {
                const valor = ev.target.value.replace(/\s/g, "");
                cambiar((x) => ({ ...x, identificacion: valor }));
              }}
              onKeyDown={(ev) => {
                if (ev.key === "Enter") {
                  ev.preventDefault();
                  alConfirmar();
                } else if (ev.key === "Escape" && cfPermitido) {
                  ev.preventDefault();
                  ev.stopPropagation();
                  cambiar(() => compradorInicial);
                }
              }}
              data-testid="identificacion"
            />
            {id.length > 0 && !v.valida && (id.length >= 10 || e.pasaporte) && <small className="rp-field__error">{v.motivo}</small>}
            {v.valida && v.advertencia && <small className="rp-field__hint">{v.advertencia}</small>}
            {v.valida && v.tipo !== "07" && e.buscado !== id && <small className="rp-secondary">Buscando…</small>}
            {mostrarFormulario && e.origen && (
              <small className="comprador__origen" data-testid="origen">
                ✓ Cliente registrado{e.origen === "NUBE" ? " (desde la nube)" : ""}
              </small>
            )}
          </label>
          <label className="comprador__pasaporte">
            <input type="checkbox" checked={e.pasaporte} onChange={(ev) => cambiar((x) => ({ ...x, pasaporte: ev.target.checked, buscado: "" }))} />
            Es pasaporte
          </label>
          {mostrarFormulario && (
            <>
              <label className="rp-field">
                <span>Nombre o razón social</span>
                <input {...campo("razonSocial")} maxLength={300} data-testid="razon-social" />
              </label>
              <label className="rp-field">
                <span>Correo</span>
                <input {...campo("email")} type="email" inputMode="email" maxLength={200} data-testid="email" />
              </label>
              <label className="rp-field">
                <span>Dirección (opcional)</span>
                <input {...campo("direccion")} maxLength={300} />
              </label>
              <label className="rp-field">
                <span>Teléfono (opcional)</span>
                <input {...campo("telefono")} inputMode="tel" maxLength={20} />
              </label>
              <label className="comprador__consentimiento">
                <input type="checkbox" checked={e.consentimiento} onChange={(ev) => cambiar((x) => ({ ...x, consentimiento: ev.target.checked }))} data-testid="consentimiento" />
                <span>
                  El cliente autoriza guardar sus datos para próximas compras.
                  <small>Se usan solo para facturarle (LOPDP). Sin esta autorización, sus datos van únicamente en esta venta.</small>
                </span>
              </label>
            </>
          )}
        </div>
      )}
    </div>
  );
});
