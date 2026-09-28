import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app.dart';
import 'core/estado.dart';
import 'data/db.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final db = await BaseLocal.abrir();
  runApp(ProviderScope(overrides: [baseLocalProvider.overrideWithValue(db)], child: const RestPosApp()));
}
