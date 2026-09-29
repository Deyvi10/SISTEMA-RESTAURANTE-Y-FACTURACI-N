import { LockKeyhole } from "lucide-react";
import { useState } from "react";
import { useSesion } from "./api/sesion";
import { useAtajo } from "./components/atajos";
import { Avatar } from "./components/Avatar";
import { Indicador } from "./components/Indicador";
import { Emparejar } from "./pages/Emparejar";
import { Mesas } from "./pages/Mesas";
import { type ACobrar, Cobro } from "./pages/Cobro";
import { Personal } from "./pages/Personal";
import { Turno, useCaja } from "./pages/Turno";
import { type MesaParaPedir, type OrdenExistente, Venta } from "./pages/Venta";

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
  const { usuario, salir, puede } = useSesion();
  // Quien toma pedidos (el Cajero, «Caja principal») también pide para mesas desde esta PC.
  const tomaPedidos = puede("TOMAR_PEDIDO");
  const [cobrando, setCobrando] = useState<ACobrar | null>(null);
  const [venta, setVenta] = useState<{ existente?: OrdenExistente; mesa?: MesaParaPedir } | null>(null);
  const [seccion, setSeccion] = useState<"mesas" | "turno" | "cobro" | "venta">("mesas");
  const [aviso, setAviso] = useState<string | null>(null);
  const caja = useCaja();
  const turno = caja.estado?.turno ?? null;
  useAtajo("F12", "Bloquear la caja", () => void salir(), "General");
  useAtajo("F9", "Ver las mesas", () => setSeccion("mesas"), "General");
  const alSalon = () => {
    setCobrando(null);
    setVenta(null);
    setAviso(null);
    setSeccion("mesas");
    caja.recargar();
  };
  const aCobrar = (o: ACobrar) => {
    setVenta(null);
    setCobrando(o);
    setSeccion("cobro");
  };
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
            <button key={id} className="barra__tab" aria-current={seccion === id || (seccion !== "turno" && id === "mesas") ? "page" : undefined} onClick={() => setSeccion(id)} title={`${nombre} (${tecla})`} data-testid={`tab-${id}`}>
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
          <span className="barra__texto">{usuario?.nombre}</span>
        </span>
        <button className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => void salir()} title="Bloquear la caja (F12)">
          <LockKeyhole aria-hidden="true" /> <span className="barra__texto">Bloquear</span>
        </button>
      </header>
      <main className="contenido">
        {seccion === "cobro" && cobrando ? (
          <Cobro
            key={cobrando.ordenId}
            orden={cobrando}
            caja={caja}
            volver={alSalon}
            irATurno={() => setSeccion("turno")}
            usuarioId={usuario?.id}
            agregar={
              tomaPedidos
                ? (o) => {
                    setVenta({ existente: { ordenId: o.id, tipo: o.tipo, mesaId: o.mesaId, etiqueta: o.etiqueta, nombre: o.mesa } });
                    setSeccion("venta");
                  }
                : undefined
            }
          />
        ) : seccion === "venta" && venta ? (
          <Venta existente={venta.existente} mesa={venta.mesa} volver={alSalon} cobrar={(ordenId, nombre) => aCobrar({ ordenId, nombre })} />
        ) : seccion === "turno" ? (
          <Turno caja={caja} />
        ) : (
          <>
            <Mesas
              abrir={(m) => {
                if (m.ordenId) aCobrar({ ordenId: m.ordenId, nombre: m.nombre });
                else if (tomaPedidos) {
                  setVenta({ mesa: { id: m.id, nombre: m.nombre } });
                  setSeccion("venta");
                } else setAviso(`${m.nombre} está libre: no hay nada que cobrar.`);
              }}
              abrirOrden={(o) => aCobrar({ ordenId: o.id, nombre: o.nombre })}
              nuevaVenta={() => {
                setVenta({});
                setSeccion("venta");
              }}
            />
            {aviso && (
              <p className="rp-secondary" role="status">
                {aviso}
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
