// ignore_for_file: invalid_annotation_target

import 'package:freezed_annotation/freezed_annotation.dart';

import '../../../core/codemeta/code_meta.dart';

part 'seller_quote_model.freezed.dart';
part 'seller_quote_model.g.dart';

const String kQuoteCurrencyTwd = 'TWD';

Map<String, dynamic> _offeredQualityToJson(OfferedQuality value) =>
    value.toJson();
Map<String, dynamic> _unitBasisToJson(UnitBasis value) => value.toJson();
Map<String, dynamic> _quoteToJson(SellerQuotePayload value) => value.toJson();

@freezed
abstract class OfferedQuality with _$OfferedQuality {
  const factory OfferedQuality({
    @Default('') String grade,
    @Default('') String inspectionNotes,
  }) = _OfferedQuality;

  factory OfferedQuality.fromJson(Map<String, dynamic> json) =>
      _$OfferedQualityFromJson(json);
}

@freezed
abstract class UnitBasis with _$UnitBasis {
  const factory UnitBasis({
    @Default('') String quantity,
    @Default('') String unit,
  }) = _UnitBasis;

  factory UnitBasis.fromJson(Map<String, dynamic> json) =>
      _$UnitBasisFromJson(json);
}

@freezed
abstract class SellerQuotePayload with _$SellerQuotePayload {
  const factory SellerQuotePayload({
    @Default('') String responseType,

    @JsonKey(toJson: _offeredQualityToJson)
    @Default(OfferedQuality())
    OfferedQuality offeredQuality,

    @JsonKey(toJson: _unitBasisToJson)
    @Default(UnitBasis())
    UnitBasis unitBasis,

    @Default('') String offeredQuantity,

    @Default('') String offeredQuantityUnit,

    @Default('') String conversionBasis,

    @Default(kQuoteCurrencyTwd) String currency,

    @Default('') String goodsAmount,

    @Default('') String shippingAmount,

    @Default('') String taxAmount,

    @Default('') String taxInclusionBasis,

    @Default('') String totalAmount,

    @Default('') String leadTimeDays,

    @Default('') String promisedDeliveryAt,

    @Default('') String validUntil,

    @Default('') String termsNote,

    @Default('') String deviationDisclosure,
  }) = _SellerQuotePayload;

  const SellerQuotePayload._();

  factory SellerQuotePayload.fromJson(Map<String, dynamic> json) =>
      _$SellerQuotePayloadFromJson(json);

  ProcurementSellerResponseType? get responseTypeValue =>
      ProcurementSellerResponseType.fromCode(responseType);

  ProcurementTaxInclusionBasis? get taxInclusionBasisValue =>
      ProcurementTaxInclusionBasis.fromCode(taxInclusionBasis);
}

@freezed
abstract class SellerResponseVersionResponse
    with _$SellerResponseVersionResponse {
  const factory SellerResponseVersionResponse({
    @Default('') String sellerResponseId,
    @Default('') String sellerResponseVersion,
    @Default('') String publicationId,

    @Default('') String status,
    @Default('') String payloadHash,
    @Default('') String submittedAt,

    @Default('') String supersededByVersion,

    @Default(false) bool outsideBuyerRequestedRange,
    @JsonKey(toJson: _quoteToJson)
    @Default(SellerQuotePayload())
    SellerQuotePayload quote,
    @Default(false) bool replayed,
  }) = _SellerResponseVersionResponse;

  const SellerResponseVersionResponse._();

  factory SellerResponseVersionResponse.fromJson(Map<String, dynamic> json) =>
      _$SellerResponseVersionResponseFromJson(json);

  ProcurementSellerResponseStatus? get statusValue =>
      ProcurementSellerResponseStatus.fromCode(status);

  bool get isLatestVersion => supersededByVersion.isEmpty;
}

@freezed
abstract class SellerResponseDraftResponse with _$SellerResponseDraftResponse {
  const factory SellerResponseDraftResponse({
    @Default('') String draftVersion,
    @Default('') String updatedAt,
    @JsonKey(toJson: _quoteToJson)
    @Default(SellerQuotePayload())
    SellerQuotePayload draft,
  }) = _SellerResponseDraftResponse;

