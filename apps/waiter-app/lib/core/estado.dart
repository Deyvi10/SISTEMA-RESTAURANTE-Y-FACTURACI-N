import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/db.dart';
import '../data/modelos.dart';
import 'api_error.dart';
import 'busqueda.dart';
import 'descubrimiento.dart';
import 'identidad.dart';
import 'nodo_api.dart';
import 'tiempo_real.dart';

/// Dependencias que se reemplazan en las pruebas.
final almacenProvider = Provider<Almacen>((ref) => AlmacenSeguro());
final baseLocalProvider = Provider<BaseLocal>((ref) => throw UnimplementedError('Se define en main()'));
final descubridorProvider = Provider<Descubridor>((ref) => Descubridor());
final apiFabricaProvider = Provider<NodoApi Function(String base)>(
  (ref) =>
      (base) => NodoApi(base: base),
);
final tiempoRealFabricaProvider = Provider<TiempoReal Function(Uri Function())>(
  (ref) =>
      (url) => TiempoReal(url),
);

// ---------- Sesión: dispositivo y usuario ----------

enum Fase { cargando, sinEmparejar, buscandoNodo, elegirPersona, adentro }

class EstadoSesion {
  const EstadoSesion({required this.fase, this.identidad, this.api, this.usuario, this.error});
  final Fase fase;
  final Identidad? identidad;
  final NodoApi? api;
  final Usuario? usuario;
  final String? error;

  EstadoSesion copia({Fase? fase, Identidad? identidad, NodoApi? api, Usuario? usuario, bool sinUsuario = false, String? error, bool sinError = false}) =>
      EstadoSesion(
        fase: fase ?? this.fase,
        identidad: identidad ?? this.identidad,
        api: api ?? this.api,
        usuario: sinUsuario ? null : (usuario ?? this.usuario),
        error: sinError ? null : (error ?? this.error),
      );
}

final sesionProvider = NotifierProvider<SesionControlador, EstadoSesion>(SesionControlador.new);

class SesionControlador extends Notifier<EstadoSesion> {
  TiempoReal? _tr;
  StreamSubscription<EventoNodo>? _sub;

  @override
  EstadoSesion build() {
    ref.onDispose(() {
      _sub?.cancel();
      _tr?.detener();
    });
    Future.microtask(iniciar);
    return const EstadoSesion(fase: Fase.cargando);
  }

  TiempoReal? get tiempoReal => _tr;

  /// Arranque en frío: identidad → nodo → sesión del dispositivo → cuadrícula de personal.
  Future<void> iniciar() async {
    final id = await Identidad.cargar(ref.read(almacenProvider));
    if (!id.emparejado) {
      state = EstadoSesion(fase: Fase.sinEmparejar, identidad: id);
      return;
    }
    await conectar(id);
  }

  Future<void> conectar(Identidad id) async {
    state = EstadoSesion(fase: Fase.buscandoNodo, identidad: id);
    final base = await ref.read(descubridorProvider).encontrar(id.urls, ref.read(apiFabricaProvider));
    if (base == null) {
      state = state.copia(error: 'No encuentro el Nodo Local. Revisa que estés en el WiFi del restaurante.');
      return;
    }
    final api = ref.read(apiFabricaProvider)(base);
    try {
      await api.abrirSesionDispositivo(id);
    } on ApiError catch (e) {
      if (e.status == 401) {
        // Revocado o desemparejado en el nodo: hay que escanear de nuevo.
        await desemparejar(motivo: e.detalle);
        return;
      }
      state = state.copia(error: e.detalle);
      return;
    }
    if (!id.urls.contains(base)) {
      id.urls = [base, ...id.urls];
      await id.guardar(ref.read(almacenProvider));
    }
    state = EstadoSesion(fase: Fase.elegirPersona, identidad: id, api: api);
    _iniciarTiempoReal(api);
  }

  void _iniciarTiempoReal(NodoApi api) {
    _sub?.cancel();
    _tr?.detener();
    _tr = ref.read(tiempoRealFabricaProvider)(api.urlTiempoReal)..iniciar();
    _sub = _tr!.eventos.listen(_alEvento);
    ref.read(tiempoRealProvider.notifier).fijar(_tr);
  }

  void _alEvento(EventoNodo e) {
    final u = state.usuario;
    if (e.tipo == 'user.deactivated' && u != null && e.datos['user_id'] == u.id) {
      salir(motivo: 'Tu usuario fue desactivado.');
    }
    if (e.tipo == 'device.revoked') {
      desemparejar(motivo: 'Este dispositivo fue revocado desde el backoffice.');
    }
  }

