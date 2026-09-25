// Tipos de la API /v1 (espejo de apps/cloud-api). Los importes viajan como texto decimal
// exacto ("12.50"): nunca se convierten a number para calcular.

export type UUID = string;
export type Rol = "ADMIN" | "CAJERO" | "MESERO" | "COCINA" | "BODEGA";

export interface Usuario {
  id: UUID;
  tenantId: UUID;
  nombreMostrar: string;
  email: string;
  rol: Rol;
  nombreComercial: string;
  debeCambiarPassword: boolean;
}
export interface Me extends Usuario {
  permisos: string[];
}
export interface Sesion {
  accessToken: string;
  expiresIn: number;
  usuario: Usuario;
}

export interface Paso {
  id: string;
  titulo: string;
  descripcion: string;
  icono: string;
  hecho: boolean;
  disponible: boolean;
  ruta: string;
}
export interface Resumen {
  nombreComercial: string;
  pasos: Paso[];
  progreso: number;
  cuentas: Record<"productos" | "conFoto" | "categorias" | "mesas" | "personal", number>;
}

export interface TarifaIVA {
  id: UUID;
  codigoSri: string;
  porcentaje: string;
  descripcion: string;
}
export interface IconoCategoria {
  icono: string;
  etiqueta: string;
}
export interface Categoria {
  id: UUID;
  nombre: string;
  orden: number;
  estacionId: UUID | null;
  color: string;
  icono: string;
  activa: boolean;
  productos: number;
  notasRapidas: string[];
}
export interface Desglose {
  base: string;
  iva: string;
  total: string;
}
export interface Producto {
  id: UUID;
  categoriaId: UUID;
  nombre: string;
  alias: string | null;
  codigo: string | null;
  descripcion: string;
  precio: string;
  tarifaIvaId: UUID;
  tipo: string;
  comportamientoStock: string;
  estacionId: UUID | null;
  imagenKey: string | null;
  imagenUrl: string | null;
  imagenMiniUrl: string | null;
  activo: boolean;
  visibleMenuQr: boolean;
  orden: number;
  gruposModificadores: UUID[];
  desglose: Desglose;
}
export interface Modificador {
  id: UUID;
  nombre: string;
  precioAdicional: string;
  activo: boolean;
}
export interface Grupo {
  id: UUID;
  nombre: string;
  obligatorio: boolean;
  min: number;
  max: number;
  modificadores: Modificador[];
  productos: number;
}
export interface Imagen {
  key: string;
  urls: Record<"sm" | "md" | "lg", string>;
}
export interface FotoGaleria extends Imagen {
  slug: string;
  nombre: string;
  categoria: string;
  palabras: string;
  licencia: string;
  fuente: string;
  autor: string;
}

export interface Local {
  id: UUID;
  nombre: string;
  direccion: string;
  codigoEstablecimiento: string;
  zonaHoraria: string;
  propinaLegalActiva: boolean;
  propinaPorcentaje: string;
  preciosIncluyenIva: boolean;
}
export interface Estacion {
  id: UUID;
  localId: UUID;
  nombre: string;
  tipo: "PRODUCCION" | "CAJA";
  icono: string;
  color: string;
  orden: number;
  esDefecto: boolean;
}
export interface Zona {
  id: UUID;
  localId: UUID;
  nombre: string;
  orden: number;
  mesas: number;
}
export interface Mesa {
  id: UUID;
  localId: UUID;
  zonaId: UUID;
  nombre: string;
  capacidad: number;
  forma: "CUADRADA" | "REDONDA" | "RECTANGULAR";
  posX: number;
  posY: number;
  activa: boolean;
}
export interface Persona {
  id: UUID;
  nombreMostrar: string;
  rol: Rol;
  email: string | null;
  esDueno: boolean;
  tienePin: boolean;
  accesoWeb: boolean;
  activo: boolean;
  avatarKey: string | null;
  avatarUrl: string | null;
}

export interface SaludNodo {
  discoLibreMb?: number;
  baseMb?: number;
  outboxPendientes?: number;
  outboxAntiguedadSeg?: number;
  derivaSegundos?: number;
  alertaReloj?: boolean;
}
export interface NodoLocal {
  id: UUID;
  localId: UUID;
  localNombre: string;
  nombreEquipo: string;
  estado: "ACTIVO" | "REVOCADO";
  version: string;
  enLinea: boolean;
  ultimoHeartbeatAt: string | null;
  salud: SaludNodo;
  activadoAt: string;
  revocadoAt: string | null;
}
export interface CodigoNodo {
  codigo: string;
  localId: UUID;
  expiraAt: string;
}
