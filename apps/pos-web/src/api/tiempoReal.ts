// WebSocket al nodo con reconexión automática: espera exponencial con tope y jitter.
// El cajero nunca tiene que «reconectar» a mano.
import type { Evento } from "@restpos/contracts";

export type EventoNodo = Evento | { type: string; data: Record<string, unknown> };

export function esperaPara(intento: number): number {
  return Math.min(30_000, 500 * 2 ** Math.max(0, intento - 1));
}

export class TiempoReal {
  private ws: WebSocket | null = null;
  private intentos = 0;
  private reintento: ReturnType<typeof setTimeout> | undefined;
  private activo = false;
  private readonly oyentes = new Set<(e: EventoNodo) => void>();
  private readonly oyentesConexion = new Set<(v: boolean) => void>();
  conectado = false;

  constructor(private readonly url: () => string) {}

  iniciar() {
    if (this.activo) return;
    this.activo = true;
    this.abrir();
  }

  /** Reabre con las credenciales actuales (tras entrar o salir un usuario). */
  reiniciar() {
    this.cerrar();
    this.intentos = 0;
    if (this.activo) this.abrir();
  }

  detener() {
    this.activo = false;
    clearTimeout(this.reintento);
    this.cerrar();
  }

  alEvento(fn: (e: EventoNodo) => void) {
    this.oyentes.add(fn);
    return () => void this.oyentes.delete(fn);
  }

  alCambiarConexion(fn: (v: boolean) => void) {
    this.oyentesConexion.add(fn);
    return () => void this.oyentesConexion.delete(fn);
  }

  private marcar(v: boolean) {
    if (this.conectado === v) return;
    this.conectado = v;
    this.oyentesConexion.forEach((f) => f(v));
  }

  private cerrar() {
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
    this.marcar(false);
  }

  private abrir() {
    let ws: WebSocket;
    try {
      ws = new WebSocket(this.url());
    } catch {
      this.programar();
      return;
    }
    this.ws = ws;
    ws.onopen = () => {
      this.intentos = 0;
      this.marcar(true);
    };
    ws.onmessage = (m) => {
      try {
        const e = JSON.parse(String(m.data)) as EventoNodo;
        this.oyentes.forEach((f) => f(e));
      } catch {
        // mensaje que no es un evento: se ignora
      }
    };
    ws.onclose = () => {
      this.ws = null;
      this.marcar(false);
      this.programar();
    };
  }

  private programar() {
    if (!this.activo) return;
    this.intentos++;
    const base = esperaPara(this.intentos);
    this.reintento = setTimeout(() => this.abrir(), base / 2 + Math.random() * (base / 2));
  }
}

/** URL del WebSocket con las credenciales en la consulta (los navegadores no permiten cabeceras). */
export function urlTiempoReal(tokens: { dispositivo: string | null; usuario: string | null }, loc: Location = location): string {
  const q = new URLSearchParams();
  if (tokens.dispositivo) q.set("dispositivo", tokens.dispositivo);
  if (tokens.usuario) q.set("usuario", tokens.usuario);
  q.set("tipo", "POS");
  return `${loc.protocol === "https:" ? "wss" : "ws"}://${loc.host}/v1/ws${q.size ? `?${q}` : ""}`;
}
