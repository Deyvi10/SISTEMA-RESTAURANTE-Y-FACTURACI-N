import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, type EstadoCaja, type Mesa, nodo } from "../api/nodo";
import { AtajosProvider } from "../components/atajos";
import { opcionesBillete, vueltoCentavos } from "../lib/billetes";
import { Cobro, porcentaje } from "../pages/Cobro";
import { vistaPrevia } from "../pages/Descuento";
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

describe("cuenta dividida", () => {
  const lineas = [
    { id: "l1", producto: "Arroz marinero", cantidad: "1", modificadores: [], estado: "ENVIADA", total: "14.00" },
    { id: "l2", producto: "Cerveza", cantidad: "2", modificadores: [], estado: "ENVIADA", total: "6.00" },
  ];
  const cuenta = (id: string, numero: number, estado: "ABIERTA" | "PAGADA", asig: [string, number][], lin: Record<string, string>, total: string) => ({
    id, numero, estado, asignaciones: asig.map(([lineaId, peso]) => ({ lineaId, peso })), lineas: lin, subtotal: "14.78", iva: "2.22", propina: "1.48", total,
  });

  it("cobra la cuenta elegida con sus totales y ofrece la siguiente", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden: { ...orden, lineas }, totales: totales("22.00") });
    vi.spyOn(nodo, "cuentas").mockResolvedValue({
      cuentas: [cuenta("c1", 1, "ABIERTA", [["l1", 1], ["l2", 1]], { l1: "14.00", l2: "3.00" }, "18.48"), cuenta("c2", 2, "ABIERTA", [["l2", 1]], { l2: "3.00" }, "3.52")],
      sinCuenta: [],
      total: "22.00",
    });
    const cobrar = vi.spyOn(nodo, "cobrar").mockResolvedValue({
      documento: { ...docCF, id: "d", cuenta: 1, codigo: "INT-000002", mesa: "Mesa 4", totales: totales("18.48"), metodo: "Efectivo", pagos: [], recibido: "18.48", vuelto: "0.00", abreCajon: true },
      impresoras: ["Caja"],
      cerrada: false,
    });
    montar();
    await waitFor(() => expect(screen.getByTestId("total")).toHaveTextContent("$18.48"));
    expect(screen.getByText("Total de la cuenta 1")).toBeInTheDocument();
    expect(screen.getByText("Compartido · 1/2")).toBeInTheDocument();
    expect(screen.getByText("$14.78")).toBeInTheDocument(); // subtotal de la cuenta, no de la orden
    expect(screen.getByTestId("elegir-cuenta-2")).toHaveTextContent("Cuenta 2 $3.52");
    fireEvent.click(screen.getByTestId("billete-18.48"));
    expect(cobrar).toHaveBeenCalledWith("o1", expect.objectContaining({ cuentaId: "c1", recibido: "18.48" }));
    expect(await screen.findByTestId("siguiente-cuenta")).toBeInTheDocument();
    expect(screen.getByText(/cuenta 1 de Mesa 4 cobrada/)).toBeInTheDocument();
  });

  it("con la división abierta, las teclas del cobro no cobran", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden: { ...orden, lineas }, totales: totales("22.00") });
    vi.spyOn(nodo, "cuentas").mockResolvedValue({ cuentas: [], sinCuenta: [], total: "22.00" });
    const cobrar = vi.spyOn(nodo, "cobrar");
    montar();
    await waitFor(() => expect(screen.getByTestId("total")).toHaveTextContent("$22.00"));
    act(() => void fireEvent.keyDown(window, { key: "v" }));
    await screen.findByTestId("division");
    act(() => void fireEvent.keyDown(window, { key: "1" }));
    expect(cobrar).not.toHaveBeenCalled();
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

