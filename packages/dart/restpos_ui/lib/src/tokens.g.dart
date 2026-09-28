// GENERADO por tools/design-tokens desde packages/design/*.json. No editar: ejecuta `make tokens`.
// ignore_for_file: public_member_api_docs

import 'package:flutter/cupertino.dart';

/// Colores semánticos del sistema de diseño.
@immutable
class RpColors {
  const RpColors({
    required this.accent,
    required this.accentFill,
    required this.accentSoft,
    required this.background,
    required this.bar,
    required this.dangerFill,
    required this.dangerSoft,
    required this.dangerText,
    required this.fill,
    required this.fillSubtle,
    required this.label,
    required this.labelSecondary,
    required this.labelTertiary,
    required this.onAccent,
    required this.onDanger,
    required this.scrim,
    required this.separator,
    required this.successSoft,
    required this.successText,
    required this.surface,
    required this.surfaceRaised,
    required this.switchOn,
    required this.warningSoft,
    required this.warningText,
  });

  /// Enlaces, acciones de texto, iconos interactivos
  final Color accent;
  /// Fondo del botón principal
  final Color accentFill;
  /// Fondo de botones teñidos y selección suave
  final Color accentSoft;
  /// Fondo agrupado de pantallas y listas
  final Color background;
  /// Barras de navegación y pestañas translúcidas
  final Color bar;
  /// Fondo del botón destructivo y del swipe para borrar
  final Color dangerFill;
  /// Fondo de píldoras y avisos de error
  final Color dangerSoft;
  /// Texto de error, acciones destructivas, «Faltante»
  final Color dangerText;
  /// Campos de búsqueda, controles segmentados, botones secundarios
  final Color fill;
  /// Hover y filas presionadas
  final Color fillSubtle;
  /// Texto principal
  final Color label;
  /// Texto secundario, subtítulos. Más oscuro que iOS para cumplir AA
  final Color labelSecondary;
  /// Solo marcadores de posición y estados deshabilitados (exentos de AA)
  final Color labelTertiary;
  /// Texto sobre accentFill
  final Color onAccent;
  /// Texto sobre dangerFill
  final Color onDanger;
  /// Fondo detrás de una hoja modal
  final Color scrim;
  /// Separadores finos (0,5 px) de listas
  final Color separator;
  /// Fondo de píldoras y avisos de éxito
  final Color successSoft;
  /// Texto de éxito, «Cuadrado», «Autorizado»
  final Color successText;
  /// Celdas, tarjetas y listas agrupadas
  final Color surface;
  /// Hojas, popovers y elementos sobre una superficie
  final Color surfaceRaised;
  /// Toggle activado (no lleva texto)
  final Color switchOn;
  /// Fondo de píldoras y avisos de advertencia
  final Color warningSoft;
  /// Texto de advertencia, «Queda poco»
  final Color warningText;

  static const light = RpColors(
    accent: Color(0xFF0066CC),
    accentFill: Color(0xFF0066CC),
    accentSoft: Color(0xFFE5F0FB),
    background: Color(0xFFF2F2F7),
    bar: Color(0xCCF9F9F9),
    dangerFill: Color(0xFFD70015),
    dangerSoft: Color(0xFFFDE8EA),
    dangerText: Color(0xFFD70015),
    fill: Color(0x1F767680),
    fillSubtle: Color(0x14767680),
    label: Color(0xFF000000),
    labelSecondary: Color(0xFF636366),
    labelTertiary: Color(0xFFAEAEB2),
    onAccent: Color(0xFFFFFFFF),
    onDanger: Color(0xFFFFFFFF),
    scrim: Color(0x47000000),
    separator: Color(0x4A3C3C43),
    successSoft: Color(0xFFE6F6EA),
    successText: Color(0xFF1B7A33),
    surface: Color(0xFFFFFFFF),
    surfaceRaised: Color(0xFFFFFFFF),
    switchOn: Color(0xFF34C759),
    warningSoft: Color(0xFFFFF1E0),
    warningText: Color(0xFFB34700),
  );

  static const dark = RpColors(
    accent: Color(0xFF409CFF),
    accentFill: Color(0xFF0071E3),
    accentSoft: Color(0xFF10283F),
    background: Color(0xFF000000),
    bar: Color(0xBD161618),
    dangerFill: Color(0xFFD70015),
    dangerSoft: Color(0xFF3F1B1B),
    dangerText: Color(0xFFFF6961),
    fill: Color(0x3D787880),
    fillSubtle: Color(0x29787880),
    label: Color(0xFFFFFFFF),
    labelSecondary: Color(0xFFAEAEB2),
    labelTertiary: Color(0xFF636366),
    onAccent: Color(0xFFFFFFFF),
    onDanger: Color(0xFFFFFFFF),
    scrim: Color(0x8C000000),
    separator: Color(0x99545458),
    successSoft: Color(0xFF173D22),
    successText: Color(0xFF30D158),
    surface: Color(0xFF1C1C1E),
    surfaceRaised: Color(0xFF2C2C2E),
    switchOn: Color(0xFF30D158),
    warningSoft: Color(0xFF3D2C12),
    warningText: Color(0xFFFF9F0A),
  );
}

