import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/features/category/data/category_repository.dart';
import 'package:hawkr_buyer/features/category/data/product_item_model.dart';
import 'package:dio/dio.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/gen/hawkr/discovery/v1alpha1/discovery.pb.dart'
    as discovery;
import '../../support/fake_discovery_client.dart';
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
    final repository = CategoryRepository(dio, fakeDiscoveryClient());

    await repository.fetchProducts(
      categoryId: '2201',
      keyword: '虱目魚',
      sort: 'PRICE_ASC',
      priceMin: '100',
      priceMax: '500',
      hashtags: const ['冷凍水產', '當季'],
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
      'price_min': '100',
      'price_max': '500',
      'hashtags': '冷凍水產,當季',
      'followed_seller_only': true,
    });
    expect(request.queryParameters, isNot(contains('delivery_temp')));
    expect(request.queryParameters, isNot(contains('certification_types')));
    expect(request.queryParameters, isNot(contains('origin')));
    expect(request.queryParameters, isNot(contains('distance')));
  });

  test('omits empty optional filters for recommended results', () async {
    final adapter = _RecordingAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;
    addTearDown(() => dio.close());

    await CategoryRepository(dio, fakeDiscoveryClient()).fetchProducts(
      categoryId: '0',
      keyword: '',
      sort: '',
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

    final result = await CategoryRepository(
      dio,
      fakeDiscoveryClient(),
    ).fetchPopularHashtags(limit: 6);

    expect(adapter.lastRequest!.path, '/public/hashtags/popular');
    expect(adapter.lastRequest!.queryParameters, {'limit': 6});
    expect(result, ['冷凍水產', '當季']);
  });

  test(
    'recommended seller stories come from SuggestRecommendedSellers',
    () async {
      final requests = <discovery.SuggestRecommendedSellersRequest>[];
      final repository = CategoryRepository(
        Dio(),
        fakeDiscoveryClient(
          requests: requests,
          sellers: [
            discovery.RecommendedSeller(
              store: StoreCard(
                id: '9001',
                storeCode: 'jinhu',
                name: '金湖水產',
                status: StoreCardStatus.STORE_CARD_STATUS_OPEN,
                townCode: '64000090',
                logo: Image(
                  uriTemplate: 'https://cdn.test/seller/{preset}.png',
                ),
              ),
            ),
            discovery.RecommendedSeller(
              store: StoreCard(
                id: '9002',
                storeCode: 'cover-only',
                name: '只有封面的商家',
                status: StoreCardStatus.STORE_CARD_STATUS_OPEN,
                coverImage: Image(
                  uriTemplate: 'https://cdn.test/cover/{preset}.png',
                ),
              ),
            ),
            discovery.RecommendedSeller(
              store: StoreCard(
                id: '9003',
                storeCode: 'paused',
                name: '暫停的商家',
                status: StoreCardStatus.STORE_CARD_STATUS_PAUSED,
              ),
            ),
          ],
        ),
      );

      final result = await repository.fetchRecommendedSellers(limit: 6);

      expect(requests.single.maxResults, 6);
      expect(result.map((s) => s.id), ['9001', '9002']);
      expect(result.first.storeCode, 'jinhu');
      expect(result.first.name, '金湖水產');
      expect(result.first.avatarUrl, 'https://cdn.test/seller/avatar.png');
      expect(result.first.location, '高雄市前鎮區');
      expect(result.last.avatarUrl, isEmpty);
      expect(result.last.location, isEmpty);
    },
  );
}
