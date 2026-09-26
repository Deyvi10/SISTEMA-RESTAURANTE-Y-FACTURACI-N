// Semáforo de conexión (F4-01): 🟢 nodo y nube · 🟡 nodo sin internet (se sigue cobrando) · 🔴 sin nodo.
import { ServerOff, Wifi, WifiOff } from "lucide-react";
import { useEffect, useState } from "react";
import { nodo } from "../api/nodo";
import { useSesion } from "../api/sesion";

export type Color = "verde" | "amarillo" | "rojo";

export function colorDe(ws: boolean, nube: string | null): Color {
  if (!ws) return "rojo";
  return nube === "EN_LINEA" ? "verde" : "amarillo";
}

const TEXTO: Record<Color, string> = {
  verde: "En línea",
  amarillo: "Sin internet · se sigue cobrando",
  rojo: "Sin conexión con el nodo",
};

const ESTADO: Record<Color, string> = { verde: "online", amarillo: "local", rojo: "sinNodo" };

export function Indicador() {
  const { tiempoReal } = useSesion();
  const [ws, setWs] = useState(tiempoReal.conectado);
  const [nube, setNube] = useState<string | null>(null);

  useEffect(() => tiempoReal.alCambiarConexion(setWs), [tiempoReal]);
  useEffect(() => {
    let vivo = true;
    const leer = () =>
      nodo
        .conectividad()
        .then((c) => vivo && setNube(c.nube))
        .catch(() => vivo && setNube(null));
    void leer();
    const t = setInterval(leer, 10_000);
    return () => {
      vivo = false;
      clearInterval(t);
    };
  }, [ws]);

  const color = colorDe(ws, nube);
  const Icono = { verde: Wifi, amarillo: WifiOff, rojo: ServerOff }[color];
  return (
    <span className="rp-conn" data-estado={ESTADO[color]} data-color={color} role="status" aria-live="polite" data-testid="indicador">
      <Icono aria-hidden="true" />
      {TEXTO[color]}
    </span>
  );
}
