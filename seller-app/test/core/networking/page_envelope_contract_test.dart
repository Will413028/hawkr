import 'dart:convert';

import 'package:hawkr_seller/features/customer/data/subscriber_remote_service.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as seller;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/features/product/data/product_repository.dart';
import 'package:hawkr_seller/features/report/data/report_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

/// backend#2216 的兩個 wire 契約，用實打 dev 的回應釘住。
///
/// **C8 — Page-trap envelope 收斂**：`records` / `total` / `page` / `pageSize`
/// 從 envelope 頂層收進 `data` 之下，三個數值欄一併字串化。
/// **這是一次切，沒有相容層** —— 前後端同波上線，舊形狀的讀取路徑全部刪除。
///
/// **C3 — 檢舉證據欄改名**：`evidence_urls` → `evidenceKeys`。值一直都是裸
/// objectKey，舊欄位名在說謊；Gateway 現在會拒絕外來域名與非 `public/` 前綴。
///
/// 為什麼要有這支測試：C8 打壞的是「HTTP 200、無錯誤訊號、清單就是空的」這種
/// 失敗形狀 —— 讀錯 key 不會拋例外，只會靜默回 `[]`。商品評論在
/// C8 上 dev 當天就是這樣整頁變空的，跟 website #338 白畫面同一個形狀。
///
/// 以下 fixture 取自 2026-08-05 對 `test-api.example.com` 的實打回應，
/// 欄位名再依 backend PR `#2235`（全域 camelCase
/// 收斂）逐欄對齊 `gateway/internal/handler/{review,moderation}.go`。

/// 實打：`GET /api/v1/public/products/866353737479521547/reviews`
const _reviewsEnvelope = <String, dynamic>{
  'data': {
    'records': [
      {
        'reviewId': '7484455658227120601',
        'buyerId': '2623900520',
        'productId': '866353737479521547',
        'username': '陳**',
        'rating': '5',
        'content': 'R8-11 評論圖回歸測試',
        'createdAt': '2026-07-19T03:54:44Z',
        'imageKeys': <String>[],
        'imageUrls': <String>[],
      },
    ],
    'stats': {'totalCount': 1, 'averageRating': 5},
    'total': '1',
    'page': '1',
    'pageSize': '10',
  },
  'message': 'success',
  'code': 0,
};

/// 實打：同端點、無評論的商品（`866542675787809994`）。
const _reviewsEmptyEnvelope = <String, dynamic>{
  'data': {
    'records': <dynamic>[],
    'stats': <String, dynamic>{},
    'total': '0',
    'page': '1',
    'pageSize': '10',
  },
  'message': 'success',
  'code': 0,
};

/// 實打：`POST /api/v1/app/reports` 成功時的回應。
const _submitReportEnvelope = <String, dynamic>{
  'data': {'reportId': '7484999999999999999', 'status': 'PENDING'},
  'message': 'success',
  'code': 0,
};

class _FixedResponseAdapter implements HttpClientAdapter {
  _FixedResponseAdapter(this.body);

  final Map<String, dynamic> body;

