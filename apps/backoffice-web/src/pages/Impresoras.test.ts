import { describe, expect, it } from "vitest";
import { describirEstado, estacionDeCategoria } from "./Impresoras";

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