  /// Emparejar con los datos del QR (o el código escrito a mano).
  Future<void> emparejar(DatosQR qr, {required String nombre, required String plataforma, required String version}) async {
    final id = state.identidad ?? await Identidad.cargar(ref.read(almacenProvider));
    final base = await ref.read(descubridorProvider).encontrar(qr.urls, ref.read(apiFabricaProvider));
    if (base == null) {
      throw const ApiError(0, 'SIN_NODO', 'No llego al Nodo Local. El teléfono debe estar en el mismo WiFi que la caja.');
    }
    final r = await ref.read(apiFabricaProvider)(base).emparejar(id, qr.codigo, nombre, plataforma, version);
    id.urls = [base, ...qr.urls.where((u) => u != base)];
    id.restaurante = r['restaurante'] as String? ?? '';
    id.local = r['local'] as String? ?? '';
    await id.guardar(ref.read(almacenProvider));
    await conectar(id);
  }

  Future<void> entrar(Persona p, String pin) async {
    final api = state.api!;
    final u = await api.entrar(p.id, pin);
    state = state.copia(fase: Fase.adentro, usuario: u, sinError: true);
    _tr?.reiniciar();
  }

  /// Bloqueo de pantalla / cambio de usuario: vuelve a la cuadrícula sin perder las mesas.
  Future<void> salir({String? motivo}) async {
    try {
      await state.api?.salir();
    } catch (_) {}
    state = state.copia(fase: Fase.elegirPersona, sinUsuario: true, error: motivo);
    _tr?.reiniciar();
  }

  Future<void> desemparejar({String? motivo}) async {
    _sub?.cancel();
    _tr?.detener();
    await Identidad.olvidar(ref.read(almacenProvider));
    final nueva = await Identidad.cargar(ref.read(almacenProvider));
    state = EstadoSesion(fase: Fase.sinEmparejar, identidad: nueva, error: motivo);
  }
}

/// El canal de tiempo real vigente (null antes de conectar).
final tiempoRealProvider = NotifierProvider<CanalVigente, TiempoReal?>(CanalVigente.new);

class CanalVigente extends Notifier<TiempoReal?> {
  @override
  TiempoReal? build() => null;
  void fijar(TiempoReal? t) => state = t;
}

/// Eventos del nodo (cualquier pantalla puede escucharlos).
final eventosProvider = StreamProvider<EventoNodo>((ref) {
  final tr = ref.watch(tiempoRealProvider);
  return tr?.eventos ?? const Stream.empty();
});

// ---------- Conectividad (🟢 🟡 🔴) ----------

final conexionProvider = NotifierProvider<ConexionControlador, Conexion>(ConexionControlador.new);

class ConexionControlador extends Notifier<Conexion> {
  Timer? _t;
  StreamSubscription<bool>? _s;

  @override
  Conexion build() {
    final tr = ref.watch(tiempoRealProvider);
    _s?.cancel();
    _s = tr?.cambiosConexion.listen((_) => revisar());
    _t?.cancel();
    _t = Timer.periodic(const Duration(seconds: 20), (_) => revisar());
    ref.onDispose(() {
      _t?.cancel();
      _s?.cancel();
    });
    Future.microtask(revisar);
    return tr?.conectado == true ? Conexion.enLinea : Conexion.sinNodo;
  }

  Future<void> revisar() async {
    final api = ref.read(sesionProvider).api;
    if (api == null) {
      state = Conexion.sinNodo;
      return;
    }
    try {
      final c = await api.conectividad();
      state = c['nube'] == 'EN_LINEA' ? Conexion.enLinea : Conexion.sinInternet;
    } on ApiError {
      state = Conexion.sinNodo;
    }
  }
}

// ---------- Catálogo offline (F3-05) ----------

class CatalogoListo {
  CatalogoListo(this.catalogo)
    : indice = IndiceBusqueda<Producto>(catalogo.productos, nombre: (p) => p.nombre, alias: (p) => p.alias, vendidos: (p) => p.vendidos);
  final Catalogo catalogo;
  final IndiceBusqueda<Producto> indice;
}

final catalogoProvider = AsyncNotifierProvider<CatalogoControlador, CatalogoListo>(CatalogoControlador.new);

class CatalogoControlador extends AsyncNotifier<CatalogoListo> {
  static const _clave = 'catalogo';

  @override
  Future<CatalogoListo> build() async {
    ref.listen(eventosProvider, (_, e) {
      if (e.value?.tipo == 'catalog.updated') actualizar();
    });
    final db = ref.read(baseLocalProvider);
    final guardado = await db.leerCache(_clave);
    if (guardado != null) {
      Future.microtask(actualizar); // mostrar ya lo guardado y refrescar detrás
      return CatalogoListo(Catalogo.fromJson(jsonDecode(guardado) as Map<String, dynamic>));
    }
    return _descargar();
  }

  Future<CatalogoListo> _descargar() async {
    final api = ref.read(sesionProvider).api;
    if (api == null) return CatalogoListo(Catalogo.vacio);
    final crudo = await api.catalogoCrudo();
    await ref.read(baseLocalProvider).guardarCache(_clave, jsonEncode(crudo));
    return CatalogoListo(Catalogo.fromJson(crudo));
  }

