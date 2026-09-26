import 'dart:async';

import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:restpos_ui/restpos_ui.dart';

import 'core/estado.dart';
import 'features/avisos/avisos_page.dart';
import 'features/emparejar/emparejar_page.dart';
import 'features/orden/orden_page.dart';
import 'features/personal/personal_page.dart';
import 'features/salon/salon_page.dart';
import 'features/shell/shell.dart';

/// Re-PIN tras 2 minutos sin tocar la pantalla (F3-04, configurable).
const inactividadMaxima = Duration(minutes: 2);

final routerProvider = Provider<GoRouter>((ref) {
  final fase = ValueNotifier<Fase>(ref.read(sesionProvider).fase);
  ref.listen(sesionProvider.select((s) => s.fase), (_, f) => fase.value = f);
  ref.onDispose(fase.dispose);
  return GoRouter(
    initialLocation: '/salon',
    refreshListenable: fase,
    redirect: (_, st) {
      final loc = st.matchedLocation;
      final destino = switch (fase.value) {
        Fase.cargando || Fase.buscandoNodo => '/conectando',
        Fase.sinEmparejar => '/emparejar',
        Fase.elegirPersona => '/personal',
        Fase.adentro => const ['/conectando', '/emparejar', '/personal'].contains(loc) ? '/salon' : null,
      };
      return destino == loc ? null : destino;
    },
    routes: [
      GoRoute(path: '/conectando', builder: (_, _) => const ConectandoPage()),
      GoRoute(
        path: '/emparejar',
        builder: (_, _) => EmparejarPage(camara: ref.read(camaraProvider)),
      ),
      GoRoute(path: '/personal', builder: (_, _) => const PersonalPage()),
      StatefulShellRoute.indexedStack(
        builder: (_, _, nav) => Shell(navegacion: nav),
        branches: [
          StatefulShellBranch(
            routes: [GoRoute(path: '/salon', builder: (_, _) => const SalonPage())],
          ),
          StatefulShellBranch(
            routes: [GoRoute(path: '/mis-mesas', builder: (_, _) => const MisMesasPage())],
          ),
          StatefulShellBranch(
            routes: [GoRoute(path: '/avisos', builder: (_, _) => const AvisosPage())],
          ),
        ],
      ),
      GoRoute(
        path: '/mesa/:id',
        pageBuilder: (_, st) => CupertinoPage(
          child: OrdenPage(mesaId: st.pathParameters['id']!, mesaNombre: st.extra as String? ?? 'Mesa'),
        ),
      ),
    ],
  );
});

class RestPosApp extends ConsumerStatefulWidget {
  const RestPosApp({super.key});
  @override
  ConsumerState<RestPosApp> createState() => _RestPosAppState();
}

class _RestPosAppState extends ConsumerState<RestPosApp> {
  Timer? _inactivo;

  void _actividad() {
    _inactivo?.cancel();
    if (ref.read(sesionProvider).fase != Fase.adentro) return;
    _inactivo = Timer(inactividadMaxima, () => ref.read(sesionProvider.notifier).salir());
  }

  @override
  void dispose() {
    _inactivo?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final brillo = MediaQuery.platformBrightnessOf(context);
    ref.listen(sesionProvider.select((s) => s.fase), (_, _) => _actividad());
    return Listener(
      onPointerDown: (_) => _actividad(),
      behavior: HitTestBehavior.translucent,
      child: CupertinoApp.router(
        title: 'RestPOS Meseros',
        debugShowCheckedModeBanner: false,
        theme: RpTheme.cupertino(brillo),
        routerConfig: ref.watch(routerProvider),
        localizationsDelegates: const [DefaultCupertinoLocalizations.delegate, DefaultWidgetsLocalizations.delegate],
      ),
    );
  }
}
