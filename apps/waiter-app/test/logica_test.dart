import 'dart:convert';

import 'package:cryptography/cryptography.dart';
import 'package:decimal/decimal.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:restpos_meseros/core/api_error.dart';
import 'package:restpos_meseros/core/busqueda.dart';
import 'package:restpos_meseros/core/dinero.dart';
import 'package:restpos_meseros/core/identidad.dart';
import 'package:restpos_meseros/core/nodo_api.dart';
import 'package:restpos_meseros/core/tiempo_real.dart';
import 'package:restpos_meseros/data/modelos.dart';

Producto prod(String nombre, {String alias = '', int vendidos = 0}) => Producto(
    id: nombre, categoriaId: 'c', nombre: nombre, alias: alias, descripcion: '', precio: '1.00', foto: null, fotoMd: null, grupos: const [], vendidos: vendidos);

void main() {
  group('Búsqueda predictiva (F3-08)', () {
    final menu = [
      prod('Ceviche Mixto', alias: 'CM', vendidos: 50),
      prod('Ceviche de Camarón', vendidos: 90),
      prod('Seco de Pollo', alias: 'SP', vendidos: 30),
      prod('Encebollado', vendidos: 70),
      prod('Jugo de Maracuyá'),
    ];
    final idx = IndiceBusqueda<Producto>(menu, nombre: (p) => p.nombre, alias: (p) => p.alias, vendidos: (p) => p.vendidos);

    test('«cev» encuentra los ceviches, primero el más vendido', () {
      expect(idx.buscar('cev').map((p) => p.nombre), ['Ceviche de Camarón', 'Ceviche Mixto']);
    });
    test('sin tildes ni mayúsculas', () {
      expect(idx.buscar('MARACUYA').single.nombre, 'Jugo de Maracuyá');
      expect(idx.buscar('camaron').single.nombre, 'Ceviche de Camarón');
    });
    test('prefijos de varias palabras: «sec pol» → Seco de Pollo', () {
      expect(idx.buscar('sec pol').single.nombre, 'Seco de Pollo');
    });
    test('alias exacto gana: «cm»', () {
      expect(idx.buscar('cm').first.nombre, 'Ceviche Mixto');
    });
    test('nada coincide', () => expect(idx.buscar('pizza'), isEmpty));

    test('p95 ≤ 50 ms por pulsación con 500 productos', () {
      final grande = [for (var i = 0; i < 500; i++) prod('Plato número $i de la casa ${i % 7 == 0 ? 'ceviche' : 'seco'}', alias: 'P$i', vendidos: i)];
      final g = IndiceBusqueda<Producto>(grande, nombre: (p) => p.nombre, alias: (p) => p.alias, vendidos: (p) => p.vendidos);
      final tiempos = <int>[];
      for (final q in ['c', 'ce', 'cev', 'cevi', 'ceviche', 's', 'se', 'sec', 'plato 4', 'casa sec', 'p12']) {
        for (var r = 0; r < 20; r++) {
          final sw = Stopwatch()..start();
          g.buscar(q);
          tiempos.add(sw.elapsedMicroseconds);
        }
      }
      tiempos.sort();
      final p95 = tiempos[(tiempos.length * .95).floor()];
      expect(p95, lessThan(50000), reason: 'p95 = $p95 µs');
    });
  });

  group('Dinero exacto', () {
    test('formato y suma sin double', () {
      expect(Dinero.texto('1234.5'), r'$1,234.50');
      expect(Dinero.formato(Decimal.parse('0.1') + Decimal.parse('0.2')), r'$0.30');
      expect(Dinero.totalLinea('9.50', ['1.00', '0.25'], '2'), Decimal.parse('21.50'));
      expect(Dinero.totalLinea('3.333333', [], '0.5'), Decimal.parse('1.67'));
    });
  });

  group('Emparejamiento', () {
    test('lee el QR del nodo', () {
      final q = DatosQR.leer('restpos://emparejar?c=ABCDEFGH&u=http%3A%2F%2F192.168.1.10%3A7080&u=http%3A%2F%2F10.0.0.5%3A7080')!;
      expect(q.codigo, 'ABCDEFGH');
      expect(q.urls, ['http://192.168.1.10:7080', 'http://10.0.0.5:7080']);
      expect(DatosQR.leer('https://otra-cosa.com'), isNull);
      expect(DatosQR.leer('restpos://emparejar?c=CORTO&u=http://x'), isNull);
    });

    test('la identidad persiste y su firma la verifica el nodo', () async {
      final almacen = AlmacenMemoria();
      final a = await Identidad.cargar(almacen);
      final b = await Identidad.cargar(almacen);
      expect(b.dispositivoId, a.dispositivoId);
      expect(b.publicaB64, a.publicaB64);
      expect(a.dispositivoId[14], '7', reason: 'UUID v7');
      final firma = base64.decode(await a.firmarDesafio('nonce-123'));
      final ok = await Ed25519().verify(utf8.encode('restpos|sesion-dispositivo|nonce-123'),
          signature: Signature(firma, publicKey: SimplePublicKey(a.publica, type: KeyPairType.ed25519)));
      expect(ok, isTrue);
      await Identidad.olvidar(almacen);
      final c = await Identidad.cargar(almacen);
      expect(c.dispositivoId, isNot(a.dispositivoId));
    });
  });

  group('Cliente del nodo', () {
    test('envía ambas credenciales en una cabecera y traduce errores', () async {
      late http.Request visto;
      final api = NodoApi(
        base: 'http://nodo:7080',
        cliente: MockClient((req) async {
          visto = req;
          if (req.url.path == '/v1/mesas/m1/bloqueo') {
            return http.Response.bytes(utf8.encode(jsonEncode({'status': 409, 'code': 'LOCKED_BY', 'detail': 'La mesa la está editando Ana R.. Espera a que termine.'})), 409,
                headers: {'content-type': 'application/json; charset=utf-8'});
          }
          return http.Response(jsonEncode({'zonas': [], 'mesas': []}), 200);
        }),
      )
        ..tokenDispositivo = 'D1'
        ..tokenUsuario = 'U1';
      await api.salon();
      expect(visto.headers['Authorization'], 'Dispositivo D1, Usuario U1');
      await expectLater(api.bloquear('m1'), throwsA(isA<ApiError>().having((e) => e.code, 'code', 'LOCKED_BY')));
      expect(api.urlTiempoReal().toString(), 'ws://nodo:7080/v1/ws?dispositivo=D1&usuario=U1');
    });

    test('sin red da un error de conexión claro', () async {
      final api = NodoApi(base: 'http://nodo:7080', cliente: MockClient((_) async => throw http.ClientException('sin red')));
      await expectLater(api.salon(), throwsA(isA<ApiError>().having((e) => e.esRed, 'esRed', true)));
    });
  });

  test('reconexión con espera exponencial y tope de 30 s', () {
    expect(TiempoReal.esperaPara(1).inMilliseconds, 500);
    expect(TiempoReal.esperaPara(2).inMilliseconds, 1000);
    expect(TiempoReal.esperaPara(20).inMilliseconds, 30000);
  });

  test('persona trae sus permisos (nodos viejos sin el campo: ninguno)', () {
    final p = Persona.fromJson({'id': 'a', 'nombre': 'Pepe Andrade', 'rol': 'ADMIN', 'permisos': ['ANULAR_ITEM_ENVIADO']});
    expect(p.permisos.contains('ANULAR_ITEM_ENVIADO'), isTrue);
    expect(Persona.fromJson({'id': 'b', 'nombre': 'Ana R.', 'rol': 'MESERO'}).permisos, isEmpty);
  });
}