  factory SellerResponseDraftResponse.fromJson(Map<String, dynamic> json) =>
      _$SellerResponseDraftResponseFromJson(json);
}

@freezed
abstract class SellerFacingPublication with _$SellerFacingPublication {
  const factory SellerFacingPublication({
    @Default('') String publicationId,

    @Default('') String publicationVersion,
    @Default('') String inviteId,

    @Default('') String lane,

    @Default('') String sellerFacingPayloadHash,

    @Default('') String deliveryState,
    @Default('') String publishedAt,
  }) = _SellerFacingPublication;

  factory SellerFacingPublication.fromJson(Map<String, dynamic> json) =>
      _$SellerFacingPublicationFromJson(json);
}

@freezed
abstract class SellerFacingPayload with _$SellerFacingPayload {
  const factory SellerFacingPayload({
    @Default('') String qualityKnowledgeState,
    @Default('') String item,
    @Default('') String itemLabel,
    @Default('') String quantity,

    @Default('') String unit,
    @Default('') String packaging,
    @Default('') String deliveryCity,
    @Default('') String deliveryDate,
    @Default('') String responseDeadline,
    @Default('') String buyerNote,
  }) = _SellerFacingPayload;

  const SellerFacingPayload._();

  factory SellerFacingPayload.fromJson(Map<String, dynamic> json) =>
      _$SellerFacingPayloadFromJson(json);

  bool get hasQualityKnowledge => qualityKnowledgeState.trim().isNotEmpty;

  bool get hasBuyerNote => buyerNote.trim().isNotEmpty;

  List<({String label, String value})> get contextFields {
    final rows = <({String label, String value})>[
      (label: '品項', value: item),
      (label: '品項說明', value: itemLabel),
      (label: '需求數量', value: quantity),
      (label: '單位', value: unit),
      (label: '包裝要求', value: packaging),
      (label: '交貨縣市', value: deliveryCity),
      (label: '希望交貨日', value: deliveryDate),
      (label: '回覆截止日', value: responseDeadline),
    ];
    return rows
        .where((row) => row.value.trim().isNotEmpty)
        .toList(growable: false);
  }
}

@freezed
abstract class SellerPublicationSummary with _$SellerPublicationSummary {
  const factory SellerPublicationSummary({
    required SellerFacingPublication publication,
    @Default(SellerFacingPayload()) SellerFacingPayload sellerFacingPayload,
  }) = _SellerPublicationSummary;
}

@freezed
abstract class SellerPublicationListItem with _$SellerPublicationListItem {
  const factory SellerPublicationListItem({
    required SellerPublicationSummary summary,

    @Default('') String latestResponseStatus,

    @Default(false) bool hasDraft,
  }) = _SellerPublicationListItem;

  const SellerPublicationListItem._();

  SellerFacingPublication get publication => summary.publication;

  SellerFacingPayload get sellerFacingPayload => summary.sellerFacingPayload;

  ProcurementSellerResponseStatus? get latestResponseStatusValue =>
      ProcurementSellerResponseStatus.fromCode(latestResponseStatus);

  bool get hasSubmittedResponse => latestResponseStatus.isNotEmpty;
}

@freezed
abstract class SellerPublicationDetail with _$SellerPublicationDetail {
  const factory SellerPublicationDetail({
    required SellerFacingPublication publication,

    @Default(SellerFacingPayload()) SellerFacingPayload sellerFacingPayload,
    SellerResponseDraftResponse? draft,
    SellerResponseVersionResponse? latestResponse,
  }) = _SellerPublicationDetail;

  const SellerPublicationDetail._();

  String get expectedDraftVersion => draft?.draftVersion ?? '';

  String get expectedSellerResponseVersion =>
      latestResponse?.sellerResponseVersion ?? '';

  bool get hasSubmittedResponse => expectedSellerResponseVersion.isNotEmpty;
}
