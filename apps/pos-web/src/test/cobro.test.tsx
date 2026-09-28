import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type EstadoCaja, type Mesa, nodo } from "../api/nodo";
import { AtajosProvider } from "../components/atajos";
import { opcionesBillete, vueltoCentavos } from "../lib/billetes";
import { Cobro } from "../pages/Cobro";
import { aEnvio, type FilaPago, problemaPagos, restante } from "../pages/PagoMixto";
import type { Caja } from "../pages/Turno";

afterEach(() => vi.restoreAllMocks());

const docCF = { comprador: "CONSUMIDOR FINAL", compradorTipo: "07", compradorIdentificacion: "9999999999999", clienteGuardado: false };

describe("billetes dinámicos", () => {
  it.each([
    ["14.50", ["14.50", "15.00", "20.00", "50.00"]], // el ejemplo del requisito
    ["20.11", ["20.11", "21.00", "25.00", "30.00"]],
    ["3.25", ["3.25", "4.00", "5.00", "10.00"]],
    ["20.00", ["20.00", "50.00", "100.00"]], // exacto en billete: no se ofrece $20 dos veces
    ["0.35", ["0.35", "1.00", "5.00", "10.00"]],
    ["187.40", ["187.40", "188.00", "190.00", "200.00"]],
  ])("para $%s", (total, esperado) => {
    expect(opcionesBillete(total)).toEqual(esperado);
  });

  it("calcula el vuelto en centavos exactos", () => {
    expect(vueltoCentavos("14.50", "20")).toBe(550);
    expect(vueltoCentavos("20.11", "50.00")).toBe(2989);
    expect(vueltoCentavos("0.30", "0.10")).toBe(-20);
  });
});

const mesa: Mesa = { id: "m1", zonaId: "z", nombre: "Mesa 4", capacidad: 4, estado: "POR_PAGAR", ordenId: "o1", numeroOrden: 7, meseroNombre: "Carlos M.",
  abiertaAt: null, precuentaAt: null, comensales: 2, platos: 3, total: "14.50", bloqueo: null };
const metodos = [
  { id: "ef", nombre: "Efectivo", tipo: "EFECTIVO", codigoSri: "01", abreCajon: true, pideReferencia: false, icono: "efectivo" },
  { id: "tc", nombre: "Tarjeta crédito", tipo: "TARJETA_CREDITO", codigoSri: "19", abreCajon: false, pideReferencia: true, icono: "tarjeta" },
];
const caja = (turno: boolean): Caja => ({
  config: { cajas: [], metodos, consumidorFinalMaximo: "50.00", denominaciones: [] },
  cajaId: "c1",
  elegir: () => {},
  estado: { caja: { id: "c1", nombre: "Caja 1", estacionId: null }, jornada: null, turno: turno ? ({ id: "t1" } as EstadoCaja["turno"]) : null },
  error: null,
  recargar: () => {},
});
const totales = (total: string) => ({ subtotal: "12.61", iva: "1.89", propina: "0.00", total });
const orden = { id: "o1", mesaId: "m1", mesa: "Mesa 4", tipo: "MESA" as const, etiqueta: "", meseroNombre: "Carlos M.", numero: 7, estado: "PRECUENTA", lineas: [] };

function montar(turno = true) {
  return render(
    <AtajosProvider>
      <Cobro orden={{ ordenId: mesa.ordenId!, nombre: mesa.nombre }} caja={caja(turno)} volver={() => {}} irATurno={() => {}} />
    </AtajosProvider>,
  );
}

describe("pantalla de cobro", () => {
  it("un toque en $20 cobra en efectivo y muestra el vuelto", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales: totales("14.50") });
    const cobrar = vi.spyOn(nodo, "cobrar").mockResolvedValue({
      documento: { ...docCF, id: "d", codigo: "INT-000001", mesa: "Mesa 4", totales: totales("14.50"), metodo: "Efectivo", pagos: [{ metodoId: "ef", metodo: "Efectivo", tipo: "EFECTIVO", monto: "14.50" }], recibido: "20.00", vuelto: "5.50", abreCajon: true },
      impresoras: ["Caja"],
    });
    montar();
    expect(await screen.findByTestId("billete-50.00")).toBeInTheDocument();
    expect(screen.getByTestId("total")).toHaveTextContent("$14.50");
    fireEvent.click(screen.getByTestId("billete-20.00"));
    expect(cobrar).toHaveBeenCalledWith("o1", expect.objectContaining({ cajaId: "c1", metodoId: "ef", recibido: "20.00", consumidorFinal: true }));
    expect(await screen.findByTestId("vuelto")).toHaveAccessibleName("Vuelto $5.50");
    expect(screen.getByText(/INT-000001 · Mesa 4 libre · cajón abierto/)).toBeInTheDocument();
  });

  it("sobre el límite, consumidor final y los billetes quedan deshabilitados", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales: totales("60.00") });
    montar();
    await waitFor(() => expect(screen.getByTestId("total")).toHaveTextContent("$60.00"));
    expect(screen.getByTestId("consumidor-final")).toBeDisabled();
    expect(screen.getByTestId("billete-60.00")).toBeDisabled();
    expect(screen.getByText(/Supera \$50.00/)).toBeInTheDocument();
  });

  it("sin turno no se cobra y ofrece abrirlo", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales: totales("14.50") });
    montar(false);
    await waitFor(() => expect(screen.getByTestId("billete-14.50")).toBeDisabled());
    expect(screen.getByRole("button", { name: "Abrir turno" })).toBeInTheDocument();
    expect(screen.getByTestId("metodo-Tarjeta crédito")).toBeDisabled();
  });
});

