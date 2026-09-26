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
  useAtajo("F12", "Bloquear la caja", () => void salir(), "General");

  return (
    <div className="caja">
      <header className="barra">
        <span className="rp-t-headline barra__marca">Caja</span>
        <nav className="barra__nav" aria-label="Secciones">
          <span className="barra__tab" aria-current="page">
            Mesas
          </span>
        </nav>
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
        <Mesas abrir={setMesa} />
        {mesa && (
          <p className="rp-secondary" role="status">
            {mesa.nombre} elegida. El cobro llega con los turnos de caja.
          </p>
        )}
      </main>
      <footer className="pie">
        Pulsa <kbd className="tecla">?</kbd> para ver los atajos de teclado
      </footer>
    </div>
  );
}
