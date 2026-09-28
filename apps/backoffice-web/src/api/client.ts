// Cliente HTTP del backoffice. El access token vive solo en memoria (nunca en localStorage:
// un XSS no puede robarlo después de recargar); la renovación usa la cookie HttpOnly.

import type { Sesion } from "./types";

export interface FieldError {
  campo: string;
  mensaje: string;
}

/** Error de la API con el mensaje para el usuario y los errores por campo (RFC 9457). */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: Record<string, string>;
  constructor(status: number, code: string, message: string, fields: FieldError[] = []) {
    super(message);
    this.status = status;
    this.code = code;
    this.fields = Object.fromEntries(fields.map((f) => [f.campo, f.mensaje]));
  }
}

let accessToken: string | null = null;
let refreshing: Promise<Sesion | null> | null = null;
let onSessionLost: () => void = () => {};

export function setSessionLostHandler(fn: () => void) {
  onSessionLost = fn;
}
export function setAccessToken(t: string | null) {
  accessToken = t;
}

/** Renueva la sesión con la cookie. Varias peticiones a la vez comparten una sola renovación. */
export function refreshSession(): Promise<Sesion | null> {
  refreshing ??= fetch("/v1/auth/refresh", { method: "POST", credentials: "include" })
    .then(async (r) => {
      if (!r.ok) return null;
      const s = (await r.json()) as Sesion;
      accessToken = s.accessToken;
      return s;
    })
    .catch(() => null)
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

async function toError(r: Response): Promise<ApiError> {
  try {
    const p = (await r.json()) as { code?: string; detail?: string; errors?: FieldError[] };
    return new ApiError(r.status, p.code ?? "ERROR", p.detail ?? "No se pudo completar la acción.", p.errors ?? []);
  } catch {
    return new ApiError(r.status, "ERROR", r.status >= 500 ? "El servidor no responde. Intenta de nuevo en un momento." : "No se pudo completar la acción.");
  }
}

type Body = BodyInit | object | undefined;

export async function api<T>(method: string, path: string, body?: Body, retry = true): Promise<T> {
  const headers: Record<string, string> = {};
  let payload: BodyInit | undefined;
  if (body instanceof FormData) {
    payload = body;
  } else if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    payload = JSON.stringify(body);
  }
  if (accessToken) headers.Authorization = `Bearer ${accessToken}`;
  let r: Response;
  try {
    r = await fetch(path, { method, headers, body: payload, credentials: "include" });
  } catch {
    throw new ApiError(0, "SIN_CONEXION", "No hay conexión con el servidor. Revisa tu internet.");
  }
  if (r.status === 401 && retry && !path.startsWith("/v1/auth/")) {
    if (await refreshSession()) return api<T>(method, path, body, false);
    onSessionLost();
  }
  if (!r.ok) throw await toError(r);
  if (r.status === 204 || r.status === 202) return undefined as T;
  return (await r.json()) as T;
}

export const get = <T>(p: string) => api<T>("GET", p);
export const post = <T>(p: string, b?: Body) => api<T>("POST", p, b);
export const put = <T>(p: string, b?: Body) => api<T>("PUT", p, b);
export const patch = <T>(p: string, b?: Body) => api<T>("PATCH", p, b);
export const del = (p: string) => api<void>("DELETE", p);

/** UUID v7 generado en el cliente: las creaciones son idempotentes si se reintentan. */
export function uuidv7(): string {
  const b = new Uint8Array(16);
  crypto.getRandomValues(b);
  let ms = BigInt(Date.now());
  for (let i = 5; i >= 0; i--) {
    b[i] = Number(ms & 0xffn);
    ms >>= 8n;
  }
  b[6] = (b[6]! & 0x0f) | 0x70;
  b[8] = (b[8]! & 0x3f) | 0x80;
  const h = [...b].map((x) => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}
