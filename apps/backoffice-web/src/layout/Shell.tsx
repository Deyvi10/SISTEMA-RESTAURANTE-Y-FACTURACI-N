import { NavLink, Outlet } from "react-router";
import { useSession } from "../api/session";
import { AppIcon, initials, tintFor } from "../components/ui";
import { Icon } from "../lib/icons";

export const SECCIONES = [
  { to: "/", label: "Inicio", icono: "inicio", tint: "blue", permiso: null },
  { to: "/menu", label: "Menú", icono: "utensils", tint: "orange", permiso: "CONFIGURAR_MENU" },
  { to: "/salon", label: "Salón", icono: "salon", tint: "green", permiso: "CONFIGURAR_SALON" },
  { to: "/personal", label: "Personal", icono: "personal", tint: "indigo", permiso: "GESTIONAR_PERSONAL" },
  { to: "/impresoras", label: "Impresoras", icono: "printer", tint: "gray", permiso: "CONFIGURAR_SALON", pronto: true },
  { to: "/facturacion", label: "Facturación SRI", icono: "receipt", tint: "pink", permiso: "CONFIGURAR_SRI", pronto: true },
  { to: "/ajustes", label: "Ajustes", icono: "ajustes", tint: "gray", permiso: "CONFIGURAR_SALON" },
] as const;

export function Shell() {
  const { me, salir, puede } = useSession();
  const visibles = SECCIONES.filter((s) => !s.permiso || puede(s.permiso));
  return (
    <div className="shell">
      <nav className="sidebar" aria-label="Secciones">
        <div className="brand">
          <img src="/icono.svg" width={40} height={40} alt="" style={{ borderRadius: 10 }} />
          <div><b>{me?.nombreComercial}</b><span>Panel del restaurante</span></div>
        </div>
        {visibles.map((s) => (
          <NavLink key={s.to} to={s.to} end={s.to === "/"} className="navlink">
            <AppIcon icono={s.icono} tint={s.tint} size={30} />
            {s.label}
            {"pronto" in s && s.pronto && <span className="soon">Pronto</span>}
          </NavLink>
        ))}
        <div className="sidebar-foot">
          <div className="userchip">
            <span className="avatar" style={{ ["--tint" as string]: `var(--rp-tint-${tintFor(me?.nombreMostrar ?? "")})`, width: 36, height: 36, fontSize: 14 }}>
              {initials(me?.nombreMostrar ?? "")}
            </span>
            <div><b style={{ fontSize: 14 }}>{me?.nombreMostrar}</b><small>{me?.email}</small></div>
          </div>
          <button type="button" className="rp-btn rp-btn--gray rp-btn--sm" onClick={() => void salir()}>
            <Icon name="salir" size={16} /> Cerrar sesión
          </button>
        </div>
      </nav>
      <main className="main" id="contenido">
        <Outlet />
      </main>
      <nav className="rp-tabbar tabbar-movil" aria-label="Secciones">
        {visibles.filter((s) => !("pronto" in s && s.pronto)).slice(0, 5).map((s) => (
          <NavLink key={s.to} to={s.to} end={s.to === "/"} className="rp-tab"
            style={({ isActive }) => ({ color: isActive ? "var(--rp-color-accent)" : undefined })}>
            <Icon name={s.icono} size={24} />
            {s.label}
          </NavLink>
        ))}
      </nav>
    </div>
  );
}
