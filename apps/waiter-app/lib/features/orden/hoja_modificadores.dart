import 'package:decimal/decimal.dart';
import 'package:flutter/cupertino.dart';
import 'package:restpos_ui/restpos_ui.dart';

import '../../core/dinero.dart';
import '../../core/haptica.dart';
import '../../data/modelos.dart';
import '../../widgets/comunes.dart';

class ResultadoMods {
  const ResultadoMods(this.mods, this.nota, this.cantidad, {this.tiempo = '', this.enEspera = false});
  final List<Modificador> mods;
  final String nota, cantidad, tiempo;
  final bool enEspera;
}

/// Hoja inferior con asa: grupos con check azul, notas rápidas y «Agregar · $12.50» fijo abajo (F3-09).
Future<ResultadoMods?> abrirModificadores(BuildContext context, Producto p, List<GrupoModificadores> grupos, List<String> notasRapidas,
        {ResultadoMods? inicial, bool edicion = false}) =>
    showCupertinoModalPopup<ResultadoMods>(
      context: context,
      builder: (_) => HojaModificadores(producto: p, grupos: grupos, notasRapidas: notasRapidas, inicial: inicial, edicion: edicion),
    );

class HojaModificadores extends StatefulWidget {
  const HojaModificadores({super.key, required this.producto, required this.grupos, required this.notasRapidas, this.inicial, this.edicion = false});
  final Producto producto;
  final List<GrupoModificadores> grupos;
  final List<String> notasRapidas;
  final ResultadoMods? inicial;
  final bool edicion;
  @override
  State<HojaModificadores> createState() => _HojaModificadoresState();
}

class _HojaModificadoresState extends State<HojaModificadores> {
  late final Set<String> _elegidos = {for (final m in widget.inicial?.mods ?? const <Modificador>[]) m.id};
  late final _nota = TextEditingController(text: widget.inicial?.nota ?? '');
  late int _cantidad = int.tryParse(widget.inicial?.cantidad ?? '1') ?? 1;
  late String _tiempo = widget.inicial?.tiempo ?? '';
  late bool _espera = widget.inicial?.enEspera ?? false;

  @override
  void dispose() {
    _nota.dispose();
    super.dispose();
  }

  List<Modificador> get _mods => [for (final g in widget.grupos) ...g.modificadores.where((m) => _elegidos.contains(m.id))];

  String? _falta() {
    for (final g in widget.grupos) {
      final n = g.modificadores.where((m) => _elegidos.contains(m.id)).length;
      if (n < g.minimo) return 'Elige ${g.nombre.toLowerCase()}';
    }
    return null;
  }

