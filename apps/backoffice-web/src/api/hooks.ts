import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { del, get, post, put, patch } from "./client";
import type {
  Caja, Categoria, CodigoNodo, MetodoPago, MotivoDescuento, ComandoNodo, Estacion, Impresora, ImpresoraInstalada, FotoGaleria, Grupo, IconoCategoria, Imagen, Local, Mesa, NodoLocal, PermisosPersona, Persona, Producto, Resumen, TarifaIVA, Zona, ConfigFiscal, Certificado, CambioRegimen, AlertaFiscal, CierreZFila, ResumenVentas, DetalleComprobante, FilaComprobante, ListaComprobantes } from "./types";

export const useResumen = () => useQuery({ queryKey: ["resumen"], queryFn: () => get<Resumen>("/v1/resumen") });
export const useTarifas = () => useQuery({ queryKey: ["tarifas"], queryFn: () => get<TarifaIVA[]>("/v1/tarifas-iva"), staleTime: Infinity });
export const useIconos = () => useQuery({ queryKey: ["iconos"], queryFn: () => get<IconoCategoria[]>("/v1/iconos-categoria"), staleTime: Infinity });
export const useGaleria = () => useQuery({ queryKey: ["galeria"], queryFn: () => get<FotoGaleria[]>("/v1/galeria"), staleTime: Infinity });
export const useCategorias = () => useQuery({ queryKey: ["categorias"], queryFn: () => get<Categoria[]>("/v1/categorias") });
export const useProductos = () => useQuery({ queryKey: ["productos"], queryFn: () => get<Producto[]>("/v1/productos") });
export const useGrupos = () => useQuery({ queryKey: ["grupos"], queryFn: () => get<Grupo[]>("/v1/grupos-modificadores") });
export const useLocales = () => useQuery({ queryKey: ["locales"], queryFn: () => get<Local[]>("/v1/locales") });
export const useEstaciones = () => useQuery({ queryKey: ["estaciones"], queryFn: () => get<Estacion[]>("/v1/estaciones") });
export const useZonas = () => useQuery({ queryKey: ["zonas"], queryFn: () => get<Zona[]>("/v1/zonas") });
export const useMesas = () => useQuery({ queryKey: ["mesas"], queryFn: () => get<Mesa[]>("/v1/mesas") });
// El estado del nodo se refresca solo cada 15 s (heartbeat cada 60 s).
export const useNodos = () => useQuery({ queryKey: ["nodos"], queryFn: () => get<NodoLocal[]>("/v1/nodos"), refetchInterval: 15_000 });
export const useInstaladas = () => useQuery({ queryKey: ["impresoras", "instaladas"], queryFn: () => get<ImpresoraInstalada[]>("/v1/impresoras/instaladas"), refetchInterval: 30_000 });
export const useImpresoras = () => useQuery({ queryKey: ["impresoras"], queryFn: () => get<Impresora[]>("/v1/impresoras"), refetchInterval: 10_000 });
/** Bóveda de comprobantes (F5-12): páginas de 50, la más reciente primero; se refresca cada 15 s. */
export const useComprobantes = (filtro: Record<string, string>) =>
  useInfiniteQuery({
    queryKey: ["comprobantes", filtro],
    initialPageParam: "",
    queryFn: ({ pageParam }) => {
      const q = new URLSearchParams(Object.entries({ ...filtro, cursor: pageParam }).filter(([, v]) => v !== ""));
      return get<ListaComprobantes>(`/v1/comprobantes?${q}`);
    },
    getNextPageParam: (ultima) => ultima.siguiente ?? undefined,
    refetchInterval: 15_000,
  });
export const useComprobante = (id: string | null) =>
  useQuery({ queryKey: ["comprobante", id], queryFn: () => get<DetalleComprobante>(`/v1/comprobantes/${id}`), enabled: !!id });
