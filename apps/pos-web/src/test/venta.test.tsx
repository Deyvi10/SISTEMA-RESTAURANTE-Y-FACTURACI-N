import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type Catalogo, nodo, type OrdenSinMesa, type Producto } from "../api/nodo";
import { AtajosProvider } from "../components/atajos";
import { filtrarSinMesa } from "../pages/Mesas";
import { agregar, precioLinea, validarMods, Venta } from "../pages/Venta";

afterEach(() => vi.restoreAllMocks());

const prod = (nombre: string, precio: string, extra: Partial<Producto> = {}): Producto => ({
  id: nombre, categoriaId: "c1", nombre, alias: "", precio, foto: null, grupos: [], vendidos: 0, orden: 0, ...extra,
});
const termino = { id: "g1", nombre: "Término", obligatorio: true, min: 1, max: 1, modificadores: [{ id: "m1", nombre: "Medio", precioAdicional: "0" }, { id: "m2", nombre: "Bien cocido", precioAdicional: "0" }] };
const extras = { id: "g2", nombre: "Extras", obligatorio: false, min: 0, max: 2, modificadores: [{ id: "x1", nombre: "Queso", precioAdicional: "1.00" }, { id: "x2", nombre: "Tocino", precioAdicional: "1.50" }] };
const catalogo: Catalogo = {
  version: "1",
  categorias: [{ id: "c1", nombre: "Platos", icono: "", color: "", orden: 1 }],
  productos: [prod("Ceviche Mixto", "12.500000", { alias: "CM", vendidos: 9 }), prod("Parrillada", "12.00", { grupos: ["g1", "g2"] }), prod("Cerveza", "3.00")],
  grupos: [termino, extras],
};

describe("carrito", () => {
  it("suma el mismo plato con los mismos modificadores y calcula en centavos", () => {
    const p = catalogo.productos[1]!;
    let c = agregar([], p, [termino.modificadores[0]!, extras.modificadores[1]!]);
    c = agregar(c, p, [extras.modificadores[1]!, termino.modificadores[0]!]); // otro orden, mismo plato
    c = agregar(c, p, [termino.modificadores[1]!]);
    expect(c.map((i) => i.cantidad)).toEqual([2, 1]);
    expect(precioLinea(c[0]!)).toBe(2700); // (12.00 + 1.50) × 2
  });

  it("valida mínimos y máximos de los grupos", () => {
    expect(validarMods([termino, extras], new Set())).toMatch(/término/);
    expect(validarMods([termino, extras], new Set(["m1"]))).toBeNull();
    expect(validarMods([termino, extras], new Set(["m1", "m2"]))).toMatch(/hasta 1/);
  });

  it("filtra las órdenes sin mesa por número o nombre", () => {
    const o = (numero: number, nombre: string) => ({ numero, nombre }) as OrdenSinMesa;
    const lista = [o(12, "Llevar #12 · Ana"), o(3, "Barra #3")];
    expect(filtrarSinMesa(lista, "#12").map((x) => x.numero)).toEqual([12]);
    expect(filtrarSinMesa(lista, "ana").map((x) => x.numero)).toEqual([12]);
    expect(filtrarSinMesa(lista, "")).toHaveLength(2);
  });
});

describe("venta en mostrador", () => {
  it("se arma solo con el teclado: buscar, Intro, modificadores y F2", async () => {
    vi.spyOn(nodo, "catalogo").mockResolvedValue(catalogo);
    const enviar = vi.spyOn(nodo, "enviarOrden").mockResolvedValue({ orden: { id: "o9", mesa: "Llevar #4 · Ana" } as never, comandaNumero: 4 });
    const cobrar = vi.fn();
    render(
      <AtajosProvider>
        <Venta volver={() => {}} cobrar={cobrar} />
      </AtajosProvider>,
    );
    const buscar = await screen.findByTestId("buscar-producto");
    await waitFor(() => expect(screen.getByTestId("producto-Parrillada")).toBeInTheDocument());

    // Alias: «cm» + Intro agrega el ceviche.
    fireEvent.change(buscar, { target: { value: "cm" } });
    fireEvent.keyDown(buscar, { key: "Enter" });
    // «parr» + Intro abre los modificadores; sin elegir el término no deja agregar.
    fireEvent.change(buscar, { target: { value: "parr" } });
    fireEvent.keyDown(buscar, { key: "Enter" });
    expect(screen.getByRole("dialog", { name: "Parrillada" })).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("agregar-con-mods"));
    expect(screen.getByRole("alert")).toHaveTextContent("Elige término");
    fireEvent.click(screen.getByRole("radio", { name: /Medio/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Queso/ }));
    fireEvent.click(screen.getByTestId("agregar-con-mods"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByTestId("total-venta")).toHaveTextContent("$25.50"); // 12.50 + 12.00 + 1.00

    fireEvent.change(screen.getByTestId("etiqueta"), { target: { value: "Ana" } });
    await act(async () => {
      fireEvent.keyDown(window, { key: "F2" });
    });
    expect(enviar).toHaveBeenCalledWith(
      expect.objectContaining({
        tipo: "LLEVAR",
        etiqueta: "Ana",
        lineas: [
          expect.objectContaining({ productoId: "Ceviche Mixto", cantidad: "1", modificadores: [] }),
          expect.objectContaining({ productoId: "Parrillada", cantidad: "1", modificadores: ["m1", "x1"] }),
        ],
      }),
    );
    expect(cobrar).toHaveBeenCalledWith("o9", "Llevar #4 · Ana");
  });
});