  void _tocar(GrupoModificadores g, Modificador m) {
    Haptica.seleccion();
    setState(() {
      if (_elegidos.contains(m.id)) {
        _elegidos.remove(m.id);
      } else {
        if (g.max == 1) _elegidos.removeAll(g.modificadores.map((x) => x.id)); // como radio
        if (g.modificadores.where((x) => _elegidos.contains(x.id)).length < g.max) _elegidos.add(m.id);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    final total = Dinero.totalLinea(widget.producto.precio, _mods.map((m) => m.precioAdicional), '$_cantidad');
    final falta = _falta();
    return DraggableScrollableSheet(
      initialChildSize: .62,
      minChildSize: .4,
      maxChildSize: .95,
      snap: true,
      snapSizes: const [.62, .95],
      expand: false,
      builder: (_, scroll) => Container(
        decoration: BoxDecoration(color: c.background, borderRadius: const BorderRadius.vertical(top: Radius.circular(RpRadius.widget))),
        child: Column(children: [
          const SizedBox(height: 8),
          Container(width: 38, height: 5, decoration: BoxDecoration(color: c.labelTertiary, borderRadius: BorderRadius.circular(3))),
          Expanded(
            child: CupertinoScrollbar(
              controller: scroll,
              child: ListView(controller: scroll, padding: const EdgeInsets.fromLTRB(RpSpace.s4, RpSpace.s3, RpSpace.s4, RpSpace.s4), children: [
                Text(widget.producto.nombre, style: RpText.title2.copyWith(color: c.label)),
                Text(Dinero.texto(widget.producto.precio), style: RpText.subhead.copyWith(color: c.labelSecondary)),
                for (final g in widget.grupos) ...[
                  Padding(
                    padding: const EdgeInsets.fromLTRB(RpSpace.s3, RpSpace.s5, RpSpace.s3, RpSpace.s2),
                    child: Row(children: [
                      Expanded(child: Text(g.nombre.toUpperCase(), style: RpText.footnote.copyWith(color: c.labelSecondary))),
                      Text(g.minimo > 0 ? 'Obligatorio' : (g.max > 1 ? 'Hasta ${g.max}' : 'Opcional'),
                          style: RpText.footnote.copyWith(color: g.minimo > 0 ? c.accent : c.labelSecondary, fontWeight: FontWeight.w600)),
                    ]),
                  ),
                  CupertinoListSection.insetGrouped(
                    margin: EdgeInsets.zero,
                    children: [
                      for (final m in g.modificadores)
                        CupertinoListTile(
                          title: Text(m.nombre),
                          additionalInfo: Dinero.parse(m.precioAdicional) > Decimal.zero ? Text('+${Dinero.texto(m.precioAdicional)}') : null,
                          trailing: _elegidos.contains(m.id) ? Icon(CupertinoIcons.checkmark_alt, color: c.accent) : null,
                          onTap: () => _tocar(g, m),
                        ),
                    ],
                  ),
                ],
                Padding(
                  padding: const EdgeInsets.fromLTRB(RpSpace.s3, RpSpace.s5, RpSpace.s3, RpSpace.s2),
                  child: Text('NOTA PARA COCINA', style: RpText.footnote.copyWith(color: c.labelSecondary)),
                ),
                CupertinoTextField(controller: _nota, placeholder: 'Sin cebolla, bien cocido…', maxLength: 140, padding: const EdgeInsets.all(12)),
                if (widget.notasRapidas.isNotEmpty) ...[
                  const SizedBox(height: RpSpace.s2),
                  Wrap(spacing: 8, runSpacing: 8, children: [
                    for (final n in widget.notasRapidas)
                      GestureDetector(
                        onTap: () => setState(() => _nota.text = _nota.text.isEmpty ? n : '${_nota.text}, $n'),
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                          decoration: BoxDecoration(color: c.fill, borderRadius: BorderRadius.circular(RpRadius.pill)),
                          child: Text(n, style: RpText.footnote.copyWith(color: c.label)),
                        ),
                      ),
                  ]),
                ],
                if (widget.edicion) ...[
                  Padding(
                    padding: const EdgeInsets.fromLTRB(RpSpace.s3, RpSpace.s5, RpSpace.s3, RpSpace.s2),
                    child: Text('TIEMPO', style: RpText.footnote.copyWith(color: c.labelSecondary)),
                  ),
                  CupertinoSlidingSegmentedControl<String>(
                    groupValue: _tiempo,
                    children: const {'': Text('—'), 'ENTRADA': Text('Entrada'), 'FUERTE': Text('Fuerte'), 'POSTRE': Text('Postre')},
                    onValueChanged: (v) => setState(() {
                      _tiempo = v ?? '';
                      if (_tiempo.isEmpty) _espera = false;
                    }),
                  ),
                  CupertinoListTile(
                    title: const Text('Mantener en espera'),
                    subtitle: const Text('Se imprime al «marchar» su tiempo'),
                    trailing: CupertinoSwitch(value: _espera, onChanged: _tiempo.isEmpty ? null : (v) => setState(() => _espera = v)),
                  ),
                ],
              ]),
            ),
          ),
          Container(
            padding: const EdgeInsets.fromLTRB(RpSpace.s4, RpSpace.s3, RpSpace.s4, RpSpace.s6),
            decoration: BoxDecoration(color: c.bar, border: Border(top: BorderSide(color: c.separator, width: .5))),
            child: Row(children: [
              _Stepper(valor: _cantidad, onCambio: (v) => setState(() => _cantidad = v)),
              const SizedBox(width: RpSpace.s3),
              Expanded(
                child: BotonPrincipal(
                  texto: falta ?? '${widget.edicion ? 'Guardar' : 'Agregar'} · ${Dinero.formato(total)}',
                  onPressed: falta != null
                      ? null
                      : () => Navigator.pop(context, ResultadoMods(_mods, _nota.text.trim(), '$_cantidad', tiempo: _tiempo, enEspera: _espera)),
                ),
              ),
            ]),
          ),
        ]),
      ),
    );
  }
}

class _Stepper extends StatelessWidget {
  const _Stepper({required this.valor, required this.onCambio});
  final int valor;
  final ValueChanged<int> onCambio;
  @override
  Widget build(BuildContext context) {
    final c = RpTheme.colorsOf(context);
    return Container(
      height: 54,
      decoration: BoxDecoration(color: c.fill, borderRadius: BorderRadius.circular(RpRadius.xl)),
      child: Row(children: [
        CupertinoButton(padding: const EdgeInsets.symmetric(horizontal: 14), onPressed: valor > 1 ? () => onCambio(valor - 1) : null, child: const Icon(CupertinoIcons.minus)),
        Text('$valor', style: RpText.headline.copyWith(color: c.label)),
        CupertinoButton(padding: const EdgeInsets.symmetric(horizontal: 14), onPressed: valor < 99 ? () => onCambio(valor + 1) : null, child: const Icon(CupertinoIcons.plus)),
      ]),
    );
  }
}
