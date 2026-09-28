import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:restpos_ui/restpos_ui.dart';

import '../../core/api_error.dart';
import '../../core/dinero.dart';
import '../../core/estado.dart';
import '../../core/haptica.dart';
import '../../data/modelos.dart';
import '../../widgets/avatar.dart';
import '../../widgets/comunes.dart';
import '../../widgets/teclado_pin.dart';
import '../personal/personal_page.dart';
import 'hoja_modificadores.dart';
import 'orden_controlador.dart';

/// Pedido de la mesa: lo enviado (atenuado, con estado) y lo nuevo, que se borra deslizando
/// como en Mail y se edita con pulsación prolongada (F3-09, F3-11).
Future<void> abrirPedido(BuildContext context, WidgetRef ref, String mesaId) => showCupertinoModalPopup<void>(
  context: context,
  builder: (_) => _HojaPedido(mesaId: mesaId),
);

class _HojaPedido extends ConsumerWidget {
  const _HojaPedido({required this.mesaId});
  final String mesaId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = RpTheme.colorsOf(context);
    final st = ref.watch(ordenProvider(mesaId));
    final ctl = ref.read(ordenProvider(mesaId).notifier);
    final cat = ref.watch(catalogoProvider).value;
    final enviadas = st.orden?.lineas ?? const <LineaEnviada>[];
    return Container(
      height: MediaQuery.sizeOf(context).height * .78,
      decoration: BoxDecoration(
        color: c.background,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(RpRadius.widget)),
      ),
      child: Column(
        children: [
          const SizedBox(height: 8),
          Container(
            width: 38,
            height: 5,
            decoration: BoxDecoration(color: c.labelTertiary, borderRadius: BorderRadius.circular(3)),
          ),
          Padding(
            padding: const EdgeInsets.all(RpSpace.s4),
            child: Row(
              children: [
                Expanded(
                  child: Text(st.orden == null ? 'Pedido nuevo' : 'Orden #${st.orden!.numero}', style: RpText.title2.copyWith(color: c.label)),
                ),
                Text(Dinero.formato(st.totalEnviado + st.totalBorrador), style: RpText.title3.copyWith(color: c.label)),
              ],
            ),
          ),
          Expanded(
            child: ListView(
              padding: const EdgeInsets.only(bottom: RpSpace.s8),
              children: [
                if (st.borrador.isNotEmpty) ...[
                  _Titulo('POR ENVIAR · desliza para quitar, mantén para editar'),
                  for (final l in st.borrador)
                    Dismissible(
                      key: ValueKey(l.id),
                      direction: DismissDirection.endToStart,
                      onDismissed: (_) {
                        Haptica.advertencia();
                        ctl.quitar(l.id);
                      },
                      background: Container(
                        color: c.dangerFill,
                        alignment: Alignment.centerRight,
                        padding: const EdgeInsets.only(right: RpSpace.s6),
                        child: const Icon(RpIcons.eliminar, color: Color(0xFFFFFFFF)),
                      ),
                      child: GestureDetector(
                        onLongPress: cat == null
                            ? null
                            : () async {
                                Haptica.seleccion();
                                final r = await abrirModificadores(
                                  context,
                                  l.producto,
                                  cat.catalogo.gruposDe(l.producto),
                                  cat.catalogo.categoria(l.producto.categoriaId)?.notas ?? const [],
                                  inicial: ResultadoMods(l.mods, l.nota, l.cantidad, tiempo: l.tiempo, enEspera: l.enEspera),
                                  edicion: true,
                                );
                                if (r != null) {
                                  ctl.reemplazar(l.copia(mods: r.mods, nota: r.nota, cantidad: r.cantidad, tiempo: r.tiempo, enEspera: r.enEspera));
                                }
                              },
                        child: _FilaLinea(
                          cantidad: l.cantidad,
                          nombre: l.producto.nombre,
                          detalle: [
                            ...l.mods.map((m) => m.nombre),
                            if (l.nota.isNotEmpty) '«${l.nota}»',
                            if (l.enEspera) 'En espera · ${l.tiempo.toLowerCase()}',
                          ].join(' · '),
                          total: Dinero.formato(l.total),
                          nueva: true,
                        ),
                      ),
                    ),
                ],
                if (enviadas.isNotEmpty) ...[
                  _Titulo('ENVIADO · mantén para anular'),
                  for (final l in enviadas)
                    GestureDetector(
                      onLongPress: l.anulada ? null : () => anularLinea(context, ref, mesaId, l),
                      child: _FilaLinea(
                        cantidad: l.cantidad,
                        nombre: l.producto,
                        detalle: [
                          ...l.modificadores,
                          if (l.nota.isNotEmpty) '«${l.nota}»',
                          if (l.enEspera) 'En espera · ${l.tiempo.toLowerCase()}',
                          if (l.anulada) 'Anulado',
                        ].join(' · '),
                        total: Dinero.texto(l.total),
                        anulada: l.anulada,
                      ),
                    ),
                ],
                if (st.borrador.isEmpty && enviadas.isEmpty)
                  Padding(
                    padding: const EdgeInsets.all(RpSpace.s8),
                    child: Text(
                      'Toca los platos para agregarlos.',
                      textAlign: TextAlign.center,
                      style: RpText.body.copyWith(color: c.labelSecondary),
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

class _Titulo extends StatelessWidget {
  const _Titulo(this.texto);
  final String texto;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(RpSpace.s5, RpSpace.s3, RpSpace.s5, RpSpace.s1),
    child: Text(texto, style: RpText.footnote.copyWith(color: RpTheme.colorsOf(context).labelSecondary)),
  );
}

class _FilaLinea extends StatelessWidget {
  const _FilaLinea({required this.cantidad, required this.nombre, required this.detalle, required this.total, this.nueva = false, this.anulada = false});
  final String cantidad, nombre, detalle, total;
  final bool nueva, anulada;
  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    final color = anulada ? c.labelTertiary : (nueva ? c.label : c.labelSecondary);
    final deco = anulada ? TextDecoration.lineThrough : null;
    return Container(
      color: c.surface,
      padding: const EdgeInsets.symmetric(horizontal: RpSpace.s5, vertical: RpSpace.s3),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 34,
            padding: const EdgeInsets.symmetric(vertical: 2),
            decoration: BoxDecoration(color: nueva ? c.accentSoft : c.fill, borderRadius: BorderRadius.circular(8)),
            alignment: Alignment.center,
            child: Text(
              cantidad,
              style: RpText.subhead.copyWith(fontWeight: FontWeight.w700, color: nueva ? c.accent : color),
            ),
          ),
          const SizedBox(width: RpSpace.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  nombre,
                  style: RpText.body.copyWith(color: color, decoration: deco),
                ),
                if (detalle.isNotEmpty)
                  Text(
                    detalle,
                    style: RpText.footnote.copyWith(color: c.labelSecondary, decoration: deco),
                  ),
              ],
            ),
          ),
          Text(
            total,
            style: RpText.body.copyWith(color: color, decoration: deco),
          ),
          if (!nueva && !anulada) ...[const SizedBox(width: 6), Icon(CupertinoIcons.checkmark_alt, size: 16, color: RpTheme.tintsOf(context).green)],
        ],
      ),
    );
  }
}

