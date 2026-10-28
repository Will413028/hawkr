// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'seller_quote_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_OfferedQuality _$OfferedQualityFromJson(Map<String, dynamic> json) =>
    _OfferedQuality(
      grade: json['grade'] as String? ?? '',
      inspectionNotes: json['inspectionNotes'] as String? ?? '',
    );

Map<String, dynamic> _$OfferedQualityToJson(_OfferedQuality instance) =>
    <String, dynamic>{
      'grade': instance.grade,
      'inspectionNotes': instance.inspectionNotes,
    };

_UnitBasis _$UnitBasisFromJson(Map<String, dynamic> json) => _UnitBasis(
  quantity: json['quantity'] as String? ?? '',
  unit: json['unit'] as String? ?? '',
);

Map<String, dynamic> _$UnitBasisToJson(_UnitBasis instance) =>
    <String, dynamic>{'quantity': instance.quantity, 'unit': instance.unit};

_SellerQuotePayload _$SellerQuotePayloadFromJson(Map<String, dynamic> json) =>
    _SellerQuotePayload(
      responseType: json['responseType'] as String? ?? '',
      offeredQuality: json['offeredQuality'] == null
          ? const OfferedQuality()
          : OfferedQuality.fromJson(
              json['offeredQuality'] as Map<String, dynamic>,
            ),
      unitBasis: json['unitBasis'] == null
          ? const UnitBasis()
          : UnitBasis.fromJson(json['unitBasis'] as Map<String, dynamic>),
      offeredQuantity: json['offeredQuantity'] as String? ?? '',
      offeredQuantityUnit: json['offeredQuantityUnit'] as String? ?? '',
      conversionBasis: json['conversionBasis'] as String? ?? '',
      currency: json['currency'] as String? ?? kQuoteCurrencyTwd,
      goodsAmount: json['goodsAmount'] as String? ?? '',
      shippingAmount: json['shippingAmount'] as String? ?? '',
      taxAmount: json['taxAmount'] as String? ?? '',
      taxInclusionBasis: json['taxInclusionBasis'] as String? ?? '',
      totalAmount: json['totalAmount'] as String? ?? '',
      leadTimeDays: json['leadTimeDays'] as String? ?? '',
      promisedDeliveryAt: json['promisedDeliveryAt'] as String? ?? '',
      validUntil: json['validUntil'] as String? ?? '',
      termsNote: json['termsNote'] as String? ?? '',
      deviationDisclosure: json['deviationDisclosure'] as String? ?? '',
    );

Map<String, dynamic> _$SellerQuotePayloadToJson(_SellerQuotePayload instance) =>
    <String, dynamic>{
      'responseType': instance.responseType,
      'offeredQuality': _offeredQualityToJson(instance.offeredQuality),
      'unitBasis': _unitBasisToJson(instance.unitBasis),
      'offeredQuantity': instance.offeredQuantity,
      'offeredQuantityUnit': instance.offeredQuantityUnit,
      'conversionBasis': instance.conversionBasis,
      'currency': instance.currency,
      'goodsAmount': instance.goodsAmount,
      'shippingAmount': instance.shippingAmount,
      'taxAmount': instance.taxAmount,
      'taxInclusionBasis': instance.taxInclusionBasis,
      'totalAmount': instance.totalAmount,
      'leadTimeDays': instance.leadTimeDays,
      'promisedDeliveryAt': instance.promisedDeliveryAt,
      'validUntil': instance.validUntil,
      'termsNote': instance.termsNote,
      'deviationDisclosure': instance.deviationDisclosure,
    };

_SellerResponseVersionResponse _$SellerResponseVersionResponseFromJson(
  Map<String, dynamic> json,
) => _SellerResponseVersionResponse(
  sellerResponseId: json['sellerResponseId'] as String? ?? '',
  sellerResponseVersion: json['sellerResponseVersion'] as String? ?? '',
  publicationId: json['publicationId'] as String? ?? '',
  status: json['status'] as String? ?? '',
  payloadHash: json['payloadHash'] as String? ?? '',
  submittedAt: json['submittedAt'] as String? ?? '',
  supersededByVersion: json['supersededByVersion'] as String? ?? '',
  outsideBuyerRequestedRange:
      json['outsideBuyerRequestedRange'] as bool? ?? false,
  quote: json['quote'] == null
      ? const SellerQuotePayload()
      : SellerQuotePayload.fromJson(json['quote'] as Map<String, dynamic>),
  replayed: json['replayed'] as bool? ?? false,
);

