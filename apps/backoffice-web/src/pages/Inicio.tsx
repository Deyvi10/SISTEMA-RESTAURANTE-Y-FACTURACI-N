import { Link } from "react-router";
import { useResumen } from "../api/hooks";
import { useSession } from "../api/session";
import { AppIcon, Spinner } from "../components/ui";
import { Icon } from "../lib/icons";

const TINT_PASO: Record<string, string> = { menu: "orange", fotos: "pink", salon: "green", personal: "indigo", impresoras: "gray", sri: "red" };

export function Inicio() {
  const { me } = useSession();
  const { data, isLoading, error } = useResumen();
  if (isLoading) return <Spinner />;
  if (error || !data) return <p className="error-inline">No pudimos cargar tu resumen. Recarga la página.</p>;
  const nombre = me?.nombreMostrar.split(" ")[0] ?? "";
  const listo = data.progreso === 100;
  return (
    <>
      <section className="welcome">
        <div>
          <h1>{listo ? `¡Todo listo, ${nombre}!` : `Hola, ${nombre}. Preparemos ${data.nombreComercial}.`}</h1>
          <p>{listo
            ? "Tu restaurante está configurado. Cuando lleguen las impresoras y la app de meseros, empiezas a vender."
            : "Sigue estos pasos y en unos 10 minutos tu menú, tu salón y tu equipo estarán listos."}</p>
        </div>
        <div className="fotos" aria-hidden="true">
          {["ceviche-camaron", "hamburguesa", "tres-leches"].map((f) => <img key={f} src={`/media/biblioteca/${f}/sm.webp`} alt="" />)}
        </div>
      </section>

      <div className="progreso">
        <div className="anillo" style={{ ["--p" as string]: data.progreso }} role="img" aria-label={`Progreso ${data.progreso} %`}>
          <span>{data.progreso}%</span>
        </div>
        <div>
          <h2 className="rp-t-title3">Configura tu restaurante</h2>
          <p className="rp-secondary" style={{ margin: 0 }}>{data.pasos.filter((p) => p.disponible && p.hecho).length} de {data.pasos.filter((p) => p.disponible).length} pasos listos</p>
        </div>
      </div>
      <div className="pasos">
        {data.pasos.map((p) => {
          const contenido = (
            <>
              <AppIcon icono={p.icono} tint={TINT_PASO[p.id] ?? "blue"} size={44} />
              <span className="estado">
                {p.hecho ? <span className="check-hecho" aria-label="Listo"><Icon name="ok" size={16} strokeWidth={3} /></span>
                  : !p.disponible ? <span className="rp-status rp-status--neutral">Pronto</span> : null}
              </span>
              <h3>{p.titulo}</h3>
              <p>{p.descripcion}</p>
            </>
          );
          return p.disponible
            ? <Link key={p.id} to={p.ruta} className="paso" data-hecho={p.hecho}>{contenido}</Link>
            : <div key={p.id} className="paso" data-disponible="false">{contenido}</div>;
        })}
      </div>

      <div className="stats">
        <div className="rp-widget"><span className="rp-widget__label" style={{ color: "var(--rp-color-warning-text)" }}><Icon name="utensils" size={15} /> Platos</span><span className="rp-widget__value">{data.cuentas.productos}</span><span className="rp-widget__delta rp-secondary">{data.cuentas.conFoto} con foto</span></div>
        <div className="rp-widget"><span className="rp-widget__label" style={{ color: "var(--rp-color-success-text)" }}><Icon name="mesa" size={15} /> Mesas</span><span className="rp-widget__value">{data.cuentas.mesas}</span><span className="rp-widget__delta rp-secondary">en tu salón</span></div>
        <div className="rp-widget"><span className="rp-widget__label" style={{ color: "var(--rp-color-accent)" }}><Icon name="personal" size={15} /> Equipo</span><span className="rp-widget__value">{data.cuentas.personal}</span><span className="rp-widget__delta rp-secondary">personas con PIN</span></div>
        <div className="rp-widget"><span className="rp-widget__label"><Icon name="etiqueta" size={15} /> Categorías</span><span className="rp-widget__value">{data.cuentas.categorias}</span><span className="rp-widget__delta rp-secondary">en tu menú</span></div>
      </div>

      <div className="promo">
        <AppIcon icono="sinInternet" tint="teal" size={52} />
        <div>
          <b>Tu salón sigue funcionando sin internet</b>
          <p>El Nodo Local mantiene comandas, impresión y cobros aunque se caiga CNT o Netlife. Lo instalamos juntos en la siguiente etapa.</p>
        </div>
      </div>
    </>
  );
}

export function Proximamente({ titulo, icono, tint, texto, beneficios }: { titulo: string; icono: string; tint: string; texto: string; beneficios: string[] }) {
  return (
    <div className="proximamente">
      <AppIcon icono={icono} tint={tint} size={84} />
      <span className="rp-status rp-status--neutral">Próximamente</span>
      <h1>{titulo}</h1>
      <p>{texto}</p>
      <ul>
        {beneficios.map((b) => <li key={b}><span className="check-hecho"><Icon name="ok" size={14} strokeWidth={3} /></span>{b}</li>)}
      </ul>
    </div>
  );
}