// ---------- Anular con PIN de supervisor (F3-14) ----------

Future<void> anularLinea(BuildContext context, WidgetRef ref, String mesaId, LineaEnviada l) async {
  final confirma = await showCupertinoDialog<bool>(
    context: context,
    builder: (ctx) => CupertinoAlertDialog(
      title: Text('¿Anular «${l.producto}»?'),
      content: const Text('Se imprimirá un ticket de ANULACIÓN en su estación y quedará registrado.'),
      actions: [
        CupertinoDialogAction(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancelar')),
        CupertinoDialogAction(isDestructiveAction: true, onPressed: () => Navigator.pop(ctx, true), child: const Text('Anular')),
      ],
    ),
  );
  if (confirma != true || !context.mounted) return;
  final motivo = await _pedirMotivo(context);
  if (motivo == null || !context.mounted) return;
  final ctl = ref.read(ordenProvider(mesaId).notifier);
  try {
    await ctl.anular([l.id], motivo.$1, motivo.$2);
    Haptica.exito();
  } on ApiError catch (e) {
    if (e.code != 'REQUIERE_SUPERVISOR' || !context.mounted) {
      if (context.mounted) await mostrarError(context, e.detalle);
      return;
    }
    final token = await pedirSupervisor(context, ref, 'ANULAR_ITEM_ENVIADO', ref.read(ordenProvider(mesaId)).orden!.id);
    if (token == null) return;
    try {
      await ctl.anular([l.id], motivo.$1, motivo.$2, autorizacion: token);
      Haptica.exito();
    } on ApiError catch (e2) {
      if (context.mounted) await mostrarError(context, e2.detalle);
    }
  }
}

Future<(String, bool)?> _pedirMotivo(BuildContext context) async {
  final ctl = TextEditingController();
  var sePreparo = false;
  return showCupertinoModalPopup<(String, bool)>(
    context: context,
    builder: (ctx) => StatefulBuilder(
      builder: (ctx, set) => SobreTeclado(
        child: CupertinoActionSheet(
          title: const Text('Motivo de la anulación'),
          message: Column(
            children: [
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final m in const ['El cliente cambió de opinión', 'Error del mesero', 'Demoró demasiado', 'Plato en mal estado'])
                    CupertinoButton(
                      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                      color: RpTheme.colorsOf(ctx).fill,
                      minimumSize: const Size(0, 30),
                      onPressed: () => set(() => ctl.text = m),
                      child: Text(m, style: RpText.footnote.copyWith(color: RpTheme.colorsOf(ctx).label)),
                    ),
                ],
              ),
              const SizedBox(height: 10),
              CupertinoTextField(controller: ctl, placeholder: 'Escribe el motivo', maxLength: 140),
              CupertinoListTile(
                title: const Text('¿Se llegó a preparar?'),
                trailing: CupertinoSwitch(value: sePreparo, onChanged: (v) => set(() => sePreparo = v)),
              ),
            ],
          ),
          actions: [
            CupertinoActionSheetAction(
              isDestructiveAction: true,
              onPressed: () {
                if (ctl.text.trim().length < 3) return;
                Navigator.pop(ctx, (ctl.text.trim(), sePreparo));
              },
              child: const Text('Anular plato'),
            ),
          ],
          cancelButton: CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx), child: const Text('Cancelar')),
        ),
      ),
    ),
  );
}

