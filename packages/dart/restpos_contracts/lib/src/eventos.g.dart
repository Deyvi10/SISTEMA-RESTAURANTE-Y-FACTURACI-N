// GENERADO por tools/contracts-gen desde contracts/events. No editar: ejecuta `make contracts`.
// ignore_for_file: public_member_api_docs, sort_constructors_first

/// Versiones vigentes de cada evento.
const versionesEventos = <String, int>{
  'catalog.updated': 1,
  'device.revoked': 1,
  'fiscal.status_changed': 1,
  'kitchen.ticket_ready': 1,
  'order.line_voided': 1,
  'order.submitted': 1,
  'printer.status': 1,
  'stock.changed': 1,
  'stock.released': 1,
  'stock.reserve': 1,
  'table.heartbeat': 1,
  'table.lock.request': 1,
  'table.locked': 1,
  'table.state_changed': 1,
  'table.unlocked': 1,
  'user.deactivated': 1,
};

/// catalog.updated: Cambió el catálogo, el salón o el personal; los clientes recargan lo afectado.
class CatalogUpdated {
  static const tipo = 'catalog.updated';
  final bool full;
  final List<String> tables;

  const CatalogUpdated({required this.full, required this.tables});

  factory CatalogUpdated.fromJson(Map<String, dynamic> j) => CatalogUpdated(
        full: j['full'] as bool,
        tables: (j['tables'] as List).map((e) => e as String).toList(),
      );

  Map<String, dynamic> toJson() => {
        'full': full,
        'tables': tables,
      };
}

/// device.revoked: Un dispositivo fue revocado: debe desconectarse y volver a emparejarse.
class DeviceRevoked {
  static const tipo = 'device.revoked';
  final String deviceID;

  const DeviceRevoked({required this.deviceID});

  factory DeviceRevoked.fromJson(Map<String, dynamic> j) => DeviceRevoked(
        deviceID: j['device_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        'device_id': deviceID,
      };
}

/// fiscal.status_changed: Cambió el estado de un comprobante electrónico.
class FiscalStatusChanged {
  static const tipo = 'fiscal.status_changed';
  final String? accessKey;
  final String invoiceID;
  final String? message;
  final String status;

  const FiscalStatusChanged({this.accessKey, required this.invoiceID, this.message, required this.status});

  factory FiscalStatusChanged.fromJson(Map<String, dynamic> j) => FiscalStatusChanged(
        accessKey: j['access_key'] == null ? null : j['access_key'] as String,
        invoiceID: j['invoice_id'] as String,
        message: j['message'] == null ? null : j['message'] as String,
        status: j['status'] as String,
      );

  Map<String, dynamic> toJson() => {
        if (accessKey != null) 'access_key': accessKey!,
        'invoice_id': invoiceID,
        if (message != null) 'message': message!,
        'status': status,
      };
}

/// kitchen.ticket_ready: Cocina marcó una comanda como lista.
class KitchenTicketReady {
  static const tipo = 'kitchen.ticket_ready';
  final String comandaID;
  final String orderID;
  final String stationID;
  final String? tableID;

  const KitchenTicketReady({required this.comandaID, required this.orderID, required this.stationID, this.tableID});

  factory KitchenTicketReady.fromJson(Map<String, dynamic> j) => KitchenTicketReady(
        comandaID: j['comanda_id'] as String,
        orderID: j['order_id'] as String,
        stationID: j['station_id'] as String,
        tableID: j['table_id'] == null ? null : j['table_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        'comanda_id': comandaID,
        'order_id': orderID,
        'station_id': stationID,
        if (tableID != null) 'table_id': tableID!,
      };
}

/// order.line_voided: Se anuló una línea ya enviada (imprime ticket de ANULACIÓN).
class OrderLineVoided {
  static const tipo = 'order.line_voided';
  final String? authorizedBy;
  final String byUserID;
  final String lineID;
  final String orderID;
  final String reason;
  final String? stationID;

