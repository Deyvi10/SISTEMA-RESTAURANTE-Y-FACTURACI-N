import { LockKeyhole } from "lucide-react";
import { useState } from "react";
import type { Mesa } from "./api/nodo";
import { useSesion } from "./api/sesion";
import { useAtajo } from "./components/atajos";
import { Avatar } from "./components/Avatar";
import { Indicador } from "./components/Indicador";
import { Emparejar } from "./pages/Emparejar";
import { Mesas } from "./pages/Mesas";
import { Personal } from "./pages/Personal";
import { Turno, useCaja } from "./pages/Turno";

export function App() {
  const { fase } = useSesion();
  if (fase === "cargando") {
    return (
      <main className="centrado">
        <div className="spinner" role="status" aria-label="Conectando con el Nodo Local" />
      </main>
    );
  }
  if (fase === "emparejar") return <Emparejar />;
  if (fase === "personal") return <Personal />;
  return <Caja />;
}

function Caja() {
  const { usuario, salir } = useSesion();
  const [mesa, setMesa] = useState<Mesa | null>(null);
  const [seccion, setSeccion] = useState<"mesas" | "turno">("mesas");
  const caja = useCaja();
  const turno = caja.estado?.turno ?? null;
  useAtajo("F12", "Bloquear la caja", () => void salir(), "General");
  useAtajo("F9", "Ver las mesas", () => setSeccion("mesas"), "General");
  useAtajo("F10", "Ver el turno de caja", () => setSeccion("turno"), "General");

  return (
    <div className="caja">
      <header className="barra">
        <span className="rp-t-headline barra__marca">Caja</span>
        <nav className="barra__nav" aria-label="Secciones">
          {(
            [
              ["mesas", "Mesas", "F9"],
              ["turno", "Turno", "F10"],
            ] as const
          ).map(([id, nombre, tecla]) => (
            <button key={id} className="barra__tab" aria-current={seccion === id ? "page" : undefined} onClick={() => setSeccion(id)} title={`${nombre} (${tecla})`} data-testid={`tab-${id}`}>
              {nombre}
            </button>
          ))}
        </nav>
        {caja.estado && (
          <button className={`rp-status ${turno ? "rp-status--success" : "rp-status--warning"} estado-turno`} onClick={() => setSeccion("turno")} data-testid="estado-turno">
            {turno ? `${caja.estado.caja.nombre} · turno de ${turno.cajeroNombre}` : `${caja.estado.caja.nombre} · sin turno`}
          </button>
        )}
        <Indicador />
        <span className="barra__usuario">
          <Avatar nombre={usuario?.nombre ?? ""} tamano={30} />
          {usuario?.nombre}
        </span>
        <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => void salir()} title="Bloquear la caja (F12)">
          <LockKeyhole aria-hidden="true" /> Bloquear
        </button>
      </header>
      <main className="contenido">
        {seccion === "turno" ? (
          <Turno caja={caja} />
        ) : (
          <>
            <Mesas abrir={setMesa} />
            {mesa && (
              <p className="rp-secondary" role="status">
                {turno ? `${mesa.nombre} elegida. El cobro llega en la siguiente entrega (F4-05).` : `${mesa.nombre} elegida. Abre el turno de caja para poder cobrar.`}
              </p>
            )}
          </>
        )}
      </main>
      <footer className="pie">
        Pulsa <kbd className="tecla">?</kbd> para ver los atajos de teclado
      </footer>
    </div>
  );
}
