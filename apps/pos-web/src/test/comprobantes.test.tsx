import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, type ComprobanteCaja, type DetalleComprobanteCaja, nodo } from "../api/nodo";
import { AtajosProvider } from "../components/atajos";
import { Comprobantes, estadoComprobante } from "../pages/Comprobantes";
import type { Caja } from "../pages/Turno";

afterEach(() => vi.restoreAllMocks());

const metodos = [
  { id: "ef", nombre: "Efectivo", tipo: "EFECTIVO", codigoSri: "01", abreCajon: true, pideReferencia: false, icono: "efectivo" },
  { id: "tc", nombre: "Tarjeta crédito", tipo: "TARJETA_CREDITO", codigoSri: "19", abreCajon: false, pideReferencia: true, icono: "tarjeta" },
];
const caja: Caja = {
  config: { cajas: [], metodos, consumidorFinalMaximo: "50.00", denominaciones: [] },
  cajaId: "c1",
  elegir: () => {},
  estado: { caja: { id: "c1", nombre: "Caja 1", estacionId: null }, jornada: null, turno: { id: "t1" } as never },
  error: null,
  recargar: () => {},
};
const factura: ComprobanteCaja = {
  id: "f1", tipo: "01", numero: "001-001-000000002", claveAcceso: "3".repeat(49), emitido: "2026-09-30T21:57:00-05:00", total: "18.47",
  comprador: "María Pérez", identificacion: "1710034065", ambiente: 1, estado: "AUTORIZADO", saldo: "17.00",
};
const detalle: DetalleComprobanteCaja = {
  ...factura, consumidorFinal: false, notasCredito: [],
  lineas: [
    { indice: 0, descripcion: "Cerveza", cantidad: "2", disponible: "2", total: "6.00" },
    { indice: 1, descripcion: "Arroz marinero", cantidad: "1", disponible: "1", total: "11.00" },
  ],
};

function montar() {
  render(
    <AtajosProvider>
      <Comprobantes caja={caja} usuarioId="u1" />
    </AtajosProvider>,
  );
}

describe("comprobantes de la caja", () => {
  it("estado ante el SRI en palabras", () => {
    expect(estadoComprobante({ estado: "EMITIDO" }).texto).toBe("Emitido en el local");
    expect(estadoComprobante({ estado: "ANULADO" })).toEqual({ texto: "Anulado por NC", tono: "neutral" });
    expect(estadoComprobante({ estado: "AUTORIZADO" }).tono).toBe("success");
  });

  it("nota de crédito parcial sin devolver dinero: no manda devolución", async () => {
    vi.spyOn(nodo, "comprobantes").mockResolvedValue([factura]);
    vi.spyOn(nodo, "comprobante").mockResolvedValue(detalle);
    const emitir = vi.spyOn(nodo, "notaCredito").mockResolvedValue({ ...factura, id: "n1", tipo: "04", numero: "001-001-000000001", valor: "3.00",
      revierteTodo: false, impresoras: ["Caja"] });
    montar();
    fireEvent.click(await screen.findByTestId("factura-001-001-000000002"));
    fireEvent.click(await screen.findByTestId("abrir-nc"));
    fireEvent.click(screen.getByTestId("nc-parcial"));
    fireEvent.change(screen.getByTestId("nc-cantidad-0"), { target: { value: "1" } });
    fireEvent.change(screen.getByTestId("nc-motivo"), { target: { value: "Cerveza caliente" } });
    // Propone devolver en efectivo; el cajero elige no devolver dinero y se respeta.
    await waitFor(() => expect(screen.getByTestId("nc-devolucion")).toHaveValue("ef"));
    fireEvent.change(screen.getByTestId("nc-devolucion"), { target: { value: "" } });
    expect(screen.getByTestId("nc-devolucion")).toHaveValue("");
    fireEvent.click(screen.getByTestId("nc-emitir"));
    await screen.findByTestId("nc-hecha");
    const body = emitir.mock.calls[0]![1];
    expect(body).toEqual(expect.objectContaining({ cajaId: "c1", todo: false, lineas: [{ indice: 0, cantidad: "1" }], motivo: "Cerveza caliente" }));
    expect(body).not.toHaveProperty("devolucion");
    expect(body).not.toHaveProperty("comprador");
  });

  it("factura a consumidor final: exige identificar al cliente; sin permiso pide supervisor", async () => {
    vi.spyOn(nodo, "comprobantes").mockResolvedValue([factura]);
    vi.spyOn(nodo, "comprobante").mockResolvedValue({ ...detalle, consumidorFinal: true, comprador: "CONSUMIDOR FINAL", identificacion: "9999999999999" });
    vi.spyOn(nodo, "personal").mockResolvedValue([]);
    const emitir = vi.spyOn(nodo, "notaCredito").mockRejectedValue(new ApiError(403, "REQUIERE_SUPERVISOR", "Requiere supervisor"));
    vi.spyOn(nodo, "buscarCliente").mockResolvedValue({ valida: true, tipo: "05", cliente: null });
    montar();
    fireEvent.click(await screen.findByTestId("factura-001-001-000000002"));
    fireEvent.click(await screen.findByTestId("abrir-nc"));
    fireEvent.change(screen.getByTestId("nc-motivo"), { target: { value: "Mesa cancelada" } });
    expect(screen.getByTestId("nc-emitir")).toBeDisabled(); // falta el cliente
    expect(screen.getByText(/exige identificar al cliente/)).toBeInTheDocument();
    fireEvent.change(screen.getByTestId("identificacion"), { target: { value: "1710034065" } });
    fireEvent.change(await screen.findByTestId("razon-social"), { target: { value: "María Pérez" } });
    fireEvent.change(screen.getByTestId("email"), { target: { value: "maria@x.ec" } });
    await waitFor(() => expect(screen.getByTestId("nc-emitir")).toBeEnabled());
    fireEvent.click(screen.getByTestId("nc-emitir"));
    await waitFor(() => expect(emitir).toHaveBeenCalled());
    expect(emitir.mock.calls[0]![1].comprador).toEqual(expect.objectContaining({ identificacion: "1710034065", razonSocial: "María Pérez" }));
    expect(await screen.findByText(/supervisor/i)).toBeInTheDocument();
  });
});

describe("reimpresión y fecha (F5-18)", () => {
  it("filtra por fecha y reimprime el RIDE", async () => {
    const lista = vi.spyOn(nodo, "comprobantes").mockResolvedValue([factura]);
    vi.spyOn(nodo, "comprobante").mockResolvedValue({ ...detalle, fechaAutorizacion: "2026-09-30T22:57:41-05:00" });
    const reimp = vi.spyOn(nodo, "reimprimirComprobante").mockResolvedValue({ impresoras: ["Caja"] });
    montar();
    fireEvent.change(await screen.findByTestId("fecha-comprobante"), { target: { value: "2026-09-30" } });
    await waitFor(() => expect(lista).toHaveBeenLastCalledWith("", "2026-09-30"));
    fireEvent.click(await screen.findByTestId("factura-001-001-000000002"));
    expect(await screen.findByTestId("autorizado-el")).toHaveTextContent("Autorizado el");
    fireEvent.click(screen.getByTestId("reimprimir"));
    await waitFor(() => expect(reimp).toHaveBeenCalledWith("f1", "c1"));
    expect(await screen.findByText("Se reimprimió en Caja.")).toBeInTheDocument();
  });
});
