// Alertas fiscales (F5-16): comprobantes sin autorizar, errores del SRI, firma por vencer.
// Cada una dice qué pasa y lleva a donde se resuelve.
import { Link } from "react-router";
import { useAlertasFiscales } from "../api/hooks";
import { useSession } from "../api/session";
import { Icon } from "../lib/icons";

export function AlertasFiscales() {
  const { puede } = useSession();
  const { data } = useAlertasFiscales(puede("VER_REPORTES"));
  if (!data || data.length === 0) return null;
  return (
    <section className="alertas-fiscales" aria-label="Alertas fiscales" data-testid="alertas-fiscales">
      {data.map((a) => (
        <Link key={a.clave} to={a.enlace} className="alerta-fiscal" data-nivel={a.nivel}>
          <Icon name={a.nivel === "CRITICA" ? "error" : "info"} size={18} />
          <span>
            <strong>{a.titulo}</strong> {a.accion}
          </span>
          <Icon name="chevronRight" size={16} />
        </Link>
      ))}
    </section>
  );
}
