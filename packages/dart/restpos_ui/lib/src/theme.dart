import 'package:flutter/cupertino.dart';

import 'tokens.g.dart';

/// Construye el tema Cupertino desde los tokens y entrega los colores del tema actual.
abstract final class RpTheme {
  /// Tema de la app para el brillo dado (claro u oscuro).
  static CupertinoThemeData cupertino(Brightness brightness) {
    final c = brightness == Brightness.dark ? RpColors.dark : RpColors.light;
    return CupertinoThemeData(
      brightness: brightness,
      primaryColor: c.accent,
      primaryContrastingColor: c.onAccent,
      scaffoldBackgroundColor: c.background,
      barBackgroundColor: c.bar,
      textTheme: CupertinoTextThemeData(
        primaryColor: c.accent,
        textStyle: RpText.body.copyWith(color: c.label),
        navLargeTitleTextStyle: RpText.largeTitle.copyWith(color: c.label),
        navTitleTextStyle: RpText.headline.copyWith(color: c.label),
        actionTextStyle: RpText.body.copyWith(color: c.accent),
        tabLabelTextStyle: RpText.caption2.copyWith(color: c.labelSecondary),
      ),
    );
  }

  /// Colores semánticos del tema vigente en [context].
  static RpColors colorsOf(BuildContext context) =>
      CupertinoTheme.brightnessOf(context) == Brightness.dark ? RpColors.dark : RpColors.light;

  /// Colores de sistema de iOS del tema vigente.
  static RpTints tintsOf(BuildContext context) =>
      CupertinoTheme.brightnessOf(context) == Brightness.dark ? RpTints.dark : RpTints.light;

  /// Fondo y texto de un estado de mesa (RF-03-02).
  static ({Color bg, Color fg}) mesa(BuildContext context, EstadoMesa estado) {
    final m = CupertinoTheme.brightnessOf(context) == Brightness.dark ? RpMesaColors.dark : RpMesaColors.light;
    return switch (estado) {
      EstadoMesa.libre => (bg: m.libreBg, fg: m.libreFg),
      EstadoMesa.ocupada => (bg: m.ocupadaBg, fg: m.ocupadaFg),
      EstadoMesa.porPagar => (bg: m.porPagarBg, fg: m.porPagarFg),
      EstadoMesa.bloqueada => (bg: m.bloqueadaBg, fg: m.bloqueadaFg),
      EstadoMesa.demorada => (bg: m.demoradaBg, fg: m.demoradaFg),
    };
  }
}

/// Estados de mesa con el mismo significado en app, caja y dashboard.
enum EstadoMesa { libre, ocupada, porPagar, bloqueada, demorada }
