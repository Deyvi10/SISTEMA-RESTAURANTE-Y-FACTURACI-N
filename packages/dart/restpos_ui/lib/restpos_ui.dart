/// Sistema de diseño estilo iOS del sistema de restaurante (F0-12).
///
/// Uso:
/// ```dart
/// CupertinoApp(theme: RpTheme.cupertino(Brightness.light), home: …);
/// final c = RpTheme.colorsOf(context); // colores del tema actual
/// Icon(RpIcons.cocina, color: c.accent);
/// ```
library;

export 'src/tokens.g.dart';
export 'src/theme.dart';
