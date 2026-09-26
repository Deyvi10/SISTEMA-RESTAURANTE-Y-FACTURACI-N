import 'package:flutter/cupertino.dart';
import 'package:restpos_ui/restpos_ui.dart';

/// Avatar circular: foto (servida por el nodo) o iniciales sobre un degradado del color de la persona.
class Avatar extends StatelessWidget {
  const Avatar({super.key, required this.nombre, required this.iniciales, this.foto, this.tamano = 76});
  final String nombre, iniciales;
  final String? foto;
  final double tamano;

  static Color colorPara(BuildContext context, String s) {
    final t = RpTheme.tintsOf(context);
    final colores = [t.blue, t.green, t.orange, t.pink, t.purple, t.teal, t.indigo, t.red];
    return colores[s.codeUnits.fold<int>(0, (a, b) => a + b) % colores.length];
  }

  @override
  Widget build(BuildContext context) {
    final c = colorPara(context, nombre);
    final base = Container(
      width: tamano,
      height: tamano,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        gradient: LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight, colors: [Color.lerp(c, const Color(0xFFFFFFFF), .3)!, c]),
      ),
      child: Text(iniciales, style: RpText.title2.copyWith(color: const Color(0xFFFFFFFF), fontSize: tamano * .36)),
    );
    if (foto == null) return base;
    return ClipOval(
      child: Image.network(foto!, width: tamano, height: tamano, fit: BoxFit.cover, errorBuilder: (_, _, _) => base),
    );
  }
}
