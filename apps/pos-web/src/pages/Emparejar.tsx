// Emparejar una PC de caja que no es la del nodo: el código sale en la PC del nodo (Estado › Emparejar).
import { MonitorSmartphone } from "lucide-react";
import { type FormEvent, useState } from "react";
import { ApiError } from "../api/nodo";
import { useSesion } from "../api/sesion";

/** WebCrypto solo existe en orígenes seguros: otra PC de la LAN necesita el TLS del nodo (F2-07). */
export function puedeEmparejar(): boolean {
  return window.isSecureContext && !!globalThis.crypto?.subtle;
}

export function Emparejar() {
  const { emparejar, error: errorSesion } = useSesion();
  const [codigo, setCodigo] = useState("");
  const [nombre, setNombre] = useState("Caja 2");
  const [error, setError] = useState<string | null>(errorSesion);
  const [enviando, setEnviando] = useState(false);

  const enviar = async (e: FormEvent) => {
    e.preventDefault();
    const c = codigo.replace(/[^A-Za-z0-9]/g, "").toUpperCase();
    if (c.length !== 8) {
      setError("El código tiene 8 caracteres. Lo ves en la PC del nodo, en Estado › Emparejar.");
      return;
    }
    setEnviando(true);
    try {
      await emparejar(c, nombre.trim() || "Caja");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo emparejar esta caja.");
    } finally {
      setEnviando(false);
    }
  };

  if (!puedeEmparejar()) {
    return (
      <main className="centrado">
        <div className="tarjeta-acceso" role="alert">
          <span className="rp-app-icon rp-app-icon--xl" style={{ ["--tint" as string]: "var(--rp-tint-orange)" }}>
            <MonitorSmartphone />
          </span>
          <h1 className="rp-t-title1">Usa la caja en la PC del nodo</h1>
          <p className="rp-secondary">Para cobrar desde otra computadora, el nodo necesita su conexión segura (HTTPS), que todavía no está activada. Mientras tanto, abre la caja en la PC donde está instalado el nodo.</p>
        </div>
      </main>
    );
  }

  return (
    <main className="centrado">
      <form className="tarjeta-acceso" onSubmit={enviar}>
        <span className="rp-app-icon rp-app-icon--xl" style={{ ["--tint" as string]: "var(--rp-tint-blue)" }}>
          <MonitorSmartphone />
        </span>
        <h1 className="rp-t-title1">Emparejar esta caja</h1>
        <p className="rp-secondary">En la PC del nodo abre Estado › Emparejar y escribe aquí el código.</p>
        <div className="rp-field" data-invalid={error ? "true" : undefined}>
          <label htmlFor="codigo">Código</label>
          <input id="codigo" value={codigo} onChange={(e) => setCodigo(e.target.value)} autoFocus autoComplete="off" placeholder="ABCD-2345" className="rp-mono" data-testid="codigo" />
        </div>
        <div className="rp-field">
          <label htmlFor="nombre">Nombre de esta caja</label>
          <input id="nombre" value={nombre} onChange={(e) => setNombre(e.target.value)} maxLength={40} />
        </div>
        {error && (
          <p className="rp-field__error" role="alert">
            {error}
          </p>
        )}
        <button className="rp-btn rp-btn--primary rp-btn--block" disabled={enviando}>
          {enviando ? "Emparejando…" : "Emparejar"}
        </button>
      </form>
    </main>
  );
}
