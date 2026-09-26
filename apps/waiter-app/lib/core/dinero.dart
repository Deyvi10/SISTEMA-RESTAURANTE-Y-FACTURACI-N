import 'package:decimal/decimal.dart';

/// Dinero exacto: nunca double (regla del proyecto). El nodo envía importes como texto.
abstract final class Dinero {
  static Decimal parse(String s) => Decimal.tryParse(s.trim()) ?? Decimal.zero;

  /// «$12.50» con dos decimales, redondeo half-up.
  static String formato(Decimal d) {
    final r = d.round(scale: 2);
    final neg = r < Decimal.zero;
    final s = r.abs().toStringAsFixed(2);
    final partes = s.split('.');
    final entero = partes[0].replaceAllMapped(RegExp(r'\B(?=(\d{3})+(?!\d))'), (_) => ',');
    return '${neg ? '-' : ''}\$$entero.${partes[1]}';
  }

  static String texto(String s) => formato(parse(s));

  /// Total de una línea: (precio + adicionales) × cantidad.
  static Decimal totalLinea(String precio, Iterable<String> adicionales, String cantidad) {
    var p = parse(precio);
    for (final a in adicionales) {
      p += parse(a);
    }
    return (p * parse(cantidad)).round(scale: 2);
  }
}
