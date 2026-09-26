import 'dart:async';

import 'package:multicast_dns/multicast_dns.dart';

import 'nodo_api.dart';

/// Encuentra el Nodo Local (F3-03): primero las direcciones conocidas (las del QR o la última
/// que funcionó) y, si ninguna responde, lo busca por mDNS (`_restpos._tcp`) en el WiFi.
class Descubridor {
  Descubridor({this.buscarMdns = true});
  final bool buscarMdns;

  Future<String?> encontrar(List<String> conocidas, NodoApi Function(String) fabrica) async {
    for (final u in conocidas) {
      if (await fabrica(u).vivo()) return u;
    }
    if (!buscarMdns) return null;
    for (final u in await porMdns()) {
      if (await fabrica(u).vivo()) return u;
    }
    return null;
  }

  /// Anuncios `_restpos._tcp.local` → http://ip:puerto.
  Future<List<String>> porMdns({Duration espera = const Duration(seconds: 3)}) async {
    final cliente = MDnsClient();
    final out = <String>[];
    try {
      await cliente.start();
      await for (final ptr in cliente.lookup<PtrResourceRecord>(ResourceRecordQuery.serverPointer('_restpos._tcp.local')).timeout(espera, onTimeout: (s) => s.close())) {
        await for (final srv in cliente.lookup<SrvResourceRecord>(ResourceRecordQuery.service(ptr.domainName)).timeout(espera, onTimeout: (s) => s.close())) {
          await for (final ip in cliente.lookup<IPAddressResourceRecord>(ResourceRecordQuery.addressIPv4(srv.target)).timeout(espera, onTimeout: (s) => s.close())) {
            out.add('http://${ip.address.address}:${srv.port}');
          }
        }
      }
    } catch (_) {
      // Redes que bloquean multicast: queda la IP manual.
    } finally {
      cliente.stop();
    }
    return out.toSet().toList();
  }
}
