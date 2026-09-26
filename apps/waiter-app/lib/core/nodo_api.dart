import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;

import '../data/modelos.dart';
import 'api_error.dart';
import 'identidad.dart';

/// Cliente HTTP del Nodo Local. Lleva las credenciales del dispositivo y del usuario en una
/// sola cabecera Authorization y traduce los errores a [ApiError] con texto para el mesero.
class NodoApi {
  NodoApi({required this.base, http.Client? cliente, this.timeout = const Duration(seconds: 8)}) : _http = cliente ?? http.Client();

  String base;
  final http.Client _http;
  final Duration timeout;
  String? tokenDispositivo;
  String? tokenUsuario;

  Map<String, String> get _cabeceras {
    final cred = [
      if (tokenDispositivo != null) 'Dispositivo $tokenDispositivo',
      if (tokenUsuario != null) 'Usuario $tokenUsuario',
    ];
    return {'Content-Type': 'application/json', 'Accept': 'application/json', if (cred.isNotEmpty) 'Authorization': cred.join(', ')};
  }

  Future<dynamic> _llamar(String metodo, String ruta, [Object? cuerpo]) async {
    final uri = Uri.parse('$base$ruta');
    final req = http.Request(metodo, uri)..headers.addAll(_cabeceras);
    if (cuerpo != null) req.body = jsonEncode(cuerpo);
    http.StreamedResponse res;
    try {
      res = await _http.send(req).timeout(timeout);
    } on TimeoutException {
      throw ApiError.sinNodo();
    } on SocketException {
      throw ApiError.sinNodo();
    } on http.ClientException {
      throw ApiError.sinNodo();
    }
    final texto = await res.stream.bytesToString();
    final datos = texto.isEmpty ? null : jsonDecode(texto);
    if (res.statusCode >= 300) {
      final m = datos is Map<String, dynamic> ? datos : const <String, dynamic>{};
      throw ApiError(res.statusCode, m['code'] as String? ?? 'ERROR', m['detail'] as String? ?? 'El nodo respondió ${res.statusCode}.');
    }
    return datos;
  }

  /// ¿Responde el nodo en esta URL?
  Future<bool> vivo() async {
    try {
      await _llamar('GET', '/health');
      return true;
    } on ApiError {
      return false;
    }
  }

  // ---------- Emparejamiento y sesión del dispositivo ----------

  Future<Map<String, dynamic>> emparejar(Identidad id, String codigo, String nombre, String plataforma, String version) async =>
      await _llamar('POST', '/v1/dispositivos/emparejar', {
        'codigo': codigo, 'dispositivoId': id.dispositivoId, 'llavePublica': id.publicaB64,
        'nombre': nombre, 'tipo': 'MOVIL', 'plataforma': plataforma, 'versionApp': version,
      }) as Map<String, dynamic>;

  /// Desafío-respuesta: el nodo manda un número, el teléfono lo firma con su llave privada.
  Future<void> abrirSesionDispositivo(Identidad id) async {
    final d = await _llamar('GET', '/v1/dispositivos/desafio?dispositivoId=${id.dispositivoId}') as Map<String, dynamic>;
    final nonce = d['nonce'] as String;
    final s = await _llamar('POST', '/v1/dispositivos/sesion', {'dispositivoId': id.dispositivoId, 'nonce': nonce, 'firma': await id.firmarDesafio(nonce)}) as Map<String, dynamic>;
    tokenDispositivo = s['token'] as String;
  }

  // ---------- Personal y PIN ----------

  Future<List<Persona>> personal() async => [for (final p in (await _llamar('GET', '/v1/personal') as List)) Persona.fromJson(p as Map<String, dynamic>)];

  Future<Usuario> entrar(String usuarioId, String pin) async {
    final s = await _llamar('POST', '/v1/sesiones', {'usuarioId': usuarioId, 'pin': pin}) as Map<String, dynamic>;
    tokenUsuario = s['token'] as String;
    return Usuario.fromJson(s['usuario'] as Map<String, dynamic>);
  }