  const OrderLineVoided({this.authorizedBy, required this.byUserID, required this.lineID, required this.orderID, required this.reason, this.stationID});

  factory OrderLineVoided.fromJson(Map<String, dynamic> j) => OrderLineVoided(
        authorizedBy: j['authorized_by'] == null ? null : j['authorized_by'] as String,
        byUserID: j['by_user_id'] as String,
        lineID: j['line_id'] as String,
        orderID: j['order_id'] as String,
        reason: j['reason'] as String,
        stationID: j['station_id'] == null ? null : j['station_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        if (authorizedBy != null) 'authorized_by': authorizedBy!,
        'by_user_id': byUserID,
        'line_id': lineID,
        'order_id': orderID,
        'reason': reason,
        if (stationID != null) 'station_id': stationID!,
      };
}

/// order.submitted: Se envió una comanda a producción.
class OrderSubmitted {
  static const tipo = 'order.submitted';
  final String byUserID;
  final String comandaID;
  final int lines;
  final int number;
  final String orderID;
  final String? tableID;

  const OrderSubmitted({required this.byUserID, required this.comandaID, required this.lines, required this.number, required this.orderID, this.tableID});

  factory OrderSubmitted.fromJson(Map<String, dynamic> j) => OrderSubmitted(
        byUserID: j['by_user_id'] as String,
        comandaID: j['comanda_id'] as String,
        lines: j['lines'] as int,
        number: j['number'] as int,
        orderID: j['order_id'] as String,
        tableID: j['table_id'] == null ? null : j['table_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        'by_user_id': byUserID,
        'comanda_id': comandaID,
        'lines': lines,
        'number': number,
        'order_id': orderID,
        if (tableID != null) 'table_id': tableID!,
      };
}

/// printer.status: Cambió el estado de una impresora.
class PrinterStatus {
  static const tipo = 'printer.status';
  final String name;
  final String printerID;
  final int queue;
  final List<String>? stationIDs;
  final String status;

  const PrinterStatus({required this.name, required this.printerID, required this.queue, this.stationIDs, required this.status});

  factory PrinterStatus.fromJson(Map<String, dynamic> j) => PrinterStatus(
        name: j['name'] as String,
        printerID: j['printer_id'] as String,
        queue: j['queue'] as int,
        stationIDs: j['station_ids'] == null ? null : (j['station_ids'] as List).map((e) => e as String).toList(),
        status: j['status'] as String,
      );

  Map<String, dynamic> toJson() => {
        'name': name,
        'printer_id': printerID,
        'queue': queue,
        if (stationIDs != null) 'station_ids': stationIDs!,
        'status': status,
      };
}

/// stock.changed: Cambió el disponible de un producto con cupo.
class StockChanged {
  static const tipo = 'stock.changed';
  final String available;
  final String productID;
  final bool soldOut;

  const StockChanged({required this.available, required this.productID, required this.soldOut});

  factory StockChanged.fromJson(Map<String, dynamic> j) => StockChanged(
        available: j['available'] as String,
        productID: j['product_id'] as String,
        soldOut: j['sold_out'] as bool,
      );

  Map<String, dynamic> toJson() => {
        'available': available,
        'product_id': productID,
        'sold_out': soldOut,
      };
}

/// stock.released: Se liberó una reserva de cupo.
class StockReleased {
  static const tipo = 'stock.released';
  final String lineID;
  final String productID;

  const StockReleased({required this.lineID, required this.productID});

  factory StockReleased.fromJson(Map<String, dynamic> j) => StockReleased(
        lineID: j['line_id'] as String,
        productID: j['product_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        'line_id': lineID,
        'product_id': productID,
      };
}

/// stock.reserve: Reserva de cupo diario de un producto para una línea.
class StockReserve {
  static const tipo = 'stock.reserve';
  final String lineID;
  final String productID;
  final String quantity;

  const StockReserve({required this.lineID, required this.productID, required this.quantity});

