import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/home/data/home_product_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

class _RecordingAdapter implements HttpClientAdapter {
  _RecordingAdapter(this.responses);

  final List<Object> responses;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    final response = responses[requests.length - 1];
    return ResponseBody.fromString(
      jsonEncode(response),
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
  test(
    'requests the page contract and parses typed compatibility fields',
    () async {
      final adapter = _RecordingAdapter([
        {
          'code': 0,
          'data': {
            'records': [
              {
                'id': 'product-1',
                'name': '高麗菜',
                'imageKey': 'products/cabbage.webp',
                'priceMin': 120,
                'priceMax': 150,
                'listPriceMin': 180,
                'listPriceMax': 200,
                'seller': {
                  'storeId': 'seller-1',
                  'name': '南畝王生技',
                  'logoUrl': 'https://cdn.test/seller.webp',
                },
                'description': {
                  'blocks': [
                    {'type': 'text', 'content': '今天採收 #產地直送 #新鮮'},
                  ],
                },
              },
              {'id': '', 'name': '缺少 ID'},
            ],
            'page': 2,
            'pageSize': 12,
            'total': 30,
          },
        },
      ]);
      final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
        ..httpClientAdapter = adapter;
      addTearDown(dio.close);

      final page = await DioHomeProductRepository(
        dio,
      ).fetchPage(page: 2, pageSize: 12);

      expect(adapter.requests, hasLength(1));
      expect(adapter.requests.single.path, '/public/products');
      expect(adapter.requests.single.queryParameters, {
        'page': 2,
        'page_size': 12,
      });
      expect(page.page, 2);
      expect(page.pageSize, 12);
      expect(page.total, 30);
      expect(page.hasMore, isTrue);
      expect(page.products, hasLength(1));
      expect(page.products.single.id, 'product-1');
      expect(page.products.single.imageUrl, 'products/cabbage.webp');
      expect(page.products.single.priceMin, '120');
      expect(page.products.single.sellerAccountId, 'seller-1');
      expect(page.products.single.sellerName, '南畝王生技');
      expect(page.products.single.hashtags, ['產地直送', '新鮮']);
      expect(
        () => page.products.single.hashtags.add('不可變更'),
        throwsUnsupportedError,
      );
    },
  );

  test('uses page length when total is absent', () async {
    final adapter = _RecordingAdapter([
      {
        'data': {
          'records': List.generate(
            2,
            (index) => {'id': '$index', 'name': '商品 $index'},
          ),
        },
      },
    ]);
    final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;
    addTearDown(dio.close);

    final page = await DioHomeProductRepository(
      dio,
    ).fetchPage(page: 1, pageSize: 3);

    expect(page.hasMore, isFalse);
    expect(page.products, hasLength(2));
  });

  test('surfaces business and page schema failures', () async {
    final adapter = _RecordingAdapter([
      {
        'code': 4001,
        'message': '推薦服務暫時不可用',
        'data': {'records': <Object>[]},
      },
      {
        'code': 0,
        'data': {'records': <String, Object>{}},
      },
    ]);
    final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;
    addTearDown(dio.close);
    final repository = DioHomeProductRepository(dio);

    await expectLater(
      repository.fetchPage(page: 1, pageSize: 12),
      throwsA(
        isA<ServerException>().having(
          (error) => error.message,
          'message',
          '推薦服務暫時不可用',
        ),
      ),
    );
    await expectLater(
      repository.fetchPage(page: 1, pageSize: 12),
      throwsA(
        isA<ServerException>().having(
          (error) => error.message,
          'message',
          '商品資料格式錯誤，請稍後再試',
        ),
      ),
    );
  });
}
