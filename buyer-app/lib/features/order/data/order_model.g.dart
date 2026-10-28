// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'order_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_OrderItem _$OrderItemFromJson(Map<String, dynamic> json) => _OrderItem(
  id: json['id'] as String? ?? '',
  productId: json['productId'] as String? ?? '',
  productName: json['productName'] as String? ?? '',
  skuId: json['skuId'] as String? ?? '',
  skuName: json['skuName'] as String? ?? '',
  quantity: json['quantity'] as String? ?? '',
  unitPrice: json['unitPrice'] as String? ?? '',
  listPrice: json['listPrice'] as String? ?? '',
  originalPrice: json['originalPrice'] as String? ?? '',
  actualPayAmount: json['actualPayAmount'] as String? ?? '',
  refundStatus: json['refundStatus'] as String? ?? '',
  shareDiscount: json['shareDiscount'] as String? ?? '',
  imageUrl: json['imageUrl'] as String? ?? '',
);

Map<String, dynamic> _$OrderItemToJson(_OrderItem instance) =>
    <String, dynamic>{
      'id': instance.id,
      'productId': instance.productId,
      'productName': instance.productName,
      'skuId': instance.skuId,
      'skuName': instance.skuName,
      'quantity': instance.quantity,
      'unitPrice': instance.unitPrice,
      'listPrice': instance.listPrice,
      'originalPrice': instance.originalPrice,
      'actualPayAmount': instance.actualPayAmount,
      'refundStatus': instance.refundStatus,
      'shareDiscount': instance.shareDiscount,
      'imageUrl': instance.imageUrl,
    };

_RefundRecord _$RefundRecordFromJson(Map<String, dynamic> json) =>
    _RefundRecord(
      requestNo: json['requestNo'] as String? ?? '',
      orderItemId: json['orderItemId'] as String? ?? '',
      status: json['status'] as String? ?? '',
      refundAmount: json['refundAmount'] as String? ?? '',
      reason: json['reason'] as String? ?? '',
      createTime: json['createTime'] as String? ?? '',
    );

Map<String, dynamic> _$RefundRecordToJson(_RefundRecord instance) =>
    <String, dynamic>{
      'requestNo': instance.requestNo,
      'orderItemId': instance.orderItemId,
      'status': instance.status,
      'refundAmount': instance.refundAmount,
      'reason': instance.reason,
      'createTime': instance.createTime,
    };

