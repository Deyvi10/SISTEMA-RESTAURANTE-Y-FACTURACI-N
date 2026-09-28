import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, get, setAccessToken, uuidv7 } from "./client";

afterEach(() => {
  vi.restoreAllMocks();
  setAccessToken(null);
});

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

describe("cliente de la API", () => {
  it("genera UUID v7 ordenables", () => {
    const a = uuidv7();
    expect(a).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
    expect(new Set(Array.from({ length: 200 }, uuidv7)).size).toBe(200);
  });

  it("traduce un problema RFC 9457 a errores por campo", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(json(422, { code: "DATOS_INVALIDOS", detail: "Revisa 2 campos.", errors: [{ campo: "precio", mensaje: "Escribe el precio con punto decimal." }] }));
    const err = await get("/v1/productos").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).fields.precio).toBe("Escribe el precio con punto decimal.");
    expect((err as ApiError).message).toBe("Revisa 2 campos.");
  });

  it("renueva la sesión una sola vez ante un 401 y reintenta", async () => {
    setAccessToken("vencido");
    const fetchMock = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(json(401, { code: "SESION_VENCIDA" }))
      .mockResolvedValueOnce(json(200, { accessToken: "nuevo", expiresIn: 900, usuario: {} }))
      .mockResolvedValueOnce(json(200, [{ id: "1" }]));
    const out = await get<{ id: string }[]>("/v1/categorias");
    expect(out).toEqual([{ id: "1" }]);
    expect(fetchMock.mock.calls[1]![0]).toBe("/v1/auth/refresh");
    const headers = (fetchMock.mock.calls[2]![1] as RequestInit).headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer nuevo");
  });

  it("sin conexión devuelve un mensaje claro", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    await expect(get("/v1/me")).rejects.toMatchObject({ code: "SIN_CONEXION" });
  });
});