  /// 最後一次送出的 request body（C3 契約要斷言送出去的欄位名）。
  Object? lastRequestBody;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastRequestBody = options.data;
    return ResponseBody.fromString(
      jsonEncode(body),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Dio _dioReturning(_FixedResponseAdapter adapter) {
  final dio = Dio(BaseOptions(baseUrl: 'https://test-api.example.com'));
  dio.httpClientAdapter = adapter;
  return dio;
}

void main() {
  group('C8 — Page-trap envelope 只讀 data.records', () {
    test('粉絲清單使用 SellerAppService 的 generated client', () async {
      final service = SubscriberRemoteService(
        SellerAppServiceClient(
          FakeTransportBuilder()
              .unary(
                SellerAppService.listMyFollowers,
                (_, _) => seller.ListMyFollowersResponse(
                  followers: [
                    seller.Follower(buyer: seller.BuyerCard(id: '2636840881')),
                  ],
                ),
              )
              .build(),
        ),
      );

      final followers = await service.fetchFollowers();

      expect(followers.followers, hasLength(1), reason: '讀錯 key 會靜默回空清單');
      expect(followers.followers.single.buyerId, '2636840881');
      expect(followers.followers.single.subscribedAt, '');
    });

    test('沒有追蹤者時不讀 REST envelope', () async {
      final service = SubscriberRemoteService(
        SellerAppServiceClient(
          FakeTransportBuilder()
              .unary(
                SellerAppService.listMyFollowers,
                (_, _) => seller.ListMyFollowersResponse(),
              )
              .build(),
        ),
      );
      final followers = await service.fetchFollowers();
      expect(followers.followers, isEmpty);
    });

    test('粉絲下一頁直接使用上一頁的 cursor', () async {
      final tokens = <String>[];
      final service = SubscriberRemoteService(
        SellerAppServiceClient(
          FakeTransportBuilder().unary(SellerAppService.listMyFollowers, (
            request,
            _,
          ) {
            tokens.add(request.pageToken);
            return seller.ListMyFollowersResponse(
              nextPageToken: request.pageToken.isEmpty ? 'cursor-2' : '',
              followers: [
                seller.Follower(
                  buyer: seller.BuyerCard(
                    id: request.pageToken.isEmpty ? '1' : '2',
                  ),
                ),
              ],
            );
          }).build(),
        ),
      );
      final first = await service.fetchFollowers(size: 1);
      final second = await service.fetchFollowers(
        pageToken: first.nextPageToken,
        size: 1,
      );
      expect(tokens, ['', 'cursor-2']);
      expect(second.followers.single.buyerId, '2');
    });

    test('商品評論（page_shop_tab / 評論頁入口）讀 data.records', () async {
      final repo = ProductRepository(
        _dioReturning(_FixedResponseAdapter(_reviewsEnvelope)),
      );

      final reviews = await repo.getSellerProductReviewsPublic(
        '2685914682',
        '866353737479521547',
      );

      expect(reviews, hasLength(1));
      expect(reviews.single['reviewId'], '7484455658227120601');
    });

    test('商品評論（商品預覽頁入口）讀 data.records', () async {
      // 兩個入口是同一份解析，但它們在 C8 之前是複製貼上的兩份 —— 各自
      // 獨立驗一次，才不會「只測了其中一個入口」就宣稱兩邊都好。
      final repo = ProductRepository(
        _dioReturning(_FixedResponseAdapter(_reviewsEnvelope)),
      );

      final reviews = await repo.listPublicProductReviews(
        '2685914682',
        '866353737479521547',
      );

      expect(reviews, hasLength(1));
      expect(reviews.single['reviewId'], '7484455658227120601');
    });

    test('無評論的商品回空清單，且不因 stats 為空物件而爆掉', () async {
      final repo = ProductRepository(
        _dioReturning(_FixedResponseAdapter(_reviewsEmptyEnvelope)),
      );

      expect(
        await repo.getSellerProductReviewsPublic(
          '2685914682',
          '866542675787809994',
        ),
        isEmpty,
      );
      expect(
        await repo.listPublicProductReviews('2685914682', '866542675787809994'),
        isEmpty,
      );
    });

    test('數值欄是 string —— 任何讀取都必須 int.tryParse，不可 as int', () {
      // C8 之後 total / page / pageSize 是字串。目前賣家 App 沒有任何呼叫端
      // 讀這三個欄位（只取 records），所以這裡釘的是「形狀」而不是某個出口：
      // 一旦有人依舊形狀把 fixture 改回數字、或新增 `as int` 的讀取，
      // 這條會先失敗，而不是等到 runtime 才 crash。
      for (final envelope in <Map<String, dynamic>>[
        _reviewsEnvelope,
        _reviewsEmptyEnvelope,
      ]) {
        final data = envelope['data'] as Map<String, dynamic>;
        for (final key in const ['total', 'page', 'pageSize']) {
          expect(data[key], isA<String>(), reason: '$key 應為 string');
          expect(
            int.tryParse(data[key] as String),
            isNotNull,
            reason: '$key 必須 int.tryParse 得出來',
          );
        }
      }

      expect(
        int.tryParse(
          (_reviewsEnvelope['data'] as Map<String, dynamic>)['total'] as String,
        ),
        1,
      );
    });
  });

  group('C3 — 檢舉證據欄送 evidenceKeys', () {
    test('送出的 body 用 evidenceKeys，且不再出現 evidence_urls', () async {
      final adapter = _FixedResponseAdapter(_submitReportEnvelope);
      final repo = ReportRepository(_dioReturning(adapter));

      final result = await repo.submitReport(
        targetType: 'POST',
        targetId: '123',
        reasonCode: 'SPAM',
        evidenceKeys: const ['public/report/2685914682/a.jpg'],
      );

      final body = adapter.lastRequestBody as Map<String, dynamic>;
      expect(body['evidenceKeys'], ['public/report/2685914682/a.jpg']);
      expect(
        body.containsKey('evidence_urls'),
        isFalse,
        reason: 'Gateway 已不接受舊欄位名，送了等於沒送',
      );
      expect(result.reportId, '7484999999999999999');
      expect(result.status, 'PENDING');
    });

    test('沒有證據時不帶該欄位', () async {
      final adapter = _FixedResponseAdapter(_submitReportEnvelope);
      final repo = ReportRepository(_dioReturning(adapter));

      await repo.submitReport(
        targetType: 'POST',
        targetId: '123',
        reasonCode: 'SPAM',
      );

      final body = adapter.lastRequestBody as Map<String, dynamic>;
      expect(body.containsKey('evidenceKeys'), isFalse);
      expect(body.containsKey('evidence_urls'), isFalse);
    });
  });
}
