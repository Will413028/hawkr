// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'hashtag_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_HashtagItem _$HashtagItemFromJson(Map<String, dynamic> json) => _HashtagItem(
  name: json['name'] as String,
  displayName: json['displayName'] as String? ?? '',
  productCount: (json['productCount'] as num?)?.toInt() ?? 0,
);

Map<String, dynamic> _$HashtagItemToJson(_HashtagItem instance) =>
    <String, dynamic>{
      'name': instance.name,
      'displayName': instance.displayName,
      'productCount': instance.productCount,
    };
