import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { uuidv7 } from "../api/identidad";
import { esPCDelNodo, type Mesa } from "../api/nodo";
import { esperaPara, urlTiempoReal } from "../api/tiempoReal";
import { AtajosProvider, useAtajo } from "../components/atajos";
import { colorDe } from "../components/Indicador";
import { TecladoPIN } from "../components/TecladoPIN";
import { filtrarMesas, minutosDesde, ordenarMesas } from "../pages/Mesas";

const mesa = (nombre: string, extra: Partial<Mesa> = {}): Mesa => ({
  id: nombre,
  zonaId: "z",
  nombre,
  capacidad: 4,
  estado: "LIBRE",
  ordenId: null,
  numeroOrden: null,
  meseroNombre: null,
  abiertaAt: null,
  precuentaAt: null,
  comensales: null,
  platos: 0,
  total: "0.00",
  bloqueo: null,
  ...extra,
});

describe("caja", () => {
  it("busca por número de orden exacto, mesa o mesero", () => {
    const mesas = [mesa("Mesa 1"), mesa("Mesa 12", { numeroOrden: 7, meseroNombre: "Ana R." }), mesa("Barra", { numeroOrden: 17 })];
    expect(filtrarMesas(mesas, "7").map((m) => m.nombre)).toEqual(["Mesa 12"]);
    expect(filtrarMesas(mesas, "#17").map((m) => m.nombre)).toEqual(["Barra"]);
    expect(filtrarMesas(mesas, "mesa 1").map((m) => m.nombre)).toEqual(["Mesa 1", "Mesa 12"]);
    expect(filtrarMesas(mesas, "ana").map((m) => m.nombre)).toEqual(["Mesa 12"]);
    expect(filtrarMesas(mesas, "  ")).toHaveLength(3);
  });

  it("ordena por zona y con números naturales", () => {
    const mesas = [mesa("Mesa 10"), mesa("Mesa 2"), mesa("T1", { zonaId: "terraza" }), mesa("Mesa 1")];
    const zonas = [{ id: "terraza", orden: 2 }, { id: "z", orden: 1 }];
    expect(ordenarMesas(mesas, zonas).map((m) => m.nombre)).toEqual(["Mesa 1", "Mesa 2", "Mesa 10", "T1"]);
  });

  it("semáforo: rojo sin nodo, amarillo sin nube, verde en línea", () => {
    expect(colorDe(false, "EN_LINEA")).toBe("rojo");
    expect(colorDe(true, "SIN_INTERNET")).toBe("amarillo");
    expect(colorDe(true, null)).toBe("amarillo");
    expect(colorDe(true, "EN_LINEA")).toBe("verde");
  });

  it("reconexión con espera exponencial y tope de 30 s", () => {
    expect([1, 2, 3, 20].map(esperaPara)).toEqual([500, 1000, 2000, 30000]);
  });

  it("URL del WebSocket con credenciales en la consulta", () => {
    const u = urlTiempoReal({ dispositivo: "d1", usuario: null }, { protocol: "https:", host: "nodo.local:7443" } as Location);
    expect(u).toBe("wss://nodo.local:7443/v1/ws?dispositivo=d1&tipo=POS");
  });

  it("la PC del nodo se reconoce por loopback", () => {
    expect(esPCDelNodo("localhost")).toBe(true);
    expect(esPCDelNodo("127.0.0.1")).toBe(true);
    expect(esPCDelNodo("192.168.1.10")).toBe(false);
  });

  it("UUID v7 ordenable por tiempo y con versión 7", () => {
    const a = uuidv7(1_700_000_000_000);
    const b = uuidv7(1_700_000_000_001);
    expect(a).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
    expect(a < b).toBe(true);
  });

  it("minutos desde la apertura", () => {
    expect(minutosDesde("2026-09-25T12:00:00Z", Date.parse("2026-09-25T12:18:30Z"))).toBe(18);
    expect(minutosDesde(null)).toBeNull();
  });
});

function ConAtajo({ fn }: { fn: () => void }) {
  useAtajo("F2", "Cobrar", fn, "Cobro");
  return <input aria-label="campo" />;
}

describe("atajos de teclado", () => {
  it("responde a su tecla, «?» muestra la ayuda y en un campo solo pasan las de función", () => {
    const fn = vi.fn();
    render(
      <AtajosProvider>
        <ConAtajo fn={fn} />
      </AtajosProvider>,
    );
    fireEvent.keyDown(window, { key: "F2" });
    expect(fn).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(window, { key: "?" });
    expect(screen.getByRole("dialog", { name: "Atajos de teclado" })).toBeInTheDocument();
    expect(screen.getByText("Cobrar")).toBeInTheDocument();
    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    // Escribiendo en un campo: «?» es texto, F2 sigue siendo atajo.
    const campo = screen.getByLabelText("campo");
    fireEvent.keyDown(campo, { key: "?" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    fireEvent.keyDown(campo, { key: "F2" });
    expect(fn).toHaveBeenCalledTimes(2);
  });
});

describe("teclado de PIN", () => {
  it("se escribe con el teclado numérico, muestra el error y se limpia", async () => {
    const pins: string[] = [];
    render(
      <TecladoPIN
        alCompletar={async (p) => {
          pins.push(p);
          return p === "8899" ? null : "PIN incorrecto. Te quedan 4 intentos.";
        }}
      />,
    );
    await act(async () => {
      for (const k of ["1", "2", "3", "4"]) fireEvent.keyDown(window, { key: k });
    });
    expect(pins).toEqual(["1234"]);
    expect(await screen.findByRole("alert")).toHaveTextContent("PIN incorrecto");
    await act(async () => {
      fireEvent.keyDown(window, { key: "5" });
      fireEvent.keyDown(window, { key: "Backspace" });
      for (const k of ["8", "8", "9", "9"]) fireEvent.keyDown(window, { key: k });
    });
    expect(pins).toEqual(["1234", "8899"]);
  });
});
