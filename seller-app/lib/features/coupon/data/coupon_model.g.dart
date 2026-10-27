// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'coupon_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_Coupon _$CouponFromJson(Map<String, dynamic> json) => _Coupon(
  templateId: json['templateId'] as String,
  instanceId: json['instanceId'] as String? ?? '',
  title: json['title'] as String? ?? '',
  type: json['type'] as String? ?? 'CASH',
  status: json['status'] as String? ?? '',
  discountAmount: json['discountAmount'] as String?,
  discountPercent: json['discountPercent'] as String?,
  thresholdAmount: json['thresholdAmount'] as String?,
  maxDiscountAmount: json['maxDiscountAmount'] as String?,
  startAt: json['startAt'] as String?,
  endAt: json['endAt'] as String?,
  issuedCount: json['issuedCount'] as String?,
  totalIssueLimit: json['totalIssueLimit'] as String?,
  usageLimitPerAccount: json['usageLimitPerAccount'] as String?,
  issuerAccountId: json['issuerAccountId'] as String?,
  issuerStoreId: json['issuerStoreId'] as String?,
  issuerType: json['issuerType'] as String?,
  applicableStoreId: json['applicableStoreId'] as String?,
  sellerName: json['sellerName'] as String?,
  createdAt: json['createdAt'] as String?,
);

Map<String, dynamic> _$CouponToJson(_Coupon instance) => <String, dynamic>{
  'templateId': instance.templateId,
  'instanceId': instance.instanceId,
  'title': instance.title,
  'type': instance.type,
  'status': instance.status,
  'discountAmount': instance.discountAmount,
  'discountPercent': instance.discountPercent,
  'thresholdAmount': instance.thresholdAmount,
  'maxDiscountAmount': instance.maxDiscountAmount,
  'startAt': instance.startAt,
  'endAt': instance.endAt,
  'issuedCount': instance.issuedCount,
  'totalIssueLimit': instance.totalIssueLimit,
  'usageLimitPerAccount': instance.usageLimitPerAccount,
  'issuerAccountId': instance.issuerAccountId,
  'issuerStoreId': instance.issuerStoreId,
  'issuerType': instance.issuerType,
  'applicableStoreId': instance.applicableStoreId,
  'sellerName': instance.sellerName,
  'createdAt': instance.createdAt,
};

_CustomerLookupResult _$CustomerLookupResultFromJson(
  Map<String, dynamic> json,
) => _CustomerLookupResult(
  uid: json['uid'] as String,
  buyerName: json['buyerName'] as String? ?? '',
  phone: json['phone'] as String? ?? '',
  avatarUrl: json['avatarUrl'] as String?,
);

Map<String, dynamic> _$CustomerLookupResultToJson(
  _CustomerLookupResult instance,
) => <String, dynamic>{
  'uid': instance.uid,
  'buyerName': instance.buyerName,
  'phone': instance.phone,
  'avatarUrl': instance.avatarUrl,
};
