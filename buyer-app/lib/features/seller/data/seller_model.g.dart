// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'seller_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_RatingStats _$RatingStatsFromJson(Map<String, dynamic> json) => _RatingStats(
  storeId: json['storeId'] as String?,
  average: (json['rating'] as num?)?.toDouble(),
  total: (json['reviewCount'] as num?)?.toInt(),
  fiveStarPct: (json['fiveStarPct'] as num?)?.toDouble(),
  fourStarPct: (json['fourStarPct'] as num?)?.toDouble(),
  threeStarPct: (json['threeStarPct'] as num?)?.toDouble(),
  twoStarPct: (json['twoStarPct'] as num?)?.toDouble(),
  oneStarPct: (json['oneStarPct'] as num?)?.toDouble(),
);

Map<String, dynamic> _$RatingStatsToJson(_RatingStats instance) =>
    <String, dynamic>{
      'storeId': instance.storeId,
      'rating': instance.average,
      'reviewCount': instance.total,
      'fiveStarPct': instance.fiveStarPct,
      'fourStarPct': instance.fourStarPct,
      'threeStarPct': instance.threeStarPct,
      'twoStarPct': instance.twoStarPct,
      'oneStarPct': instance.oneStarPct,
    };
