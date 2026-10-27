import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/features/category/data/category_repository.dart';
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// 固定回一份 /public/search/products 的真實 wire 形狀。
class _FixedAdapter implements HttpClientAdapter {
  _FixedAdapter(this.responseBody);

  final Object responseBody;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
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

/// Gateway 的 SearchResultData：分頁 key 用 `records`（§16），四個計數以 string
/// 過線（§4），商品欄位一律 camelCase（§2）。
const _wire = {
  'code': 0,
  'data': {
    'records': [
      {
        'id': 'p1',
        'name': '青江菜',
        'categoryId': '7',
        'categoryName': '蔬菜',
        'priceMin': '30',
        'priceMax': '80',
        'reviewCount': '9',
        'storeId': '2685914682',
      },
    ],
    'total': '48',
    'page': '2',
    'pageSize': '20',
    'totalPages': '3',
  },
};

CategoryRepository _repo(Object body) {
  final dio = Dio(BaseOptions(baseUrl: 'https://example.test'));
  dio.httpClientAdapter = _FixedAdapter(body);
  return CategoryRepository(dio);
}

void main() {
  group('CategoryRepository.searchProducts — /public/search/products 契約', () {
    test('分頁計數以 string 過線時要 parse 出真正的數字，不是退回預設值', () async {
      final result = await _repo(_wire).searchProducts(keyword: '菜', page: 1);

      // 這三條是本測試的重點：後端送 string，若讀取端用 `as int?` 直接接會拿到
      // null → 靜默退回預設值（total=0、page=送出去的那頁），畫面看起來正常但
      // 分頁永遠壞掉。斷言真值才擋得住。
      expect(result.total, 48);
      expect(result.page, 2);
      expect(result.pageSize, 20);
    });

    test('商品清單讀 records，且欄位吃 camelCase', () async {
      final result = await _repo(_wire).searchProducts(keyword: '菜');

      expect(result.records, hasLength(1));
      final item = result.records.single;
      expect(item.id, 'p1');
      expect(item.categoryId, '7');
      expect(item.categoryName, '蔬菜');
      expect(item.priceMin, '30');
      expect(item.priceMax, '80');
      expect(item.reviewCount, '9');
    });

    test('舊形狀（products key + snake_case + 數字型計數）不再被接受', () async {
      // 反向斷言：餵舊 wire 進去必須 parse 不出東西。若這裡仍拿得到資料，代表
      // 讀取端還留著雙讀相容層——本專案禁止相容層。
      final stale = await _repo({
        'code': 0,
        'data': {
          'products': [
            {'id': 'p1', 'name': '青江菜', 'category_id': '7', 'priceMin': '30'},
          ],
          'total': 48,
          'page': 2,
          'page_size': 20,
        },
      }).searchProducts(keyword: '菜', page: 1, pageSize: 20);

      expect(stale.records, isEmpty);
      expect(stale.total, 0);
    });

    test('略過缺少 id 或 name 的壞索引資料，不拖垮同頁有效商品', () async {
      final result = await _repo({
        'code': 0,
        'data': {
          'records': [
            {'id': '', 'name': '缺少識別碼'},
            {'id': 'broken-name', 'name': ''},
            {
              'id': '871936508549696815',
              'name': '高麗菜',
              'priceMin': '111',
              'priceMax': '111',
              'stock': '88',
            },
          ],
          'total': '3',
          'page': '1',
          'pageSize': '20',
          'totalPages': '1',
        },
      }).searchProducts(keyword: '菜');

      expect(result.records, hasLength(1));
      expect(result.records.single.id, '871936508549696815');
      expect(result.records.single.stockQty, 88);
    });

    test('略過壞資料後仍以 server metadata 判斷下一頁', () async {
      final result = await _repo({
        'code': 0,
        'data': {
          'records': [
            {'id': '', 'name': '壞資料'},
            {'id': '871936508549696815', 'name': '高麗菜'},
          ],
          'total': '40',
          'page': '1',
          'pageSize': '20',
          'totalPages': '2',
        },
      }).searchProducts(keyword: '菜');

      expect(result.records, hasLength(1));
      expect(result.isLastPage, isFalse);
    });

    test('HTTP 200 但 business code 失敗時維持錯誤，不偽裝成零結果', () async {
      final future = _repo({
        'code': 50301,
        'message': '搜尋服務暫時無法使用',
        'data': null,
      }).searchProducts(keyword: '菜');

      await expectLater(
        future,
        throwsA(
          isA<ServerException>().having(
            (error) => error.message,
            'message',
            '搜尋服務暫時無法使用',
          ),
        ),
      );
    });
  });
}
