// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'customer_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_CustomerTag _$CustomerTagFromJson(Map<String, dynamic> json) =>
    _CustomerTag(id: (json['id'] as num).toInt(), name: json['name'] as String);

Map<String, dynamic> _$CustomerTagToJson(_CustomerTag instance) =>
    <String, dynamic>{'id': instance.id, 'name': instance.name};

_Customer _$CustomerFromJson(Map<String, dynamic> json) => _Customer(
  id: json['id'] as String,
  name: json['name'] as String,
  phone: json['phone'] as String,
  totalSpent: (json['totalSpent'] as num?)?.toInt() ?? 0,
  visitCount: (json['visitCount'] as num?)?.toInt() ?? 0,
  couponCount: (json['couponCount'] as num?)?.toInt() ?? 0,
  isVip: json['isVip'] as bool? ?? false,
  isBlacklisted: json['isBlacklisted'] as bool? ?? false,
  customerType: json['customerType'] as String? ?? 'normal',
  avatarUrl: json['avatarUrl'] as String?,
  address: json['address'] as String?,
  email: json['email'] as String?,
  frequentItems:
      (json['frequentItems'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList() ??
      const [],
  tags:
      (json['tags'] as List<dynamic>?)
          ?.map((e) => CustomerTag.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
);

Map<String, dynamic> _$CustomerToJson(_Customer instance) => <String, dynamic>{
  'id': instance.id,
  'name': instance.name,
  'phone': instance.phone,
  'totalSpent': instance.totalSpent,
  'visitCount': instance.visitCount,
  'couponCount': instance.couponCount,
  'isVip': instance.isVip,
  'isBlacklisted': instance.isBlacklisted,
  'customerType': instance.customerType,
  'avatarUrl': instance.avatarUrl,
  'address': instance.address,
  'email': instance.email,
  'frequentItems': instance.frequentItems,
  'tags': instance.tags,
};

_SpendingRecord _$SpendingRecordFromJson(Map<String, dynamic> json) =>
    _SpendingRecord(
      date: json['date'] as String,
      orderId: json['orderId'] as String,
      amount: (json['amount'] as num).toInt(),
      refund: (json['refund'] as num).toInt(),
      discountAmount: (json['discountAmount'] as num?)?.toInt() ?? 0,
    );

Map<String, dynamic> _$SpendingRecordToJson(_SpendingRecord instance) =>
    <String, dynamic>{
      'date': instance.date,
      'orderId': instance.orderId,
      'amount': instance.amount,
      'refund': instance.refund,
      'discountAmount': instance.discountAmount,
    };

_OrderRecord _$OrderRecordFromJson(Map<String, dynamic> json) => _OrderRecord(
  orderId: json['orderId'] as String,
  date: json['date'] as String,
  items: json['items'] as String,
  amount: (json['amount'] as num).toInt(),
  status: json['status'] == null
      ? OrderStatus.completed
      : _parseOrderStatus(json['status']),
  rating: (json['rating'] as num?)?.toInt() ?? 0,
  review: json['review'] as String? ?? '',
);

Map<String, dynamic> _$OrderRecordToJson(_OrderRecord instance) =>
    <String, dynamic>{
      'orderId': instance.orderId,
      'date': instance.date,
      'items': instance.items,
      'amount': instance.amount,
      'status': _$OrderStatusEnumMap[instance.status]!,
      'rating': instance.rating,
      'review': instance.review,
    };

const _$OrderStatusEnumMap = {
  OrderStatus.delivering: 'delivering',
  OrderStatus.completed: 'completed',
  OrderStatus.cancelled: 'cancelled',
  OrderStatus.issue: 'issue',
};
