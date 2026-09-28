import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { validarIdentificacion, validarPasaporte } from "./identificacion.ts";

// Los mismos casos que packages/go/sri (TestVectoresIdentificacion).
const vectores = JSON.parse(readFileSync(new URL("../../../testdata/identificaciones.json", import.meta.url), "utf8")) as {
  casos: { id: string; valida: boolean; tipo: string; contribuyente?: string; advertencia?: boolean; nota: string }[];
};

for (const c of vectores.casos) {
  test(`${c.id} · ${c.nota}`, () => {
    const v = validarIdentificacion(c.id);
    assert.equal(v.valida, c.valida, v.motivo);
    assert.equal(v.tipo, c.tipo);
    if (c.contribuyente) assert.equal(v.tipoContribuyente, c.contribuyente);
    assert.equal(!!v.advertencia, !!c.advertencia);
    if (!v.valida) assert.ok(v.motivo, "una inválida siempre explica por qué");
  });
}

test("un pasaporte solo numérico se registra eligiendo el tipo", () => {
  assert.equal(validarIdentificacion("123456789").valida, false);
  assert.equal(validarPasaporte("123456789").valida, true);
});
