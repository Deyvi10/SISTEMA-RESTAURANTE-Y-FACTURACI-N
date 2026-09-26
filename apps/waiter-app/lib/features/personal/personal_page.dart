import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:restpos_ui/restpos_ui.dart';

import '../../core/api_error.dart';
import '../../core/estado.dart';
import '../../data/modelos.dart';
import '../../widgets/avatar.dart';
import '../../widgets/pildora_conexion.dart';
import '../../widgets/teclado_pin.dart';

final personalProvider = FutureProvider.autoDispose<List<Persona>>((ref) async {
  final api = ref.watch(sesionProvider.select((s) => s.api));
  return api == null ? const [] : api.personal();
});

/// Cuadrícula de personal: el mesero toca su foto y escribe su PIN (F3-04).
class PersonalPage extends ConsumerWidget {
  const PersonalPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = RpTheme.colorsOf(context);
    final sesion = ref.watch(sesionProvider);
    final gente = ref.watch(personalProvider);
    return CupertinoPageScaffold(
      child: SafeArea(
        child: CustomScrollView(
          slivers: [
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(RpSpace.s5, RpSpace.s4, RpSpace.s5, 0),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        sesion.identidad?.restaurante ?? '',
                        style: RpText.subhead.copyWith(color: c.labelSecondary),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    const PildoraConexion(),
                  ],
                ),
              ),
            ),
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(RpSpace.s5, RpSpace.s3, RpSpace.s5, RpSpace.s2),
                child: Text('¿Quién eres?', style: RpText.largeTitle.copyWith(color: c.label)),
              ),
            ),
            if (sesion.error != null)
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: RpSpace.s5),
                  child: Text(sesion.error!, style: RpText.footnote.copyWith(color: c.dangerText)),
                ),
              ),
            gente.when(
              loading: () => const SliverFillRemaining(child: Center(child: CupertinoActivityIndicator())),
              error: (e, _) => SliverFillRemaining(
                child: Center(child: Text(e is ApiError ? e.detalle : 'No se pudo cargar el personal.', textAlign: TextAlign.center)),
              ),
              data: (lista) => SliverPadding(
                padding: const EdgeInsets.all(RpSpace.s4),
                sliver: SliverGrid.builder(
                  gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(maxCrossAxisExtent: 130, mainAxisSpacing: RpSpace.s4, childAspectRatio: .8),
                  itemCount: lista.length,
                  itemBuilder: (_, i) => _TarjetaPersona(p: lista[i]),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _TarjetaPersona extends ConsumerWidget {
  const _TarjetaPersona({required this.p});
  final Persona p;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = RpTheme.colorsOf(context);
    final api = ref.watch(sesionProvider.select((s) => s.api));
    return Semantics(
      button: true,
      label: 'Entrar como ${p.nombre}',
      child: GestureDetector(
        onTap: () => pedirPIN(context, ref, p),
        child: Column(
          children: [
            Hero(
              tag: 'avatar-${p.id}',
              child: Avatar(nombre: p.nombre, iniciales: p.iniciales, foto: p.avatarUrl == null || api == null ? null : api.media(p.avatarUrl!)),
            ),
            const SizedBox(height: RpSpace.s2),
            Text(
              p.nombre,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: RpText.subhead.copyWith(color: c.label, fontWeight: FontWeight.w600),
            ),
            Text(switch (p.rol) {
              'CAJERO' => 'Caja',
              'ADMIN' => 'Administración',
              _ => 'Mesero',
            }, style: RpText.caption1.copyWith(color: c.labelSecondary)),
          ],
        ),
      ),
    );
  }
}

/// Hoja con el teclado de PIN, que sube desde abajo como el desbloqueo del iPhone.
Future<void> pedirPIN(BuildContext context, WidgetRef ref, Persona p) => showCupertinoModalPopup<void>(
  context: context,
  builder: (ctx) => _HojaPIN(p: p),
);

class _HojaPIN extends ConsumerWidget {
  const _HojaPIN({required this.p});
  final Persona p;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final api = ref.watch(sesionProvider.select((s) => s.api));
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(0, RpSpace.s6, 0, RpSpace.s8),
      decoration: const BoxDecoration(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
        gradient: LinearGradient(begin: Alignment.topCenter, end: Alignment.bottomCenter, colors: [Color(0xF0202030), Color(0xF00B0B12)]),
      ),
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Hero(
              tag: 'avatar-${p.id}',
              child: Avatar(nombre: p.nombre, iniciales: p.iniciales, tamano: 64, foto: p.avatarUrl == null || api == null ? null : api.media(p.avatarUrl!)),
            ),
            const SizedBox(height: RpSpace.s3),
            TecladoPIN(
              titulo: 'Hola, ${p.nombre.split(' ').first}',
              claroSobreOscuro: true,
              alCompletar: (pin) async {
                try {
                  await ref.read(sesionProvider.notifier).entrar(p, pin);
                  if (context.mounted) Navigator.pop(context);
                  return null;
                } on ApiError catch (e) {
                  return e.detalle;
                }
              },
            ),
            CupertinoButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('Cancelar', style: TextStyle(color: Color(0xFFFFFFFF))),
            ),
          ],
        ),
      ),
    );
  }
}
