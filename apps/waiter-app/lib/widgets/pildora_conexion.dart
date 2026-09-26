import 'dart:async';

import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:restpos_ui/restpos_ui.dart';

import '../core/estado.dart';
import '../core/tiempo_real.dart';

/// Indicador de conexión como la «Dynamic Island»: una píldora negra que se expande 2 s al
/// cambiar de estado y luego se contrae a un punto de color (F3-03).
class PildoraConexion extends ConsumerStatefulWidget {
  const PildoraConexion({super.key});
  @override
  ConsumerState<PildoraConexion> createState() => _PildoraConexionState();
}

class _PildoraConexionState extends ConsumerState<PildoraConexion> {
  var _expandida = false;
  Timer? _t;

  @override
  void dispose() {
    _t?.cancel();
    super.dispose();
  }

  void _expandir() {
    setState(() => _expandida = true);
    _t?.cancel();
    _t = Timer(const Duration(seconds: 2), () {
      if (mounted) setState(() => _expandida = false);
    });
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(conexionProvider, (a, b) {
      if (a != b) _expandir();
    });
    final c = ref.watch(conexionProvider);
    final t = RpTheme.tintsOf(context);
    final (color, texto, icono) = switch (c) {
      Conexion.enLinea => (t.green, 'En línea', RpIcons.enLinea),
      Conexion.sinInternet => (t.yellow, 'Sin internet · operando local', RpIcons.sinInternet),
      Conexion.sinNodo => (t.red, 'Sin Nodo Local', RpIcons.sinNodo),
    };
    return Semantics(
      label: 'Conexión: $texto',
      child: GestureDetector(
        onTap: _expandir,
        child: AnimatedContainer(
          duration: RpMotion.base,
          curve: RpMotion.easeSpring,
          height: 30,
          padding: EdgeInsets.symmetric(horizontal: _expandida ? 12 : 10),
          decoration: BoxDecoration(color: const Color(0xFF000000), borderRadius: BorderRadius.circular(RpRadius.pill)),
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            Container(width: 9, height: 9, decoration: BoxDecoration(color: color, shape: BoxShape.circle)),
            AnimatedSize(
              duration: RpMotion.base,
              curve: RpMotion.easeStandard,
              child: _expandida
                  ? Padding(
                      padding: const EdgeInsets.only(left: 8),
                      child: Row(mainAxisSize: MainAxisSize.min, children: [
                        Icon(icono, size: 14, color: color),
                        const SizedBox(width: 6),
                        Text(texto, style: RpText.footnote.copyWith(color: const Color(0xFFFFFFFF), fontWeight: FontWeight.w600)),
                      ]),
                    )
                  : const SizedBox.shrink(),
            ),
          ]),
        ),
      ),
    );
  }
}