  Future<void> salir() async {
    try {
      await _llamar('DELETE', '/v1/sesiones');
    } finally {
      tokenUsuario = null;
    }
  }

  Future<String> autorizar(String supervisorId, String pin, String accion, String referencia) async {
    final a = await _llamar('POST', '/v1/autorizaciones', {'usuarioId': supervisorId, 'pin': pin, 'accion': accion, 'referencia': referencia}) as Map<String, dynamic>;
    return a['token'] as String;
  }

  // ---------- Catálogo y salón ----------

  Future<Map<String, dynamic>> catalogoCrudo() async => await _llamar('GET', '/v1/catalogo') as Map<String, dynamic>;
  Future<Salon> salon() async => Salon.fromJson(await _llamar('GET', '/v1/salon') as Map<String, dynamic>);
  Future<Map<String, dynamic>> conectividad() async => await _llamar('GET', '/v1/conectividad') as Map<String, dynamic>;

  Future<void> bloquear(String mesa) => _llamar('POST', '/v1/mesas/$mesa/bloqueo');
  Future<void> latido(String mesa) => _llamar('POST', '/v1/mesas/$mesa/latido');
  Future<void> liberar(String mesa) => _llamar('DELETE', '/v1/mesas/$mesa/bloqueo');

  Future<Orden?> ordenDeMesa(String mesa) async {
    try {
      return Orden.fromJson(await _llamar('GET', '/v1/mesas/$mesa/orden') as Map<String, dynamic>);
    } on ApiError catch (e) {
      if (e.code == 'MESA_LIBRE') return null;
      rethrow;
    }
  }

  // ---------- Órdenes ----------

  /// Envía la comanda. [cuerpo] ya trae su idempotencyKey (se reenvía igual si se pierde la respuesta).
  Future<Map<String, dynamic>> enviar(Map<String, dynamic> cuerpo) async => await _llamar('POST', '/v1/ordenes/enviar', cuerpo) as Map<String, dynamic>;

  Future<Map<String, dynamic>> marchar(String orden, String clave, String tiempo) async =>
      await _llamar('POST', '/v1/ordenes/$orden/marchar', {'idempotencyKey': clave, 'tiempo': tiempo}) as Map<String, dynamic>;

  Future<Map<String, dynamic>> precuenta(String orden, int personas) async =>
      await _llamar('POST', '/v1/ordenes/$orden/precuenta', {'personas': personas}) as Map<String, dynamic>;

  Future<Orden> anular(String orden, List<String> lineas, String motivo, bool sePreparo, {String? autorizacion}) async => Orden.fromJson(
      await _llamar('POST', '/v1/ordenes/$orden/anular', {'lineas': lineas, 'motivo': motivo, 'sePreparo': sePreparo, 'autorizacion': ?autorizacion})
          as Map<String, dynamic>);

  Future<Orden> mover(String orden, String mesa, {List<String> lineas = const []}) async =>
      Orden.fromJson(await _llamar('POST', '/v1/ordenes/$orden/mover', {'mesaId': mesa, 'lineas': lineas}) as Map<String, dynamic>);

  Future<Orden> unir(String orden, String destino) async =>
      Orden.fromJson(await _llamar('POST', '/v1/ordenes/$orden/unir', {'ordenDestino': destino}) as Map<String, dynamic>);

  Future<Orden> transferir(String orden, String mesero) async =>
      Orden.fromJson(await _llamar('POST', '/v1/ordenes/$orden/transferir', {'meseroId': mesero}) as Map<String, dynamic>);

  /// URL absoluta de una foto servida por el nodo (/media/…).
  String media(String ruta) => '$base$ruta';

  /// URL del WebSocket con las credenciales como parámetros.
  Uri urlTiempoReal() {
    final u = Uri.parse(base);
    return u.replace(scheme: u.scheme == 'https' ? 'wss' : 'ws', path: '/v1/ws', queryParameters: {
      'dispositivo': ?tokenDispositivo,
      'usuario': ?tokenUsuario,
    });
  }
}
