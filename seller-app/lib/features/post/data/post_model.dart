import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_api/images.dart';

part 'post_model.freezed.dart';
part 'post_model.g.dart';

int? _asInt(Object? v) {
  if (v == null) return null;
  if (v is int) return v;
  if (v is num) return v.toInt();
  return int.tryParse(v.toString());
}

Object? _readTimestamp(Map map, String key) => map['createdAt'];

DateTime? _parseTimestamp(dynamic value) {
  if (value == null) return null;
  if (value is String) return DateTime.tryParse(value);
  if (value is int) {
    if (value > 10000000000) return DateTime.fromMillisecondsSinceEpoch(value);
    return DateTime.fromMillisecondsSinceEpoch(value * 1000);
  }
  return null;
}

/// 店主讀自己的貼文的顯示模型，由 `social.v1alpha1.Post` 轉出（[postFromProto]）。
/// [etag] 是修改、發布、下架時要原樣帶回的版本；[imageUrl] 是圖片的完整網址（空字串是沒有圖），
/// 顯示前仍過 `buildImageUrl` 依場景換 preset。
@freezed
abstract class Post with _$Post {
  const Post._();

  const factory Post({
    required int id,
    @Default('') String etag,
    @Default('') String content,
    @Default('') String imageUrl,
    @Default(0) int likeCount,
    @Default(0) int commentCount,
    @Default(0) int shareCount,
    DateTime? createdAt,
    Map<String, dynamic>? linkedProduct,
    @Default(PostStatus.POST_STATUS_UNSPECIFIED) PostStatus status,
    String? takedownMeasureId,
    DateTime? scheduledPublishAt,
    @Default('') String url,
  }) = _Post;

  bool get hasImage => imageUrl.isNotEmpty;

  /// 被平台內容下架（`takedown_measure_id` 有值）。
  bool get isHidden => takedownMeasureId != null;
}

Map<String, dynamic>? _linkedProductOf(pb.Post post) {
  if (!post.hasProduct() || post.product.id.isEmpty) return null;
  final product = post.product;
  return <String, dynamic>{
    'id': product.id,
    'name': product.name,
    'price': product.minPrice.toInt(),
    if (product.hasImage()) 'imageKey': product.image.uri(ImagePreset.original),
  };
}

Post postFromProto(pb.Post post) => Post(
  id: int.parse(post.id),
  etag: post.etag,
  content: post.body,
  imageUrl: post.hasImage() ? post.image.uri(ImagePreset.original) : '',
  likeCount: post.likeCount.toInt(),
  commentCount: post.commentCount.toInt(),
  shareCount: post.shareCount.toInt(),
  createdAt: post.hasCreateTime() ? post.createTime.toDateTime() : null,
  linkedProduct: _linkedProductOf(post),
  status: post.status,
  takedownMeasureId: post.hasTakedownMeasureId()
      ? post.takedownMeasureId
      : null,
  url: post.url,
);

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
