// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'post_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

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
