import 'dart:io';

import 'package:drift/drift.dart';
import 'package:drift/native.dart';
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';

part 'db.g.dart';

/// Caché local del teléfono (F3-05): el catálogo para tomar pedidos sin red.
class Cache extends Table {
  TextColumn get clave => text()();
  TextColumn get valor => text()();
  IntColumn get actualizado => integer()();
  @override
  Set<Column> get primaryKey => {clave};
}

/// Comandas que no llegaron al nodo (F3-10 «Pendiente de envío»). Se reenvían solas con la
/// MISMA clave de idempotencia: el nodo nunca las duplica.
class Pendientes extends Table {
  TextColumn get clave => text()();
  TextColumn get mesa => text()();
  TextColumn get cuerpo => text()();
  IntColumn get creado => integer()();
  IntColumn get intentos => integer().withDefault(const Constant(0))();
  TextColumn get ultimoError => text().nullable()();
  @override
  Set<Column> get primaryKey => {clave};
}

@DriftDatabase(tables: [Cache, Pendientes])
class BaseLocal extends _$BaseLocal {
  BaseLocal(super.e);

  /// Base en el directorio de la app.
  static Future<BaseLocal> abrir() async {
    final dir = await getApplicationSupportDirectory();
    return BaseLocal(NativeDatabase.createInBackground(File(p.join(dir.path, 'restpos.sqlite'))));
  }

  /// Base en memoria (pruebas).
  factory BaseLocal.memoria() => BaseLocal(NativeDatabase.memory());

  @override
  int get schemaVersion => 1;

  Future<String?> leerCache(String clave) async => (await (select(cache)..where((c) => c.clave.equals(clave))).getSingleOrNull())?.valor;

  Future<void> guardarCache(String clave, String valor) => into(cache).insertOnConflictUpdate(
      CacheCompanion.insert(clave: clave, valor: valor, actualizado: DateTime.now().millisecondsSinceEpoch));

  Future<void> encolar(String clave, String mesa, String cuerpo) => into(pendientes).insert(
      PendientesCompanion.insert(clave: clave, mesa: mesa, cuerpo: cuerpo, creado: DateTime.now().millisecondsSinceEpoch),
      mode: InsertMode.insertOrIgnore);

  Future<List<Pendiente>> listaPendientes() => (select(pendientes)..orderBy([(t) => OrderingTerm(expression: t.creado)])).get();
  Stream<List<Pendiente>> vigilarPendientes() => (select(pendientes)..orderBy([(t) => OrderingTerm(expression: t.creado)])).watch();

  Future<void> quitarPendiente(String clave) => (delete(pendientes)..where((t) => t.clave.equals(clave))).go();

  Future<void> falloPendiente(String clave, String error) => (update(pendientes)..where((t) => t.clave.equals(clave)))
      .write(PendientesCompanion.custom(intentos: pendientes.intentos + const Constant(1), ultimoError: Variable(error)));
}
