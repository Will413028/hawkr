import 'package:hawkr_api/dates.dart' show formatDate;
import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.pb.dart'
    as pb;
import 'package:hawkr_api/taipei_time.dart' show Timestamp;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import 'seller_quote_model.dart';

part 'seller_quote_repository.g.dart';

/// 賣家收到的詢價與回價（`SellerProcurementAppService`）。錯誤原樣丟出 `ConnectException`，
/// 由 controller 依 reason 分流、以 `errorText` 顯示。
class SellerQuoteRepository {
  SellerQuoteRepository(this._client);

  final SellerProcurementAppServiceClient _client;

  /// 發給呼叫者的詢價，最新的在前；逐頁讀完。
  Future<List<SellerPublicationListItem>> listPublications() async {
    final items = <SellerPublicationListItem>[];
    var token = '';
    do {
      final page = await _client.listSellerPublications(
        pb.ListSellerPublicationsRequest(pageSize: 50, pageToken: token),
      );
      items.addAll(page.publications.map(_listItem));
      token = page.nextPageToken;
    } while (token.isNotEmpty);
    return items;
  }

  Future<SellerPublicationDetail> getPublicationDetail(
    String publicationId,
  ) async {
    final r = await _client.getSellerPublication(
      pb.GetSellerPublicationRequest(publicationId: publicationId),
    );
    return SellerPublicationDetail(
      publication: _publication(r.publication),
      sellerFacingPayload: _payload(r.sellerFacingPayload),
      draft: r.hasDraft() ? _draft(r.draft) : null,
      latestResponse: r.hasLatestResponse() ? _version(r.latestResponse) : null,
    );
  }

  Future<SellerResponseDraftResponse> saveDraft({
    required String publicationId,
    required SellerQuotePayload draft,
    required String expectedDraftVersion,
  }) async {
    final r = await _client.upsertSellerResponseDraft(
      pb.UpsertSellerResponseDraftRequest(
        publicationId: publicationId,
        expectedDraftVersion: expectedDraftVersion,
        draft: quoteToProto(draft),
      ),
    );
    return _draft(r.draft);
  }

  Future<SellerResponseVersionResponse> submitResponse({
    required String publicationId,
    required SellerQuotePayload quote,
    required String expectedPublicationVersion,
    required String expectedSellerFacingPayloadHash,
    required String expectedSellerResponseVersion,
    required String clientRequestId,
  }) async {
    final r = await _client.submitSellerResponse(
      pb.SubmitSellerResponseRequest(
        publicationId: publicationId,
        clientRequestId: clientRequestId,
        expectedPublicationVersion: expectedPublicationVersion,
        expectedSellerFacingPayloadHash: expectedSellerFacingPayloadHash,
        expectedSellerResponseVersion: expectedSellerResponseVersion,
        quote: quoteToProto(quote),
      ),
    );
    return _version(r.sellerResponse);
  }

  /// 撤回指定版本的正式回價；操作本身冪等，不帶 request id。[reasonCode] 空字串表示沒有填。
  Future<SellerResponseClosure> withdrawResponse({
    required String publicationId,
    required String sellerResponseId,
    required String expectedSellerResponseVersion,
    String reasonCode = '',
  }) async {
    final r = await _client.withdrawSellerResponse(
      pb.WithdrawSellerResponseRequest(
        publicationId: publicationId,
        sellerResponseId: sellerResponseId,
        expectedSellerResponseVersion: expectedSellerResponseVersion,
        reasonCode: reasonCode,
      ),
    );
    return _closure(r.closure);
  }

  /// 婉拒一筆詢價；操作本身冪等，不帶 request id。[reasonCode] 空字串表示沒有填。
  Future<SellerResponseClosure> declinePublication({
    required String publicationId,
    String reasonCode = '',
  }) async {
    final r = await _client.declineRfqPublication(
      pb.DeclineRfqPublicationRequest(
        publicationId: publicationId,
        reasonCode: reasonCode,
      ),
    );
    return _closure(r.closure);
  }

  static SellerPublicationListItem _listItem(pb.SellerPublicationListItem i) =>
      SellerPublicationListItem(
        summary: SellerPublicationSummary(
          publication: _publication(i.publication),
          sellerFacingPayload: _payload(i.sellerFacingPayload),
        ),
        latestResponseStatus: i.hasLatestResponseStatus()
            ? i.latestResponseStatus
            : null,
        hasDraft: i.hasDraft,
      );

  static SellerFacingPublication _publication(pb.SellerPublicationSummary s) =>
      SellerFacingPublication(
        publicationId: s.publicationId,
        publicationVersion: s.publicationVersion,
        sellerFacingPayloadHash: s.sellerFacingPayloadHash,
      );

  static SellerFacingPayload _payload(pb.SellerFacingRfq r) =>
      SellerFacingPayload(
        item: r.item,
        itemLabel: r.itemLabel,
        quantity: r.hasQuantity() ? '${r.quantity}' : '',
        unit: r.hasUnit() ? r.unit : '',
        packaging: r.hasPackaging() ? r.packaging : '',
        deliveryCity: r.deliveryCity,
        deliveryDate: r.hasDeliveryDate() ? formatDate(r.deliveryDate) : '',
        responseDeadline: r.hasResponseDeadline()
            ? formatDate(r.responseDeadline)
            : '',
        buyerNote: r.buyerNote,
      );

