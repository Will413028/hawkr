import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_buyer/features/social/data/social_repository.dart';

import '../../support/fake_seller_repository.dart';

/// 貼文只帶 sellerId、沒有 sellerName 的列表；記下所有請求路徑。
class _PostsAdapter implements HttpClientAdapter {
  final paths = <String>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    paths.add(options.path);
    return ResponseBody.fromString(
      jsonEncode({
        'code': 0,
        'data': {
          'records': [
            {'id': 101, 'sellerId': 7},
            {'id': 102, 'sellerId': 7},
            {'id': 103, 'sellerId': 8, 'sellerName': '已有名稱'},
          ],
        },
      }),
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
    'fills seller name and logo from one BatchGetStores, not /public/sellers/{id}',
    () async {
      final adapter = _PostsAdapter();
      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = adapter;
      addTearDown(dio.close);
      final batches = <List<String>>[];
      final repository = SocialRepository(
        dio,
        sellers: fakeSellerRepository(
          cards: [StoreCard(id: '7', name: '南畝王生技')],
          batches: batches,
        ),
      );

      final posts = await repository.getPosts();

      expect(posts.map((p) => p.merchantName), ['南畝王生技', '南畝王生技', '已有名稱']);
      expect(batches, [
        ['7'],
      ]);
      expect(
        adapter.paths.where((p) => p.startsWith('/public/sellers/')),
        isEmpty,
      );
    },
  );

  test('a failed card read leaves the posts as they are', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
      ..httpClientAdapter = _PostsAdapter();
    addTearDown(dio.close);
    final repository = SocialRepository(
      dio,
      sellers: fakeSellerRepository(failing: () => true),
    );

    final posts = await repository.getPosts();

    expect(posts, hasLength(3));
    expect(posts.first.merchantName, isNull);
  });
}
