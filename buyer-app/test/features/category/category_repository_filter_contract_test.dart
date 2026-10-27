import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/features/category/data/category_repository.dart';
import 'package:hawkr_buyer/features/category/data/product_item_model.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

class _RecordingAdapter implements HttpClientAdapter {
  RequestOptions? lastRequest;
  String responseBody = jsonEncode({
    'data': {'records': <Object>[], 'total': 0, 'page': 1, 'pageSize': 20},
  });

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastRequest = options;
    return ResponseBody.fromString(
      responseBody,
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
  test('forwards only confirmed /public/products filter parameters', () async {
    final adapter = _RecordingAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;
    addTearDown(() => dio.close());
    final repository = CategoryRepository(dio);

    await repository.fetchProducts(
      categoryId: '2201',
      keyword: '虱目魚',
      sort: 'PRICE_ASC',
      deliveryTemp: '3203',
      priceMin: '100',
      priceMax: '500',
      hashtags: const ['冷凍水產', '當季'],
      certificationTypes: const ['3701', '3702'],
      followedSellerOnly: true,
      page: 2,
      pageSize: 24,
    );

    final request = adapter.lastRequest!;
    expect(request.path, '/public/products');
    expect(request.queryParameters, {
      'page': 2,
      'page_size': 24,
      'category_id': '2201',
      'keyword': '虱目魚',
      'sort': 'PRICE_ASC',
      'delivery_temp': '3203',
      'price_min': '100',
      'price_max': '500',
      'hashtags': '冷凍水產,當季',
      'certification_types': '3701,3702',
      'followed_seller_only': true,
    });
    expect(request.queryParameters, isNot(contains('origin')));
    expect(request.queryParameters, isNot(contains('distance')));
  });

  test('omits empty optional filters for recommended results', () async {
    final adapter = _RecordingAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;
    addTearDown(() => dio.close());

    await CategoryRepository(dio).fetchProducts(
      categoryId: '0',
      keyword: '',
      sort: '',
      deliveryTemp: '',
      priceMin: '',
      priceMax: '',
    );

    expect(adapter.lastRequest!.queryParameters, {'page': 1, 'page_size': 20});
  });

  test('normalises real hashtag payloads without inventing labels', () {
    final product = ProductItem.fromJson({
      'id': '1',
      'name': '小花枝',
      'hashtags': [
        '#冷凍水產',
        {'name': '花枝'},
        {'name': '冷凍水產'},
        {'id': 'missing-name'},
      ],
    });

    expect(product.hashtags, ['冷凍水產', '花枝']);
  });

  test('product cards prefer the confirmed seller logo key/url contract', () {
    expect(
      ProductItem.fromJson({
        'id': '1',
        'name': '文旦',
        'sellerLogoKey': 'public/sellers/logo.png',
        'sellerLogoUrl': 'https://media.example/logo.png',
      }).sellerAvatar,
      'public/sellers/logo.png',
    );
    expect(
      ProductItem.fromJson({
        'id': '2',
        'name': '水桃',
        'seller': {'logoUrl': 'https://media.example/nested.png'},
      }).sellerAvatar,
      'https://media.example/nested.png',
    );
  });

  test('product cards ignore retired guessed seller-avatar aliases', () {
    final product = ProductItem.fromJson({
      'id': '1',
      'name': '文旦',
      'sellerAvatarUrl': 'https://dead.example/1.png',
      'sellerAvatar': 'https://dead.example/2.png',
      'avatar': 'https://dead.example/3.png',
      'logo': 'https://dead.example/4.png',
      'seller_logo_key': 'dead/5.png',
    });

    expect(product.sellerAvatar, isNull);
  });

  test('parses and de-duplicates popular hashtag records', () async {
    final adapter = _RecordingAdapter()
      ..responseBody = jsonEncode({
        'data': {
          'records': [
            {'hashtag': '#冷凍水產', 'count': '8'},
            {'hashtag': '冷凍水產', 'count': '7'},
            {'hashtag': '當季', 'count': '5'},
            {'count': '1'},
          ],
        },
      });
    final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;
    addTearDown(() => dio.close());

    final result = await CategoryRepository(dio).fetchPopularHashtags(limit: 6);

    expect(adapter.lastRequest!.path, '/public/hashtags/popular');
    expect(adapter.lastRequest!.queryParameters, {'limit': 6});
    expect(result, ['冷凍水產', '當季']);
  });

  test('parses recommended seller stories from marketplace', () async {
    final adapter = _RecordingAdapter()
      ..responseBody = jsonEncode({
        'data': {
          'recommendedSellers': [
            {
              'storeId': '9001',
              'storeName': '金湖水產',
              // 真實 /public/marketplace 契約回 logo_key（objectKey），非舊 logo。
              // 讀空→預設牛圖的回歸鎖：誰再改回只讀 logo 這裡就 fail。
              'logoKey': 'seller.png',
              'city': '高雄市',
              'district': '前鎮區',
            },
            {
              'storeId': '9002',
              'storeName': '只有封面的商家',
              'coverUrl': 'cover-must-not-be-avatar.png',
              'city': '基隆市',
              'district': '信義區',
            },
            {'storeId': '', 'storeName': '無效商家'},
          ],
        },
      });
    final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;
    addTearDown(() => dio.close());

    final result = await CategoryRepository(
      dio,
    ).fetchRecommendedSellers(limit: 6);

    expect(adapter.lastRequest!.path, '/public/marketplace');
    expect(adapter.lastRequest!.queryParameters, {
      'seller_limit': 6,
      'post_limit': 0,
      'product_limit': 0,
    });
    expect(result, hasLength(2));
    expect(result.first.name, '金湖水產');
    expect(result.first.avatarUrl, 'seller.png');
    expect(result.first.location, '高雄市前鎮區');
    expect(result.last.name, '只有封面的商家');
    expect(result.last.avatarUrl, isEmpty);
  });
}
