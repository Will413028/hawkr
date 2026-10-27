import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/dio_provider.dart';
import 'procurement_error.dart';
import 'seller_quote_model.dart';

part 'seller_quote_repository.g.dart';

const String kIdempotencyKeyHeader = 'Idempotency-Key';

class SellerQuoteRepository {
  SellerQuoteRepository(this._dio);

  final Dio _dio;

  static const String basePath = '/api/v1/app/sellers/me/procurement';

  static String publicationPath(String publicationId) =>
      '$basePath/publications/$publicationId';

  Future<List<SellerPublicationListItem>> listPublications() async {
    try {
      final response = await _dio.get('$basePath/publications');
      return _listPayload(
        response.data,
      ).map(_parseListItem).whereType<SellerPublicationListItem>().toList();
    } on DioException catch (e) {
      throw ProcurementException.from(e);
    }
  }

  Future<SellerPublicationDetail> getPublicationDetail(
    String publicationId,
  ) async {
    try {
      final response = await _dio.get(publicationPath(publicationId));
      final body = _objectPayload(response.data);
      final summary = _parseSummary(body);
      if (body == null || summary == null) {
        throw const ProcurementException(message: '載入詢價內容失敗');
      }
      return SellerPublicationDetail(
        publication: summary.publication,
        sellerFacingPayload: summary.sellerFacingPayload,
        draft: _parseDraft(body['draft']),
        latestResponse: _parseVersion(body['latestResponse']),
      );
    } on DioException catch (e) {
      throw ProcurementException.from(e);
    }
  }

  Future<SellerResponseDraftResponse> saveDraft({
    required String publicationId,
    required SellerQuotePayload draft,
    required String expectedDraftVersion,
  }) async {
    try {
      final response = await _dio.put(
        '${publicationPath(publicationId)}/response-draft',
        data: <String, dynamic>{
          'expectedDraftVersion': expectedDraftVersion,
          'draft': draft.toJson(),
        },
      );
      final parsed = _parseDraft(_objectPayload(response.data));
      if (parsed == null) {
        throw const ProcurementException(message: '儲存草稿失敗');
      }
      return parsed;
    } on DioException catch (e) {
      throw ProcurementException.from(e);
    }
  }

  Future<SellerResponseVersionResponse> submitResponse({
    required String publicationId,
    required SellerQuotePayload quote,
    required String expectedPublicationVersion,
    required String expectedSellerFacingPayloadHash,
    required String expectedSellerResponseVersion,
    required String idempotencyKey,
  }) async {
    try {
      final response = await _dio.post(
        '${publicationPath(publicationId)}/seller-responses',
        data: <String, dynamic>{
          'expectedPublicationVersion': expectedPublicationVersion,
          'expectedSellerFacingPayloadHash': expectedSellerFacingPayloadHash,
          'expectedSellerResponseVersion': expectedSellerResponseVersion,
          'quote': quote.toJson(),
        },
        options: _idempotent(idempotencyKey),
      );
      return _requireVersion(response.data, '送出回價失敗');
    } on DioException catch (e) {
      throw ProcurementException.from(e);
    }
  }

  Future<SellerResponseVersionResponse> withdrawResponse({
    required String publicationId,
    required String sellerResponseId,
    required String expectedSellerResponseVersion,
    required String idempotencyKey,
  }) async {
    try {
      final response = await _dio.post(
        '${publicationPath(publicationId)}/seller-responses/'
        '$sellerResponseId/withdraw',
        data: <String, dynamic>{
          'expectedSellerResponseVersion': expectedSellerResponseVersion,
        },
        options: _idempotent(idempotencyKey),
      );
      return _requireVersion(response.data, '撤回回價失敗');
    } on DioException catch (e) {
      throw ProcurementException.from(e);
    }
  }

  Future<void> declinePublication({
    required String publicationId,
    required String idempotencyKey,
    String reason = '',
  }) async {
    try {
      await _dio.post(
        '${publicationPath(publicationId)}/decline',
        data: <String, dynamic>{if (reason.isNotEmpty) 'reason': reason},
        options: _idempotent(idempotencyKey),
      );
    } on DioException catch (e) {
      throw ProcurementException.from(e);
    }
  }

  Options _idempotent(String idempotencyKey) => Options(
    headers: <String, dynamic>{kIdempotencyKeyHeader: idempotencyKey},
  );

  SellerResponseVersionResponse _requireVersion(
    dynamic body,
    String failureMessage,
  ) {
    final parsed = _parseVersion(_objectPayload(body));
    if (parsed == null) throw ProcurementException(message: failureMessage);
    return parsed;
  }

  static Map<String, dynamic>? _objectPayload(dynamic body) {
    if (body is! Map) return null;
    final inner = body['data'];
    if (inner is Map) return Map<String, dynamic>.from(inner);
    return Map<String, dynamic>.from(body);
  }

  static List<dynamic> _listPayload(dynamic body) {
    if (body is List) return body;
    if (body is! Map) return const [];
    final inner = body['data'];
    if (inner is List) return inner;
    if (inner is Map) {
      final rows = inner['publications'];
      if (rows is List) return rows;
    }
    final rows = body['publications'];
    if (rows is List) return rows;
    return const [];
  }