/// Un supervisor elige su foto y escribe su PIN en este teléfono: autoriza UNA acción.
Future<String?> pedirSupervisor(BuildContext context, WidgetRef ref, String accion, String referencia) async {
  final yo = ref.read(sesionProvider).usuario?.id;
  final gente = (await ref.read(personalProvider.future)).where((p) => p.id != yo && p.permisos.contains(accion)).toList();
  if (!context.mounted) return null;
  if (gente.isEmpty) {
    await mostrarError(
      context,
      'Nadie en este local tiene permiso para autorizarlo. El administrador puede darlo en el panel, en Personal.',
      titulo: 'Sin supervisor',
    );
    return null;
  }
  final quien = await showCupertinoModalPopup<Persona>(
    context: context,
    builder: (ctx) => CupertinoActionSheet(
      title: const Text('Autorización de supervisor'),
      message: const Text('Quien autoriza elige su nombre y escribe su PIN.'),
      actions: [for (final p in gente) CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx, p), child: Text(p.nombre))],
      cancelButton: CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx), child: const Text('Cancelar')),
    ),
  );
  if (quien == null || !context.mounted) return null;
  String? token;
  await showCupertinoModalPopup<void>(
    context: context,
    builder: (ctx) => Container(
      padding: const EdgeInsets.fromLTRB(0, RpSpace.s6, 0, RpSpace.s8),
      decoration: const BoxDecoration(
        color: Color(0xF0161620),
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Avatar(nombre: quien.nombre, iniciales: quien.iniciales, tamano: 56),
            const SizedBox(height: RpSpace.s2),
            TecladoPIN(
              titulo: 'PIN de ${quien.nombre.split(' ').first}',
              claroSobreOscuro: true,
              alCompletar: (pin) async {
                try {
                  token = await ref.read(sesionProvider).api!.autorizar(quien.id, pin, accion, referencia);
                  if (ctx.mounted) Navigator.pop(ctx);
                  return null;
                } on ApiError catch (e) {
                  return e.detalle;
                }
              },
            ),
          ],
        ),
      ),
    ),
  );
  return token;
}

// ---------- Menú de la orden: pre-cuenta, marchar, mover, unir, transferir ----------

