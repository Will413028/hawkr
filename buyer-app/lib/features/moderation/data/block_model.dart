/// GET /app/blocks 回傳單筆封鎖紀錄。
///
/// **權威來源**：gateway `ListBlocks` 把 proto `UserBlock` 直接塞進 `records`
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
/// 2026-08-06 對 dev 實打（買家 2623900520，封鎖 → 查詢 → 解封）逐字確認。
///
/// `id` 故意不當作 [userId] 的候選——放了會讓「解除封鎖」打錯 id、後端
/// delete-by-pair 找不到列而變成 no-op。
/// 名稱與頭像後端**完全沒有**回傳，兩個欄位已從 model 移除：留著候選 key 只會
/// 讓下一個人以為後端可能回這些名字。
class BlockedUser {
  const BlockedUser({required this.userId, this.createTimeMs});

  final String userId;
  final int? createTimeMs;

  factory BlockedUser.fromJson(Map<String, dynamic> json) {
    final createTimeMs = json['create_time_ms'];
    return BlockedUser(
      userId: json['blocked_id']?.toString().trim() ?? '',
      createTimeMs: createTimeMs is int
          ? createTimeMs
          : createTimeMs is num
          ? createTimeMs.toInt()
          : int.tryParse(createTimeMs?.toString() ?? ''),
    );
  }
}