  static SellerPublicationListItem? _parseListItem(dynamic raw) {
    final summary = _parseSummary(raw);
    if (summary == null) return null;
    final row = Map<String, dynamic>.from(raw as Map);
    return SellerPublicationListItem(
      summary: summary,
      latestResponseStatus: _string(row['latestResponseStatus']),
      hasDraft: row['hasDraft'] == true,
    );
  }

  static SellerPublicationSummary? _parseSummary(dynamic raw) {
    if (raw is! Map) return null;
    final publication = raw['publication'];
    if (publication is! Map) return null;
    return SellerPublicationSummary(
      publication: _parsePublication(Map<String, dynamic>.from(publication)),
      sellerFacingPayload: _parseSellerFacingPayload(
        raw['sellerFacingPayload'],
      ),
    );
  }

  static SellerFacingPublication _parsePublication(Map<String, dynamic> row) {
    return SellerFacingPublication(
      publicationId: _string(row['publicationId']),
      publicationVersion: _string(row['publicationVersion']),
      inviteId: _string(row['inviteId']),
      lane: _string(row['lane']),
      sellerFacingPayloadHash: _string(row['sellerFacingPayloadHash']),
      deliveryState: _string(row['deliveryState']),
      publishedAt: _string(row['publishedAt']),
    );
  }

  static SellerFacingPayload _parseSellerFacingPayload(dynamic raw) {
    if (raw is! Map) return const SellerFacingPayload();
    return SellerFacingPayload(
      qualityKnowledgeState: _string(raw['qualityKnowledgeState']),
      item: _string(raw['item']),
      itemLabel: _string(raw['itemLabel']),
      quantity: _string(raw['quantity']),
      unit: _string(raw['unit']),
      packaging: _string(raw['packaging']),
      deliveryCity: _string(raw['deliveryCity']),
      deliveryDate: _string(raw['deliveryDate']),
      responseDeadline: _string(raw['responseDeadline']),
    );
  }

  static SellerResponseDraftResponse? _parseDraft(dynamic raw) {
    if (raw is! Map) return null;
    final row = Map<String, dynamic>.from(raw);
    return SellerResponseDraftResponse(
      draftVersion: _string(row['draftVersion']),
      updatedAt: _string(row['updatedAt']),
      draft: _parseQuote(row['draft']),
    );
  }

  static SellerResponseVersionResponse? _parseVersion(dynamic raw) {
    if (raw is! Map) return null;
    final row = Map<String, dynamic>.from(raw);
    if (_string(row['sellerResponseId']).isEmpty &&
        _string(row['sellerResponseVersion']).isEmpty) {
      return null;
    }
    return SellerResponseVersionResponse(
      sellerResponseId: _string(row['sellerResponseId']),
      sellerResponseVersion: _string(row['sellerResponseVersion']),
      publicationId: _string(row['publicationId']),
      status: _string(row['status']),
      payloadHash: _string(row['payloadHash']),
      submittedAt: _string(row['submittedAt']),
      supersededByVersion: _string(row['supersededByVersion']),
      outsideBuyerRequestedRange: row['outsideBuyerRequestedRange'] == true,
      quote: _parseQuote(row['quote']),
      replayed: row['replayed'] == true,
    );
  }

  static SellerQuotePayload _parseQuote(dynamic raw) {
    if (raw is! Map) return const SellerQuotePayload();
    final row = Map<String, dynamic>.from(raw);
    final quality = row['offeredQuality'];
    final basis = row['unitBasis'];
    return SellerQuotePayload(
      responseType: _string(row['responseType']),
      offeredQuality: quality is Map
          ? OfferedQuality(
              grade: _string(quality['grade']),
              inspectionNotes: _string(quality['inspectionNotes']),
            )
          : const OfferedQuality(),
      unitBasis: basis is Map
          ? UnitBasis(
              quantity: _string(basis['quantity']),
              unit: _string(basis['unit']),
            )
          : const UnitBasis(),
      offeredQuantity: _string(row['offeredQuantity']),
      offeredQuantityUnit: _string(row['offeredQuantityUnit']),
      conversionBasis: _string(row['conversionBasis']),
      currency: _string(row['currency'], fallback: kQuoteCurrencyTwd),
      goodsAmount: _string(row['goodsAmount']),
      shippingAmount: _string(row['shippingAmount']),
      taxAmount: _string(row['taxAmount']),
      taxInclusionBasis: _string(row['taxInclusionBasis']),
      totalAmount: _string(row['totalAmount']),
      leadTimeDays: _string(row['leadTimeDays']),
      promisedDeliveryAt: _string(row['promisedDeliveryAt']),
      validUntil: _string(row['validUntil']),
      termsNote: _string(row['termsNote']),
      deviationDisclosure: _string(row['deviationDisclosure']),
    );
  }

  static String _string(dynamic raw, {String fallback = ''}) {
    if (raw == null) return fallback;
    final value = raw.toString().trim();
    if (value.isEmpty || value == 'null') return fallback;
    return value;
  }
}

@riverpod
SellerQuoteRepository sellerQuoteRepository(Ref ref) {
  return SellerQuoteRepository(ref.watch(dioProvider));
}