export const useAlertasFiscales = (activo = true) =>
  useQuery({ queryKey: ["alertas-fiscales"], queryFn: () => get<AlertaFiscal[]>("/v1/alertas/fiscales"), refetchInterval: 60_000, enabled: activo });
export const useVentas = (desde: string, hasta: string) =>
  useQuery({ queryKey: ["ventas", desde, hasta], queryFn: () => get<ResumenVentas>(`/v1/reportes/ventas?desde=${desde}&hasta=${hasta}`) });
export const useCierresZ = (desde: string, hasta: string) =>
  useQuery({ queryKey: ["cierres", desde, hasta], queryFn: () => get<CierreZFila[]>(`/v1/reportes/cierres?desde=${desde}&hasta=${hasta}`) });
export const useCajas = () => useQuery({ queryKey: ["cajas"], queryFn: () => get<Caja[]>("/v1/cajas") });
export const useFacturacion = () => useQuery({ queryKey: ["facturacion"], queryFn: () => get<ConfigFiscal>("/v1/facturacion") });
export const useMetodosPago = () => useQuery({ queryKey: ["metodos-pago"], queryFn: () => get<MetodoPago[]>("/v1/metodos-pago") });
export const useMotivos = () => useQuery({ queryKey: ["motivos-descuento"], queryFn: () => get<MotivoDescuento[]>("/v1/motivos-descuento") });
export const usePersonal = () => useQuery({ queryKey: ["personal"], queryFn: () => get<Persona[]>("/v1/usuarios") });
export const usePermisos = (id: string | undefined) =>
  useQuery({ queryKey: ["permisos", id], queryFn: () => get<PermisosPersona>(`/v1/usuarios/${id}/permisos`), enabled: !!id });

/** Mutación que al terminar refresca las listas afectadas y el progreso de la guía. */
export function useGuardar<TIn, TOut>(fn: (v: TIn) => Promise<TOut>, invalida: string[]) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      for (const k of [...invalida, "resumen"]) void qc.invalidateQueries({ queryKey: [k] });
    },
  });
}

