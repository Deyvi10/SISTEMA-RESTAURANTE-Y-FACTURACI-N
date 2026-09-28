import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { nodo } from "../api/nodo";
import { AtajosProvider } from "../components/atajos";
import { Cobro } from "../pages/Cobro";
import { compradorInicial, compradorParaCobro, type EstadoComprador } from "../pages/Comprador";
import type { Caja } from "../pages/Turno";

afterEach(() => vi.restoreAllMocks());

const con = (x: Partial<EstadoComprador>): EstadoComprador => ({ ...compradorInicial, modo: "ID", ...x });

describe("comprador para el cobro", () => {
  it("consumidor final no pide nada; con datos exige identificación válida, nombre y correo", () => {
    expect(compradorParaCobro(compradorInicial)).toEqual({ envio: null, problema: null });
    expect(compradorParaCobro(con({ identificacion: "1710034066" })).problema).toMatch(/verificador/);
    expect(compradorParaCobro(con({ identificacion: "1710034065" })).problema).toMatch(/Buscando/);
    expect(compradorParaCobro(con({ identificacion: "1710034065", buscado: "1710034065" })).problema).toMatch(/nombre/);
    expect(compradorParaCobro(con({ identificacion: "1710034065", buscado: "1710034065", razonSocial: "Ana", email: "ana@" })).problema).toMatch(/correo/);
    const ok = compradorParaCobro(con({ identificacion: " 1710034065 ", buscado: "1710034065", razonSocial: " Ana ", email: "ana@x.ec" }));
    expect(ok).toEqual({ envio: { tipoIdentificacion: "", identificacion: "1710034065", razonSocial: "Ana", email: "ana@x.ec", direccion: "", telefono: "", consentimiento: false }, problema: null });
    // El 9999999999999 es consumidor final aunque se escriba.
    expect(compradorParaCobro(con({ identificacion: "9999999999999" }))).toEqual({ envio: null, problema: null });
    // Pasaporte explícito, aunque sea solo numérico.
    expect(compradorParaCobro(con({ identificacion: "123456789", pasaporte: true, buscado: "123456789", razonSocial: "John", email: "j@x.com" })).envio?.tipoIdentificacion).toBe("06");
  });
});

describe("cobro con datos del comprador", () => {
  const metodos = [{ id: "ef", nombre: "Efectivo", tipo: "EFECTIVO", codigoSri: "01", abreCajon: true, pideReferencia: false, icono: "efectivo" }];
  const caja: Caja = {
    config: { cajas: [], metodos, consumidorFinalMaximo: "50.00", denominaciones: [] },
    cajaId: "c1",
    elegir: () => {},
    estado: { caja: { id: "c1", nombre: "Caja 1", estacionId: null }, jornada: null, turno: { id: "t1" } as never },
    error: null,
    recargar: () => {},
  };
  const orden = { id: "o1", mesaId: "m1", mesa: "Mesa 4", tipo: "MESA" as const, etiqueta: "", meseroNombre: "Carlos M.", numero: 7, estado: "PRECUENTA", lineas: [] };
  const totales = { subtotal: "52.17", iva: "7.83", propina: "0.00", total: "60.00" };
  const montar = () =>
    render(
      <AtajosProvider>
        <Cobro orden={{ ordenId: "o1", nombre: "Mesa 4" }} caja={caja} volver={() => {}} irATurno={() => {}} />
      </AtajosProvider>,
    );

  it("sobre el límite: C, cédula de un cliente registrado, autocompleta y cobra a su nombre", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales });
    const buscar = vi.spyOn(nodo, "buscarCliente").mockResolvedValue({
      valida: true, tipo: "05", origen: "NUBE",
      cliente: { id: "k", tipoIdentificacion: "05", identificacion: "1710034065", razonSocial: "María Pérez", email: "maria@x.ec", direccion: "Quito", telefono: "099" },
    });
    const cobrar = vi.spyOn(nodo, "cobrar").mockResolvedValue({
      documento: { id: "d", codigo: "INT-000009", mesa: "Mesa 4", totales, comprador: "María Pérez", compradorTipo: "05", compradorIdentificacion: "1710034065", clienteGuardado: true,
        metodo: "Efectivo", pagos: [{ metodoId: "ef", metodo: "Efectivo", tipo: "EFECTIVO", monto: "60.00" }], recibido: "60.00", vuelto: "0.00", abreCajon: true },
      impresoras: [],
    });
    montar();
    await waitFor(() => expect(screen.getByTestId("total")).toHaveTextContent("$60.00"));
    expect(screen.getByTestId("consumidor-final")).toBeDisabled();
    expect(screen.getByTestId("billete-60.00")).toBeDisabled();

    fireEvent.keyDown(window, { key: "c" });
    const campo = await screen.findByTestId("identificacion");
    fireEvent.change(campo, { target: { value: "1710034066" } });
    expect(screen.getByText(/verificador/)).toBeInTheDocument(); // validación en la caja, sin ir al nodo
    expect(buscar).not.toHaveBeenCalled();

    fireEvent.change(campo, { target: { value: "1710034065" } });
    expect(await screen.findByTestId("origen")).toHaveTextContent("desde la nube");
    expect(screen.getByTestId("razon-social")).toHaveValue("María Pérez");
    expect(screen.getByTestId("consentimiento")).toBeChecked(); // ya lo había autorizado
    expect(screen.getByTestId("billete-60.00")).toBeEnabled();

    fireEvent.keyDown(campo, { key: "Enter" }); // con Intro se factura (monto exacto)
    expect(cobrar).toHaveBeenCalledWith("o1", expect.objectContaining({
      consumidorFinal: false,
      comprador: { tipoIdentificacion: "", identificacion: "1710034065", razonSocial: "María Pérez", email: "maria@x.ec", direccion: "Quito", telefono: "099", consentimiento: true },
    }));
    expect(await screen.findByTestId("comprador-documento")).toHaveTextContent("María Pérez · 1710034065 · guardado para la próxima");
  });

  it("cliente nuevo: formulario mínimo y el consentimiento empieza sin marcar", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales: { ...totales, total: "20.00" } });
    vi.spyOn(nodo, "buscarCliente").mockResolvedValue({ valida: true, tipo: "04", cliente: null });
    montar();
    await waitFor(() => expect(screen.getByTestId("billete-20.00")).toBeEnabled());
    fireEvent.click(screen.getByTestId("con-datos"));
    fireEvent.change(screen.getByTestId("identificacion"), { target: { value: "1790011674001" } });
    await screen.findByTestId("razon-social");
    expect(screen.getByTestId("consentimiento")).not.toBeChecked();
    expect(screen.getByTestId("billete-20.00")).toBeDisabled();
    expect(screen.getByText(/nombre o la razón social/)).toBeInTheDocument();
    fireEvent.change(screen.getByTestId("razon-social"), { target: { value: "Empresa S.A." } });
    fireEvent.change(screen.getByTestId("email"), { target: { value: "compras@empresa.ec" } });
    expect(screen.getByTestId("billete-20.00")).toBeEnabled();
  });
});
