import { type FormEvent, type ReactNode, useState } from "react";
import { Link, useLocation, useNavigate, useSearchParams } from "react-router";
import { ApiError, post } from "../api/client";
import { useSession } from "../api/session";
import type { Sesion } from "../api/types";
import { Field } from "../components/ui";
import { Icon } from "../lib/icons";

const FOTOS = ["ceviche-camaron", "hamburguesa", "tres-leches", "pescado-frito", "jugo", "seco-pollo"];

/** Portada promocional compartida por las pantallas de acceso. */
function Portada({ children }: { children: ReactNode }) {
  return (
    <div className="login">
      <section className="login-hero" aria-label="RestPOS">
        <div className="mosaic" aria-hidden="true">
          {FOTOS.map((f) => <img key={f} src={`/media/biblioteca/${f}/md.webp`} alt="" loading="eager" />)}
        </div>
        <h1>Tu restaurante nunca se detiene.</h1>
        <p>Comandas al instante, caja que cobra en 2 segundos y facturación electrónica SRI, aunque se caiga el internet.</p>
        <div className="beneficios">
          <span className="beneficio"><Icon name="sinInternet" size={16} /> Funciona sin internet</span>
          <span className="beneficio"><Icon name="receipt" size={16} /> Factura SRI sin esperas</span>
          <span className="beneficio"><Icon name="rayo" size={16} /> Meseros listos en 10 s</span>
        </div>
      </section>
      <section className="login-form">{children}</section>
    </div>
  );
}

function Logo() {
  return (
    <div className="brand" style={{ padding: 0 }}>
      <img src="/icono.svg" width={52} height={52} alt="" style={{ borderRadius: 12 }} />
      <div><b>RestPOS</b><span>Panel de tu restaurante</span></div>
    </div>
  );
}

function PasswordInput({ id, value, onChange, autoComplete, describedBy }: { id: string; value: string; onChange: (v: string) => void; autoComplete: string; describedBy?: string | undefined }) {
  const [ver, setVer] = useState(false);
  return (
    <div style={{ position: "relative" }}>
      <input id={id} type={ver ? "text" : "password"} value={value} onChange={(e) => onChange(e.target.value)} autoComplete={autoComplete}
        aria-describedby={describedBy} style={{ width: "100%", paddingRight: 52 }} required />
      <button type="button" className="rp-btn rp-btn--plain" onClick={() => setVer(!ver)} aria-label={ver ? "Ocultar contraseña" : "Mostrar contraseña"}
        style={{ position: "absolute", right: 4, top: 0, bottom: 0 }}>
        <Icon name={ver ? "ocultar" : "ver"} size={20} />
      </button>
    </div>
  );
}

export function Login() {
  const { entrar } = useSession();
  const nav = useNavigate();
  const aviso = (useLocation().state as { mensaje?: string } | null)?.mensaje;
  const [usuario, setUsuario] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const s = await post<Sesion>("/v1/auth/login", { usuario, password });
      await entrar(s);
      nav(s.usuario.debeCambiarPassword ? "/cambiar-clave" : "/", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "No se pudo iniciar sesión.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Portada>
      <Logo />
      <div>
        <h2>Hola de nuevo</h2>
        <p className="ayuda">Entra con tu correo o con el RUC de tu restaurante.</p>
      </div>
      {aviso && <p className="rp-status rp-status--success" role="status">{aviso}</p>}
      <form onSubmit={onSubmit} noValidate>
        <Field label="Correo o RUC">{(id) => <input id={id} value={usuario} onChange={(e) => setUsuario(e.target.value)} autoComplete="username" inputMode="email" required />}</Field>
        <Field label="Contraseña">{(id) => <PasswordInput id={id} value={password} onChange={setPassword} autoComplete="current-password" />}</Field>
        {error && <p className="error-inline" role="alert">{error}</p>}
        <button className="rp-btn rp-btn--primary rp-btn--block" disabled={busy || !usuario || !password}>{busy ? "Entrando…" : "Entrar"}</button>
      </form>
      <Link to="/olvide" className="ayuda">¿Olvidaste tu contraseña?</Link>
    </Portada>
  );
}

