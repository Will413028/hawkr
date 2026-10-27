// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'post_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_Post _$PostFromJson(Map<String, dynamic> json) => _Post(
  id: (_readId(json, 'id') as num).toInt(),
  sellerId: (_readSellerId(json, 'sellerId') as num).toInt(),
  title: json['title'] as String? ?? '',
  content: json['content'] as String? ?? '',
  tags:
      (json['tags'] as List<dynamic>?)?.map((e) => e as String).toList() ??
      const <String>[],
  contentType: json['contentType'] as String? ?? 'TEXT',
  imageKeys:
      (_readImageKeys(json, 'imageKeys') as List<dynamic>?)
          ?.map((e) => e as String)
          .toList() ??
      const <String>[],
  videoUrl: _readVideoUrl(json, 'videoUrl') as String?,
  likeCount: (_readLikeCount(json, 'likeCount') as num?)?.toInt() ?? 0,
  commentCount: (_readCommentCount(json, 'commentCount') as num?)?.toInt() ?? 0,
  shareCount: (_readShareCount(json, 'shareCount') as num?)?.toInt() ?? 0,
  isUnlocked: json['isUnlocked'] as bool? ?? false,
  unlockPrice: (json['unlockPrice'] as num?)?.toInt(),
  createdAt: _parseTimestamp(_readTimestamp(json, 'createdAt')),
  linkedProduct:
      _readLinkedProduct(json, 'linkedProduct') as Map<String, dynamic>?,
  publishStatus: _readPublishStatus(json, 'publishStatus') as String?,
  hiddenAt: (_readHiddenAt(json, 'hiddenAt') as num?)?.toInt(),
  scheduledPublishAt: _parseTimestamp(
    _readScheduledPublishAt(json, 'scheduledPublishAt'),
  ),
);

Map<String, dynamic> _$PostToJson(_Post instance) => <String, dynamic>{
  'id': instance.id,
  'sellerId': instance.sellerId,
  'title': instance.title,
  'content': instance.content,
  'tags': instance.tags,
  'contentType': instance.contentType,
  'imageKeys': instance.imageKeys,
  'videoUrl': instance.videoUrl,
  'likeCount': instance.likeCount,
  'commentCount': instance.commentCount,
  'shareCount': instance.shareCount,
  'isUnlocked': instance.isUnlocked,
  'unlockPrice': instance.unlockPrice,
  'createdAt': instance.createdAt?.toIso8601String(),
  'linkedProduct': instance.linkedProduct,
  'publishStatus': instance.publishStatus,
  'hiddenAt': instance.hiddenAt,
  'scheduledPublishAt': instance.scheduledPublishAt?.toIso8601String(),
};

_Comment _$CommentFromJson(Map<String, dynamic> json) => _Comment(
  id: (_readCommentId(json, 'id') as num).toInt(),
  postId: (_readCommentPostId(json, 'postId') as num?)?.toInt() ?? 0,
  isMine: json['isMine'] as bool? ?? false,
  author: _readCommentAuthor(json, 'author') as String? ?? '用戶',
  avatarKey: _readCommentAvatar(json, 'avatarKey') as String? ?? '',
  content: json['content'] as String? ?? '',
  createdAt: _parseTimestamp(_readTimestamp(json, 'createdAt')),
);

Map<String, dynamic> _$CommentToJson(_Comment instance) => <String, dynamic>{
  'id': instance.id,
  'postId': instance.postId,
  'isMine': instance.isMine,
  'author': instance.author,
  'avatarKey': instance.avatarKey,
  'content': instance.content,
  'createdAt': instance.createdAt?.toIso8601String(),
};