/// Colores de sistema de iOS para fondos, iconos y mosaicos.
@immutable
class RpTints {
  const RpTints({
    required this.blue,
    required this.gray,
    required this.green,
    required this.indigo,
    required this.orange,
    required this.pink,
    required this.purple,
    required this.red,
    required this.teal,
    required this.yellow,
  });

  final Color blue;
  final Color gray;
  final Color green;
  final Color indigo;
  final Color orange;
  final Color pink;
  final Color purple;
  final Color red;
  final Color teal;
  final Color yellow;

  static const light = RpTints(
    blue: Color(0xFF007AFF),
    gray: Color(0xFF8E8E93),
    green: Color(0xFF34C759),
    indigo: Color(0xFF5856D6),
    orange: Color(0xFFFF9500),
    pink: Color(0xFFFF2D55),
    purple: Color(0xFFAF52DE),
    red: Color(0xFFFF3B30),
    teal: Color(0xFF30B0C7),
    yellow: Color(0xFFFFCC00),
  );

  static const dark = RpTints(
    blue: Color(0xFF0A84FF),
    gray: Color(0xFF98989D),
    green: Color(0xFF30D158),
    indigo: Color(0xFF5E5CE6),
    orange: Color(0xFFFF9F0A),
    pink: Color(0xFFFF375F),
    purple: Color(0xFFBF5AF2),
    red: Color(0xFFFF453A),
    teal: Color(0xFF40C8E0),
    yellow: Color(0xFFFFD60A),
  );
}

/// Colores de los estados de mesa (RF-03-02).
@immutable
class RpMesaColors {
  const RpMesaColors({
    required this.bloqueadaBg,
    required this.bloqueadaFg,
    required this.demoradaBg,
    required this.demoradaFg,
    required this.libreBg,
    required this.libreFg,
    required this.ocupadaBg,
    required this.ocupadaFg,
    required this.porPagarBg,
    required this.porPagarFg,
  });

  final Color bloqueadaBg;
  final Color bloqueadaFg;
  final Color demoradaBg;
  final Color demoradaFg;
  final Color libreBg;
  final Color libreFg;
  final Color ocupadaBg;
  final Color ocupadaFg;
  final Color porPagarBg;
  final Color porPagarFg;

  static const light = RpMesaColors(
    bloqueadaBg: Color(0xFF636366),
    bloqueadaFg: Color(0xFFFFFFFF),
    demoradaBg: Color(0xFFD70015),
    demoradaFg: Color(0xFFFFFFFF),
    libreBg: Color(0xFF34C759),
    libreFg: Color(0xFF062E12),
    ocupadaBg: Color(0xFFFFCC00),
    ocupadaFg: Color(0xFF3A2A00),
    porPagarBg: Color(0xFF0066CC),
    porPagarFg: Color(0xFFFFFFFF),
  );

  static const dark = RpMesaColors(
    bloqueadaBg: Color(0xFF48484A),
    bloqueadaFg: Color(0xFFFFFFFF),
    demoradaBg: Color(0xFFD70015),
    demoradaFg: Color(0xFFFFFFFF),
    libreBg: Color(0xFF30D158),
    libreFg: Color(0xFF062E12),
    ocupadaBg: Color(0xFFFFD60A),
    ocupadaFg: Color(0xFF3A2A00),
    porPagarBg: Color(0xFF0071E3),
    porPagarFg: Color(0xFFFFFFFF),
  );
}

/// Escala tipográfica de iOS. Usa la fuente del sistema (SF Pro en iOS, Roboto en Android).
abstract final class RpText {
  static const amount = TextStyle(fontSize: 48, height: 1.0833, fontWeight: FontWeight.w700, letterSpacing: -1.440);
  static const body = TextStyle(fontSize: 17, height: 1.2941, fontWeight: FontWeight.w400, letterSpacing: -0.408);
  static const callout = TextStyle(fontSize: 16, height: 1.3125, fontWeight: FontWeight.w400, letterSpacing: -0.304);
  static const caption1 = TextStyle(fontSize: 12, height: 1.3333, fontWeight: FontWeight.w400, letterSpacing: 0.000);
  static const caption2 = TextStyle(fontSize: 11, height: 1.1818, fontWeight: FontWeight.w400, letterSpacing: 0.066);
  static const footnote = TextStyle(fontSize: 13, height: 1.3846, fontWeight: FontWeight.w400, letterSpacing: -0.065);
  static const headline = TextStyle(fontSize: 17, height: 1.2941, fontWeight: FontWeight.w600, letterSpacing: -0.408);
  static const largeTitle = TextStyle(fontSize: 34, height: 1.2059, fontWeight: FontWeight.w700, letterSpacing: -0.748);
  static const subhead = TextStyle(fontSize: 15, height: 1.3333, fontWeight: FontWeight.w400, letterSpacing: -0.210);
  static const title1 = TextStyle(fontSize: 28, height: 1.2143, fontWeight: FontWeight.w700, letterSpacing: -0.588);
  static const title2 = TextStyle(fontSize: 22, height: 1.2727, fontWeight: FontWeight.w700, letterSpacing: -0.352);
  static const title3 = TextStyle(fontSize: 20, height: 1.2500, fontWeight: FontWeight.w600, letterSpacing: -0.340);
}

