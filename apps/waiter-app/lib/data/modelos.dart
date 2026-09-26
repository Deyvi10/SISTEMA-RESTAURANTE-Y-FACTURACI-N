/// Modelos del catálogo, el salón y las órdenes tal como los entrega el Nodo Local.
/// Importes y cantidades siempre como texto decimal (nunca double).
library;

class Categoria {
  const Categoria({required this.id, required this.nombre, required this.icono, required this.color, required this.orden, required this.notas});
  factory Categoria.fromJson(Map<String, dynamic> j) => Categoria(
        id: j['id'] as String,
        nombre: j['nombre'] as String,
        icono: j['icono'] as String? ?? 'utensils',
        color: j['color'] as String? ?? 'orange',
        orden: j['orden'] as int? ?? 0,
        notas: [for (final n in (j['notasRapidas'] as List? ?? const [])) n as String],
      );
  final String id, nombre, icono, color;
  final int orden;
  final List<String> notas;
}

class Modificador {
  const Modificador({required this.id, required this.nombre, required this.precioAdicional});
  factory Modificador.fromJson(Map<String, dynamic> j) =>
      Modificador(id: j['id'] as String, nombre: j['nombre'] as String, precioAdicional: j['precioAdicional'] as String? ?? '0');
  final String id, nombre, precioAdicional;
}

class GrupoModificadores {
  const GrupoModificadores({required this.id, required this.nombre, required this.obligatorio, required this.min, required this.max, required this.modificadores});
  factory GrupoModificadores.fromJson(Map<String, dynamic> j) => GrupoModificadores(
        id: j['id'] as String,
        nombre: j['nombre'] as String,
        obligatorio: j['obligatorio'] as bool? ?? false,
        min: j['min'] as int? ?? 0,
        max: j['max'] as int? ?? 1,
        modificadores: [for (final m in (j['modificadores'] as List? ?? const [])) Modificador.fromJson(m as Map<String, dynamic>)],
      );
  final String id, nombre;
  final bool obligatorio;
  final int min, max;
  final List<Modificador> modificadores;

  int get minimo => obligatorio && min < 1 ? 1 : min;
}

class Producto {
  const Producto({
    required this.id, required this.categoriaId, required this.nombre, required this.alias, required this.descripcion,
    required this.precio, required this.foto, required this.fotoMd, required this.grupos, required this.vendidos,
  });
  factory Producto.fromJson(Map<String, dynamic> j) => Producto(
        id: j['id'] as String,
        categoriaId: j['categoriaId'] as String,
        nombre: j['nombre'] as String,
        alias: j['alias'] as String? ?? '',
        descripcion: j['descripcion'] as String? ?? '',
        precio: j['precio'] as String,
        foto: j['foto'] as String?,
        fotoMd: j['fotoMd'] as String?,
        grupos: [for (final g in (j['grupos'] as List? ?? const [])) g as String],
        vendidos: j['vendidos'] as int? ?? 0,
      );
  final String id, categoriaId, nombre, alias, descripcion, precio;
  final String? foto, fotoMd;
  final List<String> grupos;
  final int vendidos;
}

class Catalogo {
  const Catalogo({required this.version, required this.categorias, required this.productos, required this.grupos});
  factory Catalogo.fromJson(Map<String, dynamic> j) => Catalogo(
        version: j['version'] as String? ?? '0',
        categorias: [for (final c in (j['categorias'] as List? ?? const [])) Categoria.fromJson(c as Map<String, dynamic>)],
        productos: [for (final p in (j['productos'] as List? ?? const [])) Producto.fromJson(p as Map<String, dynamic>)],
        grupos: {for (final g in (j['grupos'] as List? ?? const [])) (g['id'] as String): GrupoModificadores.fromJson(g as Map<String, dynamic>)},
      );
  static const vacio = Catalogo(version: '', categorias: [], productos: [], grupos: {});
  final String version;
  final List<Categoria> categorias;
  final List<Producto> productos;
  final Map<String, GrupoModificadores> grupos;

  List<GrupoModificadores> gruposDe(Producto p) => [for (final g in p.grupos) if (grupos[g] != null) grupos[g]!];
  Categoria? categoria(String id) => categorias.where((c) => c.id == id).firstOrNull;
}

class Persona {
  const Persona({required this.id, required this.nombre, required this.rol, this.avatarUrl});
  factory Persona.fromJson(Map<String, dynamic> j) =>
      Persona(id: j['id'] as String, nombre: j['nombre'] as String, rol: j['rol'] as String, avatarUrl: j['avatarUrl'] as String?);
  final String id, nombre, rol;
  final String? avatarUrl;

  String get iniciales {
    final p = nombre.split(RegExp(r'\s+')).where((w) => w.isNotEmpty).toList();
    if (p.isEmpty) return '?';
    return (p.first[0] + (p.length > 1 ? p[1][0] : '')).toUpperCase();
  }
}