Future<void> accionesDeOrden(BuildContext context, WidgetRef ref, String mesaId) async {
  final st = ref.read(ordenProvider(mesaId));
  final o = st.orden;
  if (o == null) return;
  final enEspera = {for (final l in o.lineas.where((l) => l.enEspera)) l.tiempo};
  final eleccion = await showCupertinoModalPopup<String>(
    context: context,
    builder: (ctx) => CupertinoActionSheet(
      title: Text('Orden #${o.numero} · ${Dinero.texto(o.total)}'),
      actions: [
        CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx, 'precuenta'), child: const Text('Imprimir pre-cuenta')),
        for (final t in enEspera) CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx, 'marchar:$t'), child: Text('Marchar ${t.toLowerCase()}')),
        CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx, 'mover'), child: const Text('Mover a otra mesa')),
        CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx, 'unir'), child: const Text('Unir con otra mesa')),
        CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx, 'transferir'), child: const Text('Pasar a otro mesero')),
      ],
      cancelButton: CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx), child: const Text('Cancelar')),
    ),
  );
  if (eleccion == null || !context.mounted) return;
  final api = ref.read(sesionProvider).api!;
  final ctl = ref.read(ordenProvider(mesaId).notifier);
  try {
    if (eleccion == 'precuenta') {
      final personas = await _elegirPersonas(context);
      if (personas == null) return;
      final r = await ctl.precuenta(personas);
      Haptica.exito();
      final aviso = r['aviso'] as String?;
      if (context.mounted) {
        await mostrarError(
          context,
          aviso ?? 'Total ${Dinero.texto((r['totales'] as Map)['total'] as String)}. La mesa quedó «Por pagar».',
          titulo: 'Pre-cuenta impresa',
        );
      }
    } else if (eleccion.startsWith('marchar:')) {
      await ctl.marchar(eleccion.substring(8));
      Haptica.exito();
    } else if (eleccion == 'transferir') {
      final gente = await ref.read(personalProvider.future);
      if (!context.mounted) return;
      final p = await _elegir<Persona>(context, 'Pasar la mesa a…', gente.where((x) => x.nombre != o.meseroNombre).toList(), (x) => x.nombre);
      if (p == null) return;
      await api.transferir(o.id, p.id);
      await ctl.recargar();
      Haptica.exito();
    } else {
      final salon = ref.read(salonProvider).value;
      final mover = eleccion == 'mover';
      final candidatas = (salon?.mesas ?? const <Mesa>[])
          .where((m) => m.id != mesaId && (mover ? m.estado == EstadoMesaVivo.libre : m.ordenId != null) && m.bloqueo == null)
          .toList();
      if (!context.mounted) return;
      final m = await _elegir<Mesa>(context, mover ? 'Mover a la mesa…' : 'Unir con la mesa…', candidatas, (x) => x.nombre);
      if (m == null) return;
      if (mover) {
        await api.mover(o.id, m.id);
      } else {
        await api.unir(o.id, m.ordenId!);
      }
      Haptica.exito();
      ref.read(salonProvider.notifier).refrescar();
      if (context.mounted) context.pop();
    }
  } on ApiError catch (e) {
    Haptica.error();
    if (context.mounted) await mostrarError(context, e.detalle);
  }
}

Future<int?> _elegirPersonas(BuildContext context) => showCupertinoModalPopup<int>(
  context: context,
  builder: (ctx) => CupertinoActionSheet(
    title: const Text('¿Dividir entre cuántas personas?'),
    message: const Text('Solo informativo: la pre-cuenta muestra cuánto le toca a cada uno.'),
    actions: [
      CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx, 1), child: const Text('No dividir')),
      for (final n in [2, 3, 4, 5, 6]) CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx, n), child: Text('$n personas')),
    ],
    cancelButton: CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx), child: const Text('Cancelar')),
  ),
);

Future<T?> _elegir<T>(BuildContext context, String titulo, List<T> opciones, String Function(T) nombre) {
  if (opciones.isEmpty) {
    mostrarError(context, 'No hay opciones disponibles ahora.', titulo: titulo);
    return Future.value();
  }
  return showCupertinoModalPopup<T>(
    context: context,
    builder: (ctx) => CupertinoActionSheet(
      title: Text(titulo),
      actions: [for (final o in opciones) CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx, o), child: Text(nombre(o)))],
      cancelButton: CupertinoActionSheetAction(onPressed: () => Navigator.pop(ctx), child: const Text('Cancelar')),
    ),
  );
}
