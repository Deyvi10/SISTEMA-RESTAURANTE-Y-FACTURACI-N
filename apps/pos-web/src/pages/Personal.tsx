// ¿Quién cobra? El cajero elige su foto y escribe su PIN (mouse, pantalla táctil o teclado).
import { useEffect, useState } from "react";
import { ApiError, nodo, type Persona } from "../api/nodo";
import { useSesion } from "../api/sesion";
import { Avatar } from "../components/Avatar";
import { useAtajo } from "../components/atajos";
import { Indicador } from "../components/Indicador";
import { TecladoPIN } from "../components/TecladoPIN";

const ROL: Record<string, string> = { ADMIN: "Administración", CAJERO: "Caja", MESERO: "Mesero" };

export function Personal() {
  const { entrar, error: errorSesion } = useSesion();
  const [gente, setGente] = useState<Persona[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [elegida, setElegida] = useState<Persona | null>(null);

  useEffect(() => {
    nodo
      .personal()
      .then((g) => setGente([...g].sort((a, b) => orden(a) - orden(b) || a.nombre.localeCompare(b.nombre))))
      .catch((e) => setError(e instanceof ApiError ? e.message : "No se pudo cargar el personal."));
  }, []);

  useAtajo("Escape", "Volver a elegir persona", () => setElegida(null), "Acceso", !!elegida);

  return (
    <main className="acceso">
      <header className="acceso__barra">
        <span className="rp-t-headline">Caja</span>
        <Indicador />
      </header>
      {!elegida ? (
        <section className="acceso__gente">
          <h1 className="rp-large-title">¿Quién cobra?</h1>
          {errorSesion && <p className="aviso-error">{errorSesion}</p>}
          {error && <p className="aviso-error">{error}</p>}
          {gente === null && !error && <p className="rp-secondary">Cargando…</p>}
          <div className="gente">
            {gente?.map((p, i) => (
              <button key={p.id} className="persona" onClick={() => setElegida(p)} data-testid={`persona-${p.nombre}`}>
                <Avatar nombre={p.nombre} url={p.avatarUrl} tamano={88} />
                <b>{p.nombre}</b>
                <small>{ROL[p.rol] ?? p.rol}</small>
                {i < 9 && <kbd className="tecla tecla--esquina">{i + 1}</kbd>}
              </button>
            ))}
          </div>
          <AtajosPersonas gente={gente ?? []} elegir={setElegida} />
        </section>
      ) : (
        <section className="acceso__pin">
          <Avatar nombre={elegida.nombre} url={elegida.avatarUrl} tamano={72} />
          <h1 className="rp-t-title2">Hola, {elegida.nombre.split(" ")[0]}</h1>
          <TecladoPIN
            alCompletar={async (pin) => {
              try {
                await entrar(elegida.id, pin);
                return null;
              } catch (e) {
                return e instanceof ApiError ? e.message : "No se pudo entrar.";
              }
            }}
          />
          <button className="rp-btn rp-btn--plain" onClick={() => setElegida(null)}>
            No soy {elegida.nombre.split(" ")[0]}
          </button>
        </section>
      )}
    </main>
  );
}

function orden(p: Persona) {
  return p.rol === "CAJERO" ? 0 : p.rol === "ADMIN" ? 1 : 2;
}

/** Teclas 1–9 eligen a la persona de esa posición. */
function AtajosPersonas({ gente, elegir }: { gente: Persona[]; elegir: (p: Persona) => void }) {
  return (
    <>
      {gente.slice(0, 9).map((p, i) => (
        <AtajoPersona key={p.id} tecla={String(i + 1)} persona={p} elegir={elegir} />
      ))}
    </>
  );
}

function AtajoPersona({ tecla, persona, elegir }: { tecla: string; persona: Persona; elegir: (p: Persona) => void }) {
  useAtajo(tecla, `Entrar como ${persona.nombre}`, () => elegir(persona), "Acceso");
  return null;
}
