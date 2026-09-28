import 'package:flutter/cupertino.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:restpos_ui/restpos_ui.dart';

void main() {
  test('los temas usan los colores accesibles, no los tintes de iOS', () {
    expect(RpTheme.cupertino(Brightness.light).primaryColor, const Color(0xFF0066CC));
    expect(RpTheme.cupertino(Brightness.dark).primaryColor, const Color(0xFF409CFF));
    expect(RpTheme.cupertino(Brightness.dark).scaffoldBackgroundColor, const Color(0xFF000000));
  });

  testWidgets('colorsOf y mesa siguen el brillo del tema', (tester) async {
    late RpColors colors;
    late ({Color bg, Color fg}) libre;
    await tester.pumpWidget(CupertinoApp(
      theme: RpTheme.cupertino(Brightness.dark),
      home: Builder(builder: (context) {
        colors = RpTheme.colorsOf(context);
        libre = RpTheme.mesa(context, EstadoMesa.libre);
        return const Icon(RpIcons.cocina);
      }),
    ));
    expect(colors.background, RpColors.dark.background);
    expect(libre.bg, RpMesaColors.dark.libreBg);
    expect(find.byIcon(RpIcons.cocina), findsOneWidget);
  });

  test('los objetivos táctiles cumplen el mínimo de 48 dp (RNF-31)', () {
    expect(RpSize.touchMin, greaterThanOrEqualTo(48));
    expect(RpSize.keypadKey, greaterThanOrEqualTo(48));
  });
}
