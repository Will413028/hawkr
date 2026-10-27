/// 已封鎖的使用者（`GET /api/v1/app/blocks` 單筆項目，backend-payuni#645）。
///
/// **權威來源**：gateway `ListBlocks` 把 proto `UserBlock` 直接放進 `records`
/// （backend-payuni `gateway/internal/handler/moderation.go:247`），所以 wire 用的是
/// protoc-gen-go 的 **snake_case** tag，只有三個欄位
/// （`api/proto/social/social.proto:123-127`）：
///
/// ```
/// id              封鎖紀錄本身的 TSID（不是使用者 id）
/// blocked_id      被封鎖使用者的 id
/// create_time_ms  建立時間（epoch ms）
/// ```
///
/// 2026-08-06 對 dev 實打（封鎖 → 查詢 → 解封）逐字確認。
///
/// ⚠️ [fromRaw] 曾經用多重 candidate key 解析，而候選鏈裡**沒有** `blocked_id`、
/// 卻有 `id` —— 於是 [userId] 拿到的是封鎖紀錄自己的 TSID，
/// 「解除封鎖」打過去後端 delete-by-pair 找不到列，整個功能是靜默 no-op。
/// 暱稱與頭像後端從未回傳（候選鏈整條落空），兩個欄位一併移除。
class BlockedUser {
  const BlockedUser({required this.userId, required this.createTimeMs});

  final String userId;
  final int createTimeMs;

  /// 顯示標籤：後端沒有暱稱欄位，一律以 id 尾六碼標示。
  String get displayLabel =>
      '使用者 ${userId.length > 6 ? userId.substring(userId.length - 6) : userId}';

  static BlockedUser fromRaw(Map<String, dynamic> j) {
    int ms(dynamic v) {
      if (v is int) return v;
      if (v is num) return v.toInt();
      return int.tryParse(v?.toString() ?? '') ?? 0;
    }

    return BlockedUser(
      userId: (j['blocked_id'] ?? '').toString().trim(),
      createTimeMs: ms(j['create_time_ms']),
    );
  }
}
