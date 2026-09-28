/// Error del Nodo Local (RFC 9457) o de red, con un mensaje listo para el mesero.
class ApiError implements Exception {
  const ApiError(this.status, this.code, this.detalle);
  factory ApiError.sinNodo() => const ApiError(0, 'SIN_NODO', 'No hay conexión con el Nodo Local. Revisa que estés en el WiFi del restaurante.');

  final int status;
  final String code;
  final String detalle;

  bool get esRed => status == 0;
  bool get sesionVencida => status == 401;

  @override
  String toString() => 'ApiError($status $code: $detalle)';
}
