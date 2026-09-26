import 'dart:io';

import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:restpos_ui/restpos_ui.dart';

import '../../core/api_error.dart';
import '../../core/estado.dart';
import '../../core/haptica.dart';
import '../../core/identidad.dart';
import '../../widgets/comunes.dart';

/// Bienvenida y emparejamiento por QR, como al emparejar un Apple Watch (F3-02).
class EmparejarPage extends ConsumerStatefulWidget {
  const EmparejarPage({super.key, this.camara = true});
  final bool camara; // las pruebas no tienen cámara
  @override
  ConsumerState<EmparejarPage> createState() => _EmparejarPageState();
}

class _EmparejarPageState extends ConsumerState<EmparejarPage> with SingleTickerProviderStateMixin {
  var _procesando = false;
  late final AnimationController _esquinas = AnimationController(vsync: this, duration: const Duration(milliseconds: 1600))..repeat(reverse: true);

  @override
  void dispose() {
    _esquinas.dispose();
    super.dispose();
  }

  Future<void> _usar(DatosQR qr) async {
    if (_procesando) return;
    setState(() => _procesando = true);
    Haptica.exito();
    try {
      await ref.read(sesionProvider.notifier).emparejar(qr, nombre: _nombreEquipo(), plataforma: '${Platform.operatingSystem} ${Platform.operatingSystemVersion}', version: '0.1.0');
    } on ApiError catch (e) {
      if (mounted) await mostrarError(context, e.detalle, titulo: 'No se pudo emparejar');
    } finally {
      if (mounted) setState(() => _procesando = false);
    }
  }

  String _nombreEquipo() {
    try {
      return Platform.localHostname.isEmpty ? 'Teléfono' : Platform.localHostname;
    } catch (_) {
      return 'Teléfono';
    }
  }

  Future<void> _manual() async {
    final codigo = TextEditingController();
    final ip = TextEditingController();
    await showCupertinoModalPopup<void>(
      context: context,
      builder: (ctx) => CupertinoActionSheet(
        title: const Text('Escribir código'),
        message: Column(children: [
          const Text('En la caja: Estado del nodo › Emparejar un teléfono.'),
          const SizedBox(height: 12),
          CupertinoTextField(controller: codigo, placeholder: 'Código (8 letras)', textCapitalization: TextCapitalization.characters, autofocus: true),
          const SizedBox(height: 8),
          CupertinoTextField(controller: ip, placeholder: 'IP de la caja, p. ej. 192.168.1.10', keyboardType: TextInputType.url),
        ]),
        actions: [
          CupertinoActionSheetAction(
            isDefaultAction: true,
            onPressed: () {
              Navigator.pop(ctx);
              final c = codigo.text.replaceAll(RegExp(r'[^A-Za-z0-9]'), '').toUpperCase();
              var u = ip.text.trim();
              if (u.isEmpty || c.length != 8) {
                mostrarError(context, 'Escribe el código de 8 caracteres y la IP que aparece en la caja.');
                return;
              }
              if (!u.startsWith('http')) u = 'http://$u';
              if (!RegExp(r':\d+$').hasMatch(u)) u = '$u:7080';
              _usar(DatosQR(c, [u]));
            },
            child: const Text('Emparejar'),
          ),
        ],
        cancelButton: CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx), child: const Text('Cancelar')),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    final t = RpTheme.tintsOf(context);
    final error = ref.watch(sesionProvider.select((s) => s.error));
    return CupertinoPageScaffold(
      child: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: RpSpace.s6),
          child: Column(children: [
            const SizedBox(height: RpSpace.s8),
            IconoApp(icono: CupertinoIcons.person_2_fill, color: t.orange),
            const SizedBox(height: RpSpace.s5),
            Text('Escanea el código\nde tu restaurante', textAlign: TextAlign.center, style: RpText.title1.copyWith(color: c.label)),
            const SizedBox(height: RpSpace.s2),
            Text('Lo encuentras en la caja: Estado del nodo › Emparejar un teléfono.',
                textAlign: TextAlign.center, style: RpText.subhead.copyWith(color: c.labelSecondary)),
            if (error != null) ...[
              const SizedBox(height: RpSpace.s3),
              Text(error, textAlign: TextAlign.center, style: RpText.footnote.copyWith(color: c.dangerText)),
            ],
            const SizedBox(height: RpSpace.s6),
            Expanded(
              child: Center(
                child: AspectRatio(
                  aspectRatio: 1,
                  child: Stack(fit: StackFit.expand, children: [
                    ClipRRect(
                      borderRadius: BorderRadius.circular(RpRadius.widget * 1.4),
                      child: Container(
                        color: const Color(0xFF111111),
                        child: widget.camara
                            ? MobileScanner(onDetect: (cap) {
                                for (final b in cap.barcodes) {
                                  final qr = DatosQR.leer(b.rawValue ?? '');
                                  if (qr != null) {
                                    _usar(qr);
                                    break;
                                  }
                                }
                              })
                            : const Center(child: Icon(RpIcons.qr, size: 64, color: Color(0x55FFFFFF))),
                      ),
                    ),
                    AnimatedBuilder(animation: _esquinas, builder: (_, _) => CustomPaint(painter: _Esquinas(t.orange, 18 + _esquinas.value * 10))),
                    if (_procesando) const Center(child: CupertinoActivityIndicator(radius: 18, color: Color(0xFFFFFFFF))),
                  ]),
                ),
              ),
            ),
            const SizedBox(height: RpSpace.s4),
            CupertinoButton(onPressed: _procesando ? null : _manual, child: const Text('Escribir código')),
            const SizedBox(height: RpSpace.s4),
          ]),
        ),
      ),
    );
  }
}

/// Esquinas redondeadas que «respiran» alrededor del visor.
class _Esquinas extends CustomPainter {
  _Esquinas(this.color, this.inset);
  final Color color;
  final double inset;

  @override
  void paint(Canvas canvas, Size s) {
    final p = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 5
      ..strokeCap = StrokeCap.round;
    const l = 38.0, r = 18.0;
    final a = inset, b = s.width - inset, h = s.height - inset;
    for (final (x, y, dx, dy) in [(a, a, 1.0, 1.0), (b, a, -1.0, 1.0), (a, h, 1.0, -1.0), (b, h, -1.0, -1.0)]) {
      final path = Path()
        ..moveTo(x, y + dy * l)
        ..lineTo(x, y + dy * r)
        ..quadraticBezierTo(x, y, x + dx * r, y)
        ..lineTo(x + dx * l, y);
      canvas.drawPath(path, p);
    }
  }

  @override
  bool shouldRepaint(_Esquinas o) => o.inset != inset || o.color != color;
}
