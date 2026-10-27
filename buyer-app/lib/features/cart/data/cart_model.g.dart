// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cart_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_CartItem _$CartItemFromJson(Map<String, dynamic> json) => _CartItem(
  id: (json['id'] as num).toInt(),
  skuId: (json['skuId'] as num).toInt(),
  count: (json['count'] as num).toInt(),
  price: (json['price'] as num).toInt(),
  listPrice: (json['listPrice'] as num?)?.toInt(),
  title: json['title'] as String?,
  image: json['image'] as String?,
  sellerId: (json['sellerId'] as num?)?.toInt(),
  merchantName: json['merchantName'] as String?,
  specName: json['specName'] as String?,
  check: json['check'] as bool?,
  userId: json['userId'] as String?,
  userKey: json['userKey'] as String?,
  stockQty: json['stockQty'] as String?,
  purchasable: json['purchasable'] as bool? ?? true,
  unavailableReason: json['unavailableReason'] as String?,
);

Map<String, dynamic> _$CartItemToJson(_CartItem instance) => <String, dynamic>{
  'id': instance.id,
  'skuId': instance.skuId,
  'count': instance.count,
  'price': instance.price,
  'listPrice': instance.listPrice,
  'title': instance.title,
  'image': instance.image,
  'sellerId': instance.sellerId,
  'merchantName': instance.merchantName,
  'specName': instance.specName,
  'check': instance.check,
  'userId': instance.userId,
  'userKey': instance.userKey,
  'stockQty': instance.stockQty,
  'purchasable': instance.purchasable,
  'unavailableReason': instance.unavailableReason,
};

_CartSummary _$CartSummaryFromJson(Map<String, dynamic> json) => _CartSummary(
  items:
      (json['items'] as List<dynamic>?)
          ?.map((e) => CartItem.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  totalAmount: (json['totalAmount'] as num?)?.toInt() ?? 0,
);

Map<String, dynamic> _$CartSummaryToJson(_CartSummary instance) =>
    <String, dynamic>{
      'items': instance.items,
      'totalAmount': instance.totalAmount,
    };
