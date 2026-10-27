import 'dart:convert';

import 'package:hawkr_seller/features/customer/data/customer_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

/// 顧客列表 (`GET /app/sellers/me/customers`) 與詳情共用 `_parseCustomer`。
/// 2026-07-27 CRM 正名 rekey 把回應欄位從 `customerAccountId`/`accountId`
/// 換成 `buyerId`/`storeId`——parser 若沒跟上，`c.id` 會靜默變空字串（兩個
/// 舊 key 都不存在於回應裡），拿它去打客戶詳情 / 發券一律查無此人，而且
/// 沒有任何錯誤訊號（2026-07-30 標籤發券事故的根因之一）。
///
/// 這裡驅動真正的 `getCustomers()` / `getCustomerByAccountId()`，而不是直接測
/// private parser——缺口就在「真實 wire payload → parser」這一段。
class _FixedResponseAdapter implements HttpClientAdapter {
  _FixedResponseAdapter(this.body);
  final Map<String, dynamic> body;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
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

/// crm row 主鍵 — 一旦 parser 誤讀 `id`，客戶 ID 就會變成這個值。
const _crmRowId = '7301111111111111111';

/// 賣家自己的 store surrogate（`storeId`）— 拿來當客戶 ID 同樣是錯的。
const _storeId = '7302222222222222222';

/// 真正要拿到的客戶識別碼，值語意是 buyer_id（API `buyerId`）。
const _buyerId = '7303333333333333333';

CustomerRepository _repoReturning(Map<String, dynamic> body) {
  final dio = Dio(BaseOptions(baseUrl: 'https://test-api.example.com'));
  dio.httpClientAdapter = _FixedResponseAdapter(body);
  return CustomerRepository(dio);
}

void main() {
  test('顧客列表：客戶 ID 取 buyerId，不是 crm row id 也不是賣家 storeId', () async {
    final repo = _repoReturning({
      'code': 0,
      'message': 'success',
      'data': {
        'total': '1',
        'list': [
          {
            'id': _crmRowId,
            'storeId': _storeId,
            'buyerId': _buyerId,
            'customerType': 'vip',
            'totalSpent': '128000',
            'orderCount': '12',
            'couponCount': '3',
            'nickname': '王小明',
            'avatarKey': 'public/buyer_avatar/7303333333333333333/a.png',
            'avatarUrl':
                'https://media.example.com/i/original/public/buyer_avatar/7303333333333333333/a.png',
            'phone': '0912345678',
            'tags': [
              {'id': '7304444444444444444', 'storeId': _storeId, 'name': '常客'},
            ],
          },
        ],
      },
    });

    final customers = await repo.getCustomers();

    expect(customers, hasLength(1));
    final c = customers.single;
    expect(c.id, _buyerId, reason: 'crm row id / 賣家 storeId 都不能當客戶 ID');
    expect(c.id, isNot(_crmRowId));
    expect(c.id, isNot(_storeId));
    expect(c.name, '王小明');
    expect(c.phone, '0912345678');
    expect(c.totalSpent, 128000);
    expect(c.visitCount, 12);
    expect(c.couponCount, 3);
    expect(c.isVip, isTrue);
    expect(c.avatarUrl, startsWith('https://'));
    expect(c.tags.map((t) => t.name), ['常客']);
  });

  test('顧客詳情：frequentCategories 進得了「常買品項」', () async {
    final repo = _repoReturning({
      'code': 0,
      'message': 'success',
      'data': {
        'id': _crmRowId,
        'storeId': _storeId,
        'buyerId': _buyerId,
        'customerType': 'normal',
        'totalSpent': '735',
        'orderCount': '5',
        'couponCount': '0',
        'nickname': '林小明',
        'avatarKey': 'public/buyer_avatar/2623900520/b.png',
        'avatarUrl':
            'https://media.example.com/i/original/public/buyer_avatar/2623900520/b.png',
        'phone': '0911111111',
        'address': '臺北市信義區信義路五段一號',
        'email': 'buyer@example.com',
        'frequentCategories': ['蔬菜', '水果'],
        'tags': <dynamic>[],
      },
    });

    final c = await repo.getCustomerByAccountId(_buyerId);

    expect(c, isNotNull);
    expect(c!.id, _buyerId);
    expect(c.address, '臺北市信義區信義路五段一號');
    expect(c.email, 'buyer@example.com');
    // 後端欄位是 frequentCategories；讀成 frequentItems 會永遠是空的。
    expect(c.frequentItems, ['蔬菜', '水果']);
  });
}
