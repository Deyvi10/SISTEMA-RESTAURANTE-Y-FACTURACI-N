import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { desglosar, ErrorDesglose, type Factura, type Venta } from "./desglose.ts";

const vectores = JSON.parse(readFileSync(new URL("../../../testdata/desglose-sri.json", import.meta.url), "utf8")) as {
  nombre: string;
  venta: Venta;
  factura?: Factura;
  error?: boolean;
}[];

test("el desglose TS da exactamente lo mismo que el de Go en los vectores compartidos", () => {
  assert.ok(vectores.length > 300);
  for (const v of vectores) {
    if (v.error) {
      assert.throws(() => desglosar(v.venta), ErrorDesglose, v.nombre);
      continue;
    }
    assert.deepEqual(desglosar(v.venta), v.factura, v.nombre);
  }
});
