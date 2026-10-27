// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'social_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_Post _$PostFromJson(Map<String, dynamic> json) => _Post(
  id: tsidToInt(json['id']),
  content: json['content'] as String? ?? '',
  merchantId: tsidToIntOrNull(json['sellerId']),
  merchantName: json['sellerName'] as String?,
  merchantAvatar: json['logoUrl'] as String?,
  imageKeys:
      (json['imageKeys'] as List<dynamic>?)?.map((e) => e as String).toList() ??
      const <String>[],
  imageUrls:
      (json['imageUrls'] as List<dynamic>?)?.map((e) => e as String).toList() ??
      const <String>[],
  likeCount: (json['likeCount'] as num?)?.toInt() ?? 0,
  commentCount: (json['commentCount'] as num?)?.toInt() ?? 0,
  shareCount: (json['shareCount'] as num?)?.toInt() ?? 0,
  viewCount: (json['viewCount'] as num?)?.toInt() ?? 0,
  liked: json['isLiked'] as bool? ?? false,
  createdAt: json['createdAt'],
  product: json['product'] as Map<String, dynamic>?,
);

Map<String, dynamic> _$PostToJson(_Post instance) => <String, dynamic>{
  'id': instance.id,
  'content': instance.content,
  'sellerId': instance.merchantId,
  'sellerName': instance.merchantName,
  'logoUrl': instance.merchantAvatar,
  'imageKeys': instance.imageKeys,
  'imageUrls': instance.imageUrls,
  'likeCount': instance.likeCount,
  'commentCount': instance.commentCount,
  'shareCount': instance.shareCount,
  'viewCount': instance.viewCount,
  'isLiked': instance.liked,
  'createdAt': instance.createdAt,
  'product': instance.product,
};

_PostComment _$PostCommentFromJson(Map<String, dynamic> json) => _PostComment(
  id: tsidToInt(json['id']),
  content: json['content'] as String,
  isMine: json['isMine'] as bool? ?? false,
  userName: json['userName'] as String?,
  avatarUrl: json['avatarUrl'] as String?,
  likeCount: (json['likeCount'] as num?)?.toInt() ?? 0,
  liked: json['isLiked'] as bool? ?? false,
  createdAt: json['createdAt'],
);

Map<String, dynamic> _$PostCommentToJson(_PostComment instance) =>
    <String, dynamic>{
      'id': instance.id,
      'content': instance.content,
      'isMine': instance.isMine,
      'userName': instance.userName,
      'avatarUrl': instance.avatarUrl,
      'likeCount': instance.likeCount,
      'isLiked': instance.liked,
      'createdAt': instance.createdAt,
    };