export function Olvide() {
  const [email, setEmail] = useState("");
  const [enviado, setEnviado] = useState(false);
  const [busy, setBusy] = useState(false);
  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    await post("/v1/auth/password/olvide", { email }).catch(() => undefined);
    setBusy(false);
    setEnviado(true);
  }
  return (
    <Portada>
      <Logo />
      {enviado ? (
        <div>
          <h2>Revisa tu correo</h2>
          <p className="ayuda">Si {email} tiene una cuenta, te llegó un enlace para crear una contraseña nueva. Sirve una vez durante 30 minutos.</p>
          <p><Link to="/login">Volver a entrar</Link></p>
        </div>
      ) : (
        <>
          <div><h2>Recupera tu acceso</h2><p className="ayuda">Te enviaremos un enlace para crear una contraseña nueva.</p></div>
          <form onSubmit={onSubmit}>
            <Field label="Correo">{(id) => <input id={id} type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" required />}</Field>
            <button className="rp-btn rp-btn--primary rp-btn--block" disabled={busy || !email.includes("@")}>{busy ? "Enviando…" : "Enviar enlace"}</button>
          </form>
          <Link to="/login" className="ayuda">Volver a entrar</Link>
        </>
      )}
    </Portada>
  );
}

export function Restablecer() {
  const [params] = useSearchParams();
  const nav = useNavigate();
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await post("/v1/auth/password/restablecer", { token: params.get("token") ?? "", password });
      nav("/login", { replace: true, state: { mensaje: "Listo. Entra con tu contraseña nueva." } });
    } catch (err) {
      setError(err instanceof ApiError ? (err.fields.password ?? err.message) : "No se pudo cambiar la contraseña.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <Portada>
      <Logo />
      <div><h2>Crea tu contraseña</h2><p className="ayuda">Usa al menos 10 caracteres combinando letras y números.</p></div>
      <form onSubmit={onSubmit}>
        <Field label="Contraseña nueva" error={error}>{(id, d) => <PasswordInput id={id} value={password} onChange={setPassword} autoComplete="new-password" describedBy={d} />}</Field>
        <button className="rp-btn rp-btn--primary rp-btn--block" disabled={busy || password.length < 10}>{busy ? "Guardando…" : "Guardar y entrar"}</button>
      </form>
    </Portada>
  );
}

/** Cambio obligatorio de la contraseña temporal (RF-01-01.4): la API no deja hacer nada más antes. */
export function CambiarClave() {
  const { entrar, me } = useSession();
  const nav = useNavigate();
  const [actual, setActual] = useState("");
  const [nueva, setNueva] = useState("");
  const [errores, setErrores] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErrores({});
    try {
      const s = await post<Sesion>("/v1/auth/password/cambiar", { actual, nueva });
      await entrar(s);
      nav("/", { replace: true });
    } catch (err) {
      setErrores(err instanceof ApiError ? { ...err.fields, general: Object.keys(err.fields).length ? "" : err.message } : { general: "No se pudo cambiar." });
    } finally {
      setBusy(false);
    }
  }
  return (
    <Portada>
      <Logo />
      <div>
        <h2>¡Bienvenido{me ? `, ${me.nombreMostrar.split(" ")[0]}` : ""}!</h2>
        <p className="ayuda">Por seguridad, cambia la contraseña temporal que te llegó por correo.</p>
      </div>
      <form onSubmit={onSubmit}>
        <Field label="Contraseña temporal" error={errores.actual}>{(id, d) => <PasswordInput id={id} value={actual} onChange={setActual} autoComplete="current-password" describedBy={d} />}</Field>
        <Field label="Contraseña nueva" error={errores.nueva} hint="Al menos 10 caracteres, con letras y números.">{(id, d) => <PasswordInput id={id} value={nueva} onChange={setNueva} autoComplete="new-password" describedBy={d} />}</Field>
        {errores.general && <p className="error-inline" role="alert">{errores.general}</p>}
        <button className="rp-btn rp-btn--primary rp-btn--block" disabled={busy || !actual || nueva.length < 10}>{busy ? "Guardando…" : "Guardar y empezar"}</button>
      </form>
    </Portada>
  );
}
