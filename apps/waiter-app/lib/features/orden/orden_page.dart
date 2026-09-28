import 'package:decimal/decimal.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:restpos_ui/restpos_ui.dart';

import '../../core/api_error.dart';
import '../../core/dinero.dart';
import '../../core/estado.dart';
import '../../core/haptica.dart';
import '../../data/modelos.dart';
import '../../widgets/comunes.dart';
import '../../widgets/pildora_conexion.dart';
import 'acciones_orden.dart';
import 'hoja_modificadores.dart';
import 'orden_controlador.dart';

/// Toma de pedido (F3-08…F3-11): categorías, tarjetas con foto, búsqueda tipo Spotlight en la
/// zona del pulgar y «Enviar» de ancho completo.
class OrdenPage extends ConsumerStatefulWidget {
  const OrdenPage({super.key, required this.mesaId, required this.mesaNombre});
  final String mesaId, mesaNombre;
  @override
  ConsumerState<OrdenPage> createState() => _OrdenPageState();
}

class _OrdenPageState extends ConsumerState<OrdenPage> {
  final _buscar = TextEditingController();
  String? _categoria; // null = populares
  var _consulta = '';

  @override
  void dispose() {
    _buscar.dispose();
    super.dispose();
  }

  OrdenControlador get _ctl => ref.read(ordenProvider(widget.mesaId).notifier);

  Future<void> _agregar(CatalogoListo cat, Producto p) async {
    final grupos = cat.catalogo.gruposDe(p);
    if (grupos.any((g) => g.minimo > 0)) {
      final r = await abrirModificadores(context, p, grupos, cat.catalogo.categoria(p.categoriaId)?.notas ?? const []);
      if (r == null) return;
      _ctl.agregar(p, mods: r.mods, nota: r.nota, cantidad: r.cantidad);
    } else {
      _ctl.agregar(p);
    }
    Haptica.seleccion();
  }