describe("servicio (propina legal)", () => {
  it("muestra el porcentaje sin ceros de sobra", () => {
    expect(["10.00", "12.50", "10", "8.25"].map(porcentaje)).toEqual(["10", "12.5", "10", "8.25"]);
  });

  it("el cajero lo quita con un motivo y los billetes se recalculan", async () => {
    const conServicio = { subtotal: "16.09", iva: "2.41", propina: "1.61", total: "20.11", propinaActiva: true, propinaPorcentaje: "10", propinaRetirada: false };
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales: conServicio });
    const cambiar = vi.spyOn(nodo, "propina").mockResolvedValue({ ...conServicio, propina: "0.00", total: "18.50", propinaRetirada: true });
    montar();
    await waitFor(() => expect(screen.getByTestId("total")).toHaveTextContent("$20.11"));
    fireEvent.keyDown(window, { key: "s" });
    fireEvent.change(screen.getByTestId("motivo-servicio"), { target: { value: "No quiere pagar servicio" } });
    fireEvent.click(screen.getByTestId("quitar-servicio"));
    expect(cambiar).toHaveBeenCalledWith("o1", true, "No quiere pagar servicio");
    await waitFor(() => expect(screen.getByTestId("total")).toHaveTextContent("$18.50"));
    expect(screen.getByTestId("billete-18.50")).toBeInTheDocument();
    expect(screen.getByTestId("servicio")).toHaveTextContent("Reponer");
  });
});

describe("descuentos en el cobro", () => {
  it("vista previa con la misma regla que el nodo", () => {
    expect(vistaPrevia("PORCENTAJE", "10", "18.50")).toBe(185);
    expect(vistaPrevia("PORCENTAJE", "12.5", "10.00")).toBe(125);
    expect(vistaPrevia("PORCENTAJE", "15", "18.50")).toBe(278); // 2.775 → 2.78
    expect(vistaPrevia("MONTO", "30", "18.50")).toBe(1850); // nunca más que el importe
    expect(vistaPrevia("CORTESIA", "", "6.00")).toBe(600);
  });

  it("aplica un 10 % a la cuenta con motivo, lo muestra y lo quita", async () => {
    const base = { subtotal: "16.09", iva: "2.41", propina: "0.00", total: "18.50", descuento: "0.00", descuentos: [] };
    const con = { ...base, subtotal: "14.48", iva: "2.17", total: "16.65", descuento: "1.85",
      descuentos: [{ id: "d1", lineaId: null, tipo: "PORCENTAJE" as const, valor: "10", cortesia: false, motivo: "Cliente frecuente", usuarioNombre: "Luis P.", monto: "1.85" }] };
    const lineas = [{ id: "l1", producto: "Cerveza", cantidad: "2", modificadores: [], estado: "ENVIADA", total: "6.00" }, { id: "l2", producto: "Ceviche", cantidad: "1", modificadores: [], estado: "ENVIADA", total: "12.50" }];
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden: { ...orden, lineas }, totales: base });
    const descontar = vi.spyOn(nodo, "descontar").mockResolvedValue(con);
    const quitar = vi.spyOn(nodo, "quitarDescuento").mockResolvedValue(base);
    render(
      <AtajosProvider>
        <Cobro orden={{ ordenId: "o1", nombre: "Mesa 4" }} caja={{ ...caja(true), config: { ...caja(true).config!, motivos: [{ id: "m1", nombre: "Cliente frecuente", tipo: "DESCUENTO" }, { id: "m2", nombre: "Invitación", tipo: "CORTESIA" }] } }} volver={() => {}} irATurno={() => {}} />
      </AtajosProvider>,
    );
    await waitFor(() => expect(screen.getByTestId("total")).toHaveTextContent("$18.50"));
    fireEvent.keyDown(window, { key: "d" });
    await act(async () => {
      for (const k of ["1", "0"]) fireEvent.keyDown(window, { key: k });
    });
    expect(screen.getByTestId("previa")).toHaveTextContent("Descuenta $1.85 de $18.50");
    expect(screen.queryByRole("button", { name: "Invitación" })).not.toBeInTheDocument(); // es de cortesía
    fireEvent.click(screen.getByRole("button", { name: "Cliente frecuente" }));
    fireEvent.click(screen.getByTestId("aplicar-descuento"));
    expect(descontar).toHaveBeenCalledWith("o1", { lineaId: null, tipo: "PORCENTAJE", valor: "10", cortesia: false, motivoId: "m1", autorizacion: "" });
    await waitFor(() => expect(screen.getByTestId("total")).toHaveTextContent("$16.65"));
    expect(screen.getByTestId("total-descuento")).toHaveTextContent("−$1.85");
    expect(screen.getByTestId("chip-descuento")).toHaveTextContent("10 % −$1.85 · Cliente frecuente (toda la cuenta)");
    fireEvent.click(screen.getByRole("button", { name: /Quitar 10 %/ }));
    expect(quitar).toHaveBeenCalledWith("o1", "d1");
    await waitFor(() => expect(screen.getByTestId("total")).toHaveTextContent("$18.50"));
  });

  it("si pasa el límite pide un supervisor", async () => {
    const base = { subtotal: "16.09", iva: "2.41", propina: "0.00", total: "18.50" };
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales: base });
    vi.spyOn(nodo, "descontar").mockRejectedValue(new ApiError(403, "REQUIERE_SUPERVISOR", "Necesita autorización."));
    vi.spyOn(nodo, "personal").mockResolvedValue([{ id: "s1", nombre: "Sofía A.", rol: "ADMIN", avatarUrl: null, permisos: [] }]);
    render(
      <AtajosProvider>
        <Cobro orden={{ ordenId: "o1", nombre: "Mesa 4" }} caja={{ ...caja(true), config: { ...caja(true).config!, motivos: [{ id: "m1", nombre: "Cliente frecuente", tipo: "DESCUENTO" }] } }} volver={() => {}} irATurno={() => {}} />
      </AtajosProvider>,
    );
    await waitFor(() => expect(screen.getByTestId("descuento")).toBeInTheDocument());
    fireEvent.click(screen.getByTestId("descuento"));
    await act(async () => {
      for (const k of ["5", "0"]) fireEvent.keyDown(window, { key: k });
    });
    fireEvent.click(screen.getByRole("button", { name: "Cliente frecuente" }));
    await act(async () => {
      fireEvent.click(screen.getByTestId("aplicar-descuento"));
    });
    expect(await screen.findByTestId("supervisor-Sofía A.")).toBeInTheDocument();
  });
});