  factory StockReserve.fromJson(Map<String, dynamic> j) => StockReserve(
        lineID: j['line_id'] as String,
        productID: j['product_id'] as String,
        quantity: j['quantity'] as String,
      );

  Map<String, dynamic> toJson() => {
        'line_id': lineID,
        'product_id': productID,
        'quantity': quantity,
      };
}

/// table.heartbeat: El dispositivo que tiene la mesa confirma que sigue trabajando en ella.
class TableHeartbeat {
  static const tipo = 'table.heartbeat';
  final String deviceID;
  final String tableID;

  const TableHeartbeat({required this.deviceID, required this.tableID});

  factory TableHeartbeat.fromJson(Map<String, dynamic> j) => TableHeartbeat(
        deviceID: j['device_id'] as String,
        tableID: j['table_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        'device_id': deviceID,
        'table_id': tableID,
      };
}

/// table.lock.request: Un dispositivo pide bloquear una mesa para tomar el pedido.
class TableLockRequest {
  static const tipo = 'table.lock.request';
  final String deviceID;
  final String tableID;

  const TableLockRequest({required this.deviceID, required this.tableID});

  factory TableLockRequest.fromJson(Map<String, dynamic> j) => TableLockRequest(
        deviceID: j['device_id'] as String,
        tableID: j['table_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        'device_id': deviceID,
        'table_id': tableID,
      };
}

/// table.locked: La mesa quedó bloqueada por un mesero (nadie más puede editarla).
class TableLocked {
  static const tipo = 'table.locked';
  final String byName;
  final String byUserID;
  final String? deviceID;
  final DateTime expiresAt;
  final String tableID;

  const TableLocked({required this.byName, required this.byUserID, this.deviceID, required this.expiresAt, required this.tableID});

  factory TableLocked.fromJson(Map<String, dynamic> j) => TableLocked(
        byName: j['by_name'] as String,
        byUserID: j['by_user_id'] as String,
        deviceID: j['device_id'] == null ? null : j['device_id'] as String,
        expiresAt: DateTime.parse(j['expires_at'] as String),
        tableID: j['table_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        'by_name': byName,
        'by_user_id': byUserID,
        if (deviceID != null) 'device_id': deviceID!,
        'expires_at': expiresAt.toUtc().toIso8601String(),
        'table_id': tableID,
      };
}

/// table.state_changed: Cambió el estado visible de una mesa en el mapa.
class TableStateChanged {
  static const tipo = 'table.state_changed';
  final int? guests;
  final String? orderID;
  final String state;
  final String tableID;

  const TableStateChanged({this.guests, this.orderID, required this.state, required this.tableID});

  factory TableStateChanged.fromJson(Map<String, dynamic> j) => TableStateChanged(
        guests: j['guests'] == null ? null : j['guests'] as int,
        orderID: j['order_id'] == null ? null : j['order_id'] as String,
        state: j['state'] as String,
        tableID: j['table_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        if (guests != null) 'guests': guests!,
        if (orderID != null) 'order_id': orderID!,
        'state': state,
        'table_id': tableID,
      };
}

/// table.unlocked: La mesa se liberó (por el mesero, por vencimiento o por un supervisor).
class TableUnlocked {
  static const tipo = 'table.unlocked';
  final String reason;
  final String tableID;

  const TableUnlocked({required this.reason, required this.tableID});

  factory TableUnlocked.fromJson(Map<String, dynamic> j) => TableUnlocked(
        reason: j['reason'] as String,
        tableID: j['table_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        'reason': reason,
        'table_id': tableID,
      };
}

/// user.deactivated: Un usuario fue desactivado: sus dispositivos deben cerrar su sesión.
class UserDeactivated {
  static const tipo = 'user.deactivated';
  final String userID;

  const UserDeactivated({required this.userID});

  factory UserDeactivated.fromJson(Map<String, dynamic> j) => UserDeactivated(
        userID: j['user_id'] as String,
      );

  Map<String, dynamic> toJson() => {
        'user_id': userID,
      };
}
