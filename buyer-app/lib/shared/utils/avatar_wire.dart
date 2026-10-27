/// 買家頭像的 wire 契約（49 §6.1／§6.6）。
///
/// 對外 JSON 的同一張頭像過去有九個名字（`avatar`、`object_key`、
/// `user_avatar_key`、`avatarKey`、`targetAvatarKey`、`buyerAvatarKey`、
/// `userAvatarKey`、SSE `userAvatar`、WS `senderAvatar`），本批一律收斂成
/// **扁平成對**的 `avatarKey`（裸 objectKey，SSOT）＋ `avatarUrl`（Gateway 以
/// `MEDIA_BASE_URL` 組的顯示 URL）。SSE／WS 兩條通道不經 Gateway 組裝，
/// **只有 `avatarKey`**（49 §6.7 S4）。
///
/// 這裡刻意不做任何舊名 fallback：舊名在後端已物理移除，留著只會在某天有同名
/// 欄位時搶先命中並蓋掉正確值。
library;

/// 頭像 objectKey 的唯一 wire key。
const String avatarKeyWireKey = 'avatarKey';

/// 頭像顯示 URL 的唯一 wire key。
const String avatarUrlWireKey = 'avatarUrl';

/// 從任一含頭像的 JSON 物件（留言／評論／討論區／會員資料／SSE／WS）取出裸
/// objectKey。找不到或為空回 `null`。
///
/// 顯示端拿到 key 後過 `buildImageUrl(key, ImagePreset.avatar)`（或帶 scene 的
/// `AppCachedImage`）組 URL —— 不要直接用 `avatarUrl`，那是 `original` preset。
String? resolveAvatarKey(Map<String, dynamic>? map) {
  final value = map?[avatarKeyWireKey];
  if (value == null) return null;
  final s = value.toString().trim();
  return s.isEmpty ? null : s;
}

/// 從含頭像的 JSON 物件取出 Gateway 組好的顯示 URL（`original` preset；外部頭像
/// 如 Google 為 passthrough）。找不到或為空回 `null`。
///
/// `avatarUrl` 由 `BuildURL(base, avatarKey)` 產生，對非空 key 恆回非空字串，
/// 所以它為空 ⟺ `avatarKey` 為空 —— **不需要**再 fallback 到 `avatarKey`。
String? resolveAvatarUrl(Map<String, dynamic>? map) {
  final value = map?[avatarUrlWireKey];
  if (value == null) return null;
  final s = value.toString().trim();
  return s.isEmpty ? null : s;
}
