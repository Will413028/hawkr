import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:shared_preferences/shared_preferences.dart';

part 'receipt_photo_local_service.g.dart';

@riverpod
ReceiptPhotoLocalService receiptPhotoLocalService(Ref ref) {
  return SharedPreferencesReceiptPhotoLocalService();
}

abstract interface class ReceiptPhotoLocalService {
  Future<String?> read(String orderSn);

  Future<void> write(String orderSn, String url);

  Future<void> remove(String orderSn);

  /// 清除本裝置上所有帳號的收據照片快取。無帳號 scope，切換/登出帳號時呼叫。
  Future<void> clearAll();
}

final class SharedPreferencesReceiptPhotoLocalService
    implements ReceiptPhotoLocalService {
  static const _keyPrefix = 'receipt_photo_';

  static String _key(String orderSn) => '$_keyPrefix$orderSn';

  @override
  Future<String?> read(String orderSn) async {
    final preferences = await SharedPreferences.getInstance();
    return preferences.getString(_key(orderSn));
  }

  @override
  Future<void> remove(String orderSn) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.remove(_key(orderSn));
  }

  @override
  Future<void> write(String orderSn, String url) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(_key(orderSn), url);
  }

  @override
  Future<void> clearAll() async {
    final preferences = await SharedPreferences.getInstance();
    final keys = preferences.getKeys().where((k) => k.startsWith(_keyPrefix));
    for (final key in keys) {
      await preferences.remove(key);
    }
  }
}
