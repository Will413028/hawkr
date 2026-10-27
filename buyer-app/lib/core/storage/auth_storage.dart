import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:uuid/uuid.dart';

/// 訪客購物車的 ID。
class AuthStorage {
  static const _storage = FlutterSecureStorage();

  static const _kGuestCartId = 'guest_cart_id';

  static Future<String?> existingGuestCartId() =>
      _storage.read(key: _kGuestCartId);

  static Future<void> clearGuestCartId(String id) async {
    if (await existingGuestCartId() == id) {
      await _storage.delete(key: _kGuestCartId);
    }
  }

  static Future<String?> cacheOwner() => _storage.read(key: 'cache_owner');

  static Future<void> setCacheOwner(String id) =>
      _storage.write(key: 'cache_owner', value: id);

  /// Returns the persistent guest cart UUID, generating one on first call.
  static Future<String> guestCartId() async {
    final existing = await _storage.read(key: _kGuestCartId);
    if (existing != null && existing.isNotEmpty) return existing;
    final id = const Uuid().v4();
    await _storage.write(key: _kGuestCartId, value: id);
    return id;
  }
}
