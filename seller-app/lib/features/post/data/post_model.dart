import 'package:freezed_annotation/freezed_annotation.dart';

part 'post_model.freezed.dart';
part 'post_model.g.dart';

// 後端數字欄位（id / sellerId / 各 count）常回字串，統一容錯轉 int，
// 避免 json_serializable 產生的 `as num` 對 String 直接丟型別轉換例外。
int? _asInt(Object? v) {
  if (v == null) return null;
  if (v is int) return v;
  if (v is num) return v.toInt();
  return int.tryParse(v.toString());
}

/// `id`＝一般貼文端點（列表／詳情／建立／更新）；`postId`＝admin 貼文列表
/// （`AdminPostItem`，gateway/internal/handler/post.go）。兩者是**不同端點的
/// 不同欄名**，不是新舊相容層。
Object? _readId(Map map, String key) => _asInt(map['id'] ?? map['postId']);
Object? _readSellerId(Map map, String key) => _asInt(map['sellerId']);
Object? _readLikeCount(Map map, String key) => _asInt(map['likeCount']);
Object? _readCommentCount(Map map, String key) => _asInt(map['commentCount']);
Object? _readShareCount(Map map, String key) => _asInt(map['shareCount']);

/// 發布狀態碼（`PostPublishStatus` 的 value）。`/app/sellers/me/posts` 帶；其他端點缺值，
/// 缺值是「這條路徑的上游沒給」，不是「已發布」。
Object? _readPublishStatus(Map map, String key) =>
    map['publishStatus']?.toString();

/// 平台下架時間（unix 秒）；未下架時整個欄位不出現。
Object? _readHiddenAt(Map map, String key) => _asInt(map['hiddenAt']);

/// 代行者排定、尚未執行的發布時間（ISO 8601 UTC）；只有 `/app/sellers/me/posts` 帶，沒有排程時欄位不出現。
Object? _readScheduledPublishAt(Map map, String key) =>
    map['scheduledPublishAt'];
Object? _readTimestamp(Map map, String key) => map['createdAt'];

/// 貼文圖片來源。契約（49 §6.1）SSOT 是 objectKey，**唯一欄位名是複數**
/// `imageKeys`/`imageUrls`（backend PR #2235 起）：所有貼文端點（列表
/// `GET /app/sellers/me/posts`、`GET /public/posts`、詳情
/// `GET /public/posts/{id}`、建立 `POST /app/sellers/posts`、更新
/// `PUT /app/sellers/posts/{id}`）形狀一致，**無圖時回 `[]`**
/// （欄位恆存在，不再 `omitempty`）。
///
/// ⚠️ 單數 `imageKey`/`imageUrl` **已從貼文物件上移除**，讀它只會恆得 null。
/// 貼文物件上還留著單數欄的只有巢狀的關聯商品 `product`
/// （`PostProductData`，gateway/internal/handler/post.go 的 `imageKey`/`imageUrl`），
/// 那是商品圖不是貼文圖，兩者不可混用。
///
/// **`images` / `image` 在任何貼文端點都不存在**（gateway 全域無這兩個
/// response 欄位），曾列入候選是猜欄位。新增候選前必須先實打端點確認。
///
/// 為什麼不留單數 fallback：曾經讀單數欄，而更新回應只回複數 → 解析出一則
/// 「沒有任何圖片」的貼文；再編輯一次時 `PostEditScreen` 的保留判斷誤判成
/// 「本來就沒圖」，`keepAssetId` 與 `newStagedToken` 同時送空字串 —— 後端據此
/// 把圖片從 DB 刪除，重進 App 也回不來（#430 A0，資料損毀）。雙讀 fallback
/// 正是那次事故的形狀，所以現在只認一個名字。
///
/// 值為 objectKey 優先、URL 次之；顯示前必須過 `buildImageUrl`，不可直接當 URL 用。
List<String> postImageSources(Map map) {
  for (final field in const ['imageKeys', 'imageUrls']) {
    final raw = map[field];
    if (raw is! List) continue;
    final values = <String>[];
    for (final e in raw) {
      final s = e?.toString().trim() ?? '';
      if (s.isNotEmpty) values.add(s);
    }
    if (values.isNotEmpty) return values;
  }
  return const <String>[];
}

Object? _readImageKeys(Map map, String key) => postImageSources(map);

/// 貼文讚數。契約鍵是 `likeCount`（backend PR #2235 全域 camelCase 收斂）。
///
/// 曾另打 `GET /public/posts/{id}/like_count` 補這個值，那支端點在 gateway
/// 上不存在（404 page not found）、錯誤又被吞掉，於是永遠拿不到數字 → 讚數
/// 恆為 0（#2215）。讚數一律由貼文物件本身帶出來。
int postLikeCount(Map map) => _asInt(map['likeCount']) ?? 0;

/// 貼文留言數。契約鍵是 `commentCount`。
int postCommentCount(Map map) => _asInt(map['commentCount']) ?? 0;

/// ⚠️ 貼文端點**沒有影片欄位**（gateway/internal/handler/post.go 全檔無 video
/// 相關 response 欄位），此值恆為 null。保留欄位是為了 UI 既有分支，不是契約。
Object? _readVideoUrl(Map map, String key) => map['videoUrl'];

