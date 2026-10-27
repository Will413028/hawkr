import 'post_model.dart';

/// 賣家「自己頁面」用：從 raw post map 判定是否被平台下架 + 取原因。
///
/// wire 欄位（見 backend `PostListItemData`）：`hiddenAt`（unix int，
/// `omitempty` → 未下架時整個欄位不出現）、`hiddenReason`（string，同樣
/// `omitempty`）。backend PR #2235 起契約鍵為 camelCase，**無 snake 別名**。
bool isPostHidden(Map<String, dynamic> p) {
  final v = p['hiddenAt'];
  if (v == null) return false;
  final n = (v is num) ? v.toInt() : (int.tryParse('$v') ?? 0);
  return n > 0;
}

String? hiddenReasonOf(Map<String, dynamic> p) {
  final r = p['hiddenReason']?.toString();
  return (r != null && r.isNotEmpty) ? r : null;
}

extension PostHidden on Post {
  /// 被平台下架（`hiddenAt` unix 秒 > 0），與 [isPostHidden] 的 map 版同一規則。
  bool get isHidden => (hiddenAt ?? 0) > 0;
}
