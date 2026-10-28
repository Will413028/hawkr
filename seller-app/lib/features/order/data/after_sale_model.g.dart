// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'after_sale_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_AfterSaleCase _$AfterSaleCaseFromJson(
  Map<String, dynamic> json,
) => _AfterSaleCase(
  caseNo: json['caseNo'] as String? ?? '',
  orderNo: json['orderNo'] as String? ?? '',
  buyerId: json['buyerId'] as String? ?? '',
  storeId: json['storeId'] as String? ?? '',
  buyerName: json['buyerName'] as String? ?? '',
  avatarUrl: json['avatarUrl'] as String? ?? '',
  shippingAddress: json['shippingAddress'] as String? ?? '',
  status: json['status'] as String? ?? '',
  reason: json['reason'] as String? ?? '',
  description: json['description'] as String? ?? '',
  refundableAmount: json['refundableAmount'] as String? ?? '',
  returnShipmentNo: json['returnShipmentNo'] as String? ?? '',
  returnShippingMethod: json['returnShippingMethod'] as String? ?? '',
  shipTradeNo: json['shipTradeNo'] as String?,
  providerStatus: _readCaseValue(json, 'providerStatus') as String? ?? '',
  providerMessage: _readCaseValue(json, 'providerMessage') as String? ?? '',
  refundTaskStatus: _readCaseValue(json, 'refundTaskStatus') as String? ?? '',
  refundFailureMessage:
      _readCaseValue(json, 'refundFailureMessage') as String? ?? '',
  estimatedRefundDate:
      _readCaseValue(json, 'estimatedRefundDate') as String? ?? '',
  bankDate: _readCaseValue(json, 'bankDate') as String? ?? '',
  odno: json['odno'] as String?,
  validationNo: json['validationNo'] as String?,
  needRefundPage: json['needRefundPage'] as bool? ?? false,
  createdAt: json['createdAt'] as String? ?? '',
  closedAt: json['closedAt'] as String? ?? '',
  items:
      (json['items'] as List<dynamic>?)
          ?.map((e) => AfterSaleItem.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  shipmentPhotoUrls:
      (json['shipmentPhotoUrls'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList() ??
      const [],
  evidence:
      (json['evidence'] as List<dynamic>?)
          ?.map((e) => e as Map<String, dynamic>)
          .toList() ??
      const [],
  resolutions:
      (json['resolutions'] as List<dynamic>?)
          ?.map((e) => AfterSaleResolution.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  processDeadline: json['processDeadline'] as String? ?? '',
  autoEscalated: json['autoEscalated'] as bool? ?? false,
  processDeadlineHours: (json['processDeadlineHours'] as num?)?.toInt() ?? 0,
  appealState: json['appealState'] as String? ?? '',
);

Map<String, dynamic> _$AfterSaleCaseToJson(_AfterSaleCase instance) =>
    <String, dynamic>{
      'caseNo': instance.caseNo,
      'orderNo': instance.orderNo,
      'buyerId': instance.buyerId,
      'storeId': instance.storeId,
      'buyerName': instance.buyerName,
      'avatarUrl': instance.avatarUrl,
      'shippingAddress': instance.shippingAddress,
      'status': instance.status,
      'reason': instance.reason,
      'description': instance.description,
      'refundableAmount': instance.refundableAmount,
      'returnShipmentNo': instance.returnShipmentNo,
      'returnShippingMethod': instance.returnShippingMethod,
      'shipTradeNo': instance.shipTradeNo,
      'providerStatus': instance.providerStatus,
      'providerMessage': instance.providerMessage,
      'refundTaskStatus': instance.refundTaskStatus,
      'refundFailureMessage': instance.refundFailureMessage,
      'estimatedRefundDate': instance.estimatedRefundDate,
      'bankDate': instance.bankDate,
      'odno': instance.odno,
      'validationNo': instance.validationNo,
      'needRefundPage': instance.needRefundPage,
      'createdAt': instance.createdAt,
      'closedAt': instance.closedAt,
      'items': instance.items,
      'shipmentPhotoUrls': instance.shipmentPhotoUrls,
      'evidence': instance.evidence,
      'resolutions': instance.resolutions,
      'processDeadline': instance.processDeadline,
      'autoEscalated': instance.autoEscalated,
      'processDeadlineHours': instance.processDeadlineHours,
      'appealState': instance.appealState,
    };

_AfterSaleItem _$AfterSaleItemFromJson(Map<String, dynamic> json) =>
    _AfterSaleItem(
      orderItemId: json['orderItemId'] as String? ?? '',
      skuId: json['skuId'] as String? ?? '',
      skuName: json['skuName'] as String? ?? '',
      quantity: json['quantity'] as String? ?? '',
      unitPrice: json['unitPrice'] as String? ?? '',
      listPrice: json['listPrice'] as String? ?? '',
      originalPrice: json['originalPrice'] as String? ?? '',
      actualPayAmount: json['actualPayAmount'] as String? ?? '',
      shareDiscount: json['shareDiscount'] as String? ?? '',
      imageKey: json['imageKey'] as String? ?? '',
    );

Map<String, dynamic> _$AfterSaleItemToJson(_AfterSaleItem instance) =>
    <String, dynamic>{
      'orderItemId': instance.orderItemId,
      'skuId': instance.skuId,
      'skuName': instance.skuName,
      'quantity': instance.quantity,
      'unitPrice': instance.unitPrice,
      'listPrice': instance.listPrice,
      'originalPrice': instance.originalPrice,
      'actualPayAmount': instance.actualPayAmount,
      'shareDiscount': instance.shareDiscount,
      'imageKey': instance.imageKey,
    };

_AfterSaleResolution _$AfterSaleResolutionFromJson(Map<String, dynamic> json) =>
    _AfterSaleResolution(
      resolutionType: json['resolutionType'] as String? ?? '',
      refundAmount: json['refundAmount'] as String? ?? '',
      couponAmount: json['couponAmount'] as String? ?? '',
      couponMinSpend: json['couponMinSpend'] as String? ?? '',
      couponMessage: json['couponMessage'] as String? ?? '',
      rejectReason: json['rejectReason'] as String? ?? '',
      rejectMessage: json['rejectMessage'] as String? ?? '',
      negotiationReason: json['negotiationReason'] as String? ?? '',
      createdAt: json['createdAt'] as String? ?? '',
    );

Map<String, dynamic> _$AfterSaleResolutionToJson(
  _AfterSaleResolution instance,
) => <String, dynamic>{
  'resolutionType': instance.resolutionType,
  'refundAmount': instance.refundAmount,
  'couponAmount': instance.couponAmount,
  'couponMinSpend': instance.couponMinSpend,
  'couponMessage': instance.couponMessage,
  'rejectReason': instance.rejectReason,
  'rejectMessage': instance.rejectMessage,
  'negotiationReason': instance.negotiationReason,
  'createdAt': instance.createdAt,
};
