// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'wallet_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_Coupon _$CouponFromJson(Map<String, dynamic> json) => _Coupon(
  id: json['id'] as String,
  name: json['name'] as String,
  status: (json['status'] as num?)?.toInt(),
  createdAt: json['createdAt'] as String?,
  discountAmount: json['discountAmount'] as num?,
  discountPercent: json['discountPercent'] as num?,
  maxDiscountAmount: json['maxDiscountAmount'] as num?,
  thresholdAmount: json['thresholdAmount'] as num?,
  usageLimitPerAccount: json['usageLimitPerAccount'] as num?,
  startAt: json['startAt'] as String?,
  endAt: json['endAt'] as String?,
  sellerName: json['sellerName'] as String?,
  storeId: json['storeId'] as String?,
  sellerAvatarUrl: json['sellerAvatarUrl'] as String?,
);

Map<String, dynamic> _$CouponToJson(_Coupon instance) => <String, dynamic>{
  'id': instance.id,
  'name': instance.name,
  'status': instance.status,
  'createdAt': instance.createdAt,
  'discountAmount': instance.discountAmount,
  'discountPercent': instance.discountPercent,
  'maxDiscountAmount': instance.maxDiscountAmount,
  'thresholdAmount': instance.thresholdAmount,
  'usageLimitPerAccount': instance.usageLimitPerAccount,
  'startAt': instance.startAt,
  'endAt': instance.endAt,
  'sellerName': instance.sellerName,
  'storeId': instance.storeId,
  'sellerAvatarUrl': instance.sellerAvatarUrl,
};
