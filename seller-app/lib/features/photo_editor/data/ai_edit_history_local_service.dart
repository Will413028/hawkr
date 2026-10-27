import 'dart:convert';

import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'ai_edit_history_model.dart';

part 'ai_edit_history_local_service.g.dart';

class AiEditHistoryLocalService {
  const AiEditHistoryLocalService();

  static const _storageKey = 'image_editor_ai_history';

  Future<List<AiEditHistoryEntry>> load() async {
    final preferences = await SharedPreferences.getInstance();
    final encoded = preferences.getString(_storageKey);
    if (encoded == null || encoded.isEmpty) return const [];
    final decoded = jsonDecode(encoded);
    if (decoded is! List) return const [];
    return List<AiEditHistoryEntry>.unmodifiable(
      decoded.whereType<Map>().map(
        (entry) =>
            AiEditHistoryEntry.fromJson(Map<String, dynamic>.from(entry)),
      ),
    );
  }

  Future<void> save(List<AiEditHistoryEntry> entries) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(
      _storageKey,
      jsonEncode(
        entries.map((entry) => entry.toJson()).toList(growable: false),
      ),
    );
  }

  /// 登出／刪帳號重開店流程呼叫：這份歷史紀錄目前無帳號 scope，
  /// 換帳號後留著會讓下一位使用者看到上一位的修圖動作名稱/次數/時間。
  Future<void> clear() async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.remove(_storageKey);
  }
}

@riverpod
AiEditHistoryLocalService aiEditHistoryLocalService(Ref ref) {
  return const AiEditHistoryLocalService();
}
