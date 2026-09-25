import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { del, get, post, put, patch } from "./client";
import type {
  Categoria, Estacion, FotoGaleria, Grupo, IconoCategoria, Imagen, Local, Mesa, Persona, Producto, Resumen, TarifaIVA, Zona,
} from "./types";

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
export const usePersonal = () => useQuery({ queryKey: ["personal"], queryFn: () => get<Persona[]>("/v1/usuarios") });

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
  cambiarEstado: (id: string, activo: boolean) => put<Persona>(`/v1/usuarios/${id}/estado`, { activo }),
};