describe("pago mixto", () => {
  const ef = metodos[0]!;
  const tc = metodos[1]!;
  const fila = (metodo: typeof ef, monto: string, extra: Partial<FilaPago> = {}): FilaPago => ({ clave: monto, metodo, monto, recibido: "", referencia: "", lote: "", ultimos4: "", ...extra });

  it("valida que sumen el total, un solo efectivo y los últimos 4", () => {
    expect(restante("35.40", [fila(ef, "20"), fila(tc, "10")])).toBe(540);
    expect(problemaPagos("35.40", [fila(ef, "20"), fila(tc, "15.40")])).toBeNull();
    expect(problemaPagos("35.40", [fila(ef, "20"), fila(tc, "10")])).toMatch(/Faltan \$5.40/);
    expect(problemaPagos("35.40", [fila(ef, "20"), fila(tc, "20")])).toMatch(/se pasan por \$4.60/);
    expect(problemaPagos("35.40", [fila(ef, "20"), fila(ef, "15.40")])).toMatch(/solo pago/);
    expect(problemaPagos("35.40", [fila(ef, "20", { recibido: "10" }), fila(tc, "15.40")])).toMatch(/no alcanza/);
    expect(problemaPagos("35.40", [fila(ef, "20"), fila(tc, "15.40", { ultimos4: "12" })])).toMatch(/4 números/);
    expect(aEnvio([fila(tc, "15.40", { recibido: "99", ultimos4: "4821" })])[0]).toEqual({ metodoId: "tc", monto: "15.40", recibido: "", referencia: "", lote: "", ultimos4: "4821" });
  });

  it("$20 en efectivo + el resto con tarjeta, con el voucher", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales: totales("35.40") });
    const cobrar = vi.spyOn(nodo, "cobrar").mockResolvedValue({
      documento: { ...docCF, id: "d", codigo: "INT-000002", mesa: "Mesa 4", totales: totales("35.40"), metodo: "Efectivo + Tarjeta crédito", recibido: "20.00", vuelto: "0.00", abreCajon: true,
        pagos: [{ metodoId: "ef", metodo: "Efectivo", tipo: "EFECTIVO", monto: "20.00" }, { metodoId: "tc", metodo: "Tarjeta crédito", tipo: "TARJETA_CREDITO", monto: "15.40", ultimos4: "4821" }] },
      impresoras: [],
    });
    montar();
    await waitFor(() => expect(screen.getByTestId("pago-mixto")).toBeEnabled());
    fireEvent.keyDown(window, { key: "m" });
    fireEvent.click(screen.getByTestId("agregar-Efectivo"));
    fireEvent.change(screen.getByLabelText("Monto de Efectivo"), { target: { value: "20" } });
    fireEvent.click(screen.getByTestId("agregar-Tarjeta crédito"));
    expect(screen.getByLabelText("Monto de Tarjeta crédito")).toHaveValue("15.40"); // se precarga lo que falta
    expect(screen.getByTestId("restante")).toHaveTextContent("Completo");
    fireEvent.change(screen.getByLabelText("Últimos 4"), { target: { value: "48-21" } });
    fireEvent.click(screen.getByTestId("cobrar-mixto"));
    expect(cobrar).toHaveBeenCalledWith("o1", expect.objectContaining({
      pagos: [
        { metodoId: "ef", monto: "20", recibido: "", referencia: "", lote: "", ultimos4: "" },
        { metodoId: "tc", monto: "15.40", recibido: "", referencia: "", lote: "", ultimos4: "4821" },
      ],
    }));
    expect(await screen.findByTestId("detalle-pagos")).toHaveTextContent("Efectivo $20.00 · Tarjeta crédito $15.40");
  });

  it("la tarjeta abre el detalle con el total listo para anotar el voucher", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales: totales("14.50") });
    montar();
    await waitFor(() => expect(screen.getByTestId("metodo-Tarjeta crédito")).toBeEnabled());
    fireEvent.click(screen.getByTestId("metodo-Tarjeta crédito"));
    expect(screen.getByRole("dialog", { name: "Detalle del pago" })).toBeInTheDocument();
    expect(screen.getByLabelText("Monto de Tarjeta crédito")).toHaveValue("14.50");
    expect(screen.getByTestId("cobrar-mixto")).toBeEnabled();
  });
});
