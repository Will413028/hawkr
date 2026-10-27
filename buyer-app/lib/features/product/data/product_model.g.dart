// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'product_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_Product _$ProductFromJson(Map<String, dynamic> json) => _Product(
  skuId: (json['skuId'] as num).toInt(),
  title: json['title'] as String,
  price: (json['price'] as num).toInt(),
  image: json['image'] as String,
  categoryId: (json['categoryId'] as num?)?.toInt(),
  sellerId: (json['sellerId'] as num?)?.toInt(),
  merchantName: json['merchantName'] as String?,
  stock: (json['stock'] as num?)?.toInt(),
  isFavorited: json['isFavorited'] as bool? ?? false,
  isRestockSubscribed: json['isRestockSubscribed'] as bool? ?? false,
  selfOwned: json['selfOwned'] as bool? ?? false,
);

Map<String, dynamic> _$ProductToJson(_Product instance) => <String, dynamic>{
  'skuId': instance.skuId,
  'title': instance.title,
  'price': instance.price,
  'image': instance.image,
  'categoryId': instance.categoryId,
  'sellerId': instance.sellerId,
  'merchantName': instance.merchantName,
  'stock': instance.stock,
  'isFavorited': instance.isFavorited,
  'isRestockSubscribed': instance.isRestockSubscribed,
  'selfOwned': instance.selfOwned,
};

_SearchSuggest _$SearchSuggestFromJson(Map<String, dynamic> json) =>
    _SearchSuggest(
      suggestions:
          (json['suggestions'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          const [],
    );

Map<String, dynamic> _$SearchSuggestToJson(_SearchSuggest instance) =>
    <String, dynamic>{'suggestions': instance.suggestions};
