// F3-15: el flujo completo de la app contra un nodo falso que responde como el real:
// emparejar con código → sesión del dispositivo firmada → PIN → mesa → pedido → enviar.
import 'dart:convert';

import 'package:cryptography/cryptography.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:restpos_meseros/app.dart';
import 'package:restpos_meseros/core/descubrimiento.dart';
import 'package:restpos_meseros/core/estado.dart';
import 'package:restpos_meseros/core/identidad.dart';
import 'package:restpos_meseros/core/nodo_api.dart';
import 'package:restpos_meseros/core/tiempo_real.dart';
import 'package:restpos_meseros/data/db.dart';
import 'package:restpos_meseros/features/salon/salon_page.dart';

/// Sin WebSocket en las pruebas: el salón se lee por HTTP.
class _SinTiempoReal extends TiempoReal {
  _SinTiempoReal(super.url);
  @override
  void iniciar() {}
  @override
  void reiniciar() {}
}

/// Nodo Local falso: guarda lo que recibe y exige las mismas credenciales que el real.
class NodoFalso {
  static const codigo = 'ABCD2345', carlos = 'u-carlos', mesa = 'm-7', empanadas = 'p-emp';
  String? llavePublica, nonce;
  var bloqueos = 0;
  final enviados = <Map<String, dynamic>>[];
  Map<String, dynamic>? orden;

  http.Response _json(Object o, [int st = 200]) =>
      http.Response.bytes(utf8.encode(jsonEncode(o)), st, headers: {'content-type': 'application/json; charset=utf-8'});
  http.Response _error(int st, String code, String detail) => _json({'code': code, 'detail': detail}, st);

  Future<http.Response> atender(http.Request r) async {
    final ruta = '${r.method} ${r.url.path}';
    final auth = r.headers['Authorization'] ?? '';
    final cuerpo = r.body.isEmpty ? <String, dynamic>{} : jsonDecode(r.body) as Map<String, dynamic>;
    if (ruta == 'GET /health') return _json({'ok': true});
    if (ruta == 'POST /v1/dispositivos/emparejar') {
      if (cuerpo['codigo'] != codigo) return _error(400, 'CODIGO_INVALIDO', 'El código no es válido o ya venció.');
      llavePublica = cuerpo['llavePublica'] as String;
      return _json({'restaurante': 'Don Pepe', 'local': 'Matriz'});
    }
    if (ruta == 'GET /v1/dispositivos/desafio') {
      nonce = 'nonce-${DateTime.now().microsecondsSinceEpoch}';
      return _json({'nonce': nonce});
    }
    if (ruta == 'POST /v1/dispositivos/sesion') {
      final ok = await Ed25519().verify(
        utf8.encode('restpos|sesion-dispositivo|$nonce'),
        signature: Signature(base64.decode(cuerpo['firma'] as String), publicKey: SimplePublicKey(base64.decode(llavePublica!), type: KeyPairType.ed25519)),
      );
      return ok ? _json({'token': 'td'}) : _error(401, 'FIRMA_INVALIDA', 'Firma inválida.');
    }
    if (!auth.contains('Dispositivo td')) return _error(401, 'SIN_DISPOSITIVO', 'Dispositivo no emparejado.');
    if (ruta == 'GET /v1/personal') {
      return _json([
        {
          'id': carlos,
          'nombre': 'Carlos M.',
          'rol': 'MESERO',
          'permisos': ['TOMAR_PEDIDO'],
        },
      ]);
    }
    if (ruta == 'POST /v1/sesiones') {
      if (cuerpo['pin'] != '8899') return _error(401, 'PIN_INCORRECTO', 'PIN incorrecto. Te quedan 4 intentos.');
      return _json({
        'token': 'tu',
        'usuario': {
          'id': carlos,
          'nombre': 'Carlos M.',
          'rol': 'MESERO',
          'permisos': ['TOMAR_PEDIDO', 'IMPRIMIR_PRECUENTA'],
        },
      });
    }
    if (!auth.contains('Usuario tu')) return _error(401, 'SIN_SESION', 'Escribe tu PIN.');
    switch (ruta) {
      case 'GET /v1/conectividad':
        return _json({'nube': false});
      case 'GET /v1/catalogo':
        return _json({
          'version': '2026-09-25T21:00:00Z',
          'categorias': [
            {'id': 'c-ent', 'nombre': 'Entradas', 'orden': 1},
          ],
          'productos': [
            {'id': empanadas, 'categoriaId': 'c-ent', 'nombre': 'Empanadas de verde', 'alias': 'EV', 'precio': '4.50', 'grupos': <String>[]},
          ],
          'grupos': <Object>[],
        });
      case 'GET /v1/salon':
        return _json({
          'zonas': [
            {'id': 'z1', 'nombre': 'Salón'},
          ],
          'mesas': [
            {'id': mesa, 'zonaId': 'z1', 'nombre': 'Mesa 7', 'capacidad': 4, 'estado': orden == null ? 'LIBRE' : 'OCUPADA', 'total': orden?['total'] ?? '0.00'},
          ],
        });
      case 'POST /v1/mesas/m-7/bloqueo':
        bloqueos++;
        return _json({'ok': true});
      case 'POST /v1/mesas/m-7/latido' || 'DELETE /v1/mesas/m-7/bloqueo':
        return _json({'ok': true});
      case 'GET /v1/mesas/m-7/orden':
        return orden == null ? _error(404, 'MESA_LIBRE', 'La mesa está libre.') : _json(orden!);
      case 'POST /v1/ordenes/enviar':
        enviados.add(cuerpo);
        final l = (cuerpo['lineas'] as List).single as Map<String, dynamic>;
        orden = {
          'id': cuerpo['ordenId'],
          'mesa': 'Mesa 7',
          'numero': 1,
          'estado': 'ABIERTA',
          'meseroNombre': 'Carlos M.',
          'total': '9.00',
          'lineas': [
            {
              'id': l['id'],
              'productoId': empanadas,
              'producto': 'Empanadas de verde',
              'cantidad': l['cantidad'],
              'modificadores': <Object>[],
              'nota': '',
              'tiempo': '',
              'estado': 'ENVIADA',
              'total': '9.00',
            },
          ],
        };
        return _json({'orden': orden, 'comandaNumero': 1, 'envios': <Object>[], 'repetida': false});
    }
    return _error(404, 'NO_EXISTE', 'Ruta no simulada: $ruta');
  }
}

