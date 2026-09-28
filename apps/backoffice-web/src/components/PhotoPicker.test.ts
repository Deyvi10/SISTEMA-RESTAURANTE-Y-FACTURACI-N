import { describe, expect, it } from "vitest";
import type { FotoGaleria } from "../api/types";
import { normalizar, sugerir } from "./PhotoPicker";

const foto = (slug: string, nombre: string, palabras: string): FotoGaleria => ({
  slug, nombre, palabras, categoria: "", licencia: "CC0", fuente: "", autor: "", key: `biblioteca/${slug}`,
  urls: { sm: "", md: "", lg: "" },
});

describe("sugerencia de fotos por nombre del plato", () => {
  const galeria = [
    foto("hamburguesa", "Hamburguesa", "hamburguesa burger doble"),
    foto("ceviche-camaron", "Ceviche de camarón", "ceviche camaron camarones coctel"),
    foto("cafe", "Café", "cafe capuchino americano"),
  ];
  it("pone primero la foto que coincide, sin importar tildes", () => {
    expect(sugerir(galeria, "Ceviche mixto")[0]!.foto.slug).toBe("ceviche-camaron");
    expect(sugerir(galeria, "CAFÉ PASADO")[0]!.foto.slug).toBe("cafe");
    expect(sugerir(galeria, "Burger doble queso")[0]!.foto.slug).toBe("hamburguesa");
  });
  it("sin coincidencias no marca ninguna como sugerida", () => {
    expect(sugerir(galeria, "Xyz").every((s) => s.puntos === 0)).toBe(true);
  });
  it("normaliza tildes y mayúsculas", () => {
    expect(normalizar("Bolón Ñandú")).toBe("bolon nandu");
  });
});
