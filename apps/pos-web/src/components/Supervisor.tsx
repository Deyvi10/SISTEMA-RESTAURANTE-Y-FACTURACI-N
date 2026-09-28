// Autorización con el PIN de un supervisor (RF-01-07): se elige a alguien que tenga el permiso
// y escribe su PIN en esta misma caja. El nodo entrega un token de un solo uso, ligado a la
// acción y a su referencia, que vence en un minuto.
import { useEffect, useState } from "react";
import { ApiError, nodo, type Persona } from "../api/nodo";
import { Avatar } from "./Avatar";
import { useAtajo } from "./atajos";
import { TecladoPIN } from "./TecladoPIN";

/** Quién puede autorizar la acción (el administrador siempre, el resto si tiene el permiso). */
export const puedenAutorizar = (gente: Persona[], accion: string, excepto?: string) =>
  gente.filter((p) => p.id !== excepto && (p.rol === "ADMIN" || p.permisos.includes(accion)));

export function Supervisor({
  accion,
  referencia,
  excepto,
  alAutorizar,
  volver,
}: {
  accion: string;
  referencia: string;
  excepto?: string; // el usuario de la sesión no se autoriza a sí mismo
  alAutorizar: (token: string, nombre: string) => Promise<string | null>; // devuelve un error para mostrar
  volver: () => void;
}) {
  const [gente, setGente] = useState<Persona[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [elegido, setElegido] = useState<Persona | null>(null);

  useEffect(() => {
    nodo
      .personal()
      .then((g) => setGente(puedenAutorizar(g, accion, excepto)))
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo cargar el personal."));
  }, [accion, excepto]);

  useAtajo("Escape", "Volver", () => (elegido ? setElegido(null) : volver()), "Supervisor");

  if (elegido) {
    return (
      <div className="supervisor supervisor--pin">
        <Avatar nombre={elegido.nombre} url={elegido.avatarUrl} tamano={56} />
        <p className="rp-t-headline">PIN de {elegido.nombre}</p>
        <TecladoPIN
          alCompletar={async (pin) => {
            try {
              const au = await nodo.autorizar({ usuarioId: elegido.id, pin, accion, referencia });
              return await alAutorizar(au.token, au.autorizadoPor);
            } catch (e) {
              return e instanceof ApiError ? e.message : "No se pudo autorizar.";
            }
          }}
        />
        <button type="button" className="rp-btn rp-btn--plain rp-btn--sm" onClick={() => setElegido(null)}>
          Elegir a otra persona
        </button>
      </div>
    );
  }
  return (
    <div className="supervisor">
      <p className="hoja-caja__ayuda">Necesita la autorización de un supervisor. ¿Quién autoriza?</p>
      {error && <p className="aviso-error hoja-caja__ayuda">{error}</p>}
      {gente && gente.length === 0 && <p className="aviso-error hoja-caja__ayuda">Nadie con PIN tiene este permiso. El dueño lo concede en el panel web, en Personal.</p>}
      <div className="supervisor__gente">
        {gente?.map((p) => (
          <button key={p.id} type="button" className="persona" onClick={() => setElegido(p)} data-testid={`supervisor-${p.nombre}`}>
            <Avatar nombre={p.nombre} url={p.avatarUrl} tamano={48} />
            <b>{p.nombre}</b>
          </button>
        ))}
      </div>
    </div>
  );
}
