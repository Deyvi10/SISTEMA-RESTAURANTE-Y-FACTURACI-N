import 'dart:async';

import 'package:decimal/decimal.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

import '../../core/api_error.dart';
import '../../core/dinero.dart';
import '../../core/estado.dart';
import '../../data/modelos.dart';

/// Un plato aún no enviado (vive en el teléfono hasta tocar «Enviar»).
class LineaBorrador {
  LineaBorrador({required this.producto, this.cantidad = '1', List<Modificador>? mods, this.nota = '', this.tiempo = '', this.enEspera = false, String? id})
    : id = id ?? const Uuid().v7(),
      mods = mods ?? const [];
  final String id;
  final Producto producto;
  final String cantidad;
  final List<Modificador> mods;
  final String nota, tiempo;
  final bool enEspera;

  Decimal get total => Dinero.totalLinea(producto.precio, mods.map((m) => m.precioAdicional), cantidad);

  LineaBorrador copia({String? cantidad, List<Modificador>? mods, String? nota, String? tiempo, bool? enEspera}) => LineaBorrador(
    id: id,
    producto: producto,
    cantidad: cantidad ?? this.cantidad,
    mods: mods ?? this.mods,
    nota: nota ?? this.nota,
    tiempo: tiempo ?? this.tiempo,
    enEspera: enEspera ?? this.enEspera,
  );

  /// Dos unidades se juntan en una línea solo si son idénticas (F3-09).
  bool mismaQue(Producto p, List<Modificador> m, String n) =>
      producto.id == p.id && nota == n && tiempo.isEmpty && !enEspera && mods.map((x) => x.id).join(',') == m.map((x) => x.id).join(',');

  Map<String, dynamic> aJson() => {
    'id': id,
    'productoId': producto.id,
    'cantidad': cantidad,
    'modificadores': [for (final m in mods) m.id],
    'nota': nota,
    'tiempo': tiempo,
    'enEspera': enEspera,
  };
}

class EstadoOrden {
  const EstadoOrden({this.orden, this.borrador = const [], this.bloqueadaPor, this.cargando = true, this.enviando = false, this.deshacer, this.error});
  final Orden? orden;
  final List<LineaBorrador> borrador;
  final String? bloqueadaPor; // «Editando: Ana R.»
  final bool cargando, enviando;
  final (int, LineaBorrador)? deshacer;
  final String? error;

  Decimal get totalBorrador => borrador.fold(Decimal.zero, (a, l) => a + l.total);
  Decimal get totalEnviado => Dinero.parse(orden?.total ?? '0');
  bool get soloLectura => bloqueadaPor != null;

  EstadoOrden copia({
    Orden? orden,
    List<LineaBorrador>? borrador,
    String? bloqueadaPor,
    bool limpiarBloqueo = false,
    bool? cargando,
    bool? enviando,
    (int, LineaBorrador)? deshacer,
    bool limpiarDeshacer = false,
    String? error,
    bool limpiarError = false,
  }) => EstadoOrden(
    orden: orden ?? this.orden,
    borrador: borrador ?? this.borrador,
    bloqueadaPor: limpiarBloqueo ? null : (bloqueadaPor ?? this.bloqueadaPor),
    cargando: cargando ?? this.cargando,
    enviando: enviando ?? this.enviando,
    deshacer: limpiarDeshacer ? null : (deshacer ?? this.deshacer),
    error: limpiarError ? null : (error ?? this.error),
  );
}

enum ResultadoEnvio { enviado, pendiente }

final ordenProvider = NotifierProvider.autoDispose.family<OrdenControlador, EstadoOrden, String>(OrdenControlador.new);

/// Controla la mesa abierta: bloqueo con latido cada 10 s (F3-07), borrador local y envío
/// idempotente con cola «Pendiente de envío» (F3-10).
class OrdenControlador extends Notifier<EstadoOrden> {
  OrdenControlador(this.mesaId);
  final String mesaId;

  Timer? _latido, _deshacer;
  var _tengoBloqueo = false;
  String? _claveEnvio; // se conserva entre reintentos del mismo envío

  @override
  EstadoOrden build() {
    ref.onDispose(() {
      _latido?.cancel();
      _deshacer?.cancel();
      if (_tengoBloqueo) ref.read(sesionProvider).api?.liberar(mesaId).catchError((_) {});
    });
    Future.microtask(_abrir);
    return const EstadoOrden();
  }

  Future<void> _abrir() async {
    final api = ref.read(sesionProvider).api;
    if (api == null) return;
    try {
      await api.bloquear(mesaId);
      _tengoBloqueo = true;
      _latido = Timer.periodic(const Duration(seconds: 10), (_) => _latir());
    } on ApiError catch (e) {
      if (e.code == 'LOCKED_BY') {
        final nombre = RegExp(r'editando (.+?)\.').firstMatch(e.detalle)?.group(1) ?? 'otro mesero';
        state = state.copia(bloqueadaPor: nombre);
      } else if (!e.esRed) {
        state = state.copia(error: e.detalle);
      }
    }
    try {
      final o = await api.ordenDeMesa(mesaId);
      state = EstadoOrden(orden: o, borrador: state.borrador, bloqueadaPor: state.bloqueadaPor, cargando: false);
    } on ApiError {
      state = state.copia(cargando: false);
    }
  }

