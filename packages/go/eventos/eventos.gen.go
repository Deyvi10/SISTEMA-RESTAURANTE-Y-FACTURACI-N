// GENERADO por tools/contracts-gen desde contracts/events. No editar: ejecuta `make contracts`.

package eventos

import (
	"time"

	"github.com/Deyvi10/SISTEMA-RESTAURANTE-Y-FACTURACI-N/packages/go/ids"
)

// Nombres de los eventos.
const (
	TipoCatalogUpdated      = "catalog.updated"
	TipoDeviceRevoked       = "device.revoked"
	TipoFiscalStatusChanged = "fiscal.status_changed"
	TipoKitchenTicketReady  = "kitchen.ticket_ready"
	TipoOrderLineVoided     = "order.line_voided"
	TipoOrderSubmitted      = "order.submitted"
	TipoPrinterStatus       = "printer.status"
	TipoStockChanged        = "stock.changed"
	TipoStockReleased       = "stock.released"
	TipoStockReserve        = "stock.reserve"
	TipoTableHeartbeat      = "table.heartbeat"
	TipoTableLockRequest    = "table.lock.request"
	TipoTableLocked         = "table.locked"
	TipoTableStateChanged   = "table.state_changed"
	TipoTableUnlocked       = "table.unlocked"
	TipoUserDeactivated     = "user.deactivated"
)

// Versiones vigentes de cada evento (campo v del sobre).
var Versiones = map[string]int{
	TipoCatalogUpdated:      1,
	TipoDeviceRevoked:       1,
	TipoFiscalStatusChanged: 1,
	TipoKitchenTicketReady:  1,
	TipoOrderLineVoided:     1,
	TipoOrderSubmitted:      1,
	TipoPrinterStatus:       1,
	TipoStockChanged:        1,
	TipoStockReleased:       1,
	TipoStockReserve:        1,
	TipoTableHeartbeat:      1,
	TipoTableLockRequest:    1,
	TipoTableLocked:         1,
	TipoTableStateChanged:   1,
	TipoTableUnlocked:       1,
	TipoUserDeactivated:     1,
}

// catalog.updated: Cambió el catálogo, el salón o el personal; los clientes recargan lo afectado.
type CatalogUpdated struct {
	Full   bool     `json:"full"`   // Fue un volcado completo
	Tables []string `json:"tables"` // Tablas que cambiaron
}

// Tipo devuelve el nombre del evento.
func (CatalogUpdated) Tipo() string { return TipoCatalogUpdated }

// device.revoked: Un dispositivo fue revocado: debe desconectarse y volver a emparejarse.
type DeviceRevoked struct {
	DeviceID ids.ID `json:"device_id"`
}

// Tipo devuelve el nombre del evento.
func (DeviceRevoked) Tipo() string { return TipoDeviceRevoked }

// fiscal.status_changed: Cambió el estado de un comprobante electrónico.
type FiscalStatusChanged struct {
	AccessKey *string `json:"access_key,omitempty"` // Clave de acceso de 49 dígitos
	InvoiceID ids.ID  `json:"invoice_id"`
	Message   *string `json:"message,omitempty"`
	Status    string  `json:"status"` // EMITIDO, ENVIADO, AUTORIZADO, NO_AUTORIZADO, REQUIERE_ATENCION, ANULADO
}

// Tipo devuelve el nombre del evento.
func (FiscalStatusChanged) Tipo() string { return TipoFiscalStatusChanged }

// kitchen.ticket_ready: Cocina marcó una comanda como lista.
type KitchenTicketReady struct {
	ComandaID ids.ID  `json:"comanda_id"`
	OrderID   ids.ID  `json:"order_id"`
	StationID ids.ID  `json:"station_id"`
	TableID   *ids.ID `json:"table_id,omitempty"`
}

// Tipo devuelve el nombre del evento.
func (KitchenTicketReady) Tipo() string { return TipoKitchenTicketReady }

// order.line_voided: Se anuló una línea ya enviada (imprime ticket de ANULACIÓN).
type OrderLineVoided struct {
	AuthorizedBy *ids.ID `json:"authorized_by,omitempty"`
	ByUserID     ids.ID  `json:"by_user_id"`
	LineID       ids.ID  `json:"line_id"`
	OrderID      ids.ID  `json:"order_id"`
	Reason       string  `json:"reason"`
	StationID    *ids.ID `json:"station_id,omitempty"`
}

// Tipo devuelve el nombre del evento.
func (OrderLineVoided) Tipo() string { return TipoOrderLineVoided }

