import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  late _Adapter adapter;
  late PostRepository repository;

  setUp(() {
    adapter = _Adapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
      ..httpClientAdapter = adapter;
    final container = ProviderContainer(
      overrides: [dioProvider.overrideWithValue(dio)],
    );
    addTearDown(container.dispose);
    repository = container.read(postRepositoryProvider);
  });

  test(
    'owner feed reads the owner endpoint across pages, never a public route',
    () async {
      adapter.pages = [
        _page([_post(1)], total: 51, page: 1),
        _page([_post(2)], total: 51, page: 2),
      ];

      final posts = await repository.listMyPosts();

      expect(posts.map((p) => p.id), [1, 2]);
      expect(adapter.paths, [
        '/api/v1/app/sellers/me/posts',
        '/api/v1/app/sellers/me/posts',
      ]);
      expect(adapter.queries.map((q) => q['page']), ['1', '2']);
      expect(adapter.queries.map((q) => q['page_size']), ['50', '50']);
    },
  );

  test(
    'linked products are enriched from the owner product endpoint',
    () async {
      adapter.pages = [
        _page(
          [
            _post(
              1,
              product: {'id': '900', 'name': '', 'price': '', 'imageKey': ''},
            ),
          ],
          total: 1,
          page: 1,
        ),
      ];
      adapter.productDetail = {
        'id': '900',
        'name': '青江菜',
        'priceMin': '120',
        'imageKeys': ['p/900/a.jpg'],
      };

      final posts = await repository.listMyPosts();

      expect(adapter.paths, [
        '/api/v1/app/sellers/me/posts',
        '/api/v1/app/products/900',
      ]);
      expect(posts.single.linkedProduct?['name'], '青江菜');
      expect(posts.single.linkedProduct?['price'], '120');
    },
  );
}

Map<String, dynamic> _post(int id, {Map<String, dynamic>? product}) => {
  'id': id,
  'sellerId': 7,
  'contentType': 'IMAGE',
  'content': '貼文$id',
  if (product != null) 'product': product,
};

Map<String, dynamic> _page(
  List<Map<String, dynamic>> records, {
  required int total,
  required int page,
}) => {
  'code': 0,
  'data': {
    'records': records,
    'total': '$total',
    'page': '$page',
    'pageSize': '50',
  },
};

class _Adapter implements HttpClientAdapter {
  List<Map<String, dynamic>> pages = [];
  Map<String, dynamic>? productDetail;
  final paths = <String>[];
  final queries = <Map<String, dynamic>>[];
  var _next = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    paths.add(options.path);
    queries.add(options.queryParameters);
    final Map<String, dynamic> body;
    if (options.path.contains('/products/')) {
      body = productDetail == null
          ? {'code': 404, 'message': 'not found'}
          : {'code': 0, 'data': productDetail};
    } else {
      body = pages[_next++];
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
