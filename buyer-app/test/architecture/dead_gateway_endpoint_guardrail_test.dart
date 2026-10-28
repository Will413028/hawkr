import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// Gateway 上不存在的路由（2026-08-05 對 dev Gateway 逐條實打複驗：
/// 這些路徑回 gin 的純文字 `404 page not found`，而存在的路由回 JSON envelope）。
///
/// 這些呼叫過去全部把 DioException 吞掉，所以失敗完全無聲——留言數永遠取不到、
/// 商品圖恆為空、優惠券領取永遠失敗。加回任何一條就讓這支測試轉紅。
///
/// 新增條目前請先對 dev Gateway 實打確認，並在下方註明正確替代來源。
const _deadEndpointFragments = <String, String>{
  // 正解：GET /public/products/{id} 的 imageKeys / imageUrls
  '/products/\$skuId/images': '/public/sellers/{id}/products/{sku}/images',
  '/products/\$productId/images': '/public/sellers/{id}/products/{sku}/images',
  // 正解：GET /public/products/{id} 的 shippingMethods
  '/products/\$skuId/delivery': '/public/sellers/{id}/products/{sku}/delivery',
  // 正解：GET /public/posts/{id} 的 comment_count
  '/comments/count': '/public/posts/{id}/comments/count',
  // 正解：POST /app/posts/comments/{id}/like 是 toggle，沒有 DELETE 版本
  "delete<dynamic>(\n        '/app/posts/comments/\$commentId/like'":
      'DELETE /app/posts/comments/{id}/like',
  // Gateway 沒有留言按讚名單／計數端點
  '/likes/count': '/public/posts/comments/{id}/likes/count',
  "'/public/posts/comments/\$commentId/likes'":
      '/public/posts/comments/{id}/likes',
  // 正解：POST /app/coupons/{template_id}/claim
  '/receive\'': 'POST /app/coupons/{template_id}/receive',
  // ATM 退款走 POST /app/orders/{order_no}/refund_page（PayUni 退款頁）
  '/refunds/method': 'PUT /app/orders/{order_no}/refunds/method',
  '/refund_bank_info': 'POST /app/orders/{order_no}/refund_bank_info',
};

/// 去掉註解，只留下真正會送出請求的程式碼。
///
/// 註解本身可以（也應該）寫出這些死路徑來解釋為什麼不能用，所以不能連註解一起掃，
/// 否則「解釋為什麼不能用」會被誤判成「又用了一次」。
/// `://` 不視為註解起點，避免砍掉 `https://...` 字面值。
String _stripComments(String source) {
  final withoutBlocks = source.replaceAll(
    RegExp(r'/\*.*?\*/', dotAll: true),
    '',
  );
  return withoutBlocks
      .split('\n')
      .map((line) => line.replaceFirst(RegExp(r'(?<!:)//.*$'), ''))
      .join('\n');
}

void main() {
  test('lib/ 不得再出現 Gateway 沒有的死端點', () {
    final root = Directory('lib');
    expect(root.existsSync(), isTrue, reason: 'lib/ 不存在，cwd 可能不對');

    final offenders = <String>[];
    var scanned = 0;

    for (final entity in root.listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      scanned++;
      final source = _stripComments(entity.readAsStringSync());
      _deadEndpointFragments.forEach((fragment, endpoint) {
        if (!source.contains(fragment)) return;
        offenders.add('${entity.path} → $endpoint (命中片段: $fragment)');
      });
    }

    expect(scanned, greaterThan(0), reason: '一個 .dart 都沒掃到，測試本身失效');
    expect(
      offenders,
      isEmpty,
      reason:
          '以下位置呼叫了 Gateway 不存在的路由（會回 404 且錯誤被吞掉）：\n'
          '${offenders.join('\n')}',
    );
  });
}
