import 'dart:async';

import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:restpos_ui/restpos_ui.dart';

import '../../core/dinero.dart';
import '../../core/estado.dart';
import '../../core/haptica.dart';
import '../../data/modelos.dart';
import '../../widgets/avatar.dart';
import '../../widgets/comunes.dart';
import '../../widgets/pildora_conexion.dart';

final zonaElegidaProvider = NotifierProvider<ZonaElegida, String?>(ZonaElegida.new);

class ZonaElegida extends Notifier<String?> {
  @override
  String? build() => null;
  void elegir(String? z) => state = z;
}

/// Mapa del salón en vivo (F3-06): colores por estado, «Editando: nombre» y tiempos.
class SalonPage extends ConsumerStatefulWidget {
  const SalonPage({super.key});
  @override
  ConsumerState<SalonPage> createState() => _SalonPageState();
}

class _SalonPageState extends ConsumerState<SalonPage> {
  Timer? _reloj;

  @override
  void initState() {
    super.initState();
    // Refresca los «hace X min» cada 30 s sin pedir nada al nodo.
    _reloj = Timer.periodic(const Duration(seconds: 30), (_) => setState(() {}));
  }

  @override
  void dispose() {
    _reloj?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    final salon = ref.watch(salonProvider);
    final usuario = ref.watch(sesionProvider.select((s) => s.usuario));
    final pendientes = ref.watch(pendientesProvider).value ?? const [];
    final zonaSel = ref.watch(zonaElegidaProvider);
    return CupertinoPageScaffold(
      child: SafeArea(
        bottom: false,
        child: salon.when(
          loading: () => const Center(child: CupertinoActivityIndicator()),
          error: (e, _) => Center(child: Text('No se pudo cargar el salón.', style: RpText.body.copyWith(color: c.labelSecondary))),
          data: (s) {
            final zona = zonaSel ?? (s.zonas.isEmpty ? null : s.zonas.first.id);
            final mesas = s.mesas.where((m) => zona == null || m.zonaId == zona).toList();
            return CustomScrollView(slivers: [
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(RpSpace.s5, RpSpace.s3, RpSpace.s4, 0),
                  child: Row(children: [
                    const PildoraConexion(),
                    const Spacer(),
                    if (usuario != null)
                      GestureDetector(
                        onTap: () => _menuUsuario(context),
                        child: Avatar(nombre: usuario.nombre, iniciales: usuario.nombre.substring(0, 1), tamano: 36),
                      ),
                  ]),
                ),
              ),
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(RpSpace.s5, RpSpace.s2, RpSpace.s5, RpSpace.s3),
                  child: Text('Salón', style: RpText.largeTitle.copyWith(color: c.label)),
                ),
              ),
              if (pendientes.isNotEmpty)
                SliverToBoxAdapter(child: _BannerPendientes(n: pendientes.length)),
              if (s.zonas.length > 1)
                SliverToBoxAdapter(
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(RpSpace.s4, 0, RpSpace.s4, RpSpace.s3),
                    child: CupertinoSlidingSegmentedControl<String>(
                      groupValue: zona,
                      children: {for (final z in s.zonas) z.id: Padding(padding: const EdgeInsets.symmetric(vertical: 6), child: Text(z.nombre))},
                      onValueChanged: (v) {
                        Haptica.seleccion();
                        ref.read(zonaElegidaProvider.notifier).elegir(v);
                      },
                    ),
                  ),
                ),
              SliverToBoxAdapter(child: _Leyenda(mesas: s.mesas)),
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(RpSpace.s4, RpSpace.s2, RpSpace.s4, 120),
                sliver: SliverGrid.builder(
                  gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(maxCrossAxisExtent: 170, mainAxisSpacing: 14, crossAxisSpacing: 14, childAspectRatio: 1),
                  itemCount: mesas.length,
                  itemBuilder: (_, i) => MesaTile(mesa: mesas[i], yo: usuario?.id),
                ),
              ),
            ]);
          },
        ),
      ),
    );
  }

  void _menuUsuario(BuildContext context) => showCupertinoModalPopup<void>(
        context: context,
        builder: (ctx) => CupertinoActionSheet(
          actions: [
            CupertinoActionSheetAction(
              onPressed: () {
                Navigator.pop(ctx);
                ref.read(sesionProvider.notifier).salir();
              },
              child: const Text('Cambiar de usuario'),
            ),
          ],
          cancelButton: CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx), child: const Text('Cancelar')),
        ),
      );
}

class _BannerPendientes extends StatelessWidget {
  const _BannerPendientes({required this.n});
  final int n;
  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    return Container(
      margin: const EdgeInsets.fromLTRB(RpSpace.s4, 0, RpSpace.s4, RpSpace.s3),
      padding: const EdgeInsets.all(RpSpace.s3),
      decoration: BoxDecoration(color: c.warningSoft, borderRadius: BorderRadius.circular(RpRadius.lg)),
      child: Row(children: [
        Icon(RpIcons.sync, color: c.warningText, size: 18),
        const SizedBox(width: 8),
        Expanded(
          child: Text('$n ${n == 1 ? 'comanda pendiente' : 'comandas pendientes'} de envío. Se enviarán solas al volver la conexión.',
              style: RpText.footnote.copyWith(color: c.warningText)),
        ),
      ]),
    );
  }
}

