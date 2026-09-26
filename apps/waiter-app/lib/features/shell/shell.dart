import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:restpos_ui/restpos_ui.dart';

import '../../core/estado.dart';
import '../../widgets/comunes.dart';

/// Barra de pestañas inferior translúcida: Salón, Pedidos, Avisos (con contador).
class Shell extends ConsumerWidget {
  const Shell({super.key, required this.navegacion});
  final StatefulNavigationShell navegacion;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.watch(colaProvider); // la cola de envíos vive mientras hay sesión
    final avisos = ref.watch(avisosProvider).length + (ref.watch(pendientesProvider).value?.length ?? 0);
    return CupertinoPageScaffold(
      child: Column(children: [
        Expanded(child: navegacion),
        CupertinoTabBar(
          currentIndex: navegacion.currentIndex,
          onTap: (i) => navegacion.goBranch(i, initialLocation: i == navegacion.currentIndex),
          items: [
            const BottomNavigationBarItem(icon: Icon(RpIcons.salon), label: 'Salón'),
            const BottomNavigationBarItem(icon: Icon(CupertinoIcons.list_bullet), label: 'Mis mesas'),
            BottomNavigationBarItem(
              icon: Stack(clipBehavior: Clip.none, children: [
                const Icon(RpIcons.avisos),
                if (avisos > 0)
                  Positioned(
                    right: -8,
                    top: -4,
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                      decoration: BoxDecoration(color: RpTheme.tintsOf(context).red, borderRadius: BorderRadius.circular(9)),
                      child: Text('$avisos', style: const TextStyle(fontSize: 11, color: Color(0xFFFFFFFF), fontWeight: FontWeight.w700)),
                    ),
                  ),
              ]),
              label: 'Avisos',
            ),
          ],
        ),
      ]),
    );
  }
}

/// «Mis mesas»: las mesas abiertas del mesero que entró.
class MisMesasPage extends ConsumerWidget {
  const MisMesasPage({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = RpTheme.colorsOf(context);
    final yo = ref.watch(sesionProvider.select((s) => s.usuario));
    final mesas = (ref.watch(salonProvider).value?.mesas ?? const []).where((m) => m.ordenId != null && m.meseroNombre == yo?.nombre).toList();
    return CupertinoPageScaffold(
      child: CustomScrollView(slivers: [
        const CupertinoSliverNavigationBar(largeTitle: Text('Mis mesas')),
        if (mesas.isEmpty)
          SliverFillRemaining(child: Center(child: Text('No tienes mesas abiertas.', style: RpText.body.copyWith(color: c.labelSecondary))))
        else
          SliverToBoxAdapter(
            child: CupertinoListSection.insetGrouped(children: [
              for (final m in mesas)
                CupertinoListTile(
                  title: Text(m.nombre),
                  subtitle: Text('${m.platos} platos'),
                  additionalInfo: Text(m.total),
                  trailing: const CupertinoListTileChevron(),
                  onTap: () => context.push('/mesa/${m.id}', extra: m.nombre),
                ),
            ]),
          ),
      ]),
    );
  }
}

/// Pantalla mientras se busca el nodo.
class ConectandoPage extends ConsumerWidget {
  const ConectandoPage({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = RpTheme.colorsOf(context);
    final s = ref.watch(sesionProvider);
    return CupertinoPageScaffold(
      child: Center(
        child: Padding(
          padding: const EdgeInsets.all(RpSpace.s8),
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            IconoApp(icono: CupertinoIcons.person_2_fill, color: RpTheme.tintsOf(context).orange, tamano: 72),
            const SizedBox(height: RpSpace.s5),
            if (s.error == null) ...[
              const CupertinoActivityIndicator(),
              const SizedBox(height: RpSpace.s3),
              Text('Buscando el Nodo Local…', style: RpText.subhead.copyWith(color: c.labelSecondary)),
            ] else ...[
              Text(s.error!, textAlign: TextAlign.center, style: RpText.body.copyWith(color: c.label)),
              const SizedBox(height: RpSpace.s5),
              BotonPrincipal(texto: 'Reintentar', onPressed: () => ref.read(sesionProvider.notifier).iniciar()),
              CupertinoButton(onPressed: () => ref.read(sesionProvider.notifier).desemparejar(), child: const Text('Emparejar de nuevo')),
            ],
          ]),
        ),
      ),
    );
  }
}
