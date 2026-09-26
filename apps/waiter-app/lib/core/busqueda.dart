/// Búsqueda predictiva del menú (F3-08): insensible a tildes y mayúsculas, por prefijos de
/// palabras («sec pol» → Seco de Pollo), por alias («CM») y priorizando lo más vendido.
/// El índice se arma una vez al cargar el catálogo; cada pulsación es O(n) sin asignar.
class IndiceBusqueda<T> {
  IndiceBusqueda(Iterable<T> items, {required String Function(T) nombre, String Function(T)? alias, int Function(T)? vendidos})
    : _entradas = [for (final it in items) _Entrada(it, _palabras(normalizar(nombre(it))), normalizar(alias?.call(it) ?? ''), vendidos?.call(it) ?? 0)];

  final List<_Entrada<T>> _entradas;

  static final _mapa = {
    'á': 'a',
    'à': 'a',
    'ä': 'a',
    'â': 'a',
    'é': 'e',
    'è': 'e',
    'ë': 'e',
    'ê': 'e',
    'í': 'i',
    'ì': 'i',
    'ï': 'i',
    'î': 'i',
    'ó': 'o',
    'ò': 'o',
    'ö': 'o',
    'ô': 'o',
    'ú': 'u',
    'ù': 'u',
    'ü': 'u',
    'û': 'u',
    'ñ': 'n',
    'ç': 'c',
  };

  /// Minúsculas sin tildes ni signos.
  static String normalizar(String s) {
    final b = StringBuffer();
    for (final r in s.toLowerCase().runes) {
      final c = String.fromCharCode(r);
      final m = _mapa[c] ?? c;
      b.write(RegExp(r'[a-z0-9]').hasMatch(m) ? m : ' ');
    }
    return b.toString();
  }

  static List<String> _palabras(String s) => s.split(' ').where((w) => w.isNotEmpty).toList(growable: false);

  /// Resultados ordenados por relevancia y luego por lo más vendido.
  List<T> buscar(String consulta, {int limite = 40}) {
    final q = _palabras(normalizar(consulta));
    if (q.isEmpty) return const [];
    final puntuados = <(int, int, T)>[];
    for (final e in _entradas) {
      final p = e.puntaje(q);
      if (p > 0) puntuados.add((p, e.vendidos, e.item));
    }
    puntuados.sort((a, b) => a.$1 != b.$1 ? b.$1.compareTo(a.$1) : b.$2.compareTo(a.$2));
    return [for (final x in puntuados.take(limite)) x.$3];
  }
}

class _Entrada<T> {
  _Entrada(this.item, this.palabras, this.alias, this.vendidos);
  final T item;
  final List<String> palabras;
  final String alias;
  final int vendidos;

  /// 0 = no coincide. Cada término debe ser prefijo de alguna palabra (o el alias).
  int puntaje(List<String> q) {
    final a = alias.trim();
    if (q.length == 1 && a.isNotEmpty && a == q.first) return 1000;
    var total = 0;
    for (var i = 0; i < q.length; i++) {
      final t = q[i];
      var mejor = 0;
      for (var j = 0; j < palabras.length; j++) {
        final w = palabras[j];
        if (w == t) {
          mejor = mejor < 30 ? 30 : mejor;
        } else if (w.startsWith(t)) {
          final p = 20 - (j < 10 ? j : 10) + (i == j ? 5 : 0);
          if (p > mejor) mejor = p;
        } else if (t.length >= 3 && w.contains(t)) {
          if (mejor < 5) mejor = 5;
        }
      }
      if (mejor == 0) return 0;
      total += mejor;
    }
    return total;
  }
}
