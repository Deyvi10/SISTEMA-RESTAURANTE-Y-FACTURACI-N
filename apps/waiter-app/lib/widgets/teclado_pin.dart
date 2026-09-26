import 'dart:math';

import 'package:flutter/cupertino.dart';
import 'package:restpos_ui/restpos_ui.dart';

import '../core/haptica.dart';

/// Teclado de PIN idéntico al desbloqueo del iPhone: puntos que se llenan, botones circulares
/// translúcidos y sacudida con háptica de error si el PIN es incorrecto (F3-04).
class TecladoPIN extends StatefulWidget {
  const TecladoPIN({super.key, required this.alCompletar, this.largo = 4, this.titulo, this.claroSobreOscuro = false});

  /// Devuelve null si el PIN es correcto o el mensaje de error.
  final Future<String?> Function(String pin) alCompletar;
  final int largo;
  final String? titulo;
  final bool claroSobreOscuro;

  @override
  State<TecladoPIN> createState() => TecladoPINState();
}

class TecladoPINState extends State<TecladoPIN> with SingleTickerProviderStateMixin {
  var _pin = '';
  var _ocupado = false;
  String? _error;
  late final AnimationController _sacudida = AnimationController(vsync: this, duration: const Duration(milliseconds: 420));

  @override
  void dispose() {
    _sacudida.dispose();
    super.dispose();
  }

  Future<void> tocar(String d) async {
    if (_ocupado || _pin.length >= widget.largo) return;
    Haptica.seleccion();
    setState(() {
      _pin += d;
      _error = null;
    });
    if (_pin.length == widget.largo) {
      setState(() => _ocupado = true);
      final err = await widget.alCompletar(_pin);
      if (!mounted) return;
      if (err != null) {
        Haptica.error();
        _sacudida.forward(from: 0);
        setState(() {
          _error = err;
          _pin = '';
        });
      }
      setState(() => _ocupado = false);
    }
  }

  void borrar() {
    if (_pin.isEmpty || _ocupado) return;
    Haptica.seleccion();
    setState(() => _pin = _pin.substring(0, _pin.length - 1));
  }

  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    final texto = widget.claroSobreOscuro ? const Color(0xFFFFFFFF) : c.label;
    final tecla = widget.claroSobreOscuro ? const Color(0x33FFFFFF) : c.fill;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (widget.titulo != null) Text(widget.titulo!, style: RpText.title3.copyWith(color: texto)),
        const SizedBox(height: RpSpace.s5),
        AnimatedBuilder(
          animation: _sacudida,
          builder: (_, child) => Transform.translate(offset: Offset(sin(_sacudida.value * pi * 6) * 14 * (1 - _sacudida.value), 0), child: child),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              for (var i = 0; i < widget.largo; i++)
                AnimatedContainer(
                  duration: RpMotion.fast,
                  margin: const EdgeInsets.symmetric(horizontal: 9),
                  width: 14,
                  height: 14,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: i < _pin.length ? texto : const Color(0x00000000),
                    border: Border.all(color: texto, width: 1.5),
                  ),
                ),
            ],
          ),
        ),
        SizedBox(
          height: 40,
          child: Center(
            child: _ocupado
                ? CupertinoActivityIndicator(color: texto)
                : Text(
                    _error ?? '',
                    textAlign: TextAlign.center,
                    style: RpText.footnote.copyWith(color: RpTheme.tintsOf(context).red),
                  ),
          ),
        ),
        for (final fila in const [
          ['1', '2', '3'],
          ['4', '5', '6'],
          ['7', '8', '9'],
          ['', '0', '⌫'],
        ])
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 7),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                for (final k in fila)
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 14),
                    child: k.isEmpty
                        ? const SizedBox(width: RpSize.keypadKey, height: RpSize.keypadKey)
                        : _Tecla(etiqueta: k, fondo: k == '⌫' ? const Color(0x00000000) : tecla, color: texto, onTap: () => k == '⌫' ? borrar() : tocar(k)),
                  ),
              ],
            ),
          ),
      ],
    );
  }
}

class _Tecla extends StatefulWidget {
  const _Tecla({required this.etiqueta, required this.fondo, required this.color, required this.onTap});
  final String etiqueta;
  final Color fondo, color;
  final VoidCallback onTap;
  @override
  State<_Tecla> createState() => _TeclaState();
}

class _TeclaState extends State<_Tecla> {
  var _abajo = false;
  static const _letras = {'2': 'ABC', '3': 'DEF', '4': 'GHI', '5': 'JKL', '6': 'MNO', '7': 'PQRS', '8': 'TUV', '9': 'WXYZ'};

  @override
  Widget build(BuildContext context) {
    final borrar = widget.etiqueta == '⌫';
    return Semantics(
      button: true,
      label: borrar ? 'Borrar' : widget.etiqueta,
      child: GestureDetector(
        onTapDown: (_) => setState(() => _abajo = true),
        onTapUp: (_) => setState(() => _abajo = false),
        onTapCancel: () => setState(() => _abajo = false),
        onTap: widget.onTap,
        child: AnimatedContainer(
          duration: RpMotion.fast,
          width: RpSize.keypadKey,
          height: RpSize.keypadKey,
          decoration: BoxDecoration(shape: BoxShape.circle, color: _abajo && !borrar ? widget.color.withValues(alpha: .35) : widget.fondo),
          alignment: Alignment.center,
          child: borrar
              ? Icon(CupertinoIcons.delete_left, color: widget.color, size: 28)
              : Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      widget.etiqueta,
                      style: TextStyle(fontSize: 34, height: 1.05, fontWeight: FontWeight.w400, color: widget.color),
                    ),
                    if (_letras[widget.etiqueta] != null)
                      Text(
                        _letras[widget.etiqueta]!,
                        style: TextStyle(fontSize: 10, letterSpacing: 2, fontWeight: FontWeight.w600, color: widget.color),
                      ),
                  ],
                ),
        ),
      ),
    );
  }
}
