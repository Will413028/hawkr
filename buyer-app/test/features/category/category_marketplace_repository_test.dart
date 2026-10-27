import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/features/category/data/category_marketplace_model.dart';
import 'package:hawkr_buyer/features/category/data/category_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

class _RecordingAdapter implements HttpClientAdapter {
  _RecordingAdapter({required this.responseBody, this.gate});

  Object responseBody;
  final Completer<void>? gate;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    await gate?.future;
    return ResponseBody.fromString(
      jsonEncode(responseBody),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  group('CategoryMarketplaceData', () {
    test('parses aggregate ProductItem fields without secondary lookups', () {
      final result = CategoryMarketplaceData.fromResponse({
        'code': 0,
        'data': {
          'hotProducts': [
            {
              'id': 'product-1',
              'name': '高麗菜',
              'imageKey': 'products/cabbage.webp',
              // hotProducts 是 handler.ProductItem，賣家鍵吐的是 storeId
              // （e672c0a「商品卡的 accountId 改讀 storeId」配合 backend #2110）。
              'storeId': 'seller-1',
              'sellerName': '南畝王生技',
              'priceMin': '500',
              'priceMax': '650',
            },
            {
              'id': 'product-2',
              'name': '紅豆',
              'imageUrl': 'https://cdn.test/red-bean.webp',
              'priceMin': 195,
              'priceMax': 195,
              // 巢狀 seller 物件的鍵是 storeId + name（不是 accountId/storeName）。
              'seller': {'storeId': 'seller-2', 'name': '中都小農'},
            },
          ],
        },
      });

      expect(result.products, hasLength(2));
      expect(result.products.first.id, 'product-1');
      expect(result.products.first.imageUrl, 'products/cabbage.webp');
      expect(result.products.first.merchantId, 'seller-1');
      expect(result.products.first.merchantName, '南畝王生技');
      expect(result.products.first.navigationPrice, 500);
      expect(result.products.last.merchantId, 'seller-2');
      expect(result.products.last.merchantName, '中都小農');
    });

    test(
      'keeps valid products in a partial response and de-duplicates ids',
      () {
        final result = CategoryMarketplaceData.fromResponse({
          'data': {
            'hotProducts': [
              null,
              {'id': '', 'name': '缺少 ID'},
              {'id': 'valid', 'name': '有效商品', 'priceMin': '80'},
              {'id': 'valid', 'name': '重複商品', 'priceMin': '90'},
              'not-an-object',
            ],
          },
        });

        expect(result.products, hasLength(1));
        expect(result.products.single.name, '有效商品');
      },
    );

    test('treats a missing optional product section as empty', () {
      final result = CategoryMarketplaceData.fromResponse({
        'data': {'recommendedSellers': <Object>[]},
      });

      expect(result.products, isEmpty);
    });

    test('rejects malformed aggregate shapes and business failures', () {
      expect(
        () => CategoryMarketplaceData.fromResponse({
          'data': {'hotProducts': <String, Object>{}},
        }),
        throwsFormatException,
      );
      expect(
        () => CategoryMarketplaceData.fromResponse({
          'code': 4001,
          'data': {'hotProducts': <Object>[]},
        }),
        throwsFormatException,
      );
      expect(
        () => CategoryMarketplaceData.fromResponse(<String, Object>{}),
        throwsFormatException,
      );
    });
  });

  test(
    'generic recommendations use one aggregate request and session cache',
    () async {
      final gate = Completer<void>();
      final adapter = _RecordingAdapter(
        gate: gate,
        responseBody: {
          'data': {
            'hotProducts': [
              {'id': '1', 'name': '小黃瓜'},
            ],
          },
        },
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
        ..httpClientAdapter = adapter;
      addTearDown(dio.close);
      final repository = CategoryRepository(dio);

      final first = repository.fetchMarketplaceRecommendations();
      final concurrent = repository.fetchMarketplaceRecommendations();
      gate.complete();
      final results = await Future.wait([first, concurrent]);
      final cached = await repository.fetchMarketplaceRecommendations();

      expect(results.first.single.id, '1');
      expect(results.last.single.id, '1');
      expect(cached.single.id, '1');
      expect(adapter.requests, hasLength(1));
      expect(adapter.requests.single.path, '/public/marketplace');
      expect(adapter.requests.single.queryParameters, {
        'seller_limit': 0,
        'post_limit': 0,
        'product_limit': 12,
      });

      adapter.responseBody = {
        'data': {'hotProducts': <String, Object>{}},
      };
      await expectLater(
        repository.fetchMarketplaceRecommendations(forceRefresh: true),
        throwsFormatException,
      );
      final cachedAfterFailedRefresh = await repository
          .fetchMarketplaceRecommendations();
      expect(cachedAfterFailedRefresh.single.id, '1');
      expect(adapter.requests, hasLength(2));

      adapter.responseBody = {
        'data': {
          'hotProducts': [
            {'id': '2', 'name': '高麗菜'},
          ],
        },
      };
      repository.invalidateMarketplaceRecommendations();
      final afterInvalidation = await repository
          .fetchMarketplaceRecommendations();

      expect(afterInvalidation.single.id, '2');
      expect(adapter.requests, hasLength(3));
      expect(adapter.requests.map((request) => request.path).toSet(), {
        '/public/marketplace',
      });
    },
  );
}
