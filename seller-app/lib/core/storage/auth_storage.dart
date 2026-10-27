import 'package:shared_preferences/shared_preferences.dart';

import '../../features/photo_editor/data/ai_edit_history_local_service.dart';
import '../../shared/utils/hk_log.dart';

/// 管理登入資訊的持久化存儲服務
///
/// 商家資料等非敏感快取存於 [SharedPreferences]。
class AuthStorage {
  AuthStorage._();

  static const String _cacheOwnerKey = 'auth_cache_owner';

  static Future<void> restoreAccount(String accountId) async {
    final SharedPreferences prefs;
    String? owner;
    try {
      prefs = await SharedPreferences.getInstance();
      owner = prefs.getString(_cacheOwnerKey);
    } catch (error, stack) {
      HKLog.logW('[AuthStorage] cache owner read failed', error, stack);
      return;
    }
    if (owner != null && owner != accountId) await logout();
    try {
      if (!await prefs.setString(_cacheOwnerKey, accountId)) {
        HKLog.logW('[AuthStorage] cache owner write failed');
      }
    } catch (error, stack) {
      HKLog.logW('[AuthStorage] cache owner write failed', error, stack);
    }
  }

  static Future<void> logout() async {
    final prefs = await SharedPreferences.getInstance();

    await prefs.remove(_cacheOwnerKey);

    await const AiEditHistoryLocalService().clear();
  }

  static Future<void> clearAll() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.clear();
  }
}