  Future<void> _enviar() async {
    try {
      final r = await _ctl.enviar();
      Haptica.exito();
      if (!mounted) return;
      await showCupertinoDialog<void>(
        context: context,
        barrierDismissible: true,
        builder: (ctx) {
          Future.delayed(const Duration(milliseconds: 1100), () {
            if (ctx.mounted) Navigator.of(ctx).pop();
          });
          return _Confirmacion(pendiente: r == ResultadoEnvio.pendiente);
        },
      );
      if (mounted) context.pop();
    } on ApiError catch (e) {
      Haptica.error();
      if (mounted) await mostrarError(context, e.detalle, titulo: 'No se envió');
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    final st = ref.watch(ordenProvider(widget.mesaId));
    final cat = ref.watch(catalogoProvider);
    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: Text(widget.mesaNombre),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const PildoraConexion(),
            if (st.orden != null)
              CupertinoButton(
                padding: const EdgeInsets.only(left: 8),
                onPressed: () => accionesDeOrden(context, ref, widget.mesaId),
                child: const Icon(CupertinoIcons.ellipsis_circle, size: 26),
              ),
          ],
        ),
      ),
      child: SafeArea(
        child: cat.when(
          loading: () => const Center(child: CupertinoActivityIndicator()),
          error: (e, _) => Center(
            child: Text('No se pudo cargar el menú.', style: RpText.body.copyWith(color: c.labelSecondary)),
          ),
          data: (catalogo) {
            final productos = _consulta.isNotEmpty
                ? catalogo.indice.buscar(_consulta)
                : _categoria == null
                ? ([...catalogo.catalogo.productos]..sort((a, b) => b.vendidos.compareTo(a.vendidos))).take(24).toList()
                : catalogo.catalogo.productos.where((p) => p.categoriaId == _categoria).toList();
            return Column(
              children: [
                if (st.soloLectura)
                  _Aviso(
                    icono: RpIcons.bloqueo,
                    texto: 'Editando: ${st.bloqueadaPor}. Puedes ver la mesa, pero no agregar platos.',
                    color: c.warningText,
                    fondo: c.warningSoft,
                  ),
                if (st.error != null && !st.soloLectura) _Aviso(icono: RpIcons.error, texto: st.error!, color: c.dangerText, fondo: c.dangerSoft),
                _Categorias(
                  categorias: catalogo.catalogo.categorias,
                  elegida: _categoria,
                  onElegir: (id) => setState(() {
                    _categoria = id;
                    _consulta = '';
                    _buscar.clear();
                  }),
                ),
                Expanded(
                  child: productos.isEmpty
                      ? Center(
                          child: Text(
                            _consulta.isEmpty ? 'Sin platos en esta categoría.' : 'Nada coincide con «$_consulta».',
                            style: RpText.body.copyWith(color: c.labelSecondary),
                          ),
                        )
                      : GridView.builder(
                          padding: const EdgeInsets.fromLTRB(RpSpace.s4, RpSpace.s2, RpSpace.s4, RpSpace.s4),
                          gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
                            maxCrossAxisExtent: 190,
                            mainAxisSpacing: 12,
                            crossAxisSpacing: 12,
                            childAspectRatio: .78,
                          ),
                          itemCount: productos.length,
                          itemBuilder: (_, i) => TarjetaProducto(
                            producto: productos[i],
                            cantidad: st.borrador.where((l) => l.producto.id == productos[i].id).fold(Decimal.zero, (a, l) => a + Dinero.parse(l.cantidad)),
                            habilitada: !st.soloLectura,
                            onTap: () => _agregar(catalogo, productos[i]),
                          ),
                        ),
                ),
                if (st.deshacer != null) _BarraDeshacer(texto: '«${st.deshacer!.$2.producto.nombre}» quitado', onDeshacer: _ctl.deshacer),
                _ZonaPulgar(
                  buscar: _buscar,
                  onBuscar: (q) => setState(() => _consulta = q),
                  estado: st,
                  onPedido: () => abrirPedido(context, ref, widget.mesaId),
                  onEnviar: st.borrador.isEmpty || st.soloLectura ? null : _enviar,
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

class _Aviso extends StatelessWidget {
  const _Aviso({required this.icono, required this.texto, required this.color, required this.fondo});
  final IconData icono;
  final String texto;
  final Color color, fondo;
  @override
  Widget build(BuildContext context) => Container(
    width: double.infinity,
    margin: const EdgeInsets.fromLTRB(RpSpace.s4, RpSpace.s2, RpSpace.s4, 0),
    padding: const EdgeInsets.all(RpSpace.s3),
    decoration: BoxDecoration(color: fondo, borderRadius: BorderRadius.circular(RpRadius.lg)),
    child: Row(
      children: [
        Icon(icono, color: color, size: 18),
        const SizedBox(width: 8),
        Expanded(
          child: Text(texto, style: RpText.footnote.copyWith(color: color)),
        ),
      ],
    ),
  );
}

class _Categorias extends StatelessWidget {
  const _Categorias({required this.categorias, required this.elegida, required this.onElegir});
  final List<Categoria> categorias;
  final String? elegida;
  final ValueChanged<String?> onElegir;

  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    Widget chip(String? id, String nombre, IconData icono, Color color) {
      final sel = elegida == id;
      return Padding(
        padding: const EdgeInsets.only(right: 8),
        child: GestureDetector(
          onTap: () {
            Haptica.seleccion();
            onElegir(id);
          },
          child: AnimatedContainer(
            duration: RpMotion.fast,
            padding: const EdgeInsets.symmetric(horizontal: 14),
            decoration: BoxDecoration(
              color: sel ? color : c.surface,
              borderRadius: BorderRadius.circular(RpRadius.pill),
              boxShadow: [if (!sel) BoxShadow(color: const Color(0xFF000000).withValues(alpha: .05), blurRadius: 6, offset: const Offset(0, 2))],
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(icono, size: 16, color: sel ? const Color(0xFFFFFFFF) : color),
                const SizedBox(width: 6),
                Text(
                  nombre,
                  style: RpText.subhead.copyWith(fontWeight: FontWeight.w600, color: sel ? const Color(0xFFFFFFFF) : c.label),
                ),
              ],
            ),
          ),
        ),
      );
    }

    return SizedBox(
      height: 52,
      child: ListView(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: RpSpace.s4, vertical: 8),
        children: [
          chip(null, 'Populares', CupertinoIcons.star_fill, RpTheme.tintsOf(context).yellow),
          for (final k in categorias) chip(k.id, k.nombre, iconoCategoria(k.icono), tintePorNombre(context, k.color)),
        ],
      ),
    );
  }
}

/// Tarjeta de producto con foto; al agregar hace un pequeño rebote (F3-08).
class TarjetaProducto extends ConsumerStatefulWidget {
  const TarjetaProducto({super.key, required this.producto, required this.cantidad, required this.onTap, this.habilitada = true});
  final Producto producto;
  final Decimal cantidad;
  final VoidCallback onTap;
  final bool habilitada;
  @override
  ConsumerState<TarjetaProducto> createState() => _TarjetaProductoState();
}

class _TarjetaProductoState extends ConsumerState<TarjetaProducto> with SingleTickerProviderStateMixin {
  late final AnimationController _rebote = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 260),
    lowerBound: .94,
    upperBound: 1,
    value: 1,
  );

  @override
  void dispose() {
    _rebote.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    final api = ref.watch(sesionProvider.select((s) => s.api));
    final p = widget.producto;
    return Semantics(
      button: true,
      label: '${p.nombre}, ${Dinero.texto(p.precio)}',
      child: GestureDetector(
        onTap: widget.habilitada
            ? () {
                _rebote.reverse(from: 1).then((_) => _rebote.forward());
                widget.onTap();
              }
            : null,
        child: ScaleTransition(
          scale: _rebote,
          child: Container(
            decoration: BoxDecoration(
              color: c.surface,
              borderRadius: BorderRadius.circular(RpRadius.xl),
              boxShadow: [BoxShadow(color: const Color(0xFF000000).withValues(alpha: .06), blurRadius: 10, offset: const Offset(0, 4))],
            ),
            clipBehavior: Clip.antiAlias,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Expanded(
                  child: Stack(
                    fit: StackFit.expand,
                    children: [
                      if (p.foto != null && api != null)
                        Image.network(
                          api.media(p.fotoMd ?? p.foto!),
                          fit: BoxFit.cover,
                          errorBuilder: (_, _, _) => _SinFoto(nombre: p.nombre),
                        )
                      else
                        _SinFoto(nombre: p.nombre),
                      if (widget.cantidad > Decimal.zero)
                        Positioned(
                          top: 8,
                          right: 8,
                          child: Container(
                            padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 3),
                            decoration: BoxDecoration(color: c.accentFill, borderRadius: BorderRadius.circular(RpRadius.pill)),
                            child: Text(
                              '×${widget.cantidad}',
                              style: RpText.footnote.copyWith(color: c.onAccent, fontWeight: FontWeight.w700),
                            ),
                          ),
                        ),
                    ],
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.fromLTRB(10, 8, 10, 10),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        p.nombre,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: RpText.subhead.copyWith(color: c.label, fontWeight: FontWeight.w600, height: 1.15),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        Dinero.texto(p.precio),
                        style: RpText.footnote.copyWith(color: c.labelSecondary, fontWeight: FontWeight.w600),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _SinFoto extends StatelessWidget {
  const _SinFoto({required this.nombre});
  final String nombre;
  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    return Container(
      color: c.fillSubtle,
      alignment: Alignment.center,
      child: Icon(RpIcons.orden, size: 34, color: c.labelTertiary),
    );
  }
}

class _BarraDeshacer extends StatelessWidget {
  const _BarraDeshacer({required this.texto, required this.onDeshacer});
  final String texto;
  final VoidCallback onDeshacer;
  @override
  Widget build(BuildContext context) => Container(
    margin: const EdgeInsets.fromLTRB(RpSpace.s4, 0, RpSpace.s4, RpSpace.s2),
    padding: const EdgeInsets.only(left: RpSpace.s4),
    decoration: BoxDecoration(color: const Color(0xE6202024), borderRadius: BorderRadius.circular(RpRadius.lg)),
    child: Row(
      children: [
        Expanded(
          child: Text(texto, style: RpText.footnote.copyWith(color: const Color(0xFFFFFFFF))),
        ),
        CupertinoButton(
          onPressed: onDeshacer,
          child: const Text(
            'Deshacer',
            style: TextStyle(color: Color(0xFF64D2FF), fontWeight: FontWeight.w600),
          ),
        ),
      ],
    ),
  );
}

/// Búsqueda, resumen del pedido y «Enviar» juntos abajo, donde llega el pulgar.
class _ZonaPulgar extends StatelessWidget {
  const _ZonaPulgar({required this.buscar, required this.onBuscar, required this.estado, required this.onPedido, required this.onEnviar});
  final TextEditingController buscar;
  final ValueChanged<String> onBuscar;
  final EstadoOrden estado;
  final VoidCallback onPedido;
  final VoidCallback? onEnviar;

  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    final nuevos = estado.borrador.fold(Decimal.zero, (a, l) => a + Dinero.parse(l.cantidad));
    final enviados = estado.orden?.lineas.where((l) => !l.anulada).length ?? 0;
    return Container(
      padding: const EdgeInsets.fromLTRB(RpSpace.s4, RpSpace.s3, RpSpace.s4, RpSpace.s3),
      decoration: BoxDecoration(
        color: c.bar,
        border: Border(top: BorderSide(color: c.separator, width: .5)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          CupertinoSearchTextField(controller: buscar, placeholder: 'Buscar plato o alias («cev», «HB»)', onChanged: onBuscar),
          const SizedBox(height: RpSpace.s3),
          Row(
            children: [
              Expanded(
                child: GestureDetector(
                  onTap: onPedido,
                  behavior: HitTestBehavior.opaque,
                  child: Row(
                    children: [
                      Icon(RpIcons.orden, color: c.accent, size: 22),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              nuevos > Decimal.zero ? '$nuevos por enviar' : (enviados > 0 ? '$enviados enviados' : 'Pedido vacío'),
                              style: RpText.subhead.copyWith(color: c.label, fontWeight: FontWeight.w600),
                            ),
                            Text(
                              'Mesa: ${Dinero.formato(estado.totalEnviado + estado.totalBorrador)} · ver pedido',
                              style: RpText.caption1.copyWith(color: c.labelSecondary),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: RpSpace.s3),
              SizedBox(
                width: 170,
                child: BotonPrincipal(
                  texto: estado.borrador.isEmpty ? 'Enviar' : 'Enviar · ${Dinero.formato(estado.totalBorrador)}',
                  icono: CupertinoIcons.paperplane_fill,
                  cargando: estado.enviando,
                  onPressed: onEnviar,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _Confirmacion extends StatelessWidget {
  const _Confirmacion({required this.pendiente});
  final bool pendiente;
  @override
  Widget build(BuildContext context) {
    final t = RpTheme.tintsOf(context);
    return Center(
      child: Container(
        width: 190,
        padding: const EdgeInsets.all(RpSpace.s6),
        decoration: BoxDecoration(color: RpTheme.colorsOf(context).surfaceRaised, borderRadius: BorderRadius.circular(RpRadius.widget)),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TweenAnimationBuilder<double>(
              tween: Tween(begin: .3, end: 1),
              duration: const Duration(milliseconds: 380),
              curve: RpMotion.easeSpring,
              builder: (_, v, child) => Transform.scale(scale: v, child: child),
              child: Icon(pendiente ? RpIcons.sync : RpIcons.exito, size: 72, color: pendiente ? t.orange : t.green),
            ),
            const SizedBox(height: RpSpace.s3),
            Text(pendiente ? 'Pendiente de envío' : '¡Enviado!', textAlign: TextAlign.center, style: RpText.headline),
            if (pendiente) Text('Se enviará sola al volver la conexión.', textAlign: TextAlign.center, style: RpText.caption1),
          ],
        ),
      ),
    );
  }
}
