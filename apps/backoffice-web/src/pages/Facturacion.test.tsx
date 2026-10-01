import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Certificado, ConfigFiscal } from "../api/types";
import { FeedbackProvider } from "../components/feedback";
import { cuerpo, Facturacion, formDe, vigencia } from "./Facturacion";

afterEach(() => vi.restoreAllMocks());

const base: ConfigFiscal = {
  guardada: false, ambiente: 1, ruc: "1790011674001", razonSocial: "Distribuidora del Pacífico S.A.", nombreComercial: "Don Pepe",
  direccionMatriz: "Quito", obligadoContabilidad: false, contribuyenteEspecial: null, agenteRetencion: null, regimen: "GENERAL",
  facturacionActiva: false, pendientes: ["Confirma los datos del emisor y guárdalos.", "Asigna un punto de emisión a «Caja 1»."],
  cajas: [{ cajaId: "c1", caja: "Caja 1", localId: "l1", puntoId: null, establecimiento: null, puntoEmision: null, conNodo: false }],
  certificado: null, cambioProgramado: null, pruebaAprobada: false, avisos: ["Sube tu firma electrónica (.p12) para que las facturas lleguen al SRI."],
};

const cert: Certificado = {
  id: "k1", titular: "JOSÉ ANDRADE", ruc: "1790011674001", emisor: "ENTIDAD", serial: "1", validoDesde: "2026-01-01T00:00:00Z",
  validoHasta: "2026-10-20T12:00:00Z", subidoAt: "2026-09-01T00:00:00Z", diasRestantes: 21,
};

function montar(cfg: ConfigFiscal) {
  const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async () =>
    new Response(JSON.stringify(cfg), { status: 200, headers: { "Content-Type": "application/json" } }),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <FeedbackProvider>
        <Facturacion />
      </FeedbackProvider>
    </QueryClientProvider>,
  );
  return fetch;
}

