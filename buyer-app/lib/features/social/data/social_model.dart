import 'package:freezed_annotation/freezed_annotation.dart';

import '../../../core/json/tsid_json.dart';

part 'social_model.freezed.dart';
part 'social_model.g.dart';

@freezed
abstract class Post with _$Post {
  const Post._();

  const factory Post({
    @JsonKey(fromJson: tsidToInt) required int id,
    @Default('') String content,
    @JsonKey(name: 'sellerId', fromJson: tsidToIntOrNull) int? merchantId,
    // 店面網址的路徑段（列表端點由 gateway 從 seller 批次回填；詳情端點沒有）。空字串代表沒有可進的店面。
    @JsonKey(name: 'storeCode') @Default('') String storeCode,
    @JsonKey(name: 'sellerName') String? merchantName,
    @JsonKey(name: 'logoUrl') String? merchantAvatar,
    // 49 §6.1 成對契約：貼文圖只有複數欄。social 的 posts.image 是單一 TEXT 欄，
    // 所以現階段恆為 0–1 element；後端無 omitempty，無圖是 `[]` 而非缺欄。
    @JsonKey(name: 'imageKeys') @Default(<String>[]) List<String> imageKeys,
    @JsonKey(name: 'imageUrls') @Default(<String>[]) List<String> imageUrls,
    @JsonKey(name: 'likeCount') @Default(0) int likeCount,
    @JsonKey(name: 'commentCount') @Default(0) int commentCount,
    @JsonKey(name: 'shareCount') @Default(0) int shareCount,
    @JsonKey(name: 'viewCount') @Default(0) int viewCount,
    @JsonKey(name: 'isLiked') @Default(false) bool liked,
    @JsonKey(name: 'createdAt') dynamic createdAt,
    Map<String, dynamic>? product,
  }) = _Post;

  factory Post.fromJson(Map<String, dynamic> json) => _$PostFromJson(json);

  /// 封面圖顯示來源。
  ///
  /// 舊契約同時吐 `imageKey`/`imageUrl`（單數）與 `imageKeys`/`imageUrls`
  /// （複數）兩組；貼文編輯讀單數欄回送，複數欄那份就被當成「使用者刪掉了」
  /// 而清空 DB。單數欄已從後端移除，這裡由複數欄第一個元素導出。
  ///
  /// 優先 URL、其次裸 key —— 顯示端 `AppCachedImage(scene)` 走 buildImageUrl
  /// dual-mode，兩種都吃得下。
  String get imageUrl {
    if (imageUrls.isNotEmpty) return imageUrls.first;
    if (imageKeys.isNotEmpty) return imageKeys.first;
    return '';
  }
}

@freezed
abstract class PostComment with _$PostComment {
  const factory PostComment({
    @JsonKey(fromJson: tsidToInt) required int id,
    required String content,
    // 伺服器依呼叫者身分算好的「這是不是我的留言」；公開輸出沒有留言者 ID。
    @JsonKey(name: 'isMine') @Default(false) bool isMine,
    // REST 留言列表／寫入回應與 SSE comment.created 收斂後同名 userName。
    @JsonKey(name: 'userName') String? userName,
    // 49 §6.1 成對契約：留言者頭像顯示 URL（Gateway 組）。
    @JsonKey(name: 'avatarUrl') String? avatarUrl,
    @JsonKey(name: 'likeCount') @Default(0) int likeCount,
    @JsonKey(name: 'isLiked') @Default(false) bool liked,
    // REST 回 epoch 秒（int），SSE 回 ISO 8601 字串——兩通道型別不同，維持 dynamic。
    @JsonKey(name: 'createdAt') dynamic createdAt,
  }) = _PostComment;

  factory PostComment.fromJson(Map<String, dynamic> json) =>
      _$PostCommentFromJson(json);
}