/// Varias pantallas tienen animaciones en bucle: se avanza un tiempo fijo en vez de esperar calma.
Future<void> asentar(WidgetTester t) async {
  for (var i = 0; i < 15; i++) {
    await t.pump(const Duration(milliseconds: 100));
  }
}

void main() {
  testWidgets('emparejar → PIN → mesa → pedido → enviar', (t) async {
    t.view.physicalSize = const Size(1080, 2400);
    t.view.devicePixelRatio = 3;
    addTearDown(t.view.reset);
    final nodo = NodoFalso();
    final db = BaseLocal.memoria();
    addTearDown(db.close);

    await t.pumpWidget(
      ProviderScope(
        overrides: [
          almacenProvider.overrideWithValue(AlmacenMemoria()),
          baseLocalProvider.overrideWithValue(db),
          camaraProvider.overrideWithValue(false),
          descubridorProvider.overrideWithValue(Descubridor(buscarMdns: false)),
          apiFabricaProvider.overrideWithValue((base) => NodoApi(base: base, cliente: MockClient(nodo.atender))),
          tiempoRealFabricaProvider.overrideWithValue(_SinTiempoReal.new),
        ],
        child: const RestPosApp(),
      ),
    );
    await asentar(t);

    // 1. Emparejar escribiendo el código y la IP que muestra la caja.
    await t.tap(find.text('Escribir código'));
    await asentar(t);
    await t.enterText(find.byType(CupertinoTextField).first, 'abcd-2345');
    await t.enterText(find.byType(CupertinoTextField).last, '192.168.1.10');
    await t.tap(find.text('Emparejar'));
    await asentar(t);
    expect(nodo.llavePublica, isNotNull);

    // 2. La cuadrícula de personal; un PIN erróneo explica el error y el correcto entra.
    expect(find.text('¿Quién eres?'), findsOneWidget);
    await t.tap(find.text('Carlos M.'));
    await asentar(t);
    for (final d in '1234'.split('')) {
      await t.tap(find.text(d).last);
      await t.pump();
    }
    await asentar(t);
    expect(find.textContaining('PIN incorrecto'), findsOneWidget);
    for (final d in '8899'.split('')) {
      await t.tap(find.text(d).last);
      await t.pump();
    }
    await asentar(t);

    // 3. El salón en vivo: abrir la Mesa 7 la bloquea en el nodo.
    expect(find.byType(MesaTile), findsOneWidget);
    await t.tap(find.byType(MesaTile));
    await asentar(t);
    expect(nodo.bloqueos, 1);

    // 4. Dos toques en el plato juntan la cantidad; «Enviar» manda una sola comanda idempotente.
    await t.tap(find.text('Empanadas de verde'));
    await t.pump();
    await t.tap(find.text('Empanadas de verde'));
    await asentar(t);
    // El botón muestra el total del borrador en decimal exacto: 2 × $4.50.
    expect(find.text('Enviar · \$9.00'), findsOneWidget);
    await t.tap(find.text('Enviar · \$9.00'));
    await asentar(t);

    expect(nodo.enviados, hasLength(1));
    final envio = nodo.enviados.single;
    expect(envio['mesaId'], NodoFalso.mesa);
    expect((envio['idempotencyKey'] as String).length, greaterThanOrEqualTo(8));
    final linea = (envio['lineas'] as List).single as Map<String, dynamic>;
    expect(linea['productoId'], NodoFalso.empanadas);
    expect(linea['cantidad'], '2');

    // Salimos de la app: sin temporizadores colgados.
    await t.pumpWidget(const SizedBox());
    await t.pump(const Duration(seconds: 1));
  }, timeout: const Timeout(Duration(seconds: 60)));
}
