import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:restpos_ui/restpos_ui.dart';

import '../../core/estado.dart';

/// Avisos: impresoras con problemas y comandas que no se pudieron enviar (F3-11).
class AvisosPage extends ConsumerWidget {
  const AvisosPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = RpTheme.colorsOf(context);
    final avisos = ref.watch(avisosProvider);
    final pendientes = ref.watch(pendientesProvider).value ?? const [];
    return CupertinoPageScaffold(
      child: CustomScrollView(
        slivers: [
          const CupertinoSliverNavigationBar(largeTitle: Text('Avisos')),
          if (avisos.isEmpty && pendientes.isEmpty)
            SliverFillRemaining(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(RpIcons.exito, size: 56, color: RpTheme.tintsOf(context).green),
                  const SizedBox(height: RpSpace.s3),
                  Text('Todo en orden', style: RpText.title3.copyWith(color: c.label)),
                  Text('Aquí verás si una impresora se queda sin papel.', style: RpText.subhead.copyWith(color: c.labelSecondary)),
                ],
              ),
            ),
          if (pendientes.isNotEmpty)
            SliverToBoxAdapter(
              child: CupertinoListSection.insetGrouped(
                header: const Text('PENDIENTES DE ENVÍO'),
                children: [
                  for (final p in pendientes)
                    CupertinoListTile(
                      leading: Icon(RpIcons.sync, color: RpTheme.tintsOf(context).orange),
                      title: const Text('Comanda en espera'),
                      subtitle: Text(p.ultimoError ?? 'Se enviará sola al volver la conexión.'),
                    ),
                ],
              ),
            ),
          if (avisos.isNotEmpty)
            SliverToBoxAdapter(
              child: CupertinoListSection.insetGrouped(
                header: const Text('AHORA'),
                children: [
                  for (final a in avisos)
                    CupertinoListTile(
                      leading: Icon(a.grave ? RpIcons.advertencia : RpIcons.impresora, color: a.grave ? RpTheme.tintsOf(context).red : c.accent),
                      title: Text(a.titulo),
                      subtitle: Text(a.detalle, maxLines: 3),
                      trailing: CupertinoButton(
                        padding: EdgeInsets.zero,
                        onPressed: () => ref.read(avisosProvider.notifier).quitar(a.id),
                        child: const Icon(CupertinoIcons.xmark_circle_fill),
                      ),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
