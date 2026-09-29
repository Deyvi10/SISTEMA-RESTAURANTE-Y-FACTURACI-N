// Suite de caja (F4-16): abrir turno → cobrar mesa → pago mixto → dividir → cerrar turno.
// Cada cobro se registra desde la respuesta del nodo y al final se declara exactamente eso:
// el Cierre Z debe salir Cuadrado y su ticket debe mostrar lo mismo que se cobró.
import { aTexto, centavos, type Datos, desglose, ordenEnMesa } from "../support/e2e";

interface Pago {
  metodo: string;
  tipo: string;
  monto: string;
}

const FONDO = 5000; // $50.00

describe("caja: del turno al Cierre Z", { testIsolation: false }, () => {
  let d: Datos;
  const pagos: Pago[] = [];
  const documentos: string[] = [];
  const suma = (tipo: (t: string) => boolean) => pagos.filter((p) => tipo(p.tipo)).reduce((t, p) => t + centavos(p.monto), 0);

  before(() => {
    cy.request(`${Cypress.expose("aux")}/datos`).then(({ body }) => {
      expect(body, "datos sembrados por nodo-e2e").to.have.property("cajera");
      d = body as Datos;
    });
  });

  beforeEach(() => {
    cy.intercept("POST", "/v1/ordenes/*/cobrar", (req) => {
      req.continue((res) => {
        if (res.statusCode === 200) {
          pagos.push(...(res.body.documento.pagos as Pago[]));
          documentos.push(res.body.documento.codigo as string);
        }
      });
    }).as("cobro");
  });

  it("la cajera entra con su PIN y abre el turno con $50 de fondo", () => {
    cy.visit("/pos/");
    cy.contains("button", d.cajera.nombre).click();
    cy.get("body").type(d.cajera.pin);
    cy.getByTestId("tab-turno").click();
    cy.getByTestId("abrir-turno").click();
    cy.get("body").type("50");
    cy.getByTestId("monto").should("contain.text", "50");
    cy.getByTestId("confirmar-turno").click();
    cy.getByTestId("estado-turno").should("contain.text", "turno de");
    cy.getByTestId("cerrar-turno").should("be.visible");
  });

  it("cobra una mesa en efectivo con el segundo billete y da el vuelto", () => {
    ordenEnMesa("Mesa 1", [["Ceviche de camarón", 1], ["Cerveza", 2]]);
    cy.getByTestId("tab-mesas").click();
    cy.getByTestId("mesa-Mesa 1").should("have.attr", "data-estado", "ocupada").click();
    cy.getByTestId("total").should("contain.text", "$20.11"); // 18.50 con IVA + 10 % de servicio sobre la base
    cy.get("body").type("2"); // $21.00
    cy.wait("@cobro");
    cy.getByTestId("vuelto").should("have.attr", "aria-label", "Vuelto $0.89");
    cy.getByTestId("cobro-listo").should("contain.text", "INT-").and("contain.text", "Mesa 1 libre");
    cy.get("body").type("{enter}");
    cy.getByTestId("mesa-Mesa 1").should("have.attr", "data-estado", "libre");
  });

  it("cobra otra mesa con pago mixto: $10 en efectivo y el resto con tarjeta", () => {
    ordenEnMesa("Mesa 2", [["Arroz marinero", 1], ["Jugo natural", 2]]);
    cy.getByTestId("mesa-Mesa 2").should("have.attr", "data-estado", "ocupada").click();
    cy.getByTestId("total").should("contain.text", "$");
    cy.get("body").type("m");
    cy.getByTestId("agregar-Efectivo").click();
    cy.get('[aria-label="Monto de Efectivo"]').clear().type("10.00");
    cy.getByTestId("agregar-Tarjeta crédito").click();
    cy.getByTestId("pago-Tarjeta crédito").find("input").eq(0).should("not.have.value", "");
    cy.getByTestId("pago-Tarjeta crédito").contains("label", "Últimos 4").find("input").type("4821");
    cy.getByTestId("restante").should("have.text", "Completo");
    cy.getByTestId("cobrar-mixto").click();
    cy.wait("@cobro").its("response.body.documento.pagos").should("have.length", 2);
    cy.getByTestId("cobro-listo").should("contain.text", "Efectivo + Tarjeta crédito");
    cy.get("body").type("{enter}");
  });

  it("divide una mesa en dos cuentas, comparte la cerveza y cobra cada una", () => {
    ordenEnMesa("Mesa 3", [["Ceviche de camarón", 1], ["Pizza personal", 1], ["Cerveza", 2]]);
    cy.getByTestId("mesa-Mesa 3").should("have.attr", "data-estado", "ocupada").click();
    cy.getByTestId("total")
      .invoke("text")
      .then((totalOrden) => {
        cy.get("body").type("v");
        cy.getByTestId("division").should("be.visible");
        cy.getByTestId("confirmar-division").should("be.disabled"); // hay platos sin cuenta
        cy.getByTestId("plato-Ceviche de camarón").contains("Ceviche de camarón").click(); // a la cuenta 1
        cy.get("body").type("2");
        cy.getByTestId("plato-Pizza personal").contains("Pizza personal").click(); // a la cuenta 2
        cy.get('[aria-label="Compartir Cerveza"]').click();
        cy.getByTestId("confirmar-compartir").click();
        cy.getByTestId("plato-Cerveza").should("contain.text", "Entre 1, 2");
        cy.getByTestId("confirmar-division").click();

        // Las dos cuentas suman exacto el total de la orden.
        cy.getByTestId("elegir-cuenta-1").invoke("text").then((c1) => {
          cy.getByTestId("elegir-cuenta-2").invoke("text").then((c2) => {
            const monto = (s: string) => centavos(/\$[\d.]+/.exec(s)![0]);
            expect(monto(c1) + monto(c2), "cuenta 1 + cuenta 2").to.eq(centavos(totalOrden));
          });
        });
        // Cuenta 1 en efectivo exacto; la mesa sigue ocupada.
        cy.get("body").type("1");
        cy.wait("@cobro").its("response.body.cerrada").should("eq", false);
        cy.getByTestId("cobro-listo").should("contain.text", "cuenta 1 de Mesa 3");
        cy.getByTestId("siguiente-cuenta").click();
        // Cuenta 2 con tarjeta: el detalle del voucher abre con el total listo.
        cy.getByTestId("elegir-cuenta-1").should("contain.text", "✓");
        cy.getByTestId("metodo-Tarjeta crédito").click();
        cy.getByTestId("restante").should("have.text", "Completo");
        cy.getByTestId("cobrar-mixto").click();
        cy.wait("@cobro").its("response.body.cerrada").should("eq", true);
        cy.getByTestId("siguiente-cliente").click();
        cy.getByTestId("mesa-Mesa 3").should("have.attr", "data-estado", "libre");
      });
  });

  it("cierra el turno declarando lo cobrado y el Cierre Z sale Cuadrado", () => {
    const efectivo = suma((t) => t === "EFECTIVO");
    const tarjeta = suma((t) => t === "TARJETA_CREDITO");
    expect(pagos.length, "pagos registrados").to.eq(5);
    cy.getByTestId("tab-turno").click();
    cy.getByTestId("cerrar-turno").click();
    const conteo = desglose(FONDO + efectivo);
    for (const paso of ["B", "M"]) {
      for (const [clave, n] of conteo.filter(([c]) => c.startsWith(paso))) {
        cy.getByTestId(`den-${clave}`).click().type(String(n));
      }
      cy.getByTestId("siguiente").click();
    }
    cy.getByTestId("voucher-Tarjeta crédito").type(aTexto(tarjeta));
    cy.getByTestId("siguiente").click();
    cy.contains(".cierre__contado", `$${aTexto(FONDO + efectivo)}`);
    cy.getByTestId("confirmar-cierre").click();
    cy.getByTestId("resultado-cierre").should("contain.text", "Cuadrado").and("not.contain.text", "Faltante").and("not.contain.text", "Sobrante");
  });

  it("el ticket del Cierre Z coincide con la suma de los pagos", () => {
    const efectivo = suma((t) => t === "EFECTIVO");
    const tarjeta = suma((t) => t === "TARJETA_CREDITO");
    const dinero = (c: number) => `\\$${aTexto(c).replace(".", "\\.")}`;
    // La impresión es asíncrona: se reintenta hasta que llegue el Z a la impresora de la caja.
    const leerZ = (intentos: number): Cypress.Chainable<string> =>
      cy.request(`${Cypress.expose("aux")}/impresiones`).then(({ body }): string | Cypress.Chainable<string> => {
        const z = (body.Caja as string[]).find((t) => t.includes("CIERRE Z"));
        if (z || intentos === 0) return z ?? "";
        return cy.wait(250).then(() => leerZ(intentos - 1));
      }) as Cypress.Chainable<string>;
    leerZ(20).then((z) => {
      expect(z, "ticket del Cierre Z").to.match(new RegExp(`Fondo inicial\\s+${dinero(FONDO)}`));
      expect(z).to.match(new RegExp(`Ventas en efectivo\\s+${dinero(efectivo)}`));
      expect(z).to.match(new RegExp(`Tarjeta crédito\\s*\\n\\s*Esperado\\s+${dinero(tarjeta)}\\s*\\n\\s*Declarado\\s+${dinero(tarjeta)}`));
      expect(z).to.match(/=+\s*\n\s*CUADRADO/);
    });
  });

  it("cada venta salió como documento interno sin valor tributario", () => {
    cy.request(`${Cypress.expose("aux")}/impresiones`).then(({ body }) => {
      expect(documentos.length, "documentos emitidos").to.eq(4); // mesa 1, mixto y las dos cuentas
      const ventas = (body.Caja as string[]).filter((t) => documentos.some((c) => t.includes(c)));
      expect(ventas.length, "tickets de venta impresos").to.eq(documentos.length);
      for (const t of ventas) {
        const plano = t.replace(/\s+/g, " ");
        expect(plano).to.contain("DOCUMENTO INTERNO DE VENTA SIN VALOR TRIBUTARIO");
        expect(plano).to.contain("no reemplaza a la factura electrónica");
      }
    });
  });
});
