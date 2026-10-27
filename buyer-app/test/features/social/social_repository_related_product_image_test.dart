import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/features/social/data/social_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

class _RelatedProductAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString(
      jsonEncode({
        'code': 0,
        'data': {
          'records': [
            {
              'id': 101,
              'sellerId': 7,
              'sellerName': '南畝王生技',
              'imageUrl': '',
              'product': {
                'id': 55,
                'name': '南畝王生技中都－台灣紅豆',
                'price': 195,
                'imageUrl':
                    'https://test-media.example.com/i/original/public/product/red-bean.webp',
              },
            },
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
    'normalizes list product imageUrl for the related-product chip',
    () async {
      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = _RelatedProductAdapter();
      addTearDown(dio.close);

      final posts = await SocialRepository(dio).getPosts();

      expect(
        posts.single.product?['image'],
        'https://test-media.example.com/i/original/public/product/red-bean.webp',
      );
    },
  );
}