// order.submitted: Se envió una comanda a producción.
type OrderSubmitted struct {
	ByUserID  ids.ID  `json:"by_user_id"`
	ComandaID ids.ID  `json:"comanda_id"`
	Lines     int64   `json:"lines"`  // Líneas enviadas
	Number    int64   `json:"number"` // Número de comanda de la jornada
	OrderID   ids.ID  `json:"order_id"`
	TableID   *ids.ID `json:"table_id,omitempty"`
}

// Tipo devuelve el nombre del evento.
func (OrderSubmitted) Tipo() string { return TipoOrderSubmitted }

// printer.status: Cambió el estado de una impresora.
type PrinterStatus struct {
	Name       string   `json:"name"`
	PrinterID  ids.ID   `json:"printer_id"`
	Queue      int64    `json:"queue"` // Trabajos pendientes
	StationIDs []ids.ID `json:"station_ids,omitempty"`
	Status     string   `json:"status"` // OK, SIN_PAPEL, POCO_PAPEL, TAPA_ABIERTA, SIN_CONEXION, ERROR
}

// Tipo devuelve el nombre del evento.
func (PrinterStatus) Tipo() string { return TipoPrinterStatus }

// stock.changed: Cambió el disponible de un producto con cupo.
type StockChanged struct {
	Available string `json:"available"`
	ProductID ids.ID `json:"product_id"`
	SoldOut   bool   `json:"sold_out"`
}

// Tipo devuelve el nombre del evento.
func (StockChanged) Tipo() string { return TipoStockChanged }

// stock.released: Se liberó una reserva de cupo.
type StockReleased struct {
	LineID    ids.ID `json:"line_id"`
	ProductID ids.ID `json:"product_id"`
}

// Tipo devuelve el nombre del evento.
func (StockReleased) Tipo() string { return TipoStockReleased }

// stock.reserve: Reserva de cupo diario de un producto para una línea.
type StockReserve struct {
	LineID    ids.ID `json:"line_id"`
	ProductID ids.ID `json:"product_id"`
	Quantity  string `json:"quantity"` // Cantidad exacta en texto decimal
}

// Tipo devuelve el nombre del evento.
func (StockReserve) Tipo() string { return TipoStockReserve }

// table.heartbeat: El dispositivo que tiene la mesa confirma que sigue trabajando en ella.
type TableHeartbeat struct {
	DeviceID ids.ID `json:"device_id"`
	TableID  ids.ID `json:"table_id"`
}

// Tipo devuelve el nombre del evento.
func (TableHeartbeat) Tipo() string { return TipoTableHeartbeat }

// table.lock.request: Un dispositivo pide bloquear una mesa para tomar el pedido.
type TableLockRequest struct {
	DeviceID ids.ID `json:"device_id"` // Dispositivo que pide
	TableID  ids.ID `json:"table_id"`  // Mesa
}

// Tipo devuelve el nombre del evento.
func (TableLockRequest) Tipo() string { return TipoTableLockRequest }

// table.locked: La mesa quedó bloqueada por un mesero (nadie más puede editarla).
type TableLocked struct {
	ByName    string    `json:"by_name"` // Nombre visible del mesero
	ByUserID  ids.ID    `json:"by_user_id"`
	DeviceID  *ids.ID   `json:"device_id,omitempty"`
	ExpiresAt time.Time `json:"expires_at"` // Vence si no llega heartbeat (45 s)
	TableID   ids.ID    `json:"table_id"`
}

// Tipo devuelve el nombre del evento.
func (TableLocked) Tipo() string { return TipoTableLocked }

// table.state_changed: Cambió el estado visible de una mesa en el mapa.
type TableStateChanged struct {
	Guests  *int64  `json:"guests,omitempty"`
	OrderID *ids.ID `json:"order_id,omitempty"`
	State   string  `json:"state"` // LIBRE, OCUPADA, POR_PAGAR, DEMORADA, BLOQUEADA
	TableID ids.ID  `json:"table_id"`
}

// Tipo devuelve el nombre del evento.
func (TableStateChanged) Tipo() string { return TipoTableStateChanged }

// table.unlocked: La mesa se liberó (por el mesero, por vencimiento o por un supervisor).
type TableUnlocked struct {
	Reason  string `json:"reason"` // RELEASED, EXPIRED, FORCED
	TableID ids.ID `json:"table_id"`
}

// Tipo devuelve el nombre del evento.
func (TableUnlocked) Tipo() string { return TipoTableUnlocked }

// user.deactivated: Un usuario fue desactivado: sus dispositivos deben cerrar su sesión.
type UserDeactivated struct {
	UserID ids.ID `json:"user_id"`
}

// Tipo devuelve el nombre del evento.
func (UserDeactivated) Tipo() string { return TipoUserDeactivated }