  Future<void> _latir() async {
    try {
      await ref.read(sesionProvider).api?.latido(mesaId);
    } on ApiError catch (e) {
      if (e.code == 'BLOQUEO_PERDIDO' || e.code == 'LOCKED_BY') {
        _tengoBloqueo = false;
        _latido?.cancel();
        state = state.copia(error: e.detalle);
      }
    }
  }

  /// Agregar un plato (≤ 50 ms: todo local). Si ya hay una línea idéntica, suma cantidad.
  void agregar(Producto p, {List<Modificador> mods = const [], String nota = '', String cantidad = '1'}) {
    if (state.soloLectura) return;
    final b = [...state.borrador];
    final i = b.indexWhere((l) => l.mismaQue(p, mods, nota));
    if (i >= 0) {
      b[i] = b[i].copia(cantidad: (Dinero.parse(b[i].cantidad) + Dinero.parse(cantidad)).toString());
    } else {
      b.add(LineaBorrador(producto: p, mods: mods, nota: nota, cantidad: cantidad));
    }
    state = state.copia(borrador: b, limpiarError: true);
  }

  /// Quitar con «Deshacer» durante 4 s (F3-11).
  void quitar(String id) {
    final i = state.borrador.indexWhere((l) => l.id == id);
    if (i < 0) return;
    final l = state.borrador[i];
    state = state.copia(borrador: [...state.borrador]..removeAt(i), deshacer: (i, l));
    _deshacer?.cancel();
    _deshacer = Timer(const Duration(seconds: 4), () => state = state.copia(limpiarDeshacer: true));
  }

  void deshacer() {
    final d = state.deshacer;
    if (d == null) return;
    final b = [...state.borrador]..insert(d.$1.clamp(0, state.borrador.length), d.$2);
    state = state.copia(borrador: b, limpiarDeshacer: true);
  }

  void reemplazar(LineaBorrador l) => state = state.copia(borrador: [for (final x in state.borrador) x.id == l.id ? l : x]);

  /// Envía los platos nuevos. Sin nodo, queda en la cola y se envía sola al volver.
  Future<ResultadoEnvio> enviar() async {
    final api = ref.read(sesionProvider).api;
    if (state.borrador.isEmpty || api == null) return ResultadoEnvio.enviado;
    _claveEnvio ??= const Uuid().v7();
    final cuerpo = {
      'idempotencyKey': _claveEnvio,
      'ordenId': state.orden?.id ?? const Uuid().v7(),
      'mesaId': mesaId,
      'lineas': [for (final l in state.borrador) l.aJson()],
    };
    state = state.copia(enviando: true, limpiarError: true);
    try {
      final r = await api.enviar(cuerpo);
      _claveEnvio = null;
      _tengoBloqueo = false; // el nodo libera la mesa al enviar
      _latido?.cancel();
      state = state.copia(orden: Orden.fromJson(r['orden'] as Map<String, dynamic>), borrador: const [], enviando: false);
      ref.read(salonProvider.notifier).refrescar();
      return ResultadoEnvio.enviado;
    } on ApiError catch (e) {
      if (e.esRed) {
        await ref.read(colaProvider).encolar(cuerpo);
        _claveEnvio = null;
        state = state.copia(borrador: const [], enviando: false);
        return ResultadoEnvio.pendiente;
      }
      state = state.copia(enviando: false, error: e.detalle);
      rethrow;
    }
  }

  Future<void> recargar() async {
    final api = ref.read(sesionProvider).api;
    if (api == null) return;
    final o = await api.ordenDeMesa(mesaId);
    state = EstadoOrden(orden: o, borrador: state.borrador, bloqueadaPor: state.bloqueadaPor, cargando: false);
  }

  Future<Map<String, dynamic>> precuenta(int personas) async {
    final r = await ref.read(sesionProvider).api!.precuenta(state.orden!.id, personas);
    await recargar();
    ref.read(salonProvider.notifier).refrescar();
    return r;
  }

  Future<void> marchar(String tiempo) async {
    await ref.read(sesionProvider).api!.marchar(state.orden!.id, const Uuid().v7(), tiempo);
    await recargar();
  }

  /// Anular platos enviados; si el nodo pide supervisor, [autorizacion] trae su token.
  Future<void> anular(List<String> lineas, String motivo, bool sePreparo, {String? autorizacion}) async {
    final o = await ref.read(sesionProvider).api!.anular(state.orden!.id, lineas, motivo, sePreparo, autorizacion: autorizacion);
    state = state.copia(orden: o);
    ref.read(salonProvider.notifier).refrescar();
  }
}
