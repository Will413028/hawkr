import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// session token 是登入憑證：App 的 `AuthSession` 只能用 hawkr_auth 的 Keychain 保存處（`sessionTokenStoreProvider`），
/// 而且 App 不自己保存 token（SharedPreferences、檔案）。讀原始碼的守門測試。
void main() {
  final libFiles = Directory('lib')
      .listSync(recursive: true)
      .whereType<File>()
      .where((f) => f.path.endsWith('.dart'))
      .toList();

  test('authSessionProvider 用 hawkr_auth 的 sessionTokenStoreProvider', () {
    final source = File(
      'lib/core/session/auth_session_provider.dart',
    ).readAsStringSync();
    expect(
      source,
      contains('store: ref.watch(auth.sessionTokenStoreProvider)'),
    );
  });

  test('App 不覆寫保存處，也不把 session token 存進 SharedPreferences', () {
    final offenders = <String>[];
    for (final file in libFiles) {
      final source = file
          .readAsLinesSync()
          .where((line) => !line.trimLeft().startsWith('//'))
          .join('\n');
      if (source.contains('sessionTokenStoreProvider') &&
          !file.path.endsWith('auth_session_provider.dart')) {
        offenders.add('${file.path} (碰保存處)');
      }
      if (RegExp(r'session_?token', caseSensitive: false).hasMatch(source) &&
          source.contains('SharedPreferences')) {
        offenders.add('${file.path} (SharedPreferences 與 session token)');
      }
    }
    expect(offenders, isEmpty);
  });
}