_Order _$OrderFromJson(Map<String, dynamic> json) => _Order(
  id: (json['id'] as num?)?.toInt() ?? 0,
  orderSn: json['orderNo'] as String? ?? '',
  status: json['status'] as String? ?? '',
  totalOriginalAmount: json['totalOriginalAmount'] as String? ?? '',
  totalActualAmount: json['totalActualAmount'] as String? ?? '',
  createTime: json['createTime'] as String? ?? '',
  payTime: json['payTime'] as String? ?? '',
  shipTime: json['shipTime'] as String? ?? '',
  shipmentNo: json['shipmentNo'] as String? ?? '',
  transactionId: json['transactionId'] as String? ?? '',
  buyerId: json['buyerId'] as String? ?? '',
  shippingMethod: json['shippingMethod'] as String? ?? '',
  shippingFee: json['shippingFee'] as String? ?? '',
  shippingFeePayer: json['shippingFeePayer'] as String? ?? '',
  shippingArea: json['shippingArea'] as String? ?? '',
  couponId: json['couponId'] as String? ?? '',
  discountAmount: json['discountAmount'] as String? ?? '',
  shippingStatus: json['shippingStatus'] as String? ?? '',
  statusTime: json['statusTime'] as String? ?? '',
  statusSummary: json['statusSummary'] as String? ?? '',
  autoCompleteAt: json['autoCompleteAt'] as String? ?? '',
  processingDeadline: json['processingDeadline'] as String? ?? '',
  labelUrl: json['labelUrl'] as String? ?? '',
  recipientName: json['recipientName'] as String? ?? '',
  recipientPhone: json['recipientPhone'] as String? ?? '',
  shippingAddress: json['shippingAddress'] as String? ?? '',
  shippingNote: json['shippingNote'] as String? ?? '',
  orderRemark: json['orderRemark'] as String? ?? '',
  partnerId: json['partnerId'] as String? ?? '',
  odno: json['odno'] as String? ?? '',
  validationNo: json['validationNo'] as String? ?? '',
  paymentMethod: json['paymentMethod'] as String? ?? '',
  atmBankCode: json['atmBankCode'] as String? ?? '',
  atmVirtualAccount: json['atmVirtualAccount'] as String? ?? '',
  atmExpireDate: json['atmExpireDate'] as String? ?? '',
  paymentDeadline: json['paymentDeadline'] as String? ?? '',
  itemCount: json['itemCount'] as String? ?? '',
  sellerName: json['sellerName'] as String? ?? '',
  sellerAvatarUrl: json['logoUrl'] as String? ?? '',
  sellerCity: json['sellerCity'] as String? ?? '',
  sellerDistrict: json['sellerDistrict'] as String? ?? '',
  pickupStoreName: json['pickupStoreName'] as String? ?? '',
  storeId: json['storeId'] as String? ?? '',
  isReviewed: json['isReviewed'] as bool? ?? false,
  canReview: json['canReview'] as bool? ?? true,
  reviewDeadline: json['reviewDeadline'] as String? ?? '',
  reviewRating: (json['reviewRating'] as num?)?.toInt() ?? 0,
  reviewContent: json['reviewContent'] as String? ?? '',
  cancelReason: json['cancelReason'] as String? ?? '',
  cancelRemark: json['cancelRemark'] as String? ?? '',
  cancelByRole: json['cancelByRole'] as String? ?? '',
  needRefundPage: json['needRefundPage'] as bool? ?? false,
  shipmentPhotoUrls:
      (json['shipmentPhotoUrls'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList() ??
      const <String>[],
  receiptPhotoUrls:
      (json['receiptPhotoUrls'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList() ??
      const <String>[],
  availablePaymentMethods:
      (json['availablePaymentMethods'] as List<dynamic>?)
          ?.map((e) => e as Map<String, dynamic>)
          .toList() ??
      const <Map<String, dynamic>>[],
  items:
      (json['items'] as List<dynamic>?)
          ?.map((e) => OrderItem.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
  refundHistory:
      (json['refundHistory'] as List<dynamic>?)
          ?.map((e) => RefundRecord.fromJson(e as Map<String, dynamic>))
          .toList() ??
      const [],
);

Map<String, dynamic> _$OrderToJson(_Order instance) => <String, dynamic>{
  'id': instance.id,
  'orderNo': instance.orderSn,
  'status': instance.status,
  'totalOriginalAmount': instance.totalOriginalAmount,
  'totalActualAmount': instance.totalActualAmount,
  'createTime': instance.createTime,
  'payTime': instance.payTime,
  'shipTime': instance.shipTime,
  'shipmentNo': instance.shipmentNo,
  'transactionId': instance.transactionId,
  'buyerId': instance.buyerId,
  'shippingMethod': instance.shippingMethod,
  'shippingFee': instance.shippingFee,
  'shippingFeePayer': instance.shippingFeePayer,
  'shippingArea': instance.shippingArea,
  'couponId': instance.couponId,
  'discountAmount': instance.discountAmount,
  'shippingStatus': instance.shippingStatus,
  'statusTime': instance.statusTime,
  'statusSummary': instance.statusSummary,
  'autoCompleteAt': instance.autoCompleteAt,
  'processingDeadline': instance.processingDeadline,
  'labelUrl': instance.labelUrl,
  'recipientName': instance.recipientName,
  'recipientPhone': instance.recipientPhone,
  'shippingAddress': instance.shippingAddress,
  'shippingNote': instance.shippingNote,
  'orderRemark': instance.orderRemark,
  'partnerId': instance.partnerId,
  'odno': instance.odno,
  'validationNo': instance.validationNo,
  'paymentMethod': instance.paymentMethod,
  'atmBankCode': instance.atmBankCode,
  'atmVirtualAccount': instance.atmVirtualAccount,
  'atmExpireDate': instance.atmExpireDate,
  'paymentDeadline': instance.paymentDeadline,
  'itemCount': instance.itemCount,
  'sellerName': instance.sellerName,
  'logoUrl': instance.sellerAvatarUrl,
  'sellerCity': instance.sellerCity,
  'sellerDistrict': instance.sellerDistrict,
  'pickupStoreName': instance.pickupStoreName,
  'storeId': instance.storeId,
  'isReviewed': instance.isReviewed,
  'canReview': instance.canReview,
  'reviewDeadline': instance.reviewDeadline,
  'reviewRating': instance.reviewRating,
  'reviewContent': instance.reviewContent,
  'cancelReason': instance.cancelReason,
  'cancelRemark': instance.cancelRemark,
  'cancelByRole': instance.cancelByRole,
  'needRefundPage': instance.needRefundPage,
  'shipmentPhotoUrls': instance.shipmentPhotoUrls,
  'receiptPhotoUrls': instance.receiptPhotoUrls,
  'availablePaymentMethods': instance.availablePaymentMethods,
  'items': instance.items,
  'refundHistory': instance.refundHistory,
};