abstract final class RpSpace {
  static const double s0 = 0;
  static const double s1 = 4;
  static const double s2 = 8;
  static const double s3 = 12;
  static const double s4 = 16;
  static const double s5 = 20;
  static const double s6 = 24;
  static const double s8 = 32;
  static const double s10 = 40;
  static const double s12 = 48;
}

abstract final class RpRadius {
  static const double lg = 12;
  static const double md = 10;
  static const double phone = 44;
  static const double pill = 999;
  static const double sm = 8;
  static const double widget = 22;
  static const double xl = 14;
  static const double xs = 6;
}

abstract final class RpSize {
  static const double appIcon = 60;
  static const double cellMin = 52;
  static const double keypadKey = 76;
  static const double navBar = 52;
  static const double tabBar = 56;
  static const double touchMin = 48;
}

abstract final class RpMotion {
  static const base = Duration(milliseconds: 250);
  static const easeSpring = Cubic(0.3, 1.3, 0.5, 1);
  static const easeStandard = Cubic(0.2, 0.9, 0.25, 1);
  static const fast = Duration(milliseconds: 150);
  static const sheet = Duration(milliseconds: 420);
}

/// Iconos semánticos → CupertinoIcons. Nunca uses CupertinoIcons directo en una pantalla.
abstract final class RpIcons {
  static const IconData advertencia = CupertinoIcons.exclamationmark_triangle_fill;
  static const IconData agregar = CupertinoIcons.add;
  static const IconData ajustes = CupertinoIcons.gear_solid;
  static const IconData anulacion = CupertinoIcons.scissors;
  static const IconData avisos = CupertinoIcons.bell_fill;
  static const IconData bar = CupertinoIcons.drop_fill;
  static const IconData bloqueo = CupertinoIcons.lock_fill;
  static const IconData buscar = CupertinoIcons.search;
  static const IconData caja = CupertinoIcons.money_dollar_circle_fill;
  static const IconData cajon = CupertinoIcons.tray_fill;
  static const IconData chevron = CupertinoIcons.chevron_right;
  static const IconData cliente = CupertinoIcons.person_fill;
  static const IconData cocina = CupertinoIcons.flame_fill;
  static const IconData cocinaFria = CupertinoIcons.snow;
  static const IconData cortesia = CupertinoIcons.gift_fill;
  static const IconData descuento = CupertinoIcons.tag_fill;
  static const IconData editar = CupertinoIcons.pencil;
  static const IconData efectivo = CupertinoIcons.money_dollar_circle_fill;
  static const IconData eliminar = CupertinoIcons.trash_fill;
  static const IconData enLinea = CupertinoIcons.wifi;
  static const IconData error = CupertinoIcons.exclamationmark_circle_fill;
  static const IconData exito = CupertinoIcons.checkmark_circle_fill;
  static const IconData factura = CupertinoIcons.doc_text_fill;
  static const IconData impresora = CupertinoIcons.printer_fill;
  static const IconData inventario = CupertinoIcons.cube_box_fill;
  static const IconData kds = CupertinoIcons.desktopcomputer;
  static const IconData libre = CupertinoIcons.circle;
  static const IconData ocupada = CupertinoIcons.person_2_fill;
  static const IconData orden = CupertinoIcons.doc_text_fill;
  static const IconData personal = CupertinoIcons.person_2_fill;
  static const IconData platoListo = CupertinoIcons.checkmark_seal_fill;
  static const IconData qr = CupertinoIcons.qrcode;
  static const IconData reportes = CupertinoIcons.chart_bar_fill;
  static const IconData salon = CupertinoIcons.square_grid_2x2_fill;
  static const IconData sinInternet = CupertinoIcons.wifi_slash;
  static const IconData sinNodo = CupertinoIcons.exclamationmark_triangle_fill;
  static const IconData sync = CupertinoIcons.arrow_2_circlepath;
  static const IconData tarjeta = CupertinoIcons.creditcard_fill;
  static const IconData tiempo = CupertinoIcons.clock_fill;
  static const IconData transferencia = CupertinoIcons.device_phone_portrait;
}
