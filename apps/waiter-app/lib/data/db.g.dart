// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'db.dart';

// ignore_for_file: type=lint
class $CacheTable extends Cache with TableInfo<$CacheTable, CacheData> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $CacheTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _claveMeta = const VerificationMeta('clave');
  @override
  late final GeneratedColumn<String> clave = GeneratedColumn<String>('clave', aliasedName, false, type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _valorMeta = const VerificationMeta('valor');
  @override
  late final GeneratedColumn<String> valor = GeneratedColumn<String>('valor', aliasedName, false, type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _actualizadoMeta = const VerificationMeta('actualizado');
  @override
  late final GeneratedColumn<int> actualizado = GeneratedColumn<int>('actualizado', aliasedName, false, type: DriftSqlType.int, requiredDuringInsert: true);
  @override
  List<GeneratedColumn> get $columns => [clave, valor, actualizado];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'cache';
  @override
  VerificationContext validateIntegrity(Insertable<CacheData> instance, {bool isInserting = false}) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('clave')) {
      context.handle(_claveMeta, clave.isAcceptableOrUnknown(data['clave']!, _claveMeta));
    } else if (isInserting) {
      context.missing(_claveMeta);
    }
    if (data.containsKey('valor')) {
      context.handle(_valorMeta, valor.isAcceptableOrUnknown(data['valor']!, _valorMeta));
    } else if (isInserting) {
      context.missing(_valorMeta);
    }
    if (data.containsKey('actualizado')) {
      context.handle(_actualizadoMeta, actualizado.isAcceptableOrUnknown(data['actualizado']!, _actualizadoMeta));
    } else if (isInserting) {
      context.missing(_actualizadoMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {clave};
  @override
  CacheData map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return CacheData(
      clave: attachedDatabase.typeMapping.read(DriftSqlType.string, data['${effectivePrefix}clave'])!,
      valor: attachedDatabase.typeMapping.read(DriftSqlType.string, data['${effectivePrefix}valor'])!,
      actualizado: attachedDatabase.typeMapping.read(DriftSqlType.int, data['${effectivePrefix}actualizado'])!,
    );
  }

  @override
  $CacheTable createAlias(String alias) {
    return $CacheTable(attachedDatabase, alias);
  }
}

class CacheData extends DataClass implements Insertable<CacheData> {
  final String clave;
  final String valor;
  final int actualizado;
  const CacheData({required this.clave, required this.valor, required this.actualizado});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['clave'] = Variable<String>(clave);
    map['valor'] = Variable<String>(valor);
    map['actualizado'] = Variable<int>(actualizado);
    return map;
  }

  CacheCompanion toCompanion(bool nullToAbsent) {
    return CacheCompanion(clave: Value(clave), valor: Value(valor), actualizado: Value(actualizado));
  }

  factory CacheData.fromJson(Map<String, dynamic> json, {ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return CacheData(
      clave: serializer.fromJson<String>(json['clave']),
      valor: serializer.fromJson<String>(json['valor']),
      actualizado: serializer.fromJson<int>(json['actualizado']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'clave': serializer.toJson<String>(clave),
      'valor': serializer.toJson<String>(valor),
      'actualizado': serializer.toJson<int>(actualizado),
    };
  }

  CacheData copyWith({String? clave, String? valor, int? actualizado}) =>
      CacheData(clave: clave ?? this.clave, valor: valor ?? this.valor, actualizado: actualizado ?? this.actualizado);
  CacheData copyWithCompanion(CacheCompanion data) {
    return CacheData(
      clave: data.clave.present ? data.clave.value : this.clave,
      valor: data.valor.present ? data.valor.value : this.valor,
      actualizado: data.actualizado.present ? data.actualizado.value : this.actualizado,
    );
  }

  @override
  String toString() {
    return (StringBuffer('CacheData(')
          ..write('clave: $clave, ')
          ..write('valor: $valor, ')
          ..write('actualizado: $actualizado')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(clave, valor, actualizado);
  @override
  bool operator ==(Object other) =>
      identical(this, other) || (other is CacheData && other.clave == this.clave && other.valor == this.valor && other.actualizado == this.actualizado);
}

class CacheCompanion extends UpdateCompanion<CacheData> {
  final Value<String> clave;
  final Value<String> valor;
  final Value<int> actualizado;
  final Value<int> rowid;
  const CacheCompanion({
    this.clave = const Value.absent(),
    this.valor = const Value.absent(),
    this.actualizado = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  CacheCompanion.insert({required String clave, required String valor, required int actualizado, this.rowid = const Value.absent()})
    : clave = Value(clave),
      valor = Value(valor),
      actualizado = Value(actualizado);
  static Insertable<CacheData> custom({Expression<String>? clave, Expression<String>? valor, Expression<int>? actualizado, Expression<int>? rowid}) {
    return RawValuesInsertable({
      if (clave != null) 'clave': clave,
      if (valor != null) 'valor': valor,
      if (actualizado != null) 'actualizado': actualizado,
      if (rowid != null) 'rowid': rowid,
    });
  }

  CacheCompanion copyWith({Value<String>? clave, Value<String>? valor, Value<int>? actualizado, Value<int>? rowid}) {
    return CacheCompanion(clave: clave ?? this.clave, valor: valor ?? this.valor, actualizado: actualizado ?? this.actualizado, rowid: rowid ?? this.rowid);
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (clave.present) {
      map['clave'] = Variable<String>(clave.value);
    }
    if (valor.present) {
      map['valor'] = Variable<String>(valor.value);
    }
    if (actualizado.present) {
      map['actualizado'] = Variable<int>(actualizado.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('CacheCompanion(')
          ..write('clave: $clave, ')
          ..write('valor: $valor, ')
          ..write('actualizado: $actualizado, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $PendientesTable extends Pendientes with TableInfo<$PendientesTable, Pendiente> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $PendientesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _claveMeta = const VerificationMeta('clave');
  @override
  late final GeneratedColumn<String> clave = GeneratedColumn<String>('clave', aliasedName, false, type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _mesaMeta = const VerificationMeta('mesa');
  @override
  late final GeneratedColumn<String> mesa = GeneratedColumn<String>('mesa', aliasedName, false, type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _cuerpoMeta = const VerificationMeta('cuerpo');
  @override
  late final GeneratedColumn<String> cuerpo = GeneratedColumn<String>('cuerpo', aliasedName, false, type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _creadoMeta = const VerificationMeta('creado');
  @override
  late final GeneratedColumn<int> creado = GeneratedColumn<int>('creado', aliasedName, false, type: DriftSqlType.int, requiredDuringInsert: true);
  static const VerificationMeta _intentosMeta = const VerificationMeta('intentos');
  @override
  late final GeneratedColumn<int> intentos = GeneratedColumn<int>(
    'intentos',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
    defaultValue: const Constant(0),
  );
  static const VerificationMeta _ultimoErrorMeta = const VerificationMeta('ultimoError');
  @override
  late final GeneratedColumn<String> ultimoError = GeneratedColumn<String>(
    'ultimo_error',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  @override
  List<GeneratedColumn> get $columns => [clave, mesa, cuerpo, creado, intentos, ultimoError];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'pendientes';
  @override
  VerificationContext validateIntegrity(Insertable<Pendiente> instance, {bool isInserting = false}) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('clave')) {
      context.handle(_claveMeta, clave.isAcceptableOrUnknown(data['clave']!, _claveMeta));
    } else if (isInserting) {
      context.missing(_claveMeta);
    }
    if (data.containsKey('mesa')) {
      context.handle(_mesaMeta, mesa.isAcceptableOrUnknown(data['mesa']!, _mesaMeta));
    } else if (isInserting) {
      context.missing(_mesaMeta);
    }
    if (data.containsKey('cuerpo')) {
      context.handle(_cuerpoMeta, cuerpo.isAcceptableOrUnknown(data['cuerpo']!, _cuerpoMeta));
    } else if (isInserting) {
      context.missing(_cuerpoMeta);
    }
    if (data.containsKey('creado')) {
      context.handle(_creadoMeta, creado.isAcceptableOrUnknown(data['creado']!, _creadoMeta));
    } else if (isInserting) {
      context.missing(_creadoMeta);
    }
    if (data.containsKey('intentos')) {
      context.handle(_intentosMeta, intentos.isAcceptableOrUnknown(data['intentos']!, _intentosMeta));
    }
    if (data.containsKey('ultimo_error')) {
      context.handle(_ultimoErrorMeta, ultimoError.isAcceptableOrUnknown(data['ultimo_error']!, _ultimoErrorMeta));
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {clave};
  @override
  Pendiente map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return Pendiente(
      clave: attachedDatabase.typeMapping.read(DriftSqlType.string, data['${effectivePrefix}clave'])!,
      mesa: attachedDatabase.typeMapping.read(DriftSqlType.string, data['${effectivePrefix}mesa'])!,
      cuerpo: attachedDatabase.typeMapping.read(DriftSqlType.string, data['${effectivePrefix}cuerpo'])!,
      creado: attachedDatabase.typeMapping.read(DriftSqlType.int, data['${effectivePrefix}creado'])!,
      intentos: attachedDatabase.typeMapping.read(DriftSqlType.int, data['${effectivePrefix}intentos'])!,
      ultimoError: attachedDatabase.typeMapping.read(DriftSqlType.string, data['${effectivePrefix}ultimo_error']),
    );
  }

  @override
  $PendientesTable createAlias(String alias) {
    return $PendientesTable(attachedDatabase, alias);
  }
}

class Pendiente extends DataClass implements Insertable<Pendiente> {
  final String clave;
  final String mesa;
  final String cuerpo;
  final int creado;
  final int intentos;
  final String? ultimoError;
  const Pendiente({required this.clave, required this.mesa, required this.cuerpo, required this.creado, required this.intentos, this.ultimoError});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['clave'] = Variable<String>(clave);
    map['mesa'] = Variable<String>(mesa);
    map['cuerpo'] = Variable<String>(cuerpo);
    map['creado'] = Variable<int>(creado);
    map['intentos'] = Variable<int>(intentos);
    if (!nullToAbsent || ultimoError != null) {
      map['ultimo_error'] = Variable<String>(ultimoError);
    }
    return map;
  }

  PendientesCompanion toCompanion(bool nullToAbsent) {
    return PendientesCompanion(
      clave: Value(clave),
      mesa: Value(mesa),
      cuerpo: Value(cuerpo),
      creado: Value(creado),
      intentos: Value(intentos),
      ultimoError: ultimoError == null && nullToAbsent ? const Value.absent() : Value(ultimoError),
    );
  }

  factory Pendiente.fromJson(Map<String, dynamic> json, {ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return Pendiente(
      clave: serializer.fromJson<String>(json['clave']),
      mesa: serializer.fromJson<String>(json['mesa']),
      cuerpo: serializer.fromJson<String>(json['cuerpo']),
      creado: serializer.fromJson<int>(json['creado']),
      intentos: serializer.fromJson<int>(json['intentos']),
      ultimoError: serializer.fromJson<String?>(json['ultimoError']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'clave': serializer.toJson<String>(clave),
      'mesa': serializer.toJson<String>(mesa),
      'cuerpo': serializer.toJson<String>(cuerpo),
      'creado': serializer.toJson<int>(creado),
      'intentos': serializer.toJson<int>(intentos),
      'ultimoError': serializer.toJson<String?>(ultimoError),
    };
  }

  Pendiente copyWith({String? clave, String? mesa, String? cuerpo, int? creado, int? intentos, Value<String?> ultimoError = const Value.absent()}) => Pendiente(
    clave: clave ?? this.clave,
    mesa: mesa ?? this.mesa,
    cuerpo: cuerpo ?? this.cuerpo,
    creado: creado ?? this.creado,
    intentos: intentos ?? this.intentos,
    ultimoError: ultimoError.present ? ultimoError.value : this.ultimoError,
  );
  Pendiente copyWithCompanion(PendientesCompanion data) {
    return Pendiente(
      clave: data.clave.present ? data.clave.value : this.clave,
      mesa: data.mesa.present ? data.mesa.value : this.mesa,
      cuerpo: data.cuerpo.present ? data.cuerpo.value : this.cuerpo,
      creado: data.creado.present ? data.creado.value : this.creado,
      intentos: data.intentos.present ? data.intentos.value : this.intentos,
      ultimoError: data.ultimoError.present ? data.ultimoError.value : this.ultimoError,
    );
  }

  @override
  String toString() {
    return (StringBuffer('Pendiente(')
          ..write('clave: $clave, ')
          ..write('mesa: $mesa, ')
          ..write('cuerpo: $cuerpo, ')
          ..write('creado: $creado, ')
          ..write('intentos: $intentos, ')
          ..write('ultimoError: $ultimoError')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(clave, mesa, cuerpo, creado, intentos, ultimoError);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is Pendiente &&
          other.clave == this.clave &&
          other.mesa == this.mesa &&
          other.cuerpo == this.cuerpo &&
          other.creado == this.creado &&
          other.intentos == this.intentos &&
          other.ultimoError == this.ultimoError);
}

class PendientesCompanion extends UpdateCompanion<Pendiente> {
  final Value<String> clave;
  final Value<String> mesa;
  final Value<String> cuerpo;
  final Value<int> creado;
  final Value<int> intentos;
  final Value<String?> ultimoError;
  final Value<int> rowid;
  const PendientesCompanion({
    this.clave = const Value.absent(),
    this.mesa = const Value.absent(),
    this.cuerpo = const Value.absent(),
    this.creado = const Value.absent(),
    this.intentos = const Value.absent(),
    this.ultimoError = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  PendientesCompanion.insert({
    required String clave,
    required String mesa,
    required String cuerpo,
    required int creado,
    this.intentos = const Value.absent(),
    this.ultimoError = const Value.absent(),
    this.rowid = const Value.absent(),
  }) : clave = Value(clave),
       mesa = Value(mesa),
       cuerpo = Value(cuerpo),
       creado = Value(creado);
  static Insertable<Pendiente> custom({
    Expression<String>? clave,
    Expression<String>? mesa,
    Expression<String>? cuerpo,
    Expression<int>? creado,
    Expression<int>? intentos,
    Expression<String>? ultimoError,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (clave != null) 'clave': clave,
      if (mesa != null) 'mesa': mesa,
      if (cuerpo != null) 'cuerpo': cuerpo,
      if (creado != null) 'creado': creado,
      if (intentos != null) 'intentos': intentos,
      if (ultimoError != null) 'ultimo_error': ultimoError,
      if (rowid != null) 'rowid': rowid,
    });
  }

  PendientesCompanion copyWith({
    Value<String>? clave,
    Value<String>? mesa,
    Value<String>? cuerpo,
    Value<int>? creado,
    Value<int>? intentos,
    Value<String?>? ultimoError,
    Value<int>? rowid,
  }) {
    return PendientesCompanion(
      clave: clave ?? this.clave,
      mesa: mesa ?? this.mesa,
      cuerpo: cuerpo ?? this.cuerpo,
      creado: creado ?? this.creado,
      intentos: intentos ?? this.intentos,
      ultimoError: ultimoError ?? this.ultimoError,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (clave.present) {
      map['clave'] = Variable<String>(clave.value);
    }
    if (mesa.present) {
      map['mesa'] = Variable<String>(mesa.value);
    }
    if (cuerpo.present) {
      map['cuerpo'] = Variable<String>(cuerpo.value);
    }
    if (creado.present) {
      map['creado'] = Variable<int>(creado.value);
    }
    if (intentos.present) {
      map['intentos'] = Variable<int>(intentos.value);
    }
    if (ultimoError.present) {
      map['ultimo_error'] = Variable<String>(ultimoError.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('PendientesCompanion(')
          ..write('clave: $clave, ')
          ..write('mesa: $mesa, ')
          ..write('cuerpo: $cuerpo, ')
          ..write('creado: $creado, ')
          ..write('intentos: $intentos, ')
          ..write('ultimoError: $ultimoError, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

abstract class _$BaseLocal extends GeneratedDatabase {
  _$BaseLocal(QueryExecutor e) : super(e);
  $BaseLocalManager get managers => $BaseLocalManager(this);
  late final $CacheTable cache = $CacheTable(this);
  late final $PendientesTable pendientes = $PendientesTable(this);
  @override
  Iterable<TableInfo<Table, Object?>> get allTables => allSchemaEntities.whereType<TableInfo<Table, Object?>>();
  @override
  List<DatabaseSchemaEntity> get allSchemaEntities => [cache, pendientes];
}

typedef $$CacheTableCreateCompanionBuilder =
    CacheCompanion Function({required String clave, required String valor, required int actualizado, Value<int> rowid});
typedef $$CacheTableUpdateCompanionBuilder = CacheCompanion Function({Value<String> clave, Value<String> valor, Value<int> actualizado, Value<int> rowid});

class $$CacheTableFilterComposer extends Composer<_$BaseLocal, $CacheTable> {
  $$CacheTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get clave => $composableBuilder(column: $table.clave, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get valor => $composableBuilder(column: $table.valor, builder: (column) => ColumnFilters(column));

  ColumnFilters<int> get actualizado => $composableBuilder(column: $table.actualizado, builder: (column) => ColumnFilters(column));
}

class $$CacheTableOrderingComposer extends Composer<_$BaseLocal, $CacheTable> {
  $$CacheTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get clave => $composableBuilder(column: $table.clave, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get valor => $composableBuilder(column: $table.valor, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<int> get actualizado => $composableBuilder(column: $table.actualizado, builder: (column) => ColumnOrderings(column));
}

class $$CacheTableAnnotationComposer extends Composer<_$BaseLocal, $CacheTable> {
  $$CacheTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get clave => $composableBuilder(column: $table.clave, builder: (column) => column);

  GeneratedColumn<String> get valor => $composableBuilder(column: $table.valor, builder: (column) => column);

  GeneratedColumn<int> get actualizado => $composableBuilder(column: $table.actualizado, builder: (column) => column);
}

class $$CacheTableTableManager
    extends
        RootTableManager<
          _$BaseLocal,
          $CacheTable,
          CacheData,
          $$CacheTableFilterComposer,
          $$CacheTableOrderingComposer,
          $$CacheTableAnnotationComposer,
          $$CacheTableCreateCompanionBuilder,
          $$CacheTableUpdateCompanionBuilder,
          (CacheData, BaseReferences<_$BaseLocal, $CacheTable, CacheData>),
          CacheData,
          PrefetchHooks Function()
        > {
  $$CacheTableTableManager(_$BaseLocal db, $CacheTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () => $$CacheTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () => $$CacheTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () => $$CacheTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> clave = const Value.absent(),
                Value<String> valor = const Value.absent(),
                Value<int> actualizado = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => CacheCompanion(clave: clave, valor: valor, actualizado: actualizado, rowid: rowid),
          createCompanionCallback: ({required String clave, required String valor, required int actualizado, Value<int> rowid = const Value.absent()}) =>
              CacheCompanion.insert(clave: clave, valor: valor, actualizado: actualizado, rowid: rowid),
          withReferenceMapper: (p0) =>
              p0.map((e) => (e.readTable<$CacheTable, CacheData>(table), BaseReferences<_$BaseLocal, $CacheTable, CacheData>(db, table, e))).toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$CacheTableProcessedTableManager =
    ProcessedTableManager<
      _$BaseLocal,
      $CacheTable,
      CacheData,
      $$CacheTableFilterComposer,
      $$CacheTableOrderingComposer,
      $$CacheTableAnnotationComposer,
      $$CacheTableCreateCompanionBuilder,
      $$CacheTableUpdateCompanionBuilder,
      (CacheData, BaseReferences<_$BaseLocal, $CacheTable, CacheData>),
      CacheData,
      PrefetchHooks Function()
    >;
typedef $$PendientesTableCreateCompanionBuilder =
    PendientesCompanion Function({
      required String clave,
      required String mesa,
      required String cuerpo,
      required int creado,
      Value<int> intentos,
      Value<String?> ultimoError,
      Value<int> rowid,
    });
typedef $$PendientesTableUpdateCompanionBuilder =
    PendientesCompanion Function({
      Value<String> clave,
      Value<String> mesa,
      Value<String> cuerpo,
      Value<int> creado,
      Value<int> intentos,
      Value<String?> ultimoError,
      Value<int> rowid,
    });

class $$PendientesTableFilterComposer extends Composer<_$BaseLocal, $PendientesTable> {
  $$PendientesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get clave => $composableBuilder(column: $table.clave, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get mesa => $composableBuilder(column: $table.mesa, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get cuerpo => $composableBuilder(column: $table.cuerpo, builder: (column) => ColumnFilters(column));

  ColumnFilters<int> get creado => $composableBuilder(column: $table.creado, builder: (column) => ColumnFilters(column));

  ColumnFilters<int> get intentos => $composableBuilder(column: $table.intentos, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get ultimoError => $composableBuilder(column: $table.ultimoError, builder: (column) => ColumnFilters(column));
}

class $$PendientesTableOrderingComposer extends Composer<_$BaseLocal, $PendientesTable> {
  $$PendientesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get clave => $composableBuilder(column: $table.clave, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get mesa => $composableBuilder(column: $table.mesa, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get cuerpo => $composableBuilder(column: $table.cuerpo, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<int> get creado => $composableBuilder(column: $table.creado, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<int> get intentos => $composableBuilder(column: $table.intentos, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get ultimoError => $composableBuilder(column: $table.ultimoError, builder: (column) => ColumnOrderings(column));
}

class $$PendientesTableAnnotationComposer extends Composer<_$BaseLocal, $PendientesTable> {
  $$PendientesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get clave => $composableBuilder(column: $table.clave, builder: (column) => column);

  GeneratedColumn<String> get mesa => $composableBuilder(column: $table.mesa, builder: (column) => column);

  GeneratedColumn<String> get cuerpo => $composableBuilder(column: $table.cuerpo, builder: (column) => column);

  GeneratedColumn<int> get creado => $composableBuilder(column: $table.creado, builder: (column) => column);

  GeneratedColumn<int> get intentos => $composableBuilder(column: $table.intentos, builder: (column) => column);

  GeneratedColumn<String> get ultimoError => $composableBuilder(column: $table.ultimoError, builder: (column) => column);
}

class $$PendientesTableTableManager
    extends
        RootTableManager<
          _$BaseLocal,
          $PendientesTable,
          Pendiente,
          $$PendientesTableFilterComposer,
          $$PendientesTableOrderingComposer,
          $$PendientesTableAnnotationComposer,
          $$PendientesTableCreateCompanionBuilder,
          $$PendientesTableUpdateCompanionBuilder,
          (Pendiente, BaseReferences<_$BaseLocal, $PendientesTable, Pendiente>),
          Pendiente,
          PrefetchHooks Function()
        > {
  $$PendientesTableTableManager(_$BaseLocal db, $PendientesTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () => $$PendientesTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () => $$PendientesTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () => $$PendientesTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> clave = const Value.absent(),
                Value<String> mesa = const Value.absent(),
                Value<String> cuerpo = const Value.absent(),
                Value<int> creado = const Value.absent(),
                Value<int> intentos = const Value.absent(),
                Value<String?> ultimoError = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => PendientesCompanion(clave: clave, mesa: mesa, cuerpo: cuerpo, creado: creado, intentos: intentos, ultimoError: ultimoError, rowid: rowid),
          createCompanionCallback:
              ({
                required String clave,
                required String mesa,
                required String cuerpo,
                required int creado,
                Value<int> intentos = const Value.absent(),
                Value<String?> ultimoError = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => PendientesCompanion.insert(
                clave: clave,
                mesa: mesa,
                cuerpo: cuerpo,
                creado: creado,
                intentos: intentos,
                ultimoError: ultimoError,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) =>
              p0.map((e) => (e.readTable<$PendientesTable, Pendiente>(table), BaseReferences<_$BaseLocal, $PendientesTable, Pendiente>(db, table, e))).toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$PendientesTableProcessedTableManager =
    ProcessedTableManager<
      _$BaseLocal,
      $PendientesTable,
      Pendiente,
      $$PendientesTableFilterComposer,
      $$PendientesTableOrderingComposer,
      $$PendientesTableAnnotationComposer,
      $$PendientesTableCreateCompanionBuilder,
      $$PendientesTableUpdateCompanionBuilder,
      (Pendiente, BaseReferences<_$BaseLocal, $PendientesTable, Pendiente>),
      Pendiente,
      PrefetchHooks Function()
    >;

class $BaseLocalManager {
  final _$BaseLocal _db;
  $BaseLocalManager(this._db);
  $$CacheTableTableManager get cache => $$CacheTableTableManager(_db, _db.cache);
  $$PendientesTableTableManager get pendientes => $$PendientesTableTableManager(_db, _db.pendientes);
}
