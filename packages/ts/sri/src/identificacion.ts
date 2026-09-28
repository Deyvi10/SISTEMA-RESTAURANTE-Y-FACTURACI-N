// Validación de identificaciones del comprador (docs/05 §12), la misma regla que
// packages/go/sri/identificacion.go. Los casos compartidos están en
// packages/testdata/identificaciones.json: si cambias una regla, cambia las dos.

/** Tipo de identificación según el catálogo del SRI. */
export type TipoIdentificacion = "04" | "05" | "06" | "07" | "08" | "";

export const ID_RUC = "04";
export const ID_CEDULA = "05";
export const ID_PASAPORTE = "06";
export const ID_CONSUMIDOR_FINAL = "07";

/** Identificación fija del consumidor final. */
export const CONSUMIDOR_FINAL = "9999999999999";

export type TipoContribuyente = "PERSONA_NATURAL" | "SOCIEDAD_PRIVADA" | "SECTOR_PUBLICO";

/** valida=false bloquea; una advertencia (con valida=true) solo informa. */
export interface Validacion {
  valida: boolean;
  tipo: TipoIdentificacion;
  tipoContribuyente?: TipoContribuyente;
  motivo?: string;
  advertencia?: string;
}

const esDigitos = (s: string, n: number) => s.length === n && /^\d+$/.test(s);
const invalida = (tipo: TipoIdentificacion, motivo: string): Validacion => ({ valida: false, tipo, motivo });
const d = (s: string, i: number) => s.charCodeAt(i) - 48;

function motivoProvincia(s: string): string | null {
  const p = d(s, 0) * 10 + d(s, 1);
  return (p < 1 || p > 24) && p !== 30 ? `El código de provincia ${String(p).padStart(2, "0")} no existe (debe ser 01-24 o 30).` : null;
}

function mod11(digitos: string, coef: number[], verificador: number): boolean {
  let suma = 0;
  coef.forEach((c, i) => (suma += d(digitos, i) * c));
  let v = 11 - (suma % 11);
  if (v === 11) v = 0;
  return v === verificador; // 10 nunca coincide: no existe ese verificador
}

function conAdvertencia(t: TipoContribuyente, ok: boolean): Validacion {
  const v: Validacion = { valida: true, tipo: ID_RUC, tipoContribuyente: t };
  if (!ok) v.advertencia = "El dígito verificador no cumple el módulo 11. Algunos RUC de sociedades son así; el SRI hará la validación final.";
  return v;
}

/** Cédula: provincia 01-24 o 30, tercer dígito < 6 y verificador módulo 10 (2,1,2,…). */
export function validarCedula(ced: string): Validacion {
  if (!esDigitos(ced, 10)) return invalida(ID_CEDULA, "La cédula debe tener 10 dígitos.");
  const prov = motivoProvincia(ced);
  if (prov) return invalida(ID_CEDULA, prov);
  if (d(ced, 2) >= 6) return invalida(ID_CEDULA, "El tercer dígito de una cédula debe ser menor que 6.");
  let suma = 0;
  for (let i = 0; i < 9; i++) {
    let p = d(ced, i) * (2 - (i % 2));
    if (p > 9) p -= 9;
    suma += p;
  }
  if ((10 - (suma % 10)) % 10 !== d(ced, 9)) return invalida(ID_CEDULA, "El dígito verificador de la cédula no coincide. Revisa el número.");
  return { valida: true, tipo: ID_CEDULA, tipoContribuyente: "PERSONA_NATURAL" };
}

/** RUC de 13 dígitos según el tipo de contribuyente; sociedades con módulo 11 inválido: advertencia. */
export function validarRUC(ruc: string): Validacion {
  if (!esDigitos(ruc, 13)) return invalida(ID_RUC, "El RUC debe tener 13 dígitos.");
  const prov = motivoProvincia(ruc);
  if (prov) return invalida(ID_RUC, prov);
  const tercero = d(ruc, 2);
  if (tercero < 6) {
    if (!validarCedula(ruc.slice(0, 10)).valida) return invalida(ID_RUC, "Los 10 primeros dígitos no forman una cédula válida.");
    if (ruc.slice(10) === "000") return invalida(ID_RUC, "El RUC debe terminar en un establecimiento distinto de 000 (normalmente 001).");
    return { valida: true, tipo: ID_RUC, tipoContribuyente: "PERSONA_NATURAL" };
  }
  if (tercero === 9) {
    const ok = mod11(ruc.slice(0, 9), [4, 3, 2, 7, 6, 5, 4, 3, 2], d(ruc, 9));
    if (ruc.slice(10) === "000") return invalida(ID_RUC, "El RUC debe terminar en un establecimiento distinto de 000 (normalmente 001).");
    return conAdvertencia("SOCIEDAD_PRIVADA", ok);
  }
  if (tercero === 6) {
    const ok = mod11(ruc.slice(0, 8), [3, 2, 7, 6, 5, 4, 3, 2], d(ruc, 8));
    if (ruc.slice(9) === "0000") return invalida(ID_RUC, "El RUC público debe terminar en un establecimiento distinto de 0000 (normalmente 0001).");
    return conAdvertencia("SECTOR_PUBLICO", ok);
  }
  return invalida(ID_RUC, `El tercer dígito ${tercero} no corresponde a ningún tipo de RUC.`);
}

/** Pasaporte: de 3 a 20 letras o números. */
export function validarPasaporte(id: string): Validacion {
  return id.length >= 3 && id.length <= 20 && /^[A-Za-z0-9]+$/.test(id)
    ? { valida: true, tipo: ID_PASAPORTE }
    : invalida(ID_PASAPORTE, "El pasaporte debe tener de 3 a 20 letras o números, sin espacios ni guiones.");
}

/**
 * Campo único «Cédula / RUC / Pasaporte» (RF-04-05): 10 dígitos → cédula, 13 → RUC (o
 * consumidor final), con letras → pasaporte. Un número de otra longitud se rechaza.
 */
export function validarIdentificacion(id: string): Validacion {
  if (id === CONSUMIDOR_FINAL) return { valida: true, tipo: ID_CONSUMIDOR_FINAL };
  if (esDigitos(id, 10)) return validarCedula(id);
  if (esDigitos(id, 13)) return validarRUC(id);
  if (id !== "" && /^\d+$/.test(id)) return invalida("", `Tiene ${id.length} dígitos. Una cédula tiene 10 y un RUC 13. Si es un pasaporte, elige ese tipo.`);
  const p = validarPasaporte(id);
  return p.valida ? p : invalida("", "Escribe una cédula (10 dígitos), un RUC (13 dígitos) o un pasaporte (solo letras y números).");
}