describe("facturación SRI", () => {
  it("los opcionales vacíos viajan como null y el ambiente es de pruebas", () => {
    const f = { ...formDe(base), nombreComercial: "  ", contribuyenteEspecial: " 5368 " };
    expect(cuerpo(f, true)).toEqual(expect.objectContaining({ ambiente: 1, nombreComercial: null, contribuyenteEspecial: "5368", agenteRetencion: null, facturacionActiva: true }));
  });

  it("muestra lo pendiente y no deja activar hasta completarlo", async () => {
    const fetch = montar(base);
    expect(await screen.findByTestId("pendientes")).toHaveTextContent("Asigna un punto de emisión a «Caja 1»");
    await userEvent.click(screen.getByRole("switch", { name: /Facturar electrónicamente/ }));
    expect(fetch.mock.calls.filter(([, init]) => init?.method === "PUT")).toHaveLength(0);
  });

  it("asigna la serie de una caja con tres dígitos", async () => {
    const fetch = montar(base);
    const punto = await screen.findByLabelText("Punto de emisión de Caja 1");
    await userEvent.type(punto, "0a02");
    expect(punto).toHaveValue("002");
    await userEvent.click(screen.getByRole("button", { name: "Guardar" }));
    const put = fetch.mock.calls.find(([url, init]) => String(url).includes("/punto-emision") && init?.method === "PUT");
    expect(JSON.parse(String(put?.[1]?.body))).toEqual({ establecimiento: "001", puntoEmision: "002" });
  });

  it("sube la firma como formulario con el archivo y la contraseña", async () => {
    const fetch = montar(base);
    expect(await screen.findByTestId("avisos")).toHaveTextContent("Sube tu firma electrónica");
    const boton = screen.getByTestId("subir-firma");
    expect(boton).toBeDisabled();
    await userEvent.upload(screen.getByLabelText("Archivo .p12"), new File([new Uint8Array([48, 130])], "firma.p12", { type: "application/x-pkcs12" }));
    await userEvent.type(screen.getByLabelText("Contraseña de la firma"), "secreta");
    await userEvent.click(boton);
    const post = fetch.mock.calls.find(([url, init]) => String(url).includes("/v1/facturacion/certificado") && init?.method === "POST");
    const fd = post?.[1]?.body as FormData;
    expect(fd.get("clave")).toBe("secreta");
    expect((fd.get("p12") as File).name).toBe("firma.p12");
  });

  it("con firma muestra la vigencia y producción solo con una prueba autorizada", async () => {
    montar({ ...base, guardada: true, pendientes: [], avisos: [], certificado: cert });
    expect(await screen.findByTestId("certificado")).toHaveTextContent("JOSÉ ANDRADE");
    expect(screen.queryByTestId("subir-firma")).toBeNull();
    expect(screen.queryByRole("radio", { name: "Producción" })).toBeNull();
  });

  it("la vigencia avisa en naranja el último mes y en rojo al vencer", () => {
    expect(vigencia(cert)).toEqual(expect.objectContaining({ tint: "orange" }));
    expect(vigencia({ ...cert, diasRestantes: 200 }).tint).toBe("green");
    expect(vigencia({ ...cert, diasRestantes: 1 }).texto).toContain("quedan 1 día");
    expect(vigencia({ ...cert, diasRestantes: -1 })).toEqual(expect.objectContaining({ tint: "red" }));
  });

  it("programa un cambio de régimen con fecha", async () => {
    const fetch = montar({ ...base, guardada: true, pendientes: [], avisos: [] });
    await userEvent.click(await screen.findByTestId("programar-cambio"));
    await userEvent.click(screen.getByTestId("guardar-cambio"));
    const put = fetch.mock.calls.find(([url, init]) => String(url).includes("/cambio-programado") && init?.method === "PUT");
    expect(JSON.parse(String(put?.[1]?.body))).toEqual(expect.objectContaining({ regimen: "RIMPE_EMPRENDEDOR", obligadoContabilidad: false }));
  });

  it("muestra el cambio programado con su fecha", async () => {
    montar({ ...base, guardada: true, pendientes: [], avisos: [],
      cambioProgramado: { desde: "2027-01-01", regimen: "RIMPE_EMPRENDEDOR", obligadoContabilidad: true, contribuyenteEspecial: null, agenteRetencion: "7" } });
    expect(await screen.findByTestId("cambio-actual")).toHaveTextContent("1 de enero de 2027: RIMPE Emprendedor");
    expect(screen.getByTestId("cambio-actual")).toHaveTextContent("agente de retención 7");
  });

  it("pasar a producción pide confirmación dentro de la página", async () => {
    const fetch = montar({ ...base, guardada: true, pendientes: [], avisos: [], certificado: cert, pruebaAprobada: true });
    await userEvent.click(await screen.findByRole("radio", { name: "Producción" }));
    await userEvent.click(screen.getByTestId("guardar-emisor"));
    expect(screen.getByTestId("confirmar-produccion")).toHaveTextContent("No se puede volver al ambiente de pruebas");
    expect(fetch.mock.calls.filter(([, init]) => init?.method === "PUT")).toHaveLength(0);
    await userEvent.click(screen.getByTestId("si-produccion"));
    const put = fetch.mock.calls.find(([url, init]) => String(url).endsWith("/v1/facturacion") && init?.method === "PUT");
    expect(JSON.parse(String(put?.[1]?.body))).toEqual(expect.objectContaining({ ambiente: 2 }));
  });

  it("en producción no se ofrece volver a pruebas", async () => {
    montar({ ...base, guardada: true, ambiente: 2, pendientes: [], avisos: [], certificado: cert, pruebaAprobada: true });
    expect(await screen.findByRole("radio", { name: "Producción" })).toBeChecked();
    expect(screen.queryByRole("radio", { name: "Pruebas" })).toBeNull();
  });
});
