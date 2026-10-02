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
  /** Diferencia de caja (USD) que convierte el correo del Cierre Z en alerta crítica. */
  umbralAlertaCierre: string;
  /** Descuento (%) que puede dar sin autorización quien no es administrador. */
  descuentoMaximoPct: string;
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

export type EstadoImpresora = "OK" | "POCO_PAPEL" | "SIN_PAPEL" | "TAPA_ABIERTA" | "SIN_CONEXION" | "ERROR" | "DESCONOCIDO";
export interface Impresora {
  id: UUID;
  localId: UUID;
  nombre: string;
  conexion: "TCP" | "USB" | "WINDOWS";
  nombreWindows: string | null;
  host: string | null;
  puerto: number | null;
  mac: string | null;
  modelo: string;
  anchoPapel: 58 | 80;
  origen: "MANUAL" | "DETECTADA";
  activa: boolean;
  estaciones: UUID[];
  estado: EstadoImpresora;
  cola: number;
  estadoAt: string | null;
  nodoEnLinea: boolean;
}
export interface ComandoNodo {
  id: UUID;
  tipo: string;
  createdAt: string;
  ejecutadoAt: string | null;
  resultado: string | null;
}

/** Impresora instalada en Windows en la PC del Nodo Local. */
export interface ImpresoraInstalada {
  nombre: string;
  puerto: string;
  driver: string;
  host?: string;
  puertoTcp?: number;
  estado: EstadoImpresora;
  anchoSugerido: 58 | 80;
  predeterminada?: boolean;
  localId: UUID;
  impresoraId: UUID | null;
}

// ---------- Caja (F4-03, F4-06, F4-09) ----------

export type Regimen = "GENERAL" | "RIMPE_EMPRENDEDOR" | "RIMPE_NEGOCIO_POPULAR";

/** Caja con su serie del SRI (establecimiento-punto). */
export interface PuntoCaja {
  cajaId: string;
  caja: string;
  localId: string;
  puntoId: string | null;
  establecimiento: string | null;
  puntoEmision: string | null;
  conNodo: boolean;
}

/** Datos tributarios del emisor y estado de la facturación electrónica (F5-02, F5-06). */
export interface ConfigFiscal {
  guardada: boolean;
  ambiente: 1 | 2;
  ruc: string;
  razonSocial: string;
  nombreComercial: string | null;
  direccionMatriz: string;
  obligadoContabilidad: boolean;
  contribuyenteEspecial: string | null;
  agenteRetencion: string | null;
  regimen: Regimen;
  facturacionActiva: boolean;
  cajas: PuntoCaja[];
  certificado: Certificado | null;
  cambioProgramado: CambioRegimen | null;
  pruebaAprobada: boolean;
  pendientes: string[];
  avisos: string[];
}

/** Cambio de régimen o calificaciones con fecha de vigencia (F5-14). */
export interface CambioRegimen {
  desde: string;
  regimen: Regimen;
  obligadoContabilidad: boolean;
  contribuyenteEspecial: string | null;
  agenteRetencion: string | null;
}

/** Firma electrónica activa (F5-07): solo sus datos visibles, nunca el archivo. */
export interface Certificado {
  id: string;
  titular: string;
  ruc: string | null;
  emisor: string;
  serial: string;
  validoDesde: string;
  validoHasta: string;
  subidoAt: string;
  diasRestantes: number;
  aviso?: string;
}

export interface Caja {
  id: string;
  localId: string;
  nombre: string;
  estacionId: string | null;
  activa: boolean;
}

export type TipoMetodoPago = "EFECTIVO" | "TARJETA_CREDITO" | "TARJETA_DEBITO" | "TRANSFERENCIA" | "BILLETERA" | "OTRO";

export interface MetodoPago {
  id: string;
  nombre: string;
  tipo: TipoMetodoPago;
  codigoSri: string;
  abreCajon: boolean;
  pideReferencia: boolean;
  icono: string;
  orden: number;
  activo: boolean;
}

export interface MotivoDescuento {
  id: string;
  nombre: string;
  tipo: "DESCUENTO" | "CORTESIA";
  activo: boolean;
}

/** Un permiso de la matriz RBAC para una persona (F4-14). */
export interface PermisoPersona {
  permiso: string;
  nombre: string;
  descripcion: string;
  concedido: boolean;
  configurable: boolean; // ⚙️: el dueño lo puede cambiar para su rol
  porDefecto: boolean;
}
export interface PermisosPersona {
  usuarioId: UUID;
  rol: Rol;
  permisos: PermisoPersona[];
  /** Límite propio de descuento sin autorización; null = el del local. */
  descuentoMaximoPct: string | null;
}

/** Grupo de estado de un comprobante en la bóveda (RF-05-07). */
export type GrupoComprobante = "ENVIADO" | "AUTORIZADO" | "NO_AUTORIZADO" | "REQUIERE_ATENCION" | "ANULADO";

export interface Explicacion {
  que: string;
  accion: string;
  soporte: boolean;
}

export interface FilaComprobante {
  id: string;
  tipo: "01" | "04";
  numero: string;
  claveAcceso: string;
  fechaEmision: string;
  comprador: string | null;
  identificacion: string | null;
  importeTotal: string;
  ambiente: 1 | 2;
  estado: string;
  grupo: GrupoComprobante;
  serie: string;
  tieneCorreo: boolean;
  correoEnviado: boolean;
  explicacion?: Explicacion;
  fechaAutorizacion: string | null;
}

export interface ListaComprobantes {
  filas: FilaComprobante[];
  siguiente: string | null;
  pendientes: Partial<Record<GrupoComprobante, number>>;
}

export interface DetalleComprobante extends FilaComprobante {
  correo: string | null;
  eventos: { estado: string; fecha: string; detalle: Record<string, unknown> }[];
  correos: { destino: string; motivo: string; ok: boolean; fecha: string }[];
  mensajes: { Identificador: string; Mensaje: string; InformacionAdicional: string; Tipo: string }[];
}

/** Alerta fiscal (F5-16). */
export interface AlertaFiscal {
  clave: string;
  nivel: "ADVERTENCIA" | "CRITICA";
  titulo: string;
  accion: string;
  enlace: string;
}

export interface TotalesVentas {
  documentos: number;
  subtotal: string;
  iva: string;
  propina: string;
  descuento: string;
  total: string;
}

export interface ResumenVentas {
  desde: string;
  hasta: string;
  dias: (TotalesVentas & { fecha: string })[];
  total: TotalesVentas;
  ticketPromedio: string;
  porMetodo: { metodo: string; monto: string; pagos: number }[];
  notasCredito: { cantidad: number; valor: string };
  neto: string;
}

export interface CierreZFila {
  id: string;
  numero: number;
  caja: string;
  cajero: string;
  fechaNegocio: string;
  cerradoAt: string;
  resultado: "CUADRADO" | "SOBRANTE" | "FALTANTE";
  hashValido: boolean;
}
