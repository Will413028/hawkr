import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// session token 是登入憑證：只能經 `SessionTokenStore`，而它的預設實作只用 flutter_secure_storage。
/// 這支守門測試讀原始碼，擋住「token 改存 SharedPreferences、檔案」或「預設保存處被換成別的」。
void main() {
  final libFiles = Directory('lib')
      .listSync(recursive: true)
      .whereType<File>()
      .where((f) => f.path.endsWith('.dart'))
      .toList();

  test('只有 session_token_store.dart 碰儲存 API，而且是 flutter_secure_storage', () {
    final offenders = <String>[];
    for (final file in libFiles) {
      final source = file.readAsStringSync();
      final isStore = file.path.endsWith('session_token_store.dart');
      final usesPlainStorage = RegExp(
        r"shared_preferences|dart:io|hive|sqflite|drift|path_provider",
      ).hasMatch(source);
      if (usesPlainStorage) offenders.add(file.path);
      if (!isStore && source.contains('flutter_secure_storage')) {
        offenders.add('${file.path} (secure storage 在保存處以外)');
      }
    }
    expect(offenders, isEmpty);
  });

  test('預設的 sessionTokenStoreProvider 是 SecureSessionTokenStore', () {
    final source = File('lib/providers.dart').readAsStringSync();
    expect(
      source,
      matches(
        RegExp(
          r'sessionTokenStoreProvider\s*=\s*Provider<SessionTokenStore>\(\s*\(ref\)\s*=>\s*const SecureSessionTokenStore\(\)',
        ),
      ),
    );
  });
}
