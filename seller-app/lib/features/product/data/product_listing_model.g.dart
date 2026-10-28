// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'product_listing_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_PendingAiDraft _$PendingAiDraftFromJson(Map<String, dynamic> json) =>
    _PendingAiDraft(
      name: json['name'] as String?,
      origin: json['origin'] as String?,
      categoryId: json['categoryId'] as String?,
      description: json['description'] as String?,
      tags:
          (json['tags'] as List<dynamic>?)?.map((e) => e as String).toList() ??
          const [],
      skus:
          (json['skus'] as List<dynamic>?)
              ?.map((e) => SpecDraftPending.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const [],
      filledFields:
          (json['filledFields'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toSet() ??
          const {},
      patchVersion: (json['patchVersion'] as num?)?.toInt() ?? 0,
    );

Map<String, dynamic> _$PendingAiDraftToJson(_PendingAiDraft instance) =>
    <String, dynamic>{
      'name': instance.name,
      'origin': instance.origin,
      'categoryId': instance.categoryId,
      'description': instance.description,
      'tags': instance.tags,
      'skus': instance.skus,
      'filledFields': instance.filledFields.toList(),
      'patchVersion': instance.patchVersion,
    };

_SpecDraftPending _$SpecDraftPendingFromJson(Map<String, dynamic> json) =>
    _SpecDraftPending(
      skuName: json['skuName'] as String,
      price: (json['price'] as num?)?.toInt() ?? 0,
      salePrice: (json['salePrice'] as num?)?.toInt() ?? 0,
      unit: json['unit'] as String? ?? '',
      weight: (json['weight'] as num?)?.toInt() ?? 0,
      stockQty: (json['stockQty'] as num?)?.toInt() ?? 0,
    );

Map<String, dynamic> _$SpecDraftPendingToJson(_SpecDraftPending instance) =>
    <String, dynamic>{
      'skuName': instance.skuName,
      'price': instance.price,
      'salePrice': instance.salePrice,
      'unit': instance.unit,
      'weight': instance.weight,
      'stockQty': instance.stockQty,
    };
