# restpos_ui (Flutter)

Tokens y tema estilo iOS para la app de meseros. `lib/src/tokens.g.dart` se **genera** con `make tokens` desde `packages/design/`; no lo edites.

```dart
CupertinoApp(theme: RpTheme.cupertino(MediaQuery.platformBrightnessOf(context)), …);
final c = RpTheme.colorsOf(context);
Text('Mesa 4', style: RpText.headline.copyWith(color: c.label));
Icon(RpIcons.platoListo, color: RpTheme.tintsOf(context).green);
```

Reglas: nunca uses `CupertinoIcons.x` ni colores literales en una pantalla; usa `RpIcons` y `RpTheme`.
