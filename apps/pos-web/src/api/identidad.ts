// Identidad de una PC de caja que no es la del nodo: par de llaves Ed25519 de WebCrypto.
// La llave privada no se puede exportar y vive en IndexedDB; el nodo guarda solo la pública.

export interface Identidad {
  id: string;
  llaves: CryptoKeyPair;
  publicaB64: string;
}

const BASE = "restpos-caja";
const ALMACEN = "identidad";
const CLAVE = "dispositivo";

function abrir(): Promise<IDBDatabase> {
  return new Promise((ok, mal) => {
    const r = indexedDB.open(BASE, 1);
    r.onupgradeneeded = () => r.result.createObjectStore(ALMACEN);
    r.onsuccess = () => ok(r.result);
    r.onerror = () => mal(r.error);
  });
}

async function operar<T>(modo: IDBTransactionMode, fn: (s: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await abrir();
  try {
    return await new Promise<T>((ok, mal) => {
      const r = fn(db.transaction(ALMACEN, modo).objectStore(ALMACEN));
      r.onsuccess = () => ok(r.result);
      r.onerror = () => mal(r.error);
    });
  } finally {
    db.close();
  }
}

export function aBase64(b: ArrayBuffer): string {
  return btoa(String.fromCharCode(...new Uint8Array(b)));
}

/** UUID v7: los IDs se generan en el origen (regla del proyecto). */
export function uuidv7(ahora = Date.now()): string {
  const b = crypto.getRandomValues(new Uint8Array(16));
  const ms = BigInt(ahora);
  for (let i = 0; i < 6; i++) b[i] = Number((ms >> BigInt(8 * (5 - i))) & 0xffn);
  b[6] = (b[6]! & 0x0f) | 0x70;
  b[8] = (b[8]! & 0x3f) | 0x80;
  const h = [...b].map((x) => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

export async function cargarIdentidad(): Promise<Identidad | null> {
  return (await operar("readonly", (s) => s.get(CLAVE) as IDBRequest<Identidad | undefined>)) ?? null;
}

export async function crearIdentidad(): Promise<Identidad> {
  const llaves = (await crypto.subtle.generateKey({ name: "Ed25519" }, false, ["sign", "verify"])) as CryptoKeyPair;
  const id: Identidad = { id: uuidv7(), llaves, publicaB64: aBase64(await crypto.subtle.exportKey("raw", llaves.publicKey)) };
  await operar("readwrite", (s) => s.put(id, CLAVE));
  return id;
}

export async function olvidarIdentidad(): Promise<void> {
  await operar("readwrite", (s) => s.delete(CLAVE));
}

/** Firma el desafío del nodo: `restpos|sesion-dispositivo|nonce` (igual que la app de meseros). */
export async function firmarDesafio(id: Identidad, nonce: string): Promise<string> {
  const firma = await crypto.subtle.sign({ name: "Ed25519" }, id.llaves.privateKey, new TextEncoder().encode(`restpos|sesion-dispositivo|${nonce}`));
  return aBase64(firma);
}
