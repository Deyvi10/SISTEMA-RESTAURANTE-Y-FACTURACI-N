// GENERADO por tools/contracts-gen desde contracts/events. No editar: ejecuta `make contracts`.

/** Sobre de todo mensaje WebSocket (docs/03 §6). */
export interface Sobre<T extends string = string, D = unknown> {
  v: number;
  id: string;
  type: T;
  ts: string;
  data: D;
}

/** catalog.updated: Cambió el catálogo, el salón o el personal; los clientes recargan lo afectado. */
export interface CatalogUpdated {
  /** Fue un volcado completo */
  full: boolean;
  /** Tablas que cambiaron */
  tables: string[];
}

/** device.revoked: Un dispositivo fue revocado: debe desconectarse y volver a emparejarse. */
export interface DeviceRevoked {
  device_id: string;
}

/** fiscal.status_changed: Cambió el estado de un comprobante electrónico. */
export interface FiscalStatusChanged {
  /** Clave de acceso de 49 dígitos */
  access_key?: string;
  invoice_id: string;
  message?: string;
  status: "EMITIDO" | "ENVIADO" | "AUTORIZADO" | "NO_AUTORIZADO" | "REQUIERE_ATENCION" | "ANULADO";
}

/** kitchen.ticket_ready: Cocina marcó una comanda como lista. */
export interface KitchenTicketReady {
  comanda_id: string;
  order_id: string;
  station_id: string;
  table_id?: string;
}

/** order.line_voided: Se anuló una línea ya enviada (imprime ticket de ANULACIÓN). */
export interface OrderLineVoided {
  authorized_by?: string;
  by_user_id: string;
  line_id: string;
  order_id: string;
  reason: string;
  station_id?: string;
}

/** order.submitted: Se envió una comanda a producción. */
export interface OrderSubmitted {
  by_user_id: string;
  comanda_id: string;
  /** Líneas enviadas */
  lines: number;
  /** Número de comanda de la jornada */
  number: number;
  order_id: string;
  table_id?: string;
}

/** printer.status: Cambió el estado de una impresora. */
export interface PrinterStatus {
  name: string;
  printer_id: string;
  /** Trabajos pendientes */
  queue: number;
  station_ids?: string[];
  status: "OK" | "SIN_PAPEL" | "POCO_PAPEL" | "TAPA_ABIERTA" | "SIN_CONEXION" | "ERROR";
}

/** stock.changed: Cambió el disponible de un producto con cupo. */
export interface StockChanged {
  available: string;
  product_id: string;
  sold_out: boolean;
}

/** stock.released: Se liberó una reserva de cupo. */
export interface StockReleased {
  line_id: string;
  product_id: string;
}

/** stock.reserve: Reserva de cupo diario de un producto para una línea. */
export interface StockReserve {
  line_id: string;
  product_id: string;
  /** Cantidad exacta en texto decimal */
  quantity: string;
}

/** table.heartbeat: El dispositivo que tiene la mesa confirma que sigue trabajando en ella. */
export interface TableHeartbeat {
  device_id: string;
  table_id: string;
}

/** table.lock.request: Un dispositivo pide bloquear una mesa para tomar el pedido. */
export interface TableLockRequest {
  /** Dispositivo que pide */
  device_id: string;
  /** Mesa */
  table_id: string;
}

/** table.locked: La mesa quedó bloqueada por un mesero (nadie más puede editarla). */
export interface TableLocked {
  /** Nombre visible del mesero */
  by_name: string;
  by_user_id: string;
  device_id?: string;
  /** Vence si no llega heartbeat (45 s) */
  expires_at: string;
  table_id: string;
}

/** table.state_changed: Cambió el estado visible de una mesa en el mapa. */
export interface TableStateChanged {
  guests?: number;
  order_id?: string;
  state: "LIBRE" | "OCUPADA" | "POR_PAGAR" | "DEMORADA" | "BLOQUEADA";
  table_id: string;
}

/** table.unlocked: La mesa se liberó (por el mesero, por vencimiento o por un supervisor). */
export interface TableUnlocked {
  reason: "RELEASED" | "EXPIRED" | "FORCED";
  table_id: string;
}

/** user.deactivated: Un usuario fue desactivado: sus dispositivos deben cerrar su sesión. */
export interface UserDeactivated {
  user_id: string;
}

/** Todos los eventos, discriminados por `type`. */
export type Evento =
  | Sobre<"catalog.updated", CatalogUpdated>
  | Sobre<"device.revoked", DeviceRevoked>
  | Sobre<"fiscal.status_changed", FiscalStatusChanged>
  | Sobre<"kitchen.ticket_ready", KitchenTicketReady>
  | Sobre<"order.line_voided", OrderLineVoided>
  | Sobre<"order.submitted", OrderSubmitted>
  | Sobre<"printer.status", PrinterStatus>
  | Sobre<"stock.changed", StockChanged>
  | Sobre<"stock.released", StockReleased>
  | Sobre<"stock.reserve", StockReserve>
  | Sobre<"table.heartbeat", TableHeartbeat>
  | Sobre<"table.lock.request", TableLockRequest>
  | Sobre<"table.locked", TableLocked>
  | Sobre<"table.state_changed", TableStateChanged>
  | Sobre<"table.unlocked", TableUnlocked>
  | Sobre<"user.deactivated", UserDeactivated>;

export const VERSIONES: Record<Evento["type"], number> = {
  "catalog.updated": 1,
  "device.revoked": 1,
  "fiscal.status_changed": 1,
  "kitchen.ticket_ready": 1,
  "order.line_voided": 1,
  "order.submitted": 1,
  "printer.status": 1,
  "stock.changed": 1,
  "stock.released": 1,
  "stock.reserve": 1,
  "table.heartbeat": 1,
  "table.lock.request": 1,
  "table.locked": 1,
  "table.state_changed": 1,
  "table.unlocked": 1,
  "user.deactivated": 1,
};
