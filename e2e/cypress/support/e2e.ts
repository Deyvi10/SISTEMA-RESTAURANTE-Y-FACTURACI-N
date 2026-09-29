// Ayudas de la suite de caja: datos sembrados por nodo-e2e, órdenes enviadas como lo haría
// la app de meseros y dinero en centavos enteros (nunca float).

export interface Persona {
  id: string;
  nombre: string;
  pin: string;
}
export interface Datos {
  cajera: Persona;
  supervisor: Persona;
  mesero: Persona;
  mesas: Record<string, string>;
  productos: Record<string, string>;
}

/** «18.48» o «$18.48» → 1848. */
export function centavos(s: string): number {
  const m = /^-?\$?(\d+)(?:\.(\d{1,2}))?$/.exec(s.trim().replace("$", ""));
  if (!m) throw new Error(`monto inválido: ${s}`);
  return Number(m[1]) * 100 + Number((m[2] ?? "0").padEnd(2, "0"));
}
export const aTexto = (c: number) => `${Math.floor(c / 100)}.${String(c % 100).padStart(2, "0")}`;

/** Billetes y monedas (claves del asistente de cierre) que suman exacto, de mayor a menor. */
export function desglose(c: number): [string, number][] {
  const den: [string, number][] = [["B100", 10000], ["B50", 5000], ["B20", 2000], ["B10", 1000], ["B5", 500], ["B1", 100], ["M0.50", 50], ["M0.25", 25], ["M0.10", 10], ["M0.05", 5], ["M0.01", 1]];
  const out: [string, number][] = [];
  for (const [clave, v] of den) {
    const n = Math.floor(c / v);
    if (n > 0) out.push([clave, n]);
    c -= n * v;
  }
  return out;
}

/**
 * El mesero envía una orden desde un teléfono emparejado (nodo-e2e lo simula con la misma API
 * que la app Flutter). No se hace desde el navegador: la PC de caja tiene una sola sesión.
 */
export const ordenEnMesa = (mesa: string, platos: [string, number][]) =>
  cy.request("POST", `${Cypress.expose("aux")}/ordenes`, { mesa, platos }).its("status").should("eq", 200);

declare global {
  namespace Cypress {
    interface Chainable {
      getByTestId(id: string): Chainable<JQuery<HTMLElement>>;
    }
  }
}
Cypress.Commands.add("getByTestId", (id: string) => cy.get(`[data-testid="${id}"]`));