/// 關聯商品。後端**同一份資料有兩種欄位形狀**（實打 dev 確認）：
/// - 列表 `GET /app/sellers/me/posts` 回
///   **單數物件 `product`**（`PostListItemData.Product *PostProductData`）
/// - 詳情 `GET /public/posts/{id}`、建立 `POST /app/sellers/posts`、更新
///   `PUT /app/sellers/posts/{id}` 回**陣列 `products`**（`PostDetailData.Products`）
///
/// `linkedProduct` 是 App 內部合併後的欄位（見 `PostRepository._attachLinkedProduct`），
/// 不是後端欄位，故排最前面。
/// 只讀 `products` 的話，賣家自己頁面的貼文列表永遠拿不到關聯商品 —— 卡片整個
/// 不渲染（2026-08-05 實打坐實：列表只回 `product`）。
Object? _readLinkedProduct(Map map, String key) {
  final direct = map['linkedProduct'];
  if (direct is Map) return Map<String, dynamic>.from(direct);
  final single = map['product'];
  if (single is Map) return Map<String, dynamic>.from(single);
  final products = map['products'];
  if (products is List && products.isNotEmpty && products.first is Map) {
    return Map<String, dynamic>.from(products.first as Map);
  }
  return null;
}

DateTime? _parseTimestamp(dynamic value) {
  if (value == null) return null;
  if (value is String) return DateTime.tryParse(value);
  if (value is int) {
    if (value > 10000000000) return DateTime.fromMillisecondsSinceEpoch(value);
    return DateTime.fromMillisecondsSinceEpoch(value * 1000);
  }
  return null;
}

@freezed
abstract class Post with _$Post {
  const factory Post({
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readId) required int id,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readSellerId) required int sellerId,
    @Default('') String title,
    @Default('') String content,
    @Default(<String>[]) List<String> tags,
    @Default('TEXT') String contentType, // 'TEXT', 'IMAGE', 'VIDEO'
    /// 貼文圖片來源清單（objectKey 優先、URL 次之，見 [_readImageKeys]）。
    /// 顯示前必須過 `buildImageUrl`；「這則貼文有沒有圖」一律以本清單是否為空
    /// 判斷，不要另外從原始 JSON 猜欄位名。
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readImageKeys)
    @Default(<String>[])
    List<String> imageKeys,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readVideoUrl) String? videoUrl,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readLikeCount) @Default(0) int likeCount,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readCommentCount) @Default(0) int commentCount,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readShareCount) @Default(0) int shareCount,
    @Default(false) bool isUnlocked,
    int? unlockPrice,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp)
    DateTime? createdAt,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readLinkedProduct) Map<String, dynamic>? linkedProduct,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readPublishStatus) String? publishStatus,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readHiddenAt) int? hiddenAt,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readScheduledPublishAt, fromJson: _parseTimestamp)
    DateTime? scheduledPublishAt,
  }) = _Post;

  factory Post.fromJson(Map<String, dynamic> json) => _$PostFromJson(json);
}

Object? _readCommentId(Map map, String key) => _asInt(map['id']);
// TSID 一律以字串傳輸（10 檔 §4），這裡統一容錯轉 int，
// 否則 json_serializable 產的 `as num?` 會對 String 直接丟型別轉換例外。
Object? _readCommentPostId(Map map, String key) => _asInt(map['postId']);
Object? _readCommentAuthor(Map map, String key) => map['userName'];

/// 留言者頭像來源。契約成對回 `avatarKey` + `avatarUrl`
/// （gateway/internal/handler/post.go `CommentItem`），SSOT 是 key。
/// 讀取（`GET /public/posts/{id}/comments`）與寫入（`POST /app/posts/{id}/comments`）
/// 現在同形狀；舊欄位 `userAvatarKey` / `userAvatarUrl` / `user_avatar_key` /
/// `userAvatar` 皆已不存在，讀它們只會恆得 null → 頭像永遠是佔位。
Object? _readCommentAvatar(Map map, String key) =>
    map['avatarKey'] ?? map['avatarUrl'];

@freezed
abstract class Comment with _$Comment {
  const factory Comment({
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readCommentId) required int id,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readCommentPostId) @Default(0) int postId,
    @Default(false) bool isMine,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readCommentAuthor) @Default('用戶') String author,

    /// objectKey 優先、URL 次之（見 [_readCommentAvatar]）；顯示前必須過
    /// `buildImageUrl(..., ImagePreset.avatar)`，不要直接當 URL 用。
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readCommentAvatar) @Default('') String avatarKey,
    @Default('') String content,
    // ignore: invalid_annotation_target
    @JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp)
    DateTime? createdAt,
  }) = _Comment;

  factory Comment.fromJson(Map<String, dynamic> json) =>
      _$CommentFromJson(json);
}

/// 開 `/post/feed` 到 [postId] 那一篇的 route extra；缺值或不是數字時沒有 extra（停在第一篇）。
Map<String, Object?>? postFeedExtra(String? postId) {
  final id = int.tryParse(postId?.trim() ?? '');
  return id == null ? null : {'postId': id};
}

/// 全螢幕貼文頁（`/post/feed`）一開始要停在清單的哪一篇：[postId] 在 [posts] 的序位；
/// 沒給或找不到就是第一篇。清單與頁面是同一份（含未發布與被平台下架的），不跳過任何一篇。
int feedIndexForPost(List<Post> posts, int? postId) {
  if (postId == null) return 0;
  final index = posts.indexWhere((post) => post.id == postId);
  return index < 0 ? 0 : index;
}
