/// `GET /app/chat/conversations` 回傳的 conversation map 欄位解析。
///
/// **權威來源**：Gateway 的 REST DTO
/// （backend-payuni `gateway/internal/handler/chat.go:398-412`），它把
/// `api/proto/im/im.proto` 的 `message Conversation`（10 個欄位）轉成下列 12 個
/// key —— `*_key` 另外組成成對的 `*Url`：
///
/// ```
/// targetUserId / targetNickname / avatarKey / avatarUrl
/// lastMessage / lastMessageType / lastMessageTime / unreadCount
/// storeName / logoKey / logoUrl / storeCode
/// ```
///
/// ⚠️ 對話對象頭像的 REST key 是 **`avatarKey` / `avatarUrl`**，不是 proto 欄名
/// `target_avatar_key` 的 camel 版；`targetAvatarKey` / `targetAvatarUrl` 從來
/// 沒有出現在 wire 上（2026-08-06 對 dev 實打確認）。
///
/// 特別注意**沒有** `conversationId`——WebSocket 訊息才帶它。
///
/// 這個檔案存在的理由：這幾個 key 過去散在畫面裡，每個都寫成一長串
/// `a ?? b ?? c ?? d` 的候選清單，其中大多數後端從未回過。候選清單看起來周全，
/// 實際效果是讓下一個人以為後端可能回這些名字（hawkr_buyer#89）。
/// **要加 key 之前先去 im.proto 確認它真的存在。**
library;

const String conversationTargetUserIdKey = 'targetUserId';
const String conversationTargetNicknameKey = 'targetNickname';
const String conversationStoreNameKey = 'storeName';
const String conversationLogoUrlKey = 'logoUrl';
const String conversationLastMessageKey = 'lastMessage';
const String conversationLastMessageTypeKey = 'lastMessageType';
const String conversationLastMessageTimeKey = 'lastMessageTime';
const String conversationUnreadCountKey = 'unreadCount';

String _str(Map<String, dynamic> c, String key) =>
    (c[key] ?? '').toString().trim();

/// 對方（買家視角＝賣家）的 user id。列表點擊、標記已讀、ValueKey 都用它。
String resolveConversationTargetId(Map<String, dynamic> c) =>
    _str(c, conversationTargetUserIdKey);

/// 列表顯示名稱：店名優先、對象暱稱兜底，都沒有才回預設字樣。
String resolveConversationDisplayName(Map<String, dynamic> c) {
  final storeName = _str(c, conversationStoreNameKey);
  if (storeName.isNotEmpty) return storeName;
  final nickname = _str(c, conversationTargetNicknameKey);
  if (nickname.isNotEmpty) return nickname;
  return '商家';
}

/// 列表頭像：買家看到的對話對象是賣家，取店家 logo。
///
/// 同一個 conversation 物件另有 `avatarKey` / `avatarUrl`（對話對象的個人頭像），
/// 賣家視角才用得到，這裡刻意不讀。
String resolveConversationAvatarUrl(Map<String, dynamic> c) =>
    _str(c, conversationLogoUrlKey);

/// 最後訊息時間。wire 是 ISO 8601 UTC（§2 時間契約），不是 epoch 數字。
DateTime? resolveConversationLastMessageTime(Map<String, dynamic> c) {
  final parsed = DateTime.tryParse(_str(c, conversationLastMessageTimeKey));
  if (parsed == null || parsed.millisecondsSinceEpoch <= 0) return null;
  return parsed.isUtc ? parsed.toLocal() : parsed;
}

/// 未讀數。wire 是字串（proto `string unread_count`）。
int resolveConversationUnreadCount(Map<String, dynamic> c) {
  final raw = c[conversationUnreadCountKey];
  if (raw is int) return raw;
  return int.tryParse('$raw') ?? 0;
}
