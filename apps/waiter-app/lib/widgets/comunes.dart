import 'package:flutter/cupertino.dart';
import 'package:restpos_ui/restpos_ui.dart';

/// Botón de ancho completo en la zona del pulgar.
class BotonPrincipal extends StatelessWidget {
  const BotonPrincipal({super.key, required this.texto, required this.onPressed, this.cargando = false, this.icono, this.destructivo = false});
  final String texto;
  final VoidCallback? onPressed;
  final bool cargando, destructivo;
  final IconData? icono;

  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    return SizedBox(
      width: double.infinity,
      height: 54,
      child: CupertinoButton(
        padding: EdgeInsets.zero,
        color: destructivo ? c.dangerFill : c.accentFill,
        disabledColor: c.fill,
        borderRadius: BorderRadius.circular(RpRadius.xl),
        onPressed: cargando ? null : onPressed,
        child: cargando
            ? const CupertinoActivityIndicator(color: Color(0xFFFFFFFF))
            : Row(mainAxisSize: MainAxisSize.min, children: [
                if (icono != null) ...[Icon(icono, color: c.onAccent, size: 20), const SizedBox(width: 8)],
                Text(texto, style: RpText.headline.copyWith(color: c.onAccent)),
              ]),
      ),
    );
  }
}

/// Icono de app: cuadrado redondeado con degradado (como en la pantalla de inicio de iOS).
class IconoApp extends StatelessWidget {
  const IconoApp({super.key, required this.icono, required this.color, this.tamano = 88});
  final IconData icono;
  final Color color;
  final double tamano;

  @override
  Widget build(BuildContext context) => Container(
        width: tamano,
        height: tamano,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(tamano * .25),
          gradient: LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight, colors: [Color.lerp(color, const Color(0xFFFFFFFF), .25)!, color]),
          boxShadow: [BoxShadow(color: color.withValues(alpha: .35), blurRadius: 24, offset: const Offset(0, 10))],
        ),
        child: Icon(icono, color: const Color(0xFFFFFFFF), size: tamano * .52),
      );
}

/// Muestra un error de forma amable (alerta iOS).
Future<void> mostrarError(BuildContext context, String mensaje, {String titulo = 'No se pudo completar'}) => showCupertinoDialog<void>(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: Text(titulo),
        content: Text(mensaje),
        actions: [CupertinoDialogAction(isDefaultAction: true, onPressed: () => Navigator.pop(ctx), child: const Text('Entendido'))],
      ),
    );

/// Mapa de iconos de categoría del backoffice (lucide) a íconos de iOS.
IconData iconoCategoria(String nombre) => switch (nombre) {
      'coffee' || 'milk' || 'cup-soda' => CupertinoIcons.cart_fill,
      'beer' || 'wine' || 'martini' || 'glass-water' || 'citrus' => RpIcons.bar,
      'fish' || 'soup' || 'salad' => CupertinoIcons.leaf_arrow_circlepath,
      'cake-slice' || 'ice-cream-cone' || 'dessert' || 'donut' || 'cookie' || 'cherry' => CupertinoIcons.gift_fill,
      'pizza' || 'sandwich' || 'beef' || 'drumstick' || 'ham' || 'flame' || 'cooking-pot' || 'chef-hat' => RpIcons.cocina,
      'egg-fried' || 'croissant' => CupertinoIcons.sun_max_fill,
      _ => CupertinoIcons.square_grid_2x2_fill,
    };

/// Color de la paleta de tintes por nombre (el mismo que usa el backoffice).
Color tintePorNombre(BuildContext context, String nombre) {
  final t = RpTheme.tintsOf(context);
  return switch (nombre) {
    'blue' => t.blue,
    'green' => t.green,
    'red' => t.red,
    'yellow' => t.yellow,
    'indigo' => t.indigo,
    'purple' => t.purple,
    'teal' => t.teal,
    'pink' => t.pink,
    'gray' => t.gray,
    _ => t.orange,
  };
}