class _Leyenda extends StatelessWidget {
  const _Leyenda({required this.mesas});
  final List<Mesa> mesas;
  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    int n(EstadoMesaVivo e) => mesas.where((m) => m.estado == e).length;
    Widget chip(String t, Color col) => Row(mainAxisSize: MainAxisSize.min, children: [
          Container(width: 9, height: 9, decoration: BoxDecoration(color: col, shape: BoxShape.circle)),
          const SizedBox(width: 5),
          Text(t, style: RpText.caption1.copyWith(color: c.labelSecondary)),
        ]);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: RpSpace.s5),
      child: Wrap(spacing: 14, runSpacing: 6, children: [
        chip('${n(EstadoMesaVivo.libre)} libres', RpTheme.mesa(context, EstadoMesa.libre).bg),
        chip('${n(EstadoMesaVivo.ocupada)} ocupadas', RpTheme.mesa(context, EstadoMesa.ocupada).bg),
        chip('${n(EstadoMesaVivo.porPagar) + n(EstadoMesaVivo.demorada)} por pagar', RpTheme.mesa(context, EstadoMesa.porPagar).bg),
      ]),
    );
  }
}

String haceCuanto(DateTime t, [DateTime? ahora]) {
  final m = (ahora ?? DateTime.now()).difference(t).inMinutes;
  if (m < 1) return 'recién';
  if (m < 60) return '$m min';
  return '${m ~/ 60} h ${m % 60} min';
}

/// Mesa como forma redondeada con número grande (F3-06).
class MesaTile extends ConsumerWidget {
  const MesaTile({super.key, required this.mesa, this.yo});
  final Mesa mesa;
  final String? yo;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ajena = mesa.bloqueo != null && mesa.bloqueo!.usuarioId != yo;
    final estado = ajena
        ? EstadoMesa.bloqueada
        : switch (mesa.estado) {
            EstadoMesaVivo.libre => EstadoMesa.libre,
            EstadoMesaVivo.ocupada => EstadoMesa.ocupada,
            EstadoMesaVivo.porPagar => EstadoMesa.porPagar,
            EstadoMesaVivo.demorada => EstadoMesa.demorada,
          };
    final col = RpTheme.mesa(context, estado);
    final numero = RegExp(r'\d+').firstMatch(mesa.nombre)?.group(0) ?? mesa.nombre;
    final detalle = ajena
        ? 'Editando: ${mesa.bloqueo!.usuarioNombre}'
        : switch (mesa.estado) {
            EstadoMesaVivo.libre => '${mesa.capacidad} personas',
            _ => '${mesa.abiertaAt == null ? '' : '${haceCuanto(mesa.abiertaAt!.toLocal())} · '}${Dinero.texto(mesa.total)}',
          };
    return Semantics(
      button: true,
      label: '${mesa.nombre}. $detalle',
      child: GestureDetector(
        onTap: () {
          if (ajena) {
            Haptica.advertencia();
            mostrarError(context, '${mesa.bloqueo!.usuarioNombre} está tomando el pedido de esta mesa. Espera a que termine.', titulo: mesa.nombre);
            return;
          }
          Haptica.seleccion();
          context.push('/mesa/${mesa.id}', extra: mesa.nombre);
        },
        child: AnimatedContainer(
          duration: RpMotion.base,
          curve: RpMotion.easeStandard,
          padding: const EdgeInsets.all(RpSpace.s3),
          decoration: BoxDecoration(
            color: col.bg,
            borderRadius: BorderRadius.circular(RpRadius.widget),
            boxShadow: [BoxShadow(color: col.bg.withValues(alpha: .35), blurRadius: 14, offset: const Offset(0, 6))],
          ),
          child: Stack(children: [
            Positioned.fill(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(mesa.nombre.replaceAll(numero, '').trim().isEmpty ? 'Mesa' : mesa.nombre.replaceAll(numero, '').trim(),
                    style: RpText.footnote.copyWith(color: col.fg.withValues(alpha: .85), fontWeight: FontWeight.w600)),
                Expanded(
                  child: FittedBox(fit: BoxFit.scaleDown, alignment: Alignment.centerLeft, child: Text(numero, style: RpText.amount.copyWith(color: col.fg))),
                ),
                Text(detalle, maxLines: 1, overflow: TextOverflow.ellipsis, style: RpText.caption1.copyWith(color: col.fg, fontWeight: FontWeight.w600)),
                if (mesa.meseroNombre != null && !ajena)
                  Text(mesa.meseroNombre!, maxLines: 1, overflow: TextOverflow.ellipsis, style: RpText.caption2.copyWith(color: col.fg.withValues(alpha: .8))),
              ]),
            ),
            if (ajena) Positioned(right: 0, top: 0, child: Icon(RpIcons.bloqueo, color: col.fg, size: 18)),
            if (mesa.estado == EstadoMesaVivo.demorada && !ajena) Positioned(right: 0, top: 0, child: Icon(RpIcons.tiempo, color: col.fg, size: 18)),
          ]),
        ),
      ),
    );
  }
}
