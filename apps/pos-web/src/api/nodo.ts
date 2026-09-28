// Cliente HTTP del Nodo Local. Los tokens viven solo en memoria: al recargar, la caja vuelve a
// pedir el PIN (la llave del dispositivo sí persiste, en IndexedDB y sin poder exportarse).

export interface FieldError {
  campo: string;
  mensaje: string;
}

/** Error del nodo con el mensaje para el usuario (RFC 9457). */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: Record<string, string>;
  constructor(status: number, code: string, message: string, fields: FieldError[] = []) {
    super(message);
    this.status = status;
    this.code = code;
    this.fields = Object.fromEntries(fields.map((f) => [f.campo, f.mensaje]));
  }
  get esRed() {
    return this.status === 0;
  }
}

const tokens: { dispositivo: string | null; usuario: string | null } = { dispositivo: null, usuario: null };
let alPerderSesion: (code: string) => void = () => {};

export function setTokenDispositivo(t: string | null) {
  tokens.dispositivo = t;
}
export function setTokenUsuario(t: string | null) {
  tokens.usuario = t;
}
export function tokenUsuario() {
  return tokens.usuario;
}
export function tokenDispositivo() {
  return tokens.dispositivo;
}
/** Se llama cuando el nodo rechaza la sesión del usuario (vencida o cerrada en otro lado). */
export function setAlPerderSesion(fn: (code: string) => void) {
  alPerderSesion = fn;
}

/** La PC del nodo no necesita emparejarse: el nodo confía en las peticiones de loopback. */
export function esPCDelNodo(hostname = location.hostname): boolean {
  return hostname === "localhost" || hostname === "127.0.0.1" || hostname === "[::1]" || hostname === "::1";
}

function credenciales(): string | undefined {
  const c: string[] = [];
  if (tokens.dispositivo) c.push(`Dispositivo ${tokens.dispositivo}`);
  if (tokens.usuario) c.push(`Usuario ${tokens.usuario}`);
  return c.length ? c.join(", ") : undefined;
}

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const auth = credenciales();
  if (auth) headers.Authorization = auth;
  let r: Response;
  try {
    r = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  } catch {
    throw new ApiError(0, "SIN_NODO", "No hay conexión con el Nodo Local. Revisa el cable de red o el WiFi de la caja.");
  }
  if (r.status === 204) return undefined as T;
  let datos: unknown = null;
  try {
    datos = await r.json();
  } catch {
    // respuesta sin cuerpo
  }
  if (!r.ok) {
    const p = (datos ?? {}) as { code?: string; detail?: string; errors?: FieldError[] };
    const e = new ApiError(r.status, p.code ?? "ERROR", p.detail ?? (r.status >= 500 ? "El nodo tuvo un problema. Intenta de nuevo." : "No se pudo completar la acción."), p.errors ?? []);
    if (r.status === 401 && tokens.usuario && !path.startsWith("/v1/sesiones")) alPerderSesion(e.code);
    throw e;
  }
  return datos as T;
}

// ---------- Tipos que devuelve el nodo ----------

export interface Persona {
  id: string;
  nombre: string;
  rol: string;
  avatarUrl: string | null;
  permisos: string[];
}

export interface Usuario {
  id: string;
  nombre: string;
  rol: string;
  permisos: string[];
}

export interface SesionUsuario {
  token: string;
  expiraAt: string;
  usuario: Usuario;
}

export interface Conectividad {
  indicador: "VERDE" | "AMARILLO";
  nube: "EN_LINEA" | "SIN_INTERNET" | "SIN_ACTIVAR" | "REVOCADO";
  ultimaNube?: string;
  dispositivos: number;
  version: string;
}

export interface Bloqueo {
  usuarioId: string;
  usuarioNombre: string;
  dispositivoId: string;
  expiraAt: string;
}

export interface Zona {
  id: string;
  nombre: string;
  orden: number;
}

export type EstadoMesa = "LIBRE" | "OCUPADA" | "POR_PAGAR" | "DEMORADA";