class Usuario {
  const Usuario({required this.id, required this.nombre, required this.rol, required this.permisos});
  factory Usuario.fromJson(Map<String, dynamic> j) => Usuario(
        id: j['id'] as String,
        nombre: j['nombre'] as String,
        rol: j['rol'] as String,
        permisos: {for (final p in (j['permisos'] as List? ?? const [])) p as String},
      );
  final String id, nombre, rol;
  final Set<String> permisos;
  bool puede(String p) => permisos.contains(p);
}

enum EstadoMesaVivo { libre, ocupada, porPagar, demorada }

class Bloqueo {
  const Bloqueo({required this.usuarioId, required this.usuarioNombre, required this.dispositivoId, required this.expiraAt});
  factory Bloqueo.fromJson(Map<String, dynamic> j) => Bloqueo(
        usuarioId: j['usuarioId'] as String,
        usuarioNombre: j['usuarioNombre'] as String,
        dispositivoId: j['dispositivoId'] as String,
        expiraAt: DateTime.parse(j['expiraAt'] as String),
      );
  final String usuarioId, usuarioNombre, dispositivoId;
  final DateTime expiraAt;
}

class Zona {
  const Zona({required this.id, required this.nombre});
  factory Zona.fromJson(Map<String, dynamic> j) => Zona(id: j['id'] as String, nombre: j['nombre'] as String);
  final String id, nombre;
}

class Mesa {
  const Mesa({
    required this.id, required this.zonaId, required this.nombre, required this.capacidad, required this.estado,
    this.ordenId, this.meseroNombre, this.abiertaAt, this.comensales, required this.platos, required this.total, this.bloqueo,
  });
  factory Mesa.fromJson(Map<String, dynamic> j) => Mesa(
        id: j['id'] as String,
        zonaId: j['zonaId'] as String,
        nombre: j['nombre'] as String,
        capacidad: j['capacidad'] as int? ?? 4,
        estado: switch (j['estado']) {
          'OCUPADA' => EstadoMesaVivo.ocupada,
          'POR_PAGAR' => EstadoMesaVivo.porPagar,
          'DEMORADA' => EstadoMesaVivo.demorada,
          _ => EstadoMesaVivo.libre,
        },
        ordenId: j['ordenId'] as String?,
        meseroNombre: j['meseroNombre'] as String?,
        abiertaAt: j['abiertaAt'] == null ? null : DateTime.parse(j['abiertaAt'] as String),
        comensales: j['comensales'] as int?,
        platos: j['platos'] as int? ?? 0,
        total: j['total'] as String? ?? '0.00',
        bloqueo: j['bloqueo'] == null ? null : Bloqueo.fromJson(j['bloqueo'] as Map<String, dynamic>),
      );
  final String id, zonaId, nombre, total;
  final int capacidad, platos;
  final EstadoMesaVivo estado;
  final String? ordenId, meseroNombre;
  final DateTime? abiertaAt;
  final int? comensales;
  final Bloqueo? bloqueo;
}

class Salon {
  const Salon({required this.zonas, required this.mesas});
  factory Salon.fromJson(Map<String, dynamic> j) => Salon(
        zonas: [for (final z in (j['zonas'] as List? ?? const [])) Zona.fromJson(z as Map<String, dynamic>)],
        mesas: [for (final m in (j['mesas'] as List? ?? const [])) Mesa.fromJson(m as Map<String, dynamic>)],
      );
  static const vacio = Salon(zonas: [], mesas: []);
  final List<Zona> zonas;
  final List<Mesa> mesas;
}

class LineaEnviada {
  const LineaEnviada({
    required this.id, required this.productoId, required this.producto, required this.cantidad, required this.modificadores,
    required this.nota, required this.tiempo, required this.estado, required this.total,
  });
  factory LineaEnviada.fromJson(Map<String, dynamic> j) => LineaEnviada(
        id: j['id'] as String,
        productoId: j['productoId'] as String,
        producto: j['producto'] as String,
        cantidad: j['cantidad'] as String,
        modificadores: [for (final m in (j['modificadores'] as List? ?? const [])) (m as Map<String, dynamic>)['nombre'] as String],
        nota: j['nota'] as String? ?? '',
        tiempo: j['tiempo'] as String? ?? '',
        estado: j['estado'] as String,
        total: j['total'] as String? ?? '0.00',
      );
  final String id, productoId, producto, cantidad, nota, tiempo, estado, total;
  final List<String> modificadores;
  bool get anulada => estado == 'ANULADA';
  bool get enEspera => estado == 'EN_ESPERA';
}

class Orden {
  const Orden({required this.id, required this.mesa, required this.numero, required this.estado, required this.lineas, required this.total, required this.meseroNombre});
  factory Orden.fromJson(Map<String, dynamic> j) => Orden(
        id: j['id'] as String,
        mesa: j['mesa'] as String? ?? '',
        numero: j['numero'] as int? ?? 0,
        estado: j['estado'] as String,
        lineas: [for (final l in (j['lineas'] as List? ?? const [])) LineaEnviada.fromJson(l as Map<String, dynamic>)],
        total: j['total'] as String? ?? '0.00',
        meseroNombre: j['meseroNombre'] as String? ?? '',
      );
  final String id, mesa, estado, total, meseroNombre;
  final int numero;
  final List<LineaEnviada> lineas;
}