  Future<void> actualizar() async {
    try {
      final c = await _descargar();
      state = AsyncData(c);
    } on ApiError {
      // Sin red: se sigue con lo guardado.
    }
  }
}

// ---------- Salón en vivo (F3-06) ----------

final salonProvider = AsyncNotifierProvider<SalonControlador, Salon>(SalonControlador.new);

class SalonControlador extends AsyncNotifier<Salon> {
  Timer? _debounce, _poll;

  @override
  Future<Salon> build() async {
    ref.listen(eventosProvider, (_, e) {
      final t = e.value?.tipo ?? '';
      if (t.startsWith('table.') || t.startsWith('order.')) {
        _debounce?.cancel();
        _debounce = Timer(const Duration(milliseconds: 120), refrescar);
      }
    });
    _poll = Timer.periodic(const Duration(seconds: 20), (_) => refrescar());
    ref.onDispose(() {
      _debounce?.cancel();
      _poll?.cancel();
    });
    final api = ref.read(sesionProvider).api;
    return api == null ? Salon.vacio : api.salon();
  }

  Future<void> refrescar() async {
    final api = ref.read(sesionProvider).api;
    if (api == null) return;
    try {
      state = AsyncData(await api.salon());
    } on ApiError {
      // se conserva el último estado conocido
    }
  }
}

// ---------- Avisos (impresoras, envíos pendientes) ----------

class Aviso {
  const Aviso(this.id, this.titulo, this.detalle, {this.grave = false});
  final String id, titulo, detalle;
  final bool grave;
}

final avisosProvider = NotifierProvider<AvisosControlador, List<Aviso>>(AvisosControlador.new);

class AvisosControlador extends Notifier<List<Aviso>> {
  @override
  List<Aviso> build() {
    ref.listen(eventosProvider, (_, e) {
      final ev = e.value;
      if (ev?.tipo != 'printer.status') return;
      final d = ev!.datos;
      final id = 'imp-${d['printer_id']}';
      final estado = d['status'] as String? ?? 'OK';
      if (estado == 'OK' || estado == 'POCO_PAPEL') {
        quitar(id);
      } else {
        const txt = {'SIN_PAPEL': 'se quedó sin papel', 'TAPA_ABIERTA': 'tiene la tapa abierta', 'SIN_CONEXION': 'no responde', 'ERROR': 'tiene un error'};
        agregar(
          Aviso(
            id,
            'Impresora ${d['name']}',
            '${d['name']} ${txt[estado] ?? 'tiene un problema'}. Tus comandas esperan y salen solas al resolverlo.',
            grave: true,
          ),
        );
      }
    });
    return const [];
  }

  void agregar(Aviso a) => state = [a, ...state.where((x) => x.id != a.id)];
  void quitar(String id) => state = state.where((x) => x.id != id).toList();
}

// ---------- Cola «Pendiente de envío» (F3-10) ----------

final colaProvider = Provider<ColaEnvios>((ref) {
  final c = ColaEnvios(ref);
  ref.onDispose(c.cerrar);
  return c;
});

final pendientesProvider = StreamProvider<List<Pendiente>>((ref) => ref.read(baseLocalProvider).vigilarPendientes());

/// Reenvía sola las comandas que no llegaron al nodo, en orden y con su misma clave de
/// idempotencia (el nodo nunca las duplica, QA-08).
class ColaEnvios {
  ColaEnvios(this._ref) {
    _t = Timer.periodic(const Duration(seconds: 5), (_) => vaciar());
    _ref.listen(conexionProvider, (_, c) {
      if (c != Conexion.sinNodo) vaciar();
    });
  }

  final Ref _ref;
  Timer? _t;
  var _enCurso = false;

  Future<void> encolar(Map<String, dynamic> cuerpo) =>
      _ref.read(baseLocalProvider).encolar(cuerpo['idempotencyKey'] as String, cuerpo['mesaId'] as String, jsonEncode(cuerpo));

  Future<int> vaciar() async {
    if (_enCurso) return 0;
    final api = _ref.read(sesionProvider).api;
    if (api == null || api.tokenUsuario == null) return 0;
    _enCurso = true;
    var enviados = 0;
    try {
      final db = _ref.read(baseLocalProvider);
      for (final p in await db.listaPendientes()) {
        try {
          await api.enviar(jsonDecode(p.cuerpo) as Map<String, dynamic>);
          await db.quitarPendiente(p.clave);
          enviados++;
        } on ApiError catch (e) {
          if (e.esRed) break; // sigue sin nodo: se reintenta luego, en orden
          await db.falloPendiente(p.clave, e.detalle);
          _ref.read(avisosProvider.notifier).agregar(Aviso('pend-${p.clave}', 'Comanda sin enviar', e.detalle, grave: true));
        }
      }
    } finally {
      _enCurso = false;
    }
    if (enviados > 0) _ref.read(salonProvider.notifier).refrescar();
    return enviados;
  }

  void cerrar() => _t?.cancel();
}