export interface Mesa {
  id: string;
  zonaId: string;
  nombre: string;
  capacidad: number;
  estado: EstadoMesa;
  ordenId: string | null;
  numeroOrden: number | null;
  meseroNombre: string | null;
  abiertaAt: string | null;
  precuentaAt: string | null;
  comensales: number | null;
  platos: number;
  total: string;
  bloqueo: Bloqueo | null;
}

export interface Salon {
  zonas: Zona[];
  mesas: Mesa[];
}

// ---------- Caja (F4-02, F4-03, F4-11) ----------

export interface CajaInfo {
  id: string;
  nombre: string;
  estacionId: string | null;
}

export interface MetodoPago {
  id: string;
  nombre: string;
  tipo: string; // EFECTIVO, TARJETA_CREDITO, TARJETA_DEBITO, TRANSFERENCIA, BILLETERA, OTRO
  codigoSri: string;
  abreCajon: boolean;
  pideReferencia: boolean;
  icono: string;
}

/** Billete o moneda del asistente de cierre (la lista la define el nodo). */
export interface Denominacion {
  clave: string; // B20, M0.25…
  valor: string;
  moneda: boolean;
  etiqueta: string;
}

export interface MotivoDescuento {
  id: string;
  nombre: string;
  tipo: "DESCUENTO" | "CORTESIA";
}

export interface ConfigCaja {
  cajas: CajaInfo[];
  metodos: MetodoPago[];
  motivos?: MotivoDescuento[];
  consumidorFinalMaximo: string;
  denominaciones: Denominacion[];
}

export interface Jornada {
  id: string;
  fechaNegocio: string;
  abiertaAt: string;
  abiertaPor: string;
  cerradaAt?: string;
}

export interface Turno {
  id: string;
  cajaId: string;
  jornadaId: string;
  cajeroId: string;
  cajeroNombre: string;
  fondoInicial: string;
  abiertoAt: string;
  estado: "ABIERTO" | "CERRADO";
}

/** Lo que ve la caja al entrar: nunca trae el esperado del cierre (cierre ciego). */
export interface EstadoCaja {
  caja: CajaInfo;
  jornada: Jornada | null;
  turno: Turno | null;
}

export type TipoMovimiento = "RETIRO" | "INGRESO" | "GASTO";

export interface Movimiento {
  id: string;
  turnoId: string;
  tipo: TipoMovimiento;
  monto: string;
  motivo: string;
  usuarioNombre: string;
  createdAt: string;
}

export interface LineaOrden {
  id: string;
  producto: string;
  cantidad: string;
  modificadores: { nombre: string }[];
  estado: string; // EN_ESPERA, ENVIADA, ANULADA
  total: string;
}

export type TipoOrden = "MESA" | "LLEVAR" | "BARRA" | "DELIVERY";

export interface Orden {
  id: string;
  mesaId: string | null;
  mesa: string; // nombre visible: «Mesa 4» o «Llevar #12 · Ana»
  tipo: TipoOrden;
  etiqueta: string;
  meseroNombre: string;
  numero: number;
  estado: string;
  lineas: LineaOrden[];
}

/** Orden abierta sin mesa (para llevar, barra o delivery). */
export interface OrdenSinMesa {
  id: string;
  tipo: TipoOrden;
  nombre: string;
  etiqueta: string;
  numero: number;
  estado: string;
  meseroNombre: string;
  abiertaAt: string;
  platos: number;
  total: string;
}

export interface Categoria {
  id: string;
  nombre: string;
  icono: string;
  color: string;
  orden: number;
}

export interface Modificador {
  id: string;
  nombre: string;
  precioAdicional: string;
}

export interface GrupoModificadores {
  id: string;
  nombre: string;
  obligatorio: boolean;
  min: number;
  max: number;
  modificadores: Modificador[];
}

export interface Producto {
  id: string;
  categoriaId: string;
  nombre: string;
  alias: string;
  precio: string;
  foto: string | null;
  grupos: string[];
  vendidos: number;
  orden: number;
}

export interface Catalogo {
  version: string;
  categorias: Categoria[];
  productos: Producto[];
  grupos: GrupoModificadores[];
}

export interface LineaNueva {
  id: string;
  productoId: string;
  cantidad: string;
  modificadores: string[];
  nota: string;
}