export const api = {
  crearCaja: (b: object) => post<Caja>("/v1/cajas", b),
  editarCaja: (id: string, b: object) => put<Caja>(`/v1/cajas/${id}`, b),
  guardarFacturacion: (b: object) => put<ConfigFiscal>("/v1/facturacion", b),
  programarCambio: (b: CambioRegimen) => put<ConfigFiscal>("/v1/facturacion/cambio-programado", b),
  cancelarCambio: () => del("/v1/facturacion/cambio-programado"),
  subirCertificado: (archivo: File, clave: string) => {
    const fd = new FormData();
    fd.append("p12", archivo);
    fd.append("clave", clave);
    return post<Certificado>("/v1/facturacion/certificado", fd);
  },
  reenviarComprobante: (id: string, correo: string) => post<void>(`/v1/comprobantes/${id}/reenviar`, { correo }),
  reintentarComprobante: (id: string) => post<FilaComprobante>(`/v1/comprobantes/${id}/reintentar`, {}),
  asignarPunto: (cajaId: string, b: { establecimiento: string; puntoEmision: string }) => put<ConfigFiscal>(`/v1/cajas/${cajaId}/punto-emision`, b),
  borrarCaja: (id: string) => del(`/v1/cajas/${id}`),
  crearMetodo: (b: object) => post<MetodoPago>("/v1/metodos-pago", b),
  editarMetodo: (id: string, b: object) => put<MetodoPago>(`/v1/metodos-pago/${id}`, b),
  borrarMetodo: (id: string) => del(`/v1/metodos-pago/${id}`),
  crearMotivo: (b: object) => post<MotivoDescuento>("/v1/motivos-descuento", b),
  borrarMotivo: (id: string) => del(`/v1/motivos-descuento/${id}`),
  subirFoto: (f: File) => {
    const fd = new FormData();
    fd.append("foto", f);
    return post<Imagen>("/v1/imagenes", fd);
  },
  crearCategoria: (b: object) => post<Categoria>("/v1/categorias", b),
  editarCategoria: (id: string, b: object) => put<Categoria>(`/v1/categorias/${id}`, b),
  borrarCategoria: (id: string) => del(`/v1/categorias/${id}`),
  ordenarCategorias: (orden: string[]) => put<void>("/v1/categorias/orden", { orden }),
  crearProducto: (b: object) => post<Producto>("/v1/productos", b),
  editarProducto: (id: string, b: object) => put<Producto>(`/v1/productos/${id}`, b),
  borrarProducto: (id: string) => del(`/v1/productos/${id}`),
  crearGrupo: (b: object) => post<Grupo>("/v1/grupos-modificadores", b),
  editarGrupo: (id: string, b: object) => put<Grupo>(`/v1/grupos-modificadores/${id}`, b),
  borrarGrupo: (id: string) => del(`/v1/grupos-modificadores/${id}`),
  editarLocal: (id: string, b: object) => patch<Local>(`/v1/locales/${id}`, b),
  crearEstacion: (b: object) => post<Estacion>("/v1/estaciones", b),
  editarEstacion: (id: string, b: object) => put<Estacion>(`/v1/estaciones/${id}`, b),
  borrarEstacion: (id: string) => del(`/v1/estaciones/${id}`),
  crearZona: (b: object) => post<Zona>("/v1/zonas", b),
  renombrarZona: (id: string, nombre: string) => patch<Zona>(`/v1/zonas/${id}`, { nombre }),
  borrarZona: (id: string) => del(`/v1/zonas/${id}`),
  crearMesas: (b: object) => post<Mesa[]>("/v1/mesas/lote", b),
  editarMesa: (id: string, b: object) => put<Mesa>(`/v1/mesas/${id}`, b),
  borrarMesa: (id: string) => del(`/v1/mesas/${id}`),
  crearPersona: (b: object) => post<Persona>("/v1/usuarios", b),
  editarPersona: (id: string, b: object) => put<Persona>(`/v1/usuarios/${id}`, b),
  cambiarPIN: (id: string, pin: string) => put<void>(`/v1/usuarios/${id}/pin`, { pin }),
  cambiarLimiteDescuento: (id: string, porcentaje: string | null) => put<PermisosPersona>(`/v1/usuarios/${id}/limite-descuento`, { porcentaje }),
  cambiarPermiso: (id: string, permiso: string, concedido: boolean) => put<PermisosPersona>(`/v1/usuarios/${id}/permisos`, { permiso, concedido }),
  cambiarEstado: (id: string, activo: boolean) => put<Persona>(`/v1/usuarios/${id}/estado`, { activo }),
  crearImpresora: (b: object) => post<Impresora>("/v1/impresoras", b),
  editarImpresora: (id: string, b: object) => put<Impresora>(`/v1/impresoras/${id}`, b),
  borrarImpresora: (id: string) => del(`/v1/impresoras/${id}`),
  probarImpresora: (id: string) => post<ComandoNodo>(`/v1/impresoras/${id}/prueba`, {}),
  comandoNodo: (id: string) => get<ComandoNodo>(`/v1/comandos-nodo/${id}`),
  conectarImpresora: (b: object) => post<Impresora>("/v1/impresoras/conectar", b),
  buscarImpresoras: (localId: string) => post<ComandoNodo>("/v1/impresoras/buscar", { localId }),
  asignarImpresoras: (estacionId: string, impresoras: string[]) => put<void>(`/v1/estaciones/${estacionId}/impresoras`, { impresoras }),
  rutearCategoria: (categoriaId: string, estacionId: string | null) => put<void>(`/v1/categorias/${categoriaId}/estacion`, { estacionId }),
  generarCodigoNodo: (localId: string) => post<CodigoNodo>("/v1/nodos/codigos", { localId }),
  revocarNodo: (id: string) => post<void>(`/v1/nodos/${id}/revocar`, {}),
};
