// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'follow_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_FansCount _$FansCountFromJson(Map<String, dynamic> json) => _FansCount(
  merchantId: (json['merchantId'] as num?)?.toInt(),
  count: (json['count'] as num?)?.toInt() ?? 0,
);

Map<String, dynamic> _$FansCountToJson(_FansCount instance) =>
    <String, dynamic>{
      'merchantId': instance.merchantId,
      'count': instance.count,
    };
