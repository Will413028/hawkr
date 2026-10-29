import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/features/category/data/category_marketplace_model.dart';
import 'package:hawkr_buyer/features/category/data/category_repository.dart';
import 'package:hawkr_buyer/features/category/data/product_item_model.dart';
import '../../support/fake_discovery_client.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

class _RecordingAdapter implements HttpClientAdapter {
  _RecordingAdapter({required this.responseBody, this.gate});

  Object responseBody;
  final Completer<void>? gate;
  int status = 200;
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
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  group('CategoryMarketplaceProduct.fromProductItem', () {
    test('keeps the card fields of a product list record', () {
      final product = CategoryMarketplaceProduct.fromProductItem(
        ProductItem.fromJson({
          'id': 'product-1',
          'name': '高麗菜',
          'imageKey': 'products/cabbage.webp',
          'storeId': 'seller-1',
          'sellerName': '南畝王生技',
          'priceMin': '500',
          'priceMax': '650',
        }),
      );

      expect(product!.id, 'product-1');
      expect(product.imageUrl, 'products/cabbage.webp');
      expect(product.merchantId, 'seller-1');
      expect(product.merchantName, '南畝王生技');
      expect(product.navigationPrice, 500);
    });

    test('reads the seller from the nested seller object', () {
      final product = CategoryMarketplaceProduct.fromProductItem(
        ProductItem.fromJson({
          'id': 'product-2',
          'name': '紅豆',
          'imageUrl': 'https://cdn.test/red-bean.webp',
          'priceMin': 195,
          'seller': {'storeId': 'seller-2', 'name': '中都小農'},
        }),
      );

      expect(product!.merchantId, 'seller-2');
      expect(product.merchantName, '中都小農');
    });

    test('rejects records without an id or a name', () {
      expect(
        CategoryMarketplaceProduct.fromProductItem(
          ProductItem.fromJson({'id': '', 'name': '缺少 ID'}),
        ),
        isNull,
      );
      expect(
        CategoryMarketplaceProduct.fromProductItem(
          ProductItem.fromJson({'id': '1', 'name': ' '}),
        ),
        isNull,
      );
    });
  });

  test(
    'generic recommendations use one SALES-sorted request and session cache',
    () async {
      final gate = Completer<void>();
      final adapter = _RecordingAdapter(
        gate: gate,
        responseBody: {
          'data': {
            'records': [
              {'id': '1', 'name': '小黃瓜'},
            ],
          },
        },
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
        ..httpClientAdapter = adapter;
      addTearDown(dio.close);
      final repository = CategoryRepository(dio, fakeDiscoveryClient());

      final first = repository.fetchMarketplaceRecommendations();
      final concurrent = repository.fetchMarketplaceRecommendations();
      gate.complete();
      final results = await Future.wait([first, concurrent]);
      final cached = await repository.fetchMarketplaceRecommendations();

      expect(results.first.single.id, '1');
      expect(results.last.single.id, '1');
      expect(cached.single.id, '1');
      expect(adapter.requests, hasLength(1));
      expect(adapter.requests.single.path, '/public/products');
      expect(adapter.requests.single.queryParameters, {
        'page': 1,
        'page_size': 12,
        'sort': 'SALES',
      });

      adapter.status = 500;
      await expectLater(
        repository.fetchMarketplaceRecommendations(forceRefresh: true),
        throwsA(isA<DioException>()),
      );
      final cachedAfterFailedRefresh = await repository
          .fetchMarketplaceRecommendations();
      expect(cachedAfterFailedRefresh.single.id, '1');
      expect(adapter.requests, hasLength(2));

      adapter.status = 200;
      adapter.responseBody = {
        'data': {
          'records': [
            {'id': '2', 'name': '高麗菜'},
          ],
        },
      };
      repository.invalidateMarketplaceRecommendations();
      final afterInvalidation = await repository
          .fetchMarketplaceRecommendations();

      expect(afterInvalidation.single.id, '2');
      expect(adapter.requests, hasLength(3));
    },
  );
}
