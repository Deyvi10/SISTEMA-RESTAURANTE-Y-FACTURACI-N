import { Navigate, Route, Routes, useLocation } from "react-router";
import { useSession } from "./api/session";
import { Spinner } from "./components/ui";
import { Shell } from "./layout/Shell";
import { CambiarClave, Login, Olvide, Restablecer } from "./pages/Acceso";
import { Inicio, Proximamente } from "./pages/Inicio";
import { Menu } from "./pages/Menu";
import { Ajustes, Personal } from "./pages/Personal";
import { Salon } from "./pages/Salon";

function Protegida({ children }: { children: React.ReactNode }) {
  const { estado, me } = useSession();
  const loc = useLocation();
  if (estado === "cargando") return <Spinner label="Abriendo tu restaurante…" />;
  if (estado === "anonimo") return <Navigate to="/login" replace state={{ desde: loc.pathname }} />;
  if (me?.debeCambiarPassword && loc.pathname !== "/cambiar-clave") return <Navigate to="/cambiar-clave" replace />;
  return <>{children}</>;
}

function SoloAnonimo({ children }: { children: React.ReactNode }) {
  const { estado } = useSession();
  if (estado === "cargando") return <Spinner />;
  return estado === "activo" ? <Navigate to="/" replace /> : <>{children}</>;
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<SoloAnonimo><Login /></SoloAnonimo>} />
      <Route path="/olvide" element={<SoloAnonimo><Olvide /></SoloAnonimo>} />
      <Route path="/restablecer" element={<Restablecer />} />
      <Route path="/cambiar-clave" element={<Protegida><CambiarClave /></Protegida>} />
      <Route element={<Protegida><Shell /></Protegida>}>
        <Route index element={<Inicio />} />
        <Route path="menu" element={<Menu />} />
        <Route path="salon" element={<Salon />} />
        <Route path="personal" element={<Personal />} />
        <Route path="ajustes" element={<Ajustes />} />
        <Route path="impresoras" element={<Proximamente titulo="Impresoras sin complicaciones" icono="printer" tint="gray"
          texto="Tu Nodo Local encontrará solas las impresoras de cocina, bar y caja. Tú solo arrastras cada categoría a su estación."
          beneficios={["Detección automática por red y USB", "Comandas en cocina en menos de 1,5 segundos", "Si se acaba el papel, no se pierde ningún pedido"]} />} />
        <Route path="facturacion" element={<Proximamente titulo="Facturación electrónica SRI" icono="receipt" tint="pink"
          texto="Sube tu firma electrónica una vez y factura sin hacer esperar al cajero, incluso cuando el SRI está lento o caído."
          beneficios={["Comprobante impreso al instante con clave de acceso", "Autorización automática en segundo plano", "Tus XML guardados 7 años sin posibilidad de borrarse"]} />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  );
}
