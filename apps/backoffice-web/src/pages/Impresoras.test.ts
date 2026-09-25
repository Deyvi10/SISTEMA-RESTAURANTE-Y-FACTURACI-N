import { describe, expect, it } from "vitest";
import { describirConexion, describirEstado, estacionDeCategoria, tipoInstalada } from "./Impresoras";

describe("describirEstado", () => {
  it("habla en lenguaje del dueño y prioriza lo que importa", () => {
    expect(describirEstado({ estado: "SIN_PAPEL", nodoEnLinea: true, activa: true })).toEqual({ texto: "Sin papel", tono: "danger" });
    expect(describirEstado({ estado: "OK", nodoEnLinea: true, activa: true }).tono).toBe("success");
    expect(describirEstado({ estado: "OK", nodoEnLinea: false, activa: true }).texto).toBe("Nodo sin conexión");
    expect(describirEstado({ estado: "SIN_PAPEL", nodoEnLinea: true, activa: false }).texto).toBe("Desactivada");
  });
});

describe("estacionDeCategoria", () => {
  const est = [{ id: "cocina", esDefecto: true }, { id: "bar", esDefecto: false }];
  it("usa la estación de la categoría o la de por defecto", () => {
    expect(estacionDeCategoria({ estacionId: "bar" }, est)).toBe("bar");
    expect(estacionDeCategoria({ estacionId: null }, est)).toBe("cocina");
    expect(estacionDeCategoria({ estacionId: "borrada" }, est)).toBe("cocina");
    expect(estacionDeCategoria({ estacionId: null }, [])).toBeUndefined();
  });
});

describe("conexiones", () => {
  it("describe cómo llega cada impresora", () => {
    expect(describirConexion({ conexion: "WINDOWS", host: null, puerto: null, nombreWindows: "EPSON TM-T20III Receipt" })).toBe("Windows · EPSON TM-T20III Receipt");
    expect(describirConexion({ conexion: "TCP", host: "192.168.1.50", puerto: 9100, nombreWindows: null })).toBe("192.168.1.50:9100");
    expect(tipoInstalada({ puerto: "USB001" })).toBe("USB");
    expect(tipoInstalada({ puerto: "IP_192.168.1.50", host: "192.168.1.50" })).toBe("Red · 192.168.1.50");
    expect(tipoInstalada({ puerto: "WSD-5c1e" })).toBe("Red (WSD)");
    expect(tipoInstalada({ puerto: "COM3:" })).toBe("COM3");
  });
});
