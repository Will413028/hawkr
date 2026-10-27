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
  status: json['status'] as String? ?? '',
  reason: json['reason'] as String? ?? '',
  description: json['description'] as String? ?? '',
  refundableAmount: json['refundableAmount'] as String? ?? '',
  returnShipmentNo: json['returnShipmentNo'] as String? ?? '',
  returnShippingMethod: json['returnShippingMethod'] as String? ?? '',
  shipTradeNo: json['shipTradeNo'] as String? ?? '',
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
          ?.map((e) => AfterSaleEvidence.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  resolutions:
      (json['resolutions'] as List<dynamic>?)
          ?.map((e) => AfterSaleResolution.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  appealState: json['appealState'] as String? ?? '',
  canAppeal: json['canAppeal'] as bool? ?? false,
  processDeadline: json['processDeadline'] as String? ?? '',
  autoEscalated: json['autoEscalated'] as bool? ?? false,
  processDeadlineHours: (json['processDeadlineHours'] as num?)?.toInt() ?? 0,
  refundTaskStatus: json['refundTaskStatus'] as String? ?? '',
  refundFailureMessage: json['refundFailureMessage'] as String? ?? '',
  estimatedRefundDate: json['estimatedRefundDate'] as String? ?? '',
  bankDate: json['bankDate'] as String? ?? '',
  providerStatus: json['providerStatus'] as String? ?? '',
  providerMessage: json['providerMessage'] as String? ?? '',
  appealDeadline: json['appealDeadline'] as String? ?? '',
  partnerId: json['partnerId'] as String? ?? '',
  odno: json['odno'] as String? ?? '',
  validationNo: json['validationNo'] as String? ?? '',
  caseResolutionType: json['caseResolutionType'] as String? ?? '',
  shipBackDeadlineAt: json['shipBackDeadlineAt'] as String? ?? '',
);

Map<String, dynamic> _$AfterSaleCaseToJson(_AfterSaleCase instance) =>
    <String, dynamic>{
      'caseNo': instance.caseNo,
      'orderNo': instance.orderNo,
      'buyerId': instance.buyerId,
      'storeId': instance.storeId,
      'status': instance.status,
      'reason': instance.reason,
      'description': instance.description,
      'refundableAmount': instance.refundableAmount,
      'returnShipmentNo': instance.returnShipmentNo,
      'returnShippingMethod': instance.returnShippingMethod,
      'shipTradeNo': instance.shipTradeNo,
      'needRefundPage': instance.needRefundPage,
      'createdAt': instance.createdAt,
      'closedAt': instance.closedAt,
      'items': instance.items,
      'shipmentPhotoUrls': instance.shipmentPhotoUrls,
      'evidence': instance.evidence,
      'resolutions': instance.resolutions,
      'appealState': instance.appealState,
      'canAppeal': instance.canAppeal,
      'processDeadline': instance.processDeadline,
      'autoEscalated': instance.autoEscalated,
      'processDeadlineHours': instance.processDeadlineHours,
      'refundTaskStatus': instance.refundTaskStatus,
      'refundFailureMessage': instance.refundFailureMessage,
      'estimatedRefundDate': instance.estimatedRefundDate,
      'bankDate': instance.bankDate,
      'providerStatus': instance.providerStatus,
      'providerMessage': instance.providerMessage,
      'appealDeadline': instance.appealDeadline,
      'partnerId': instance.partnerId,
      'odno': instance.odno,
      'validationNo': instance.validationNo,
      'caseResolutionType': instance.caseResolutionType,
      'shipBackDeadlineAt': instance.shipBackDeadlineAt,
    };

_AfterSaleEvidence _$AfterSaleEvidenceFromJson(Map<String, dynamic> json) =>
    _AfterSaleEvidence(
      assetId: json['assetId'] as String? ?? '',
      imageUrl: json['imageUrl'] as String? ?? '',
      evidenceType: json['evidenceType'] as String? ?? '',
      createdAt: json['createdAt'] as String? ?? '',
    );

Map<String, dynamic> _$AfterSaleEvidenceToJson(_AfterSaleEvidence instance) =>
    <String, dynamic>{
      'assetId': instance.assetId,
      'imageUrl': instance.imageUrl,
      'evidenceType': instance.evidenceType,
      'createdAt': instance.createdAt,
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
      imageUrl: json['imageUrl'] as String? ?? '',
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
      'imageUrl': instance.imageUrl,
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
      deductionReason: json['deductionReason'] as String? ?? '',
      inspectionNote: json['inspectionNote'] as String? ?? '',
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
  'deductionReason': instance.deductionReason,
  'inspectionNote': instance.inspectionNote,
};
