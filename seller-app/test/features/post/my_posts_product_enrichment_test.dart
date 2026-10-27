import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

// ── fixture 全部取自 2026-08-05 對 dev 的實打回應 ─────────────────────────
// `GET /app/sellers/me/posts` 的 record 把關聯商品放在**單數 `product`**
// （`PostListItemData.Product`），只有 `imageKey`／`imageUrl`，沒有複數欄位；
// 剛建立的貼文其 product 欄位值全為空字串（不是缺欄位），要靠 enrichment 補。
// `GET /public/products/{id}` 的 `data` 回 `imageKeys`（objectKey 陣列）。
const String _key =
    'public/product/2685914682/20260719/cd527201-0b42-417d-8476-98d780c92a47.jpg';

class _PostResponseAdapter implements HttpClientAdapter {
  final requestedPaths = <String>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requestedPaths.add(options.path);

    final Map<String, dynamic> body = switch (options.path) {
      '/api/v1/app/sellers/me/posts' => {
        'code': 0,
        'message': 'success',
        'data': {
          'records': [
            {
              'id': '7490603907356134565',
              'seller_id': '2685914682',
              'content': '測試貼文',
              'imageKey': '',
              'imageUrl': '',
              'product': {
                'name': '',
                'imageKey': '',
                'imageUrl': '',
                'price': '',
                'id': '866542675787809994',
              },
            },
          ],
          'total': '1',
          'page': '1',
          'pageSize': '20',
        },
      },
      '/api/v1/app/products/866542675787809994' => {
        'code': 0,
        'message': 'success',
        'data': {
          'id': '866542675787809994',
          'name': '[範例] 快樂農夫 三地門 40年老欉 土芒果青',
          'priceMin': '320',
          'imageKey': _key,
          'imageKeys': [_key],
        },
      },
      _ => {'code': 404, 'message': 'unexpected ${options.path}'},
    };

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

void main() {
  test('自己的貼文列表：單數 product 會被收斂成 linkedProduct，空欄位補齊成 objectKey', () async {
    final adapter = _PostResponseAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://test-api.example.com'))
      ..httpClientAdapter = adapter;
    final repositoryProvider = Provider<PostRepository>(
      (ref) => PostRepository(dio, ref),
    );
    final container = ProviderContainer();
    addTearDown(container.dispose);
    final repository = container.read(repositoryProvider);

    final result = await repository.listMyPostsRaw();

    // 列表回應本身已帶完整圖片欄位（imageKey/imageUrl），不該再對每篇貼文
    // 補打一次 /public/posts/{id}——2026-07-31 排查 N+1 bug 移除的正是這段。
    expect(
      adapter.requestedPaths,
      containsAllInOrder([
        '/api/v1/app/sellers/me/posts',
        '/api/v1/app/products/866542675787809994',
      ]),
    );
    expect(
      adapter.requestedPaths,
      isNot(contains('/api/v1/public/posts/7490603907356134565')),
    );

    final linked = result.posts.single['linkedProduct'] as Map<String, dynamic>;
    // 回填的是 objectKey（49 §6.1 跨層只帶 key），欄位名與後端一致。
    expect(linked['imageKey'], _key);
    expect(linked['name'], '[範例] 快樂農夫 三地門 40年老欉 土芒果青');
    expect(linked['price'], '320');
    expect(linked['id'], '866542675787809994');
  });

  test(
    '列表 record 只有單數 product 時，Post.linkedProduct 不得為 null（卡片才會渲染）',
    () async {
      final adapter = _PostResponseAdapter();
      final dio = Dio(BaseOptions(baseUrl: 'https://test-api.example.com'))
        ..httpClientAdapter = adapter;
      final container = ProviderContainer();
      addTearDown(container.dispose);
      final repository = container.read(
        Provider<PostRepository>((ref) => PostRepository(dio, ref)),
      );

      final result = await repository.listMyPostsRaw();
      expect(result.posts.single['linkedProduct'], isNotNull);
    },
  );
}
