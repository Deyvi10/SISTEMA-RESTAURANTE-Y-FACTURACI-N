import 'dart:convert';

import 'package:cryptography/cryptography.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:uuid/uuid.dart';

/// Dónde se guarda la identidad del dispositivo. En el teléfono, el llavero seguro
/// (Keystore en Android, Keychain en iOS); en las pruebas, memoria.
abstract class Almacen {
  Future<String?> leer(String clave);
  Future<void> escribir(String clave, String valor);
  Future<void> borrar(String clave);
}

class AlmacenSeguro implements Almacen {
  AlmacenSeguro([FlutterSecureStorage? s]) : _s = s ?? const FlutterSecureStorage();
  final FlutterSecureStorage _s;
  @override
  Future<String?> leer(String clave) => _s.read(key: clave);
  @override
  Future<void> escribir(String clave, String valor) => _s.write(key: clave, value: valor);
  @override
  Future<void> borrar(String clave) => _s.delete(key: clave);
}

class AlmacenMemoria implements Almacen {
  final Map<String, String> datos = {};
  @override
  Future<String?> leer(String clave) async => datos[clave];
  @override
  Future<void> escribir(String clave, String valor) async => datos[clave] = valor;
  @override
  Future<void> borrar(String clave) async => datos.remove(clave);
}

/// Identidad del dispositivo: su id (UUID v7) y su par ed25519. La llave privada nunca sale
/// del teléfono; al nodo solo viaja la pública y firmas de desafíos (F3-02).
class Identidad {
  Identidad._(this.dispositivoId, this._par, this.publica, {required this.urls, required this.restaurante, required this.local});

  final String dispositivoId;
  final SimpleKeyPair _par;
  final List<int> publica;
  List<String> urls;
  String restaurante;
  String local;

  bool get emparejado => urls.isNotEmpty;
  String get publicaB64 => base64.encode(publica);

  static final _ed = Ed25519();
  static const _clave = 'restpos.identidad.v1';

  /// Lee la identidad guardada o crea una nueva (sin emparejar).
  static Future<Identidad> cargar(Almacen a) async {
    final raw = await a.leer(_clave);
    if (raw != null) {
      final j = jsonDecode(raw) as Map<String, dynamic>;
      final semilla = base64.decode(j['semilla'] as String);
      final par = await _ed.newKeyPairFromSeed(semilla);
      final pub = (await par.extractPublicKey()).bytes;
      return Identidad._(j['id'] as String, par, pub,
          urls: [for (final u in (j['urls'] as List? ?? const [])) u as String],
          restaurante: j['restaurante'] as String? ?? '', local: j['local'] as String? ?? '');
    }
    final par = await _ed.newKeyPair();
    final pub = (await par.extractPublicKey()).bytes;
    final id = Identidad._(const Uuid().v7(), par, pub, urls: const [], restaurante: '', local: '');
    await id.guardar(a);
    return id;
  }

  Future<void> guardar(Almacen a) async {
    final semilla = await _par.extractPrivateKeyBytes();
    await a.escribir(_clave, jsonEncode({'id': dispositivoId, 'semilla': base64.encode(semilla), 'urls': urls, 'restaurante': restaurante, 'local': local}));
  }

  /// Desempareja: borra todo y la próxima vez nace una identidad nueva.
  static Future<void> olvidar(Almacen a) => a.borrar(_clave);

  /// Firma el desafío del nodo: `restpos|sesion-dispositivo|nonce`.
  Future<String> firmarDesafio(String nonce) async {
    final firma = await _ed.sign(utf8.encode('restpos|sesion-dispositivo|$nonce'), keyPair: _par);
    return base64.encode(firma.bytes);
  }
}

/// Datos que trae el QR: `restpos://emparejar?c=CODIGO&u=http://ip:puerto`.
class DatosQR {
  const DatosQR(this.codigo, this.urls);
  final String codigo;
  final List<String> urls;

  static DatosQR? leer(String texto) {
    final u = Uri.tryParse(texto.trim());
    if (u == null || u.scheme != 'restpos' || u.host != 'emparejar') return null;
    final c = u.queryParameters['c'];
    final urls = u.queryParametersAll['u'] ?? const [];
    if (c == null || c.length != 8 || urls.isEmpty) return null;
    return DatosQR(c, urls);
  }
}