  static SellerResponseDraftResponse _draft(pb.SellerResponseDraft d) =>
      SellerResponseDraftResponse(
        draftVersion: d.draftVersion,
        draft: quoteFromProto(d.draft),
      );

  static SellerResponseVersionResponse _version(pb.SellerResponseVersion v) =>
      SellerResponseVersionResponse(
        sellerResponseId: v.sellerResponseId,
        sellerResponseVersion: v.sellerResponseVersion,
        publicationId: v.publicationId,
        status: v.status,
        payloadHash: v.payloadHash,
        supersededByVersion: v.supersededByVersion,
        outsideBuyerRequestedRange: v.outsideBuyerRequestedRange,
        quote: quoteFromProto(v.quote),
        replayed: v.replayed,
      );

  static SellerResponseClosure _closure(pb.SellerResponseClosure c) =>
      SellerResponseClosure(
        sellerResponseId: c.sellerResponseId,
        latestVersion: c.latestVersion,
        withdrawnVersion: c.withdrawnVersion,
      );
}

/// 表單內容轉成契約的 `SellerQuote`。空欄位不設；金額是非負整數字串（驗證在 `validateSellerQuote`），
/// 草稿不驗，格式不合的欄位不送。
pb.SellerQuote quoteToProto(SellerQuotePayload q) {
  final quote = pb.SellerQuote(
    responseType: q.responseType,
    specNote: q.specNote,
    offeredQuantity: q.offeredQuantity.trim(),
    offeredQuantityUnit: q.offeredQuantityUnit.trim(),
    conversionBasis: q.conversionBasis,
    taxInclusionBasis: q.taxInclusionBasis,
    termsNote: q.termsNote,
    deviationDisclosure: q.deviationDisclosure,
  );
  if (q.unitBasis.quantity.trim().isNotEmpty ||
      q.unitBasis.unit.trim().isNotEmpty) {
    quote.unitBasis = pb.UnitBasis(
      quantity: q.unitBasis.quantity.trim(),
      unit: q.unitBasis.unit.trim(),
    );
  }
  final goods = _int64(q.goodsAmount);
  if (goods != null) quote.goodsAmount = goods;
  final shipping = _int64(q.shippingAmount);
  if (shipping != null) quote.shippingAmount = shipping;
  final tax = _int64(q.taxAmount);
  if (tax != null) quote.taxAmount = tax;
  final total = _int64(q.totalAmount);
  if (total != null) quote.totalAmount = total;
  final lead = int.tryParse(q.leadTimeDays.trim());
  if (lead != null) quote.leadTimeDays = lead;
  final promised = _timestamp(q.promisedDeliveryAt);
  if (promised != null) quote.promisedDeliveryTime = promised;
  final valid = _timestamp(q.validUntil);
  if (valid != null) quote.validUntilTime = valid;
  return quote;
}

SellerQuotePayload quoteFromProto(pb.SellerQuote q) => SellerQuotePayload(
  responseType: q.responseType,
  specNote: q.specNote,
  unitBasis: q.hasUnitBasis()
      ? QuoteUnitBasis(quantity: q.unitBasis.quantity, unit: q.unitBasis.unit)
      : const QuoteUnitBasis(),
  offeredQuantity: q.offeredQuantity,
  offeredQuantityUnit: q.offeredQuantityUnit,
  conversionBasis: q.conversionBasis,
  goodsAmount: _amount(q.goodsAmount),
  shippingAmount: _amount(q.shippingAmount),
  taxAmount: _amount(q.taxAmount),
  taxInclusionBasis: q.taxInclusionBasis,
  totalAmount: _amount(q.totalAmount),
  leadTimeDays: q.hasLeadTimeDays() ? '${q.leadTimeDays}' : '',
  promisedDeliveryAt: q.hasPromisedDeliveryTime()
      ? _rfc3339(q.promisedDeliveryTime)
      : '',
  validUntil: q.hasValidUntilTime() ? _rfc3339(q.validUntilTime) : '',
  termsNote: q.termsNote,
  deviationDisclosure: q.deviationDisclosure,
);

/// 生成碼的 int64 欄位是 `Int64`；值由欄位的預設值（0）加上去取得，seller-app 不必直接依賴 fixnum。
final _zero = pb.SellerQuote().goodsAmount;

/// 非負整數字串（TWD 金額，遠小於 2^53）轉 int64；格式不合回 null。型別由 [_zero] 推得。
dynamic _int64(String raw) {
  final v = raw.trim();
  return RegExp(r'^\d{1,15}$').hasMatch(v) ? _zero + int.parse(v) : null;
}

String _amount(Object v) => v == _zero ? '' : v.toString();

Timestamp? _timestamp(String raw) {
  final v = raw.trim();
  if (v.isEmpty) return null;
  final parsed = DateTime.tryParse(v);
  if (parsed == null) return null;
  return Timestamp.fromDateTime(parsed.toUtc());
}

String _rfc3339(Timestamp t) {
  final utc = t.toDateTime();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${utc.year.toString().padLeft(4, '0')}-${two(utc.month)}-'
      '${two(utc.day)}T${two(utc.hour)}:${two(utc.minute)}:'
      '${two(utc.second)}Z';
}

@riverpod
SellerQuoteRepository sellerQuoteRepository(Ref ref) {
  return SellerQuoteRepository(ref.watch(sellerProcurementClientProvider));
}
