import { useQueryClient } from "@tanstack/react-query";
import { createContext, type ReactNode, useCallback, useContext, useEffect, useState } from "react";
import { get, post, refreshSession, setAccessToken, setSessionLostHandler } from "./client";
import type { Me, Sesion } from "./types";

interface SessionCtx {
  estado: "cargando" | "anonimo" | "activo";
  me: Me | null;
  entrar: (s: Sesion) => Promise<void>;
  salir: () => Promise<void>;
  puede: (permiso: string) => boolean;
}

const Ctx = createContext<SessionCtx | null>(null);

export function useSession(): SessionCtx {
  const c = useContext(Ctx);
  if (!c) throw new Error("useSession fuera de SessionProvider");
  return c;
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const [estado, setEstado] = useState<SessionCtx["estado"]>("cargando");
  const [me, setMe] = useState<Me | null>(null);

  const cargarMe = useCallback(async () => {
    const m = await get<Me>("/v1/me");
    setMe(m);
    setEstado("activo");
  }, []);

  const entrar = useCallback(async (s: Sesion) => {
    setAccessToken(s.accessToken);
    await cargarMe();
  }, [cargarMe]);

  const salir = useCallback(async () => {
    await post("/v1/auth/logout").catch(() => undefined);
    setAccessToken(null);
    setMe(null);
    qc.clear();
    setEstado("anonimo");
  }, [qc]);

  useEffect(() => {
    setSessionLostHandler(() => {
      setAccessToken(null);
      setMe(null);
      setEstado("anonimo");
    });
    // Al abrir el panel se intenta recuperar la sesión con la cookie (sin pedir contraseña).
    void refreshSession().then((s) => (s ? cargarMe().catch(() => setEstado("anonimo")) : setEstado("anonimo")));
  }, [cargarMe]);

  const puede = useCallback((p: string) => me?.permisos.includes(p) ?? false, [me]);
  return <Ctx.Provider value={{ estado, me, entrar, salir, puede }}>{children}</Ctx.Provider>;
}
