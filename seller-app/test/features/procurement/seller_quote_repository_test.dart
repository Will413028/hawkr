import 'dart:convert';

import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/features/procurement/data/procurement_error.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_model.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

class _RecordingAdapter implements HttpClientAdapter {
  _RecordingAdapter({required this.statusCode, required this.body});

  final int statusCode;
  final Map<String, dynamic> body;

  RequestOptions? lastRequest;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastRequest = options;
    return ResponseBody.fromString(
      jsonEncode(body),
      statusCode,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

({SellerQuoteRepository repo, _RecordingAdapter adapter}) _repo({
  int statusCode = 200,
  Map<String, dynamic> body = const {'code': 0, 'message': 'success'},
}) {
  final dio = Dio(
    BaseOptions(
      baseUrl: 'https://test-api.example.com',
      validateStatus: (code) => code != null && code < 400,
    ),
  );
  final adapter = _RecordingAdapter(statusCode: statusCode, body: body);
  dio.httpClientAdapter = adapter;
  return (repo: SellerQuoteRepository(dio), adapter: adapter);
}

const _publicationId = '7401111111111111111';

const Map<String, dynamic> _publicationJson = <String, dynamic>{
  'publicationId': _publicationId,
  'publicationVersion': '4',
  'inviteId': 'inv-1',
  'lane': 'PLATFORM',
  'sellerFacingPayloadHash': 'hash-4',
  'deliveryState': 'SENT',
  'publishedAt': '2026-09-13T02:00:00Z',
};

const Map<String, dynamic> _sellerFacingPayloadJson = <String, dynamic>{
  'qualityKnowledgeState': '',
  'item': '高麗菜',
  'itemLabel': '梨山高麗菜',
  'quantity': '200',
  'unit': 'BOX',
  'packaging': '紙箱',
  'deliveryCity': '臺北市',
  'deliveryDate': '2026-09-20',
  'responseDeadline': '2026-09-18',
};

SellerQuotePayload _quote() => const SellerQuotePayload(
  responseType: 'MANUAL_QUOTE',
  offeredQuality: OfferedQuality(grade: '特級', inspectionNotes: '無壓傷'),
  unitBasis: UnitBasis(quantity: '1', unit: '公斤'),
  offeredQuantity: '500',
  offeredQuantityUnit: '公斤',
  conversionBasis: '1 箱 ＝ 10 公斤',
  goodsAmount: '10000',
  shippingAmount: '500',
  taxAmount: '525',
  taxInclusionBasis: 'TAX_EXCLUDED',
  totalAmount: '11025',
  leadTimeDays: '3',
  promisedDeliveryAt: '2026-12-20T02:00:00Z',
  validUntil: '2026-12-31T15:00:00Z',
  termsNote: '到貨後付款',
  deviationDisclosure: '',
);

Map<String, dynamic> _requestBody(RequestOptions options) =>
    Map<String, dynamic>.from(options.data as Map);

void main() {
  test('base path 是平台賣家面的凍結路徑', () {
    expect(
      SellerQuoteRepository.basePath,
      '/api/v1/app/sellers/me/procurement',
    );
  });

  group('十七欄 wire 契約', () {
    test('toJson 是 camelCase 的十七欄，順序照契約表', () {
      expect(_quote().toJson().keys.toList(), <String>[
        'responseType',
        'offeredQuality',
        'unitBasis',
        'offeredQuantity',
        'offeredQuantityUnit',
        'conversionBasis',
        'currency',
        'goodsAmount',
        'shippingAmount',
        'taxAmount',
        'taxInclusionBasis',
        'totalAmount',
        'leadTimeDays',
        'promisedDeliveryAt',
        'validUntil',
        'termsNote',
        'deviationDisclosure',
      ]);
    });

    test('offeredQuality 與 unitBasis 是巢狀物件', () {
      final json = _quote().toJson();
      expect(json['offeredQuality'], {'grade': '特級', 'inspectionNotes': '無壓傷'});
      expect(json['unitBasis'], {'quantity': '1', 'unit': '公斤'});
    });

    test('金額欄位一律是 string，不是數字', () {
      final json = _quote().toJson();
      for (final key in const [
        'goodsAmount',
        'shippingAmount',
        'taxAmount',
        'totalAmount',
        'offeredQuantity',
        'leadTimeDays',
      ]) {
        expect(json[key], isA<String>(), reason: key);
      }
    });

    test('server 若把金額送成數字，parse 後仍是 string', () async {
      final harness = _repo(
        body: {
          'publication': _publicationJson,
          'latestResponse': {
            'sellerResponseId': 'r-1',
            'sellerResponseVersion': '2',
            'quote': {'goodsAmount': 10000, 'totalAmount': 11025.5},
          },
        },
      );
      final detail = await harness.repo.getPublicationDetail(_publicationId);
      expect(detail.latestResponse!.quote.goodsAmount, '10000');
      expect(detail.latestResponse!.quote.totalAmount, '11025.5');
    });
  });

  group('冪等鍵', () {
    test('草稿 PUT 不帶 Idempotency-Key，CAS 走 expectedDraftVersion', () async {
      final harness = _repo(
        body: {
          'code': 0,
          'data': {'draftVersion': '1', 'updatedAt': '2026-09-13T00:00:00Z'},
        },
      );
      await harness.repo.saveDraft(
        publicationId: _publicationId,
        draft: _quote(),
        expectedDraftVersion: '',
      );

      final request = harness.adapter.lastRequest!;
      expect(request.method, 'PUT');
      expect(
        request.path,
        '/api/v1/app/sellers/me/procurement/publications/'
        '$_publicationId/response-draft',
      );
      expect(request.headers.containsKey(kIdempotencyKeyHeader), isFalse);
      expect(_requestBody(request)['expectedDraftVersion'], '');
    });

    test('正式提交帶 Idempotency-Key header，不放 body', () async {
      final harness = _repo(
        body: {
          'code': 0,
          'data': {
            'sellerResponseId': 'r-1',
            'sellerResponseVersion': '1',
            'status': 'ACTIVE',
          },
        },
      );
      await harness.repo.submitResponse(
        publicationId: _publicationId,
        quote: _quote(),
        expectedPublicationVersion: '4',
        expectedSellerFacingPayloadHash: 'hash-4',
        expectedSellerResponseVersion: '',
        idempotencyKey: 'client-request-id-1',
      );

      final request = harness.adapter.lastRequest!;
      expect(request.method, 'POST');
      expect(
        request.path,
        '/api/v1/app/sellers/me/procurement/publications/'
        '$_publicationId/seller-responses',
      );
      expect(request.headers[kIdempotencyKeyHeader], 'client-request-id-1');
      final body = _requestBody(request);
      expect(body.containsKey('idempotencyKey'), isFalse);
      expect(body.containsKey('clientRequestId'), isFalse);
      expect(body['expectedPublicationVersion'], '4');
      expect(body['expectedSellerFacingPayloadHash'], 'hash-4');
      expect(body['expectedSellerResponseVersion'], '');
      expect(body['quote'], _quote().toJson());
    });

    test('撤回帶冪等鍵與 expectedSellerResponseVersion', () async {
      final harness = _repo(
        body: {
          'code': 0,
          'data': {
            'sellerResponseId': 'r-1',
            'sellerResponseVersion': '3',
            'status': 'WITHDRAWN',
          },
        },
      );
      final result = await harness.repo.withdrawResponse(
        publicationId: _publicationId,
        sellerResponseId: 'r-1',
        expectedSellerResponseVersion: '3',
        idempotencyKey: 'withdraw-key',
      );

      final request = harness.adapter.lastRequest!;
      expect(
        request.path,
        '/api/v1/app/sellers/me/procurement/publications/'
        '$_publicationId/seller-responses/r-1/withdraw',
      );
      expect(request.headers[kIdempotencyKeyHeader], 'withdraw-key');
      expect(_requestBody(request)['expectedSellerResponseVersion'], '3');
      expect(result.statusValue, ProcurementSellerResponseStatus.withdrawn);
    });

    test('拒絕帶冪等鍵，理由可選', () async {
      final harness = _repo();
      await harness.repo.declinePublication(
        publicationId: _publicationId,
        idempotencyKey: 'decline-key',
      );
      var request = harness.adapter.lastRequest!;
      expect(
        request.path,
        '/api/v1/app/sellers/me/procurement/publications/'
        '$_publicationId/decline',
      );
      expect(request.headers[kIdempotencyKeyHeader], 'decline-key');
      expect(_requestBody(request).containsKey('reason'), isFalse);

      await harness.repo.declinePublication(
        publicationId: _publicationId,
        idempotencyKey: 'decline-key',
        reason: '這批貨我調不到',
      );
      request = harness.adapter.lastRequest!;
      expect(_requestBody(request)['reason'], '這批貨我調不到');
    });
  });

  group('submit 與 revise 是同一支端點', () {
    test('空 expectedSellerResponseVersion ＝ 我還沒有正式回價', () async {
      final harness = _repo(
        body: {
          'code': 0,
          'data': {'sellerResponseId': 'r-1', 'sellerResponseVersion': '1'},
        },
      );
      await harness.repo.submitResponse(
        publicationId: _publicationId,
        quote: _quote(),
        expectedPublicationVersion: '4',
        expectedSellerFacingPayloadHash: 'hash-4',
        expectedSellerResponseVersion: '',
        idempotencyKey: 'k',
      );
      expect(
        _requestBody(
          harness.adapter.lastRequest!,
        )['expectedSellerResponseVersion'],
        '',
      );
    });

    test('帶版本號 ＝ 在該版之上改版，路徑不變', () async {
      final harness = _repo(
        body: {
          'code': 0,
          'data': {'sellerResponseId': 'r-1', 'sellerResponseVersion': '4'},
        },
      );
      await harness.repo.submitResponse(
        publicationId: _publicationId,
        quote: _quote(),
        expectedPublicationVersion: '4',
        expectedSellerFacingPayloadHash: 'hash-4',
        expectedSellerResponseVersion: '3',
        idempotencyKey: 'k',
      );
      final request = harness.adapter.lastRequest!;
      expect(
        request.path,
        '/api/v1/app/sellers/me/procurement/publications/'
        '$_publicationId/seller-responses',
      );
      expect(_requestBody(request)['expectedSellerResponseVersion'], '3');
    });
  });

  group('回應解析', () {
    test('SellerResponseVersionResponse 十欄都吃得到', () async {
      final harness = _repo(
        body: {
          'code': 0,
          'data': {
            'sellerResponseId': 'r-9',
            'sellerResponseVersion': '2',
            'publicationId': _publicationId,
            'status': 'ACTIVE',
            'payloadHash': 'ph-2',
            'submittedAt': '2026-09-13T02:00:00Z',
            'supersededByVersion': '',
            'outsideBuyerRequestedRange': true,
            'replayed': true,
            'quote': _quote().toJson(),
          },
        },
      );
      final result = await harness.repo.submitResponse(
        publicationId: _publicationId,
        quote: _quote(),
        expectedPublicationVersion: '4',
        expectedSellerFacingPayloadHash: 'hash-4',
        expectedSellerResponseVersion: '1',
        idempotencyKey: 'k',
      );
      expect(result.sellerResponseId, 'r-9');
      expect(result.statusValue, ProcurementSellerResponseStatus.active);
      expect(result.payloadHash, 'ph-2');
      expect(result.isLatestVersion, isTrue);
      expect(result.outsideBuyerRequestedRange, isTrue);
      expect(result.replayed, isTrue);
      expect(result.quote.offeredQuality.grade, '特級');
    });

    test('supersededByVersion 非空 ＝ 已被取代', () async {
      final harness = _repo(
        body: {
          'code': 0,
          'data': {
            'sellerResponseId': 'r-9',
            'sellerResponseVersion': '2',
            'supersededByVersion': '3',
            'status': 'SUPERSEDED',
          },
        },
      );
      final result = await harness.repo.withdrawResponse(
        publicationId: _publicationId,
        sellerResponseId: 'r-9',
        expectedSellerResponseVersion: '2',
        idempotencyKey: 'k',
      );
      expect(result.isLatestVersion, isFalse);
      expect(result.statusValue, ProcurementSellerResponseStatus.superseded);
    });

    test('publication 是巢狀的，七個欄位都吃得到', () async {
      final harness = _repo(body: {'publication': _publicationJson});
      final detail = await harness.repo.getPublicationDetail(_publicationId);
      final publication = detail.publication;
      expect(publication.publicationId, _publicationId);
      expect(publication.publicationVersion, '4');
      expect(publication.inviteId, 'inv-1');
      expect(publication.lane, 'PLATFORM');
      expect(publication.sellerFacingPayloadHash, 'hash-4');
      expect(publication.deliveryState, 'SENT');
      expect(publication.publishedAt, '2026-09-13T02:00:00Z');
    });

    test('sellerFacingPayload 是頂層 key，十欄逐欄解析', () async {
      final harness = _repo(
        body: {
          'publication': _publicationJson,
          'sellerFacingPayload': _sellerFacingPayloadJson,
        },
      );
      final detail = await harness.repo.getPublicationDetail(_publicationId);
      final payload = detail.sellerFacingPayload;
      expect(payload.item, '高麗菜');
      expect(payload.itemLabel, '梨山高麗菜');
      expect(payload.quantity, '200');
      expect(payload.unit, 'BOX');
      expect(payload.packaging, '紙箱');
      expect(payload.deliveryCity, '臺北市');
      expect(payload.deliveryDate, '2026-09-20');
      expect(payload.responseDeadline, '2026-09-18');
      expect(payload.qualityKnowledgeState, '');
      expect(payload.hasQualityKnowledge, isFalse);
    });

    test('個別欄位是空字串不算載入失敗，只是這張 RFQ 沒填', () async {
      final harness = _repo(
        body: {
          'publication': _publicationJson,
          'sellerFacingPayload': <String, dynamic>{
            'item': '高麗菜',
            'itemLabel': '',
            'quantity': '',
            'unit': '',
            'packaging': '',
            'deliveryCity': '',
            'deliveryDate': '',
            'responseDeadline': '',
            'qualityKnowledgeState': '',
          },
        },
      );
      final detail = await harness.repo.getPublicationDetail(_publicationId);
      expect(detail.sellerFacingPayload.item, '高麗菜');
      expect(detail.sellerFacingPayload.contextFields, hasLength(1));
    });

    test('⛔ 回應多帶敏感欄位時，解析層就丟掉，進不了 model', () async {
      final harness = _repo(
        body: {
          'publication': _publicationJson,
          'sellerFacingPayload': <String, dynamic>{
            ..._sellerFacingPayloadJson,
            'budgetPerUnit': '38',
            'acceptableRange': '35-40',
            'candidateRank': '2',
            'buyerPhone': '0912345678',
          },
        },
      );
      final detail = await harness.repo.getPublicationDetail(_publicationId);
      final serialized = detail.sellerFacingPayload.toJson();
      expect(serialized.keys, hasLength(10));
      for (final leaked in const ['38', '35-40', '0912345678']) {
        expect(
          serialized.values.map((v) => '$v'),
          isNot(contains(leaked)),
          reason: leaked,
        );
      }
    });

    test('sellerFacingPayload 真的缺席時退回空內容，不丟例外', () async {
      final harness = _repo(body: {'publication': _publicationJson});
      final detail = await harness.repo.getPublicationDetail(_publicationId);
      expect(detail.sellerFacingPayload, const SellerFacingPayload());
      expect(detail.sellerFacingPayload.contextFields, isEmpty);
    });

    test('draft／latestResponse 兩個 key 缺席時，兩個 CAS 都是空字串', () async {
      final harness = _repo(body: {'publication': _publicationJson});
      final detail = await harness.repo.getPublicationDetail(_publicationId);
      expect(detail.draft, isNull);
      expect(detail.latestResponse, isNull);
      expect(detail.expectedDraftVersion, '');
      expect(detail.expectedSellerResponseVersion, '');
      expect(detail.hasSubmittedResponse, isFalse);
    });

    test(
      '兩個 CAS 分別取自 draft.draftVersion 與 latestResponse.sellerResponseVersion',
      () async {
        final harness = _repo(
          body: {
            'publication': _publicationJson,
            'draft': {
              'draftVersion': '5',
              'updatedAt': '2026-09-13T03:00:00Z',
              'draft': _quote().toJson(),
            },
            'latestResponse': {
              'sellerResponseId': 'r-7',
              'sellerResponseVersion': '3',
              'publicationId': _publicationId,
              'status': 'ACTIVE',
              'payloadHash': 'ph-3',
              'submittedAt': '2026-09-13T02:30:00Z',
              'supersededByVersion': '',
              'outsideBuyerRequestedRange': false,
              'quote': _quote().toJson(),
              'replayed': false,
            },
          },
        );
        final detail = await harness.repo.getPublicationDetail(_publicationId);
        expect(detail.expectedDraftVersion, '5');
        expect(detail.expectedSellerResponseVersion, '3');
        expect(detail.hasSubmittedResponse, isTrue);
        expect(detail.draft!.draft.offeredQuality.grade, '特級');
        expect(
          detail.latestResponse!.statusValue,
          ProcurementSellerResponseStatus.active,
        );
        expect(detail.latestResponse!.isLatestVersion, isTrue);
      },
    );

    test('supersededByVersion 是 omitempty，缺席等同空字串（仍是最新版）', () async {
      final harness = _repo(
        body: {
          'publication': _publicationJson,
          'latestResponse': {
            'sellerResponseId': 'r-7',
            'sellerResponseVersion': '3',
            'status': 'ACTIVE',
          },
        },
      );
      final detail = await harness.repo.getPublicationDetail(_publicationId);
      expect(detail.latestResponse!.supersededByVersion, '');
      expect(detail.latestResponse!.isLatestVersion, isTrue);
    });

    test('詳情沒有 publication 這個 key 就是壞回應，不吞成空表單', () async {
      final harness = _repo(
        body: {
          'draft': {'draftVersion': '1'},
        },
      );
      await expectLater(
        harness.repo.getPublicationDetail(_publicationId),
        throwsA(isA<ProcurementException>()),
      );
    });

    test('清單列是 {publication, sellerFacingPayload}，與詳情前兩段同形', () async {
      final harness = _repo(
        body: {
          'publications': [
            {
              'publication': _publicationJson,
              'sellerFacingPayload': _sellerFacingPayloadJson,
            },
          ],
        },
      );
      final rows = await harness.repo.listPublications();
      expect(rows, hasLength(1));
      expect(rows.single.publication.publicationId, _publicationId);
      expect(rows.single.publication.lane, 'PLATFORM');
      expect(rows.single.publication.deliveryState, 'SENT');
      expect(rows.single.sellerFacingPayload.item, '高麗菜');
      expect(rows.single.sellerFacingPayload.quantity, '200');
      expect(harness.adapter.lastRequest!.method, 'GET');
      expect(
        harness.adapter.lastRequest!.path,
        '/api/v1/app/sellers/me/procurement/publications',
      );
    });

    test('清單列與詳情走同一支解析器，出來的兩段完全一致', () async {
      const row = {
        'publication': _publicationJson,
        'sellerFacingPayload': _sellerFacingPayloadJson,
      };
      final listHarness = _repo(
        body: {
          'publications': [row],
        },
      );
      final detailHarness = _repo(body: row);
      final listRow = (await listHarness.repo.listPublications()).single;
      final detail = await detailHarness.repo.getPublicationDetail(
        _publicationId,
      );
      expect(listRow.summary.publication, detail.publication);
      expect(listRow.summary.sellerFacingPayload, detail.sellerFacingPayload);
    });

    test('清單列讀得到 latestResponseStatus 與 hasDraft', () async {
      final harness = _repo(
        body: {
          'publications': [
            {
              'publication': _publicationJson,
              'sellerFacingPayload': _sellerFacingPayloadJson,
              'latestResponseStatus': 'ACTIVE',
              'hasDraft': true,
            },
          ],
        },
      );
      final row = (await harness.repo.listPublications()).single;
      expect(row.latestResponseStatus, 'ACTIVE');
      expect(
        row.latestResponseStatusValue,
        ProcurementSellerResponseStatus.active,
      );
      expect(row.hasSubmittedResponse, isTrue);
      expect(row.hasDraft, isTrue);
    });

    test('⛔ latestResponseStatus 空字串＝還沒提交過正式回價，不是未知', () async {
      final harness = _repo(
        body: {
          'publications': [
            {
              'publication': _publicationJson,
              'sellerFacingPayload': _sellerFacingPayloadJson,
              'latestResponseStatus': '',
              'hasDraft': false,
            },
          ],
        },
      );
      final row = (await harness.repo.listPublications()).single;
      expect(row.latestResponseStatus, '');
      expect(row.hasSubmittedResponse, isFalse);
      expect(row.latestResponseStatusValue, isNull);
    });

    test('認不得的 status code 解析得出來，仍算已提交過', () async {
      final harness = _repo(
        body: {
          'publications': [
            {
              'publication': _publicationJson,
              'sellerFacingPayload': _sellerFacingPayloadJson,
              'latestResponseStatus': 'SOMETHING_NEW',
            },
          ],
        },
      );
      final row = (await harness.repo.listPublications()).single;
      expect(row.latestResponseStatusValue, isNull);
      expect(row.hasSubmittedResponse, isTrue);
    });

    test('兩欄缺席時退成「還沒回價、沒草稿」，不丟例外', () async {
      final harness = _repo(
        body: {
          'publications': [
            {
              'publication': _publicationJson,
              'sellerFacingPayload': _sellerFacingPayloadJson,
            },
          ],
        },
      );
      final row = (await harness.repo.listPublications()).single;
      expect(row.latestResponseStatus, '');
      expect(row.hasDraft, isFalse);
    });

    test('⛔ 詳情就算回了這兩個鍵也不吃——詳情的真相是 draft／latestResponse 物件', () async {
      final harness = _repo(
        body: {
          'publication': _publicationJson,
          'sellerFacingPayload': _sellerFacingPayloadJson,
          'latestResponseStatus': '',
          'hasDraft': false,
          'draft': {'draftVersion': '2', 'draft': _quote().toJson()},
          'latestResponse': {
            'sellerResponseId': 'sr-1',
            'sellerResponseVersion': '3',
            'status': 'ACTIVE',
          },
        },
      );
      final detail = await harness.repo.getPublicationDetail(_publicationId);
      expect(detail.draft, isNotNull);
      expect(detail.expectedDraftVersion, '2');
      expect(detail.hasSubmittedResponse, isTrue);
    });

    test('清單包在 gateway envelope 的 data 底下也讀得到', () async {
      final harness = _repo(
        body: {
          'code': 0,
          'data': {
            'publications': [
              {
                'publication': _publicationJson,
                'sellerFacingPayload': _sellerFacingPayloadJson,
              },
            ],
          },
        },
      );
      final rows = await harness.repo.listPublications();
      expect(rows.single.publication.publicationId, _publicationId);
    });

    test('清單裡沒有 publication 的列跳過，不讓整串清單陪葬', () async {
      final harness = _repo(
        body: {
          'publications': [
            {'sellerFacingPayload': _sellerFacingPayloadJson},
            'not-a-row',
            {
              'publication': _publicationJson,
              'sellerFacingPayload': _sellerFacingPayloadJson,
            },
          ],
        },
      );
      final rows = await harness.repo.listPublications();
      expect(rows, hasLength(1));
      expect(rows.single.publication.publicationId, _publicationId);
    });

    test('清單列的 sellerFacingPayload 一樣是 deny-by-default 的十欄', () async {
      final harness = _repo(
        body: {
          'publications': [
            {
              'publication': _publicationJson,
              'sellerFacingPayload': <String, dynamic>{
                ..._sellerFacingPayloadJson,
                'budgetPerUnit': '38',
                'candidateRank': '2',
              },
            },
          ],
        },
      );
      final rows = await harness.repo.listPublications();
      final serialized = rows.single.sellerFacingPayload.toJson();
      expect(serialized.keys, hasLength(10));
      expect(serialized.values.map((v) => '$v'), isNot(contains('38')));
    });
  });

  group('錯誤碼', () {
    test('409 STALE_SELLER_RESPONSE 轉成要求重新載入的例外', () async {
      final harness = _repo(
        statusCode: 409,
        body: {
          'reason': ProcurementErrorReason.staleSellerResponse.code,
          'message': 'stale seller response',
        },
      );
      await expectLater(
        harness.repo.submitResponse(
          publicationId: _publicationId,
          quote: _quote(),
          expectedPublicationVersion: '4',
          expectedSellerFacingPayloadHash: 'hash-4',
          expectedSellerResponseVersion: '3',
          idempotencyKey: 'k',
        ),
        throwsA(
          isA<ProcurementException>()
              .having((e) => e.code, 'code', 'STALE_SELLER_RESPONSE')
              .having((e) => e.statusCode, 'statusCode', 409)
              .having((e) => e.requiresReload, 'requiresReload', isTrue)
              .having((e) => e.message, 'message', contains('報價已經變動')),
        ),
      );
    });

    test('422 QUOTE_QUALITY_REQUIRED 是可改欄位後重送', () async {
      final harness = _repo(
        statusCode: 422,
        body: {'reason': ProcurementErrorReason.quoteQualityRequired.code},
      );
      await expectLater(
        harness.repo.submitResponse(
          publicationId: _publicationId,
          quote: _quote(),
          expectedPublicationVersion: '4',
          expectedSellerFacingPayloadHash: 'hash-4',
          expectedSellerResponseVersion: '',
          idempotencyKey: 'k',
        ),
        throwsA(
          isA<ProcurementException>()
              .having((e) => e.requiresReload, 'requiresReload', isFalse)
              .having((e) => e.isTerminal, 'isTerminal', isFalse),
        ),
      );
    });

    test('錯誤碼包在 error 物件底下也讀得到', () async {
      final harness = _repo(
        statusCode: 409,
        body: {
          'error': {'reason': ProcurementErrorReason.rfqAlreadyAwarded.code},
        },
      );
      await expectLater(
        harness.repo.declinePublication(
          publicationId: _publicationId,
          idempotencyKey: 'k',
        ),
        throwsA(
          isA<ProcurementException>().having(
            (e) => e.isTerminal,
            'isTerminal',
            isTrue,
          ),
        ),
      );
    });
  });
}
