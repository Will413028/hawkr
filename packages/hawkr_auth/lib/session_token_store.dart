import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// identity 的 session token（opaque）的保存處。token 是登入憑證，只放安全儲存（iOS Keychain），不放
/// SharedPreferences 或檔案。
abstract interface class SessionTokenStore {
  Future<String?> read();

  Future<void> write(String token);

  Future<void> delete();
}

/// 兩支 App 的實作：iOS Keychain（flutter_secure_storage）。
final class SecureSessionTokenStore implements SessionTokenStore {
  const SecureSessionTokenStore([this._storage = const FlutterSecureStorage()]);

  static const key = 'hawkr.identity.session_token';

  final FlutterSecureStorage _storage;

  @override
  Future<String?> read() => _storage.read(key: key);

  @override
  Future<void> write(String token) => _storage.write(key: key, value: token);

  @override
  Future<void> delete() => _storage.delete(key: key);
}
