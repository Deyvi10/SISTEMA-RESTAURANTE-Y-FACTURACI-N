import { strict as assert } from "node:assert";
import { test } from "node:test";
import { formatHour, formatMoney } from "./format.ts";

test("formatMoney usa decimal exacto y half-up", () => {
  assert.equal(formatMoney("1250.5"), "$1,250.50");
  assert.equal(formatMoney("14.50"), "$14.50");
  assert.equal(formatMoney("2.675"), "$2.68"); // con float daría $2.67
  assert.equal(formatMoney("0.005"), "$0.01");
  assert.equal(formatMoney("0"), "$0.00");
  assert.equal(formatMoney("-3.335"), "-$3.34");
  assert.equal(formatMoney("1234567.891"), "$1,234,567.89");
  assert.throws(() => formatMoney("1,250.50"));
  assert.throws(() => formatMoney("abc"));
});

test("formatHour usa la hora de Ecuador", () => {
  // 02:00 UTC del 25 = 21:00 del 24 en Guayaquil
  assert.equal(formatHour(new Date("2026-09-25T02:00:00Z")), "21:00");
});
