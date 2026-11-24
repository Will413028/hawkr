import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.pbenum.dart';

part 'seller_quote_model.freezed.dart';

/// 賣家在表單上可以選的回應方式。
const List<SellerResponseType> kSelectableResponseTypes = [
  SellerResponseType.SELLER_RESPONSE_TYPE_MANUAL_QUOTE,
  SellerResponseType.SELLER_RESPONSE_TYPE_ACCEPT_NON_PRICE_TERMS_WITH_QUOTE,
];

/// 賣家在表單上可以選的含稅基準。
const List<TaxInclusionBasis> kSelectableTaxBases = [
  TaxInclusionBasis.TAX_INCLUSION_BASIS_TAX_INCLUDED,
  TaxInclusionBasis.TAX_INCLUSION_BASIS_TAX_EXCLUDED,
];

@freezed
abstract class QuoteUnitBasis with _$QuoteUnitBasis {
  const factory QuoteUnitBasis({
    @Default('') String quantity,
    @Default('') String unit,
  }) = _QuoteUnitBasis;
}

/// 回價表單的內容。欄位是使用者輸入的字串；送出時才轉成契約型別（`SellerQuote`）。
/// 金額是非負整數（TWD，幣別由伺服器補上），時間是 RFC3339 字串。
@freezed
abstract class SellerQuotePayload with _$SellerQuotePayload {
  const factory SellerQuotePayload({
    @Default(SellerResponseType.SELLER_RESPONSE_TYPE_UNSPECIFIED)
    SellerResponseType responseType,

    /// 賣家對規格的補充說明，選填。
    @Default('') String specNote,
    @Default(QuoteUnitBasis()) QuoteUnitBasis unitBasis,
    @Default('') String offeredQuantity,
    @Default('') String offeredQuantityUnit,
    @Default('') String conversionBasis,
    @Default('') String goodsAmount,
    @Default('') String shippingAmount,
    @Default('') String taxAmount,
    @Default(TaxInclusionBasis.TAX_INCLUSION_BASIS_UNSPECIFIED)
    TaxInclusionBasis taxInclusionBasis,
    @Default('') String totalAmount,
    @Default('') String leadTimeDays,
    @Default('') String promisedDeliveryAt,
    @Default('') String validUntil,
    @Default('') String termsNote,
    @Default('') String deviationDisclosure,
  }) = _SellerQuotePayload;

  const SellerQuotePayload._();

  SellerResponseType? get responseTypeValue =>
      responseType == SellerResponseType.SELLER_RESPONSE_TYPE_UNSPECIFIED
      ? null
      : responseType;

  TaxInclusionBasis? get taxInclusionBasisValue =>
      taxInclusionBasis == TaxInclusionBasis.TAX_INCLUSION_BASIS_UNSPECIFIED
      ? null
      : taxInclusionBasis;
}

/// 一個正式回價版本（不可變）。
@freezed
abstract class SellerResponseVersionResponse
    with _$SellerResponseVersionResponse {
  const factory SellerResponseVersionResponse({
    @Default('') String sellerResponseId,
    @Default('') String sellerResponseVersion,
    @Default('') String publicationId,
    @Default(SellerResponseStatus.SELLER_RESPONSE_STATUS_UNSPECIFIED)
    SellerResponseStatus status,
    @Default('') String payloadHash,
    @Default('') String supersededByVersion,
    @Default(false) bool outsideBuyerRequestedRange,
    @Default(SellerQuotePayload()) SellerQuotePayload quote,
    @Default(false) bool replayed,
  }) = _SellerResponseVersionResponse;

  const SellerResponseVersionResponse._();

  SellerResponseStatus? get statusValue =>
      status == SellerResponseStatus.SELLER_RESPONSE_STATUS_UNSPECIFIED
      ? null
      : status;

  bool get isLatestVersion => supersededByVersion.isEmpty;
}

@freezed
abstract class SellerResponseDraftResponse with _$SellerResponseDraftResponse {
  const factory SellerResponseDraftResponse({
    @Default('') String draftVersion,
    @Default(SellerQuotePayload()) SellerQuotePayload draft,
  }) = _SellerResponseDraftResponse;
}

@freezed
abstract class SellerFacingPublication with _$SellerFacingPublication {
  const factory SellerFacingPublication({
    @Default('') String publicationId,
    @Default('') String publicationVersion,
    @Default('') String sellerFacingPayloadHash,
  }) = _SellerFacingPublication;
}

/// 賣家看到的詢價內容（只含白名單欄位）；日期是 `YYYY-MM-DD`，沒有的欄位是空字串。
@freezed
abstract class SellerFacingPayload with _$SellerFacingPayload {
  const factory SellerFacingPayload({
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
    SellerResponseStatus? latestResponseStatus,
    @Default(false) bool hasDraft,
  }) = _SellerPublicationListItem;

  const SellerPublicationListItem._();

  SellerFacingPublication get publication => summary.publication;

  SellerFacingPayload get sellerFacingPayload => summary.sellerFacingPayload;

  bool get hasSubmittedResponse => latestResponseStatus != null;
}

/// 回價被撤回或婉拒後的結案狀態。
@freezed
abstract class SellerResponseClosure with _$SellerResponseClosure {
  const factory SellerResponseClosure({
    @Default('') String sellerResponseId,
    @Default('') String latestVersion,
    @Default('') String withdrawnVersion,
  }) = _SellerResponseClosure;
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
