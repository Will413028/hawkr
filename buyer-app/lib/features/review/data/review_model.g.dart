// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'review_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_Review _$ReviewFromJson(Map<String, dynamic> json) => _Review(
  id: (json['id'] as num).toInt(),
  skuId: (json['skuId'] as num).toInt(),
  orderItemId: (json['orderItemId'] as num?)?.toInt(),
  rating: (json['rating'] as num).toInt(),
  content: json['content'] as String,
  images:
      (json['images'] as List<dynamic>?)?.map((e) => e as String).toList() ??
      const [],
  userId: (json['userId'] as num?)?.toInt(),
  username: json['username'] as String?,
  avatarKey: json['avatarKey'] as String?,
  createdAt: json['createdAt'] as String?,
);

Map<String, dynamic> _$ReviewToJson(_Review instance) => <String, dynamic>{
  'id': instance.id,
  'skuId': instance.skuId,
  'orderItemId': instance.orderItemId,
  'rating': instance.rating,
  'content': instance.content,
  'images': instance.images,
  'userId': instance.userId,
  'username': instance.username,
  'avatarKey': instance.avatarKey,
  'createdAt': instance.createdAt,
};
