import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:restpos_meseros/core/estado.dart';
import 'package:restpos_meseros/data/modelos.dart';
import 'package:restpos_meseros/features/orden/orden_controlador.dart';
import 'package:restpos_meseros/features/salon/salon_page.dart';
import 'package:restpos_meseros/widgets/teclado_pin.dart';
import 'package:restpos_ui/restpos_ui.dart';

Widget envolver(Widget w) => ProviderScope(
      child: CupertinoApp(theme: RpTheme.cupertino(Brightness.light), home: CupertinoPageScaffold(child: Center(child: w))),
    );

class _SesionFija extends SesionControlador {
  @override
  EstadoSesion build() => const EstadoSesion(fase: Fase.adentro);
}

void main() {
  testWidgets('teclado de PIN: completa, sacude y muestra el error', (t) async {
    final pins = <String>[];
    await t.pumpWidget(envolver(TecladoPIN(alCompletar: (pin) async {
      pins.add(pin);
      return pin == '8899' ? null : 'PIN incorrecto.';
    })));
    for (final d in ['1', '2', '3', '4']) {
      await t.tap(find.text(d));
      await t.pump();
    }
    await t.pumpAndSettle();
    expect(pins, ['1234']);
    expect(find.text('PIN incorrecto.'), findsOneWidget);
    await t.tap(find.text('8'));
    await t.tap(find.bySemanticsLabel('Borrar'));
    for (final d in ['8', '8', '9', '9']) {
      await t.tap(find.text(d));
      await t.pump();
    }
    await t.pumpAndSettle();
    expect(pins.last, '8899');
    expect(find.text('PIN incorrecto.'), findsNothing);
  });

  testWidgets('mesa editada por otro muestra «Editando» y el candado', (t) async {
    final m = Mesa(
      id: 'm1', zonaId: 'z', nombre: 'Mesa 4', capacidad: 4, estado: EstadoMesaVivo.ocupada, platos: 3, total: '36.00',
      abiertaAt: DateTime.now().subtract(const Duration(minutes: 12)), meseroNombre: 'Ana R.',
      bloqueo: Bloqueo(usuarioId: 'ana', usuarioNombre: 'Ana R.', dispositivoId: 'd', expiraAt: DateTime.now().add(const Duration(seconds: 40))),
    );
    await t.pumpWidget(envolver(SizedBox(width: 160, height: 160, child: MesaTile(mesa: m, yo: 'carlos'))));
    expect(find.text('Editando: Ana R.'), findsOneWidget);
    expect(find.byIcon(RpIcons.bloqueo), findsOneWidget);
    expect(find.text('4'), findsOneWidget);
    // Para quien la bloqueó, es su mesa ocupada con tiempo y total.
    await t.pumpWidget(envolver(SizedBox(width: 160, height: 160, child: MesaTile(mesa: m, yo: 'ana'))));
    expect(find.textContaining(r'$36.00'), findsOneWidget);
    expect(find.textContaining('12 min'), findsOneWidget);
  });

  test('borrador: junta idénticos, separa distintos, quita y deshace', () async {
    final c = ProviderContainer(overrides: [sesionProvider.overrideWith(_SesionFija.new)]);
    addTearDown(c.dispose);
    const p = Producto(id: 'hb', categoriaId: 'c', nombre: 'Hamburguesa', alias: '', descripcion: '', precio: '9.50', foto: null, fotoMd: null, grupos: [], vendidos: 0);
    const queso = Modificador(id: 'q', nombre: 'Extra queso', precioAdicional: '1.00');
    final sub = c.listen(ordenProvider('m1'), (_, _) {});
    addTearDown(sub.close);
    final ctl = c.read(ordenProvider('m1').notifier);
    ctl.agregar(p);
    ctl.agregar(p);
    ctl.agregar(p, mods: [queso]);
    ctl.agregar(p, nota: 'sin cebolla');
    var st = c.read(ordenProvider('m1'));
    expect(st.borrador.length, 3);
    expect(st.borrador.first.cantidad, '2');
    expect(st.totalBorrador.toString(), '39'); // 19.00 + 10.50 + 9.50
    ctl.quitar(st.borrador[1].id);
    st = c.read(ordenProvider('m1'));
    expect(st.borrador.length, 2);
    expect(st.deshacer, isNotNull);
    ctl.deshacer();
    st = c.read(ordenProvider('m1'));
    expect(st.borrador.length, 3);
    expect(st.borrador[1].mods.single.nombre, 'Extra queso');
  });
}
