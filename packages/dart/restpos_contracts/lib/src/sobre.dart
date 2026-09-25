/// Sobre de todo mensaje WebSocket: {v, id, type, ts, data}.
class Sobre {
  final int v;
  final String id;
  final String type;
  final DateTime ts;
  final Map<String, dynamic> data;

  const Sobre({required this.v, required this.id, required this.type, required this.ts, required this.data});

  factory Sobre.fromJson(Map<String, dynamic> j) => Sobre(
        v: j['v'] as int,
        id: j['id'] as String,
        type: j['type'] as String,
        ts: DateTime.parse(j['ts'] as String),
        data: (j['data'] as Map).cast<String, dynamic>(),
      );

  Map<String, dynamic> toJson() => {'v': v, 'id': id, 'type': type, 'ts': ts.toUtc().toIso8601String(), 'data': data};
}
