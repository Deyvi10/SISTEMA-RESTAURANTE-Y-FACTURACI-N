import 'package:flutter/services.dart';

/// Háptica centralizada (F3-01): cada gesto importante se siente igual en toda la app.
abstract final class Haptica {
  static Future<void> seleccion() => HapticFeedback.selectionClick();
  static Future<void> exito() => HapticFeedback.mediumImpact();
  static Future<void> advertencia() => HapticFeedback.heavyImpact();
  static Future<void> error() async {
    await HapticFeedback.heavyImpact();
    await Future<void>.delayed(const Duration(milliseconds: 90));
    await HapticFeedback.heavyImpact();
  }
}
