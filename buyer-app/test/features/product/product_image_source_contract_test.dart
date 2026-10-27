import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/features/product/data/product_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// 商品圖片的資料來源契約。
///
/// 背景：舊碼打 `GET /public/sellers/{id}/products/{sku}/images`，但 Gateway 沒有
/// 這條路由（2026-08-05 對 dev 實打回 gin 純文字 `404 page not found`），DioException
/// 被 catch 吞掉 → 永遠回空陣列。正解是商品詳情／商品列表 payload 裡既有的
/// `imageKeys`／`imageUrls`（49 號成對契約，key 優先）。
class _StubAdapter implements HttpClientAdapter {
  _StubAdapter(this.responses);

  /// path → JSON body
  final Map<String, Map<String, dynamic>> responses;
  final requestedPaths = <String>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requestedPaths.add(options.path);
    final body = responses[options.path];
    if (body == null) {
      // 模擬 gin 對未註冊路由的回應：純文字，不是 JSON envelope。
      return ResponseBody.fromString('404 page not found', 404);
    }
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

Dio _dioWith(_StubAdapter adapter) {
  final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
    ..httpClientAdapter = adapter;
  addTearDown(dio.close);
  return dio;
}

ProductRepository _repository(Dio dio) => ProductRepository(
  dio,
  BuyerAppServiceClient(FakeTransportBuilder().build()),
);

void main() {
  group('商品圖片資料來源', () {
    test('getProductImages 讀商品詳情的 imageKeys，且不打死掉的 /images 路由', () async {
      final adapter = _StubAdapter({
        '/public/products/866542675787809994': {
          'code': 0,
          'message': 'success',
          'data': {
            'id': '866542675787809994',
            'imageKey': 'public/product/a.jpg',
            'imageUrl': 'https://cdn.example/a.jpg',
            'imageKeys': ['public/product/a.jpg', 'public/product/b.jpg'],
            'imageUrls': [
              'https://cdn.example/a.jpg',
              'https://cdn.example/b.jpg',
            ],
          },
        },
      });

      final images = await _repository(
        _dioWith(adapter),
      ).getProductImages('866542675787809994');

      expect(images, ['public/product/a.jpg', 'public/product/b.jpg']);
      expect(adapter.requestedPaths, ['/public/products/866542675787809994']);
      expect(
        adapter.requestedPaths.where((p) => p.endsWith('/images')),
        isEmpty,
        reason: '不得再打 /public/sellers/{id}/products/{sku}/images',
      );
    });

    test('getProductImages 在只有封面圖時退回 imageKey', () async {
      final adapter = _StubAdapter({
        '/public/products/1': {
          'code': 0,
          'data': {'id': '1', 'imageKey': 'public/product/cover.jpg'},
        },
      });

      final images = await _repository(_dioWith(adapter)).getProductImages('1');

      expect(images, ['public/product/cover.jpg']);
    });

    test('getMerchantFirstProductImage 讀賣家商品列表既有的圖片欄位', () async {
      final adapter = _StubAdapter({
        '/public/sellers/2685914682/products': {
          'code': 0,
          'data': {
            'records': [
              {'id': '1', 'name': '無圖商品'},
              {
                'id': '2',
                'imageKeys': ['public/product/first.jpg'],
              },
            ],
          },
        },
      });

      final url = await _repository(
        _dioWith(adapter),
      ).getMerchantFirstProductImage('2685914682');

      expect(url, 'public/product/first.jpg');
      expect(adapter.requestedPaths, ['/public/sellers/2685914682/products']);
    });

    test('空 id 不送出任何請求', () async {
      final adapter = _StubAdapter({});
      final repo = _repository(_dioWith(adapter));

      expect(await repo.getProductImages(''), isEmpty);
      expect(await repo.getMerchantFirstProductImage(''), '');
      expect(adapter.requestedPaths, isEmpty);
    });
  });
}