describe("cuenta invitada", () => {
  it("con total en cero por cortesía se cierra sin cobro", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales: { subtotal: "0.00", iva: "0.00", propina: "0.00", total: "0.00", descuento: "12.50" } });
    const cobrar = vi.spyOn(nodo, "cobrar").mockResolvedValue({
      documento: { ...docCF, id: "d", codigo: "INT-000010", mesa: "Mesa 4", totales: { subtotal: "0.00", iva: "0.00", propina: "0.00", total: "0.00" }, metodo: "Cortesía", pagos: [], recibido: "0.00", vuelto: "0.00", abreCajon: false },
      impresoras: [],
    });
    montar();
    const boton = await screen.findByTestId("cerrar-cortesia");
    expect(screen.queryByTestId("billete-0.00")).not.toBeInTheDocument();
    fireEvent.click(boton);
    expect(cobrar).toHaveBeenCalledWith("o1", expect.objectContaining({ metodoId: "ef", recibido: "" }));
    expect(await screen.findByTestId("cobro-listo")).toHaveTextContent("Cortesía · INT-000010");
  });
});

describe("factura electrónica", () => {
  it("al cobrar con la facturación activa muestra la factura emitida", async () => {
    vi.spyOn(nodo, "orden").mockResolvedValue({ orden, totales: totales("14.50") });
    vi.spyOn(nodo, "cobrar").mockResolvedValue({
      documento: { ...docCF, id: "d", tipo: "FACTURA", codigo: "001-002-000000067", claveAcceso: "2909202601171003406500110010020000000671234567818", ambiente: 1,
        mesa: "Mesa 4", totales: totales("14.50"), metodo: "Efectivo", pagos: [], recibido: "14.50", vuelto: "0.00", abreCajon: true },
      impresoras: ["Caja"],
    });
    montar();
    fireEvent.click(await screen.findByTestId("billete-14.50"));
    expect(await screen.findByTestId("factura-emitida")).toHaveTextContent("Factura 001-002-000000067 emitida en ambiente de pruebas");
  });
});
