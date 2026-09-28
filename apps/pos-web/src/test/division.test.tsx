import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type Cuenta, type LineaOrden, nodo } from "../api/nodo";
import { AtajosProvider } from "../components/atajos";
import { compartir, consumo, Division, mover, nuevaCuenta, partesIguales, pendientes, sinCuenta } from "../pages/Division";

afterEach(() => vi.restoreAllMocks());

const linea = (id: string, producto: string, total: string, estado = "ENVIADA"): LineaOrden => ({ id, producto, cantidad: "1", modificadores: [], estado, total });
const lineas = [linea("l1", "Ceviche", "12.50"), linea("l2", "Pizza", "10.00"), linea("l3", "Cola", "1.50"), linea("l4", "Anulado", "9.99", "ANULADA")];
const pagada = (lin: Record<string, string>): Cuenta => ({ id: "p", numero: 1, estado: "PAGADA", asignaciones: [], lineas: lin, subtotal: "0", iva: "0", propina: "0", total: "0" });

describe("reglas de la división", () => {
  it("lo pendiente descuenta lo pagado y deja fuera lo anulado", () => {
    expect(pendientes(lineas, undefined, [])).toEqual({ l1: 1250, l2: 1000, l3: 150 });
    // con descuentos manda el importe final de la línea
    expect(pendientes(lineas, { l1: "10.00", l2: "10.00", l3: "1.50" }, [pagada({ l2: "3.33" })])).toEqual({ l1: 1000, l2: 667, l3: 150 });
  });

  it("partes iguales deja el residuo en la última cuenta y suma exacto", () => {
    const pend = pendientes(lineas, undefined, []);
    const cuentas = partesIguales(3, pend);
    const c = consumo(pend, cuentas);
    expect(c.reduce((a, b) => a + b, 0)).toBe(2400);
    expect(c).toEqual([800, 800, 800]);
    const c2 = consumo({ l1: 1250 }, partesIguales(3, { l1: 1250 }));
    expect(c2).toEqual([416, 417, 417]);
  });

  it("mover pasa un plato a una sola cuenta y compartir lo reparte", () => {
    let cuentas = [nuevaCuenta(), nuevaCuenta(), nuevaCuenta()];
    cuentas = mover(cuentas, "l1", 0);
    cuentas = mover(cuentas, "l1", 2);
    expect(cuentas.map((c) => c.pesos.l1 ?? 0)).toEqual([0, 0, 1]);
    cuentas = compartir(cuentas, "l2", [0, 1, 2]);
    expect(consumo({ l1: 1250, l2: 1000 }, cuentas)).toEqual([333, 333, 1584]);
    expect(sinCuenta({ l1: 1250, l2: 1000, l3: 150, l5: 0 }, cuentas)).toEqual(["l3"]);
  });
});

describe("pantalla de división", () => {
  const totales = { subtotal: "20.87", iva: "3.13", propina: "0.00", total: "24.00" };
  function montar(listo = () => {}) {
    return render(
      <AtajosProvider>
        <Division ordenId="o1" lineas={lineas} totales={totales} volver={() => {}} listo={listo} />
      </AtajosProvider>,
    );
  }

  it("no deja confirmar con platos sin cuenta y envía los pesos al nodo", async () => {
    vi.spyOn(nodo, "cuentas").mockResolvedValue({ cuentas: [], sinCuenta: [], total: "24.00" });
    const dividir = vi.spyOn(nodo, "dividir").mockResolvedValue({ cuentas: [], sinCuenta: [], total: "24.00" });
    const listo = vi.fn();
    montar(listo);
    await screen.findByTestId("cuenta-2");
    expect(screen.getByTestId("confirmar-division")).toBeDisabled();
    expect(screen.getByText("Faltan 3 platos por asignar.")).toBeInTheDocument();

    // Cuenta 1: ceviche; cuenta 2 (tecla 2): pizza y cola.
    fireEvent.click(screen.getByText("Ceviche"));
    act(() => void fireEvent.keyDown(window, { key: "2" }));
    fireEvent.click(screen.getByText("Pizza"));
    fireEvent.click(screen.getByText("Cola"));
    expect(screen.getByTestId("cuenta-1")).toHaveTextContent("$12.50");
    expect(screen.getByTestId("cuenta-2")).toHaveTextContent("$11.50");

    act(() => void fireEvent.keyDown(window, { key: "F2" }));
    await waitFor(() => expect(listo).toHaveBeenCalled());
    expect(dividir).toHaveBeenCalledWith("o1", [{ asignaciones: [{ lineaId: "l1", peso: 1 }] }, { asignaciones: [{ lineaId: "l2", peso: 1 }, { lineaId: "l3", peso: 1 }] }]);
  });

  it("partes iguales entre 3 muestra el centavo en la última", async () => {
    vi.spyOn(nodo, "cuentas").mockResolvedValue({ cuentas: [], sinCuenta: [], total: "24.00" });
    render(
      <AtajosProvider>
        <Division ordenId="o1" lineas={[linea("l1", "Ceviche", "12.50")]} totales={totales} volver={() => {}} listo={() => {}} />
      </AtajosProvider>,
    );
    await screen.findByTestId("iguales-3");
    fireEvent.click(screen.getByTestId("iguales-3"));
    expect(screen.getByTestId("cuenta-1")).toHaveTextContent("$4.16");
    expect(screen.getByTestId("cuenta-3")).toHaveTextContent("$4.17");
    expect(screen.getByTestId("confirmar-division")).toBeEnabled();
  });

  it("las cuentas pagadas quedan fijas y numeradas primero", async () => {
    vi.spyOn(nodo, "cuentas").mockResolvedValue({
      cuentas: [
        { ...pagada({ l1: "12.50" }), total: "12.50", documento: "INT-000009" },
        { id: "a", numero: 2, estado: "ABIERTA", asignaciones: [{ lineaId: "l2", peso: 1 }, { lineaId: "l3", peso: 1 }], lineas: {}, subtotal: "0", iva: "0", propina: "0", total: "11.50" },
      ],
      sinCuenta: [],
      total: "24.00",
    });
    montar();
    await screen.findByText(/pagada \(INT-000009\)/);
    expect(screen.queryByText("Ceviche")).not.toBeInTheDocument(); // ya pagado: no se reasigna
    expect(screen.getByTestId("cuenta-2")).toHaveTextContent("$11.50");
    expect(screen.queryByTestId("deshacer-division")).not.toBeInTheDocument();
  });
});