export interface Totales {
  subtotal: string;
  iva: string;
  propina: string;
  total: string;
  propinaActiva?: boolean; // el local cobra servicio
  propinaPorcentaje?: string;
  propinaRetirada?: boolean; // el cliente lo rechazó y el cajero lo quitó
  descuento?: string; // total descontado
  descuentos?: DescuentoAplicado[];
  lineas?: Record<string, string>; // importe final de cada línea tras descuentos
}

/** Descuento vigente de una línea (lineaId) o de la cuenta (lineaId nulo). */
export interface DescuentoAplicado {
  id: string;
  lineaId: string | null;
  tipo: "PORCENTAJE" | "MONTO";
  valor: string;
  cortesia: boolean;
  motivo: string;
  usuarioNombre: string;
  monto: string;
}

export interface ClienteRegistrado {
  id: string;
  tipoIdentificacion: string;
  identificacion: string;
  razonSocial: string;
  direccion: string;
  email: string;
  telefono: string;
}

/** Validación del nodo + cliente encontrado en la cascada nodo → nube. */
export interface BusquedaCliente {
  valida: boolean;
  tipo: string;
  motivo?: string;
  advertencia?: string;
  cliente: ClienteRegistrado | null;
  origen?: "NODO" | "NUBE";
}

export interface PagoDocumento {
  metodoId: string;
  metodo: string;
  tipo: string;
  monto: string;
  recibido?: string;
  vuelto?: string;
  ultimos4?: string;
}

export interface Asignacion {
  lineaId: string;
  peso: number;
}

/** Una cuenta de la división (F4-08). */
export interface Cuenta {
  id: string;
  numero: number;
  estado: "ABIERTA" | "PAGADA";
  asignaciones: Asignacion[];
  lineas: Record<string, string>; // lo que lleva de cada plato
  subtotal: string;
  iva: string;
  propina: string;
  total: string;
  documento?: string;
}

export interface Division {
  cuentas: Cuenta[];
  sinCuenta: string[];
  total: string;
}

export interface DocumentoVenta {
  id: string;
  cuenta?: number;
  codigo: string; // INT-000123
  mesa: string;
  totales: Totales;
  comprador: string;
  compradorTipo: string; // 07 = consumidor final
  compradorIdentificacion: string;
  clienteGuardado: boolean;
  metodo: string; // «Efectivo + Tarjeta crédito»
  pagos: PagoDocumento[];
  recibido: string;
  vuelto: string;
  abreCajon: boolean;
}

export interface CobroOut {
  documento: DocumentoVenta;
  cerrada?: boolean; // con división: la orden se cierra al cobrar la última cuenta
  impresoras: string[];
  aviso?: string;
}

export type ResultadoCierre = "CUADRADO" | "SOBRANTE" | "FALTANTE";

export interface LineaCierre {
  metodoId: string;
  metodo: string;
  tipo: string;
  diferencia: string;
  resultado: ResultadoCierre;
}

/** Lo que la caja muestra del Cierre Z ya generado: resultados, no el esperado. */
export interface CierreZ {
  id: string;
  numero: number;
  caja: string;
  cajero: string;
  efectivoContado: string;
  lineas: LineaCierre[];
  resultado: ResultadoCierre;
}

export interface CierreOut {
  cierre: CierreZ;
  impresoras: string[];
  aviso?: string;
}

// ---------- Llamadas ----------