Map<String, dynamic> _$SellerResponseVersionResponseToJson(
  _SellerResponseVersionResponse instance,
) => <String, dynamic>{
  'sellerResponseId': instance.sellerResponseId,
  'sellerResponseVersion': instance.sellerResponseVersion,
  'publicationId': instance.publicationId,
  'status': instance.status,
  'payloadHash': instance.payloadHash,
  'submittedAt': instance.submittedAt,
  'supersededByVersion': instance.supersededByVersion,
  'outsideBuyerRequestedRange': instance.outsideBuyerRequestedRange,
  'quote': _quoteToJson(instance.quote),
  'replayed': instance.replayed,
};

_SellerResponseDraftResponse _$SellerResponseDraftResponseFromJson(
  Map<String, dynamic> json,
) => _SellerResponseDraftResponse(
  draftVersion: json['draftVersion'] as String? ?? '',
  updatedAt: json['updatedAt'] as String? ?? '',
  draft: json['draft'] == null
      ? const SellerQuotePayload()
      : SellerQuotePayload.fromJson(json['draft'] as Map<String, dynamic>),
);

Map<String, dynamic> _$SellerResponseDraftResponseToJson(
  _SellerResponseDraftResponse instance,
) => <String, dynamic>{
  'draftVersion': instance.draftVersion,
  'updatedAt': instance.updatedAt,
  'draft': _quoteToJson(instance.draft),
};

_SellerFacingPublication _$SellerFacingPublicationFromJson(
  Map<String, dynamic> json,
) => _SellerFacingPublication(
  publicationId: json['publicationId'] as String? ?? '',
  publicationVersion: json['publicationVersion'] as String? ?? '',
  inviteId: json['inviteId'] as String? ?? '',
  lane: json['lane'] as String? ?? '',
  sellerFacingPayloadHash: json['sellerFacingPayloadHash'] as String? ?? '',
  deliveryState: json['deliveryState'] as String? ?? '',
  publishedAt: json['publishedAt'] as String? ?? '',
);

Map<String, dynamic> _$SellerFacingPublicationToJson(
  _SellerFacingPublication instance,
) => <String, dynamic>{
  'publicationId': instance.publicationId,
  'publicationVersion': instance.publicationVersion,
  'inviteId': instance.inviteId,
  'lane': instance.lane,
  'sellerFacingPayloadHash': instance.sellerFacingPayloadHash,
  'deliveryState': instance.deliveryState,
  'publishedAt': instance.publishedAt,
};

_SellerFacingPayload _$SellerFacingPayloadFromJson(Map<String, dynamic> json) =>
    _SellerFacingPayload(
      item: json['item'] as String? ?? '',
      itemLabel: json['itemLabel'] as String? ?? '',
      quantity: json['quantity'] as String? ?? '',
      unit: json['unit'] as String? ?? '',
      packaging: json['packaging'] as String? ?? '',
      deliveryCity: json['deliveryCity'] as String? ?? '',
      deliveryDate: json['deliveryDate'] as String? ?? '',
      responseDeadline: json['responseDeadline'] as String? ?? '',
      buyerNote: json['buyerNote'] as String? ?? '',
    );

Map<String, dynamic> _$SellerFacingPayloadToJson(
  _SellerFacingPayload instance,
) => <String, dynamic>{
  'item': instance.item,
  'itemLabel': instance.itemLabel,
  'quantity': instance.quantity,
  'unit': instance.unit,
  'packaging': instance.packaging,
  'deliveryCity': instance.deliveryCity,
  'deliveryDate': instance.deliveryDate,
  'responseDeadline': instance.responseDeadline,
  'buyerNote': instance.buyerNote,
};
