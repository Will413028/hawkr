import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

/// 用記憶體 map 取代 flutter_secure_storage 的原生層，回傳這個 map 供測試檢查寫入的結果。
///
/// 呼叫前必須先 `TestWidgetsFlutterBinding.ensureInitialized()`。
/// [initial] 的鍵用 `AuthStorage` 的實際鍵名：`auth_user_id`、`auth_nickname`。
Map<String, String> mockSecureStorage(Map<String, String> initial) {
  final store = Map<String, String>.from(initial);
  TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
      .setMockMethodCallHandler(
        const MethodChannel('plugins.it_nomads.com/flutter_secure_storage'),
        (call) async {
          final args =
              (call.arguments as Map?)?.cast<String, dynamic>() ?? const {};
          switch (call.method) {
            case 'read':
              return store[args['key'] as String?];
            case 'readAll':
              return store;
            case 'write':
              store[args['key'] as String] = args['value'] as String;
              return null;
            case 'delete':
              store.remove(args['key'] as String);
              return null;
            case 'deleteAll':
              store.clear();
              return null;
            case 'containsKey':
              return store.containsKey(args['key'] as String?);
            default:
              return null;
          }
        },
      );
  return store;
}
