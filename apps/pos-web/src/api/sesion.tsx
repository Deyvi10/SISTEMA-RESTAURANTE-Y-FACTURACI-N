// Sesión de la caja: dispositivo (PC del nodo o PC emparejada) → persona con PIN.
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { cargarIdentidad, crearIdentidad, firmarDesafio, type Identidad, olvidarIdentidad } from "./identidad";
import { ApiError, esPCDelNodo, nodo, setAlPerderSesion, setTokenDispositivo, setTokenUsuario, tokenDispositivo, tokenUsuario, type Usuario } from "./nodo";
import { TiempoReal, urlTiempoReal } from "./tiempoReal";

export type Fase = "cargando" | "emparejar" | "personal" | "adentro";

/** Re-PIN tras 5 minutos sin tocar la caja (el teléfono usa 2: la caja suele estar a la vista del cajero). */
export const INACTIVIDAD_MS = 5 * 60_000;

interface Estado {
  fase: Fase;
  usuario: Usuario | null;
  error: string | null;
  tiempoReal: TiempoReal;
  emparejar: (codigo: string, nombre: string) => Promise<void>;
  entrar: (usuarioId: string, pin: string) => Promise<void>;
  salir: (motivo?: string) => Promise<void>;
  puede: (permiso: string) => boolean;
}

const Ctx = createContext<Estado | null>(null);

export function useSesion(): Estado {
  const s = useContext(Ctx);
  if (!s) throw new Error("useSesion fuera de <SesionProvider>");
  return s;
}

async function abrirSesionDispositivo(id: Identidad) {
  const { nonce } = await nodo.desafio(id.id);
  const { token } = await nodo.sesionDispositivo({ dispositivoId: id.id, nonce, firma: await firmarDesafio(id, nonce) });
  setTokenDispositivo(token);
}

export function SesionProvider({ children, pcDelNodo = esPCDelNodo() }: { children: ReactNode; pcDelNodo?: boolean }) {
  const [fase, setFase] = useState<Fase>("cargando");
  const [usuario, setUsuario] = useState<Usuario | null>(null);
  const [error, setError] = useState<string | null>(null);
  const tiempoReal = useMemo(() => new TiempoReal(() => urlTiempoReal({ dispositivo: tokenDispositivo(), usuario: tokenUsuario() })), []);

  const conectar = useCallback(async () => {
    if (pcDelNodo) {
      setFase("personal");
      tiempoReal.iniciar();
      return;
    }
    const id = await cargarIdentidad();
    if (!id) {
      setFase("emparejar");
      return;
    }
    try {
      await abrirSesionDispositivo(id);
      setFase("personal");
      tiempoReal.iniciar();
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        // Revocada en el panel o desemparejada en el nodo: hay que emparejar de nuevo.
        await olvidarIdentidad();
        setError(e.message);
        setFase("emparejar");
      } else {
        setError(e instanceof ApiError ? e.message : "No se pudo conectar con el Nodo Local.");
        setFase("personal");
      }
    }
  }, [pcDelNodo, tiempoReal]);

  useEffect(() => {
    void conectar();
    return () => tiempoReal.detener();
  }, [conectar, tiempoReal]);

  const salir = useCallback(
    async (motivo?: string) => {
      try {
        if (tokenUsuario()) await nodo.salir();
      } catch {
        // sin nodo: igual se cierra en la caja
      }
      setTokenUsuario(null);
      setUsuario(null);
      setError(motivo ?? null);
      setFase("personal");
      tiempoReal.reiniciar();
    },
    [tiempoReal],
  );

  useEffect(() => setAlPerderSesion(() => void salir("Tu sesión terminó. Escribe tu PIN de nuevo.")), [salir]);

  // Bloqueo por inactividad: vuelve a la cuadrícula de personal.
  const temporizador = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => {
    if (fase !== "adentro") return;
    const reiniciar = () => {
      clearTimeout(temporizador.current);
      temporizador.current = setTimeout(() => void salir("La caja se bloqueó por inactividad."), INACTIVIDAD_MS);
    };
    reiniciar();
    const eventos = ["pointerdown", "keydown"] as const;
    eventos.forEach((e) => window.addEventListener(e, reiniciar, { passive: true }));
    return () => {
      clearTimeout(temporizador.current);
      eventos.forEach((e) => window.removeEventListener(e, reiniciar));
    };
  }, [fase, salir]);

  const valor = useMemo<Estado>(
    () => ({
      fase,
      usuario,
      error,
      tiempoReal,
      emparejar: async (codigo, nombre) => {
        const id = (await cargarIdentidad()) ?? (await crearIdentidad());
        await nodo.emparejar({ codigo, dispositivoId: id.id, llavePublica: id.publicaB64, nombre, plataforma: navigator.userAgent.slice(0, 40) });
        setError(null);
        await conectar();
      },
      entrar: async (usuarioId, pin) => {
        const s = await nodo.entrar(usuarioId, pin);
        setTokenUsuario(s.token);
        setUsuario(s.usuario);
        setError(null);
        setFase("adentro");
        tiempoReal.reiniciar();
      },
      salir,
      puede: (p) => usuario?.permisos.includes(p) ?? false,
    }),
    [fase, usuario, error, tiempoReal, conectar, salir],
  );

  return <Ctx.Provider value={valor}>{children}</Ctx.Provider>;
}
