import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:web_socket_channel/web_socket_channel.dart';

/// Estado de la conexión para el indicador tipo «Dynamic Island» (RF-02-09.3).
enum Conexion {
  /// 🟢 Nodo y nube en línea.
  enLinea,

  /// 🟡 Nodo en línea, sin internet: se opera local.
  sinInternet,

  /// 🔴 Sin Nodo Local (fuera del WiFi o nodo apagado).
  sinNodo,
}

/// Evento de tiempo real del nodo: {v, id, type, ts, data}.
class EventoNodo {
  const EventoNodo(this.tipo, this.datos);
  final String tipo;
  final Map<String, dynamic> datos;
}

/// WebSocket al Nodo Local con reconexión automática (espera exponencial con tope y jitter):
/// el mesero nunca tiene que «reconectar» a mano (F3-03).
class TiempoReal {
  TiempoReal(this._url, {WebSocketChannel Function(Uri)? conectar}) : _conectar = conectar ?? WebSocketChannel.connect;

  final Uri Function() _url;
  final WebSocketChannel Function(Uri) _conectar;
  final _eventos = StreamController<EventoNodo>.broadcast();
  final _conectado = StreamController<bool>.broadcast();
  WebSocketChannel? _canal;
  StreamSubscription<dynamic>? _sub;
  Timer? _reintento;
  var _intentos = 0;
  var _activo = false;
  bool conectado = false;

  Stream<EventoNodo> get eventos => _eventos.stream;
  Stream<bool> get cambiosConexion => _conectado.stream;

  void iniciar() {
    if (_activo) return;
    _activo = true;
    _abrir();
  }

  /// Reabre con las credenciales actuales (p. ej. tras cambiar de usuario).
  void reiniciar() {
    _cerrarCanal();
    _intentos = 0;
    if (_activo) _abrir();
  }

  void detener() {
    _activo = false;
    _reintento?.cancel();
    _cerrarCanal();
  }

  void _cerrarCanal() {
    _sub?.cancel();
    _sub = null;
    _canal?.sink.close();
    _canal = null;
    _marcar(false);
  }

  void _marcar(bool v) {
    if (conectado == v) return;
    conectado = v;
    _conectado.add(v);
  }

  Future<void> _abrir() async {
    try {
      final c = _conectar(_url());
      _canal = c;
      await c.ready;
      _intentos = 0;
      _marcar(true);
      _sub = c.stream.listen(_recibir, onDone: _caida, onError: (_) => _caida(), cancelOnError: true);
    } catch (_) {
      _caida();
    }
  }

  void _recibir(dynamic raw) {
    try {
      final j = jsonDecode(raw as String) as Map<String, dynamic>;
      final tipo = j['type'] as String?;
      if (tipo == null) return; // respuestas de error del hub
      _eventos.add(EventoNodo(tipo, (j['data'] as Map?)?.cast<String, dynamic>() ?? const {}));
    } catch (_) {}
  }

  void _caida() {
    _sub?.cancel();
    _sub = null;
    _canal = null;
    _marcar(false);
    if (!_activo) return;
    _intentos++;
    final base = min(30000, 500 * pow(2, min(_intentos - 1, 6)).toInt());
    final espera = Duration(milliseconds: (base * (0.8 + Random().nextDouble() * 0.4)).round());
    _reintento?.cancel();
    _reintento = Timer(espera, _abrir);
  }

  /// Espera para el intento n (visible para pruebas).
  static Duration esperaPara(int intento) => Duration(milliseconds: min(30000, 500 * pow(2, min(intento - 1, 6)).toInt()));

  Future<void> cerrar() async {
    detener();
    await _eventos.close();
    await _conectado.close();
  }
}