export const nodo = {
  salud: () => api<{ estado: string; version: string }>("GET", "/health"),
  conectividad: () => api<Conectividad>("GET", "/v1/conectividad"),
  emparejar: (b: { codigo: string; dispositivoId: string; llavePublica: string; nombre: string; plataforma: string }) =>
    api<{ restaurante: string; local: string }>("POST", "/v1/dispositivos/emparejar", { ...b, tipo: "POS", versionApp: "0.1.0" }),
  desafio: (dispositivoId: string) => api<{ nonce: string }>("GET", `/v1/dispositivos/desafio?dispositivoId=${encodeURIComponent(dispositivoId)}`),
  sesionDispositivo: (b: { dispositivoId: string; nonce: string; firma: string }) => api<{ token: string }>("POST", "/v1/dispositivos/sesion", b),
  personal: () => api<Persona[]>("GET", "/v1/personal"),
  entrar: (usuarioId: string, pin: string) => api<SesionUsuario>("POST", "/v1/sesiones", { usuarioId, pin }),
  salir: () => api<void>("DELETE", "/v1/sesiones"),
  salon: () => api<Salon>("GET", "/v1/salon"),
  configCaja: () => api<ConfigCaja>("GET", "/v1/caja/config"),
  estadoCaja: (cajaId: string) => api<EstadoCaja>("GET", `/v1/cajas/${cajaId}/estado`),
  abrirJornada: () => api<Jornada>("POST", "/v1/jornada/abrir", {}),
  cerrarJornada: (transferirOrdenes: boolean) => api<Jornada>("POST", "/v1/jornada/cerrar", { transferirOrdenes }),
  abrirTurno: (cajaId: string, fondoInicial: string) => api<Turno>("POST", "/v1/turnos", { cajaId, fondoInicial }),
  movimiento: (b: { cajaId: string; tipo: TipoMovimiento; monto: string; motivo: string; idempotencyKey: string }) => api<Movimiento>("POST", "/v1/caja/movimientos", b),
  cerrarTurno: (b: { cajaId: string; conteo: { clave: string; cantidad: number }[]; declarado: { metodoId: string; monto: string }[]; idempotencyKey: string }) =>
    api<CierreOut>("POST", "/v1/turnos/cerrar", b),
  catalogo: () => api<Catalogo>("GET", "/v1/catalogo"),
  ordenesSinMesa: () => api<OrdenSinMesa[]>("GET", "/v1/ordenes/sin-mesa"),
  enviarOrden: (b: { idempotencyKey: string; ordenId: string; tipo: TipoOrden; etiqueta: string; lineas: LineaNueva[] }) =>
    api<{ orden: Orden; comandaNumero: number }>("POST", "/v1/ordenes/enviar", b),
  buscarCliente: (identificacion: string, tipo = "") =>
    api<BusquedaCliente>("GET", `/v1/clientes/buscar?${new URLSearchParams({ identificacion, tipo })}`),
  cuentas: (ordenId: string) => api<Division>("GET", `/v1/ordenes/${ordenId}/cuentas`),
  dividir: (ordenId: string, cuentas: { asignaciones: Asignacion[] }[]) => api<Division>("PUT", `/v1/ordenes/${ordenId}/cuentas`, { cuentas }),
  descontar: (ordenId: string, b: { lineaId: string | null; tipo: string; valor: string; cortesia: boolean; motivoId: string; autorizacion: string }) =>
    api<Totales>("POST", `/v1/ordenes/${ordenId}/descuentos`, b),
  quitarDescuento: (ordenId: string, id: string) => api<Totales>("DELETE", `/v1/ordenes/${ordenId}/descuentos/${id}`),
  propina: (ordenId: string, retirar: boolean, motivo = "") => api<Totales>("POST", `/v1/ordenes/${ordenId}/propina`, { retirar, motivo }),
  orden: (id: string) => api<{ orden: Orden; totales: Totales }>("GET", `/v1/ordenes/${id}`),
  cobrar: (
    ordenId: string,
    b: {
      cajaId: string;
      consumidorFinal: boolean;
      idempotencyKey: string;
      metodoId?: string;
      recibido?: string;
      pagos?: { metodoId: string; monto: string; recibido: string; referencia: string; lote: string; ultimos4: string }[];
      comprador?: { tipoIdentificacion: string; identificacion: string; razonSocial: string; email: string; direccion: string; telefono: string; consentimiento: boolean };
      cuentaId?: string;
    },
  ) =>
    api<CobroOut>("POST", `/v1/ordenes/${ordenId}/cobrar`, b),
  autorizar: (b: { usuarioId: string; pin: string; accion: string; referencia: string }) =>
    api<{ token: string; expiraAt: string; autorizadoPor: string }>("POST", "/v1/autorizaciones", b),
  abrirCajon: (b: { cajaId: string; motivo: string; autorizacion: string }) =>
    api<{ impresora: string; autorizadoPor?: string; aviso?: string }>("POST", "/v1/caja/cajon", b),
  movimientos: (turnoId: string) => api<Movimiento[]>("GET", `/v1/turnos/${turnoId}/movimientos`),
};
