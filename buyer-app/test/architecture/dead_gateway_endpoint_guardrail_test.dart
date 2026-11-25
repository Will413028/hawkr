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
  '/refunds/method': 'PUT /app/orders/{order_no}/refunds/method',
  '/refund_bank_info': 'POST /app/orders/{order_no}/refund_bank_info',
};

/// 允許出現的舊 REST 呼叫（A6 的允許殘留）：路徑 → 理由與上限。上限是該檔允許的命中次數，
/// 多一次就失敗，所以允許檔裡不能偷偷加第二個呼叫。
typedef _Allowance = ({String reason, int max});

/// 已改走 Connect 或由用戶端組合、不得再以手寫 REST 呼叫的路由；[allowed] 是這條路由唯一允許出現的檔案。
class _Retired {
  // ignore: unused_element_parameter
  const _Retired(this.endpoint, [this.allowed = const {}]);

  final String endpoint;
  final Map<String, _Allowance> allowed;
}

final _retiredEndpointPatterns = <RegExp, _Retired>{
  RegExp(r'/public/sellers/\$[\w{}.]+/rating'): _Retired(
    'GET /public/sellers/{id}/rating',
  ),
  RegExp(r'/public/sellers/\$[\w{}.]+/stats'): _Retired(
    'GET /public/sellers/{id}/stats',
  ),
  RegExp(r'/public/marketplace'): _Retired('GET /public/marketplace'),
  RegExp(r'/public/recommended'): _Retired('GET /public/recommended'),
  RegExp(r'/public/products/\$[\w{}.]+/view'): _Retired(
    'POST /public/products/{id}/view',
  ),
  // 貼文、留言、討論區已改走 SocialPublicService／SocialAppService（A7-4c-2）。
  RegExp(r'/public/posts'): _Retired('GET /public/posts…（貼文與留言讀取）'),
  RegExp(r'/app/posts'): _Retired('POST /app/posts…（按讚、留言、回覆）'),
  RegExp(r'/public/sellers/\$[\w{}.]+/posts'): _Retired(
    'GET /public/sellers/{id}/posts',
  ),
  RegExp(r'/public/sellers/\$[\w{}.]+/forum'): _Retired(
    'GET /public/sellers/{id}/forum',
  ),
  RegExp(r'/public/sellers/forum'): _Retired(
    'GET /public/sellers/forum/{id}/replies',
  ),
  RegExp(r'/app/sellers/\$[\w{}.]+/forum'): _Retired(
    'POST /app/sellers/{id}/forum',
  ),
  RegExp(r'/app/sellers/forum'): _Retired('/app/sellers/forum/{id}（回覆、修改、刪除）'),
  RegExp(r'/comments/stream'): _Retired(
    'GET /public/posts/{id}/comments/stream（SSE）',
  ),
  // 檢舉、封鎖、封鎖名單與貼文分享已改走 ModerationAppService、BuyerAppService 與 `Post.url`；聊天改走
  // ChatAppService（A6-6c2），不再有允許的舊 REST 呼叫。
  RegExp(r'/app/blocks'): _Retired('POST|GET|DELETE /app/blocks…（封鎖與封鎖名單）'),
  // ATM 退款頁與 PayUni 付款路由已隨 Java payment 刪除（B2 步驟 6）；退款一律原路退回，付款走 PaymentService。
  RegExp(r'/refund_page\b'): _Retired(
    'POST /app/orders/{order_no}/refund_page（ATM 退款頁）',
  ),
  RegExp(r'/atm-refund\b'): _Retired('POST /atm-refund/return（ATM 退款頁回跳）'),
  RegExp(r'/app/payments\b'): _Retired('/app/payments…（舊付款 REST）'),
  RegExp(r'/app/reports/me'): _Retired('GET /app/reports/me（我的檢舉）'),
  RegExp(r'/app/reports(?!/)'): _Retired('POST /app/reports（檢舉）'),
  RegExp(r'/app/chat/'): _Retired('/app/chat/*（聊天的 REST 與 WebSocket 連線票）'),
  RegExp(r"getShareUrl\(\s*type:\s*'post'"): _Retired(
    "getShareUrl(type: 'post')（貼文分享改用 Post.url）",
  ),
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
      _retiredEndpointPatterns.forEach((pattern, retired) {
        final hits = pattern.allMatches(source).length;
        if (hits == 0) return;
        final allowance = retired.allowed[entity.path];
        if (allowance != null && hits <= allowance.max) return;
        offenders.add(
          '${entity.path} → ${retired.endpoint} (命中: ${pattern.pattern}'
          '${allowance == null ? '' : '，$hits 次超過允許的 ${allowance.max} 次'})',
        );
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

  test('允許殘留的檔案仍存在且仍有該呼叫（清單不留過期項目）', () {
    _retiredEndpointPatterns.forEach((pattern, retired) {
      retired.allowed.forEach((path, allowance) {
        final file = File(path);
        expect(file.existsSync(), isTrue, reason: '$path 已不存在，從允許清單移除');
        final hits = pattern.allMatches(
          _stripComments(file.readAsStringSync()),
        );
        expect(
          hits,
          isNotEmpty,
          reason: '$path 已沒有 ${pattern.pattern}，從允許清單移除（${allowance.reason}）',
        );
      });
    });
  });

  test('/app/blocks 與 /app/reports 沒有任何允許的檔案', () {
    for (final pattern in [r'/app/blocks', r'/app/reports(?!/)']) {
      final retired = _retiredEndpointPatterns.entries
          .firstWhere((e) => e.key.pattern == pattern)
          .value;
      expect(retired.allowed, isEmpty, reason: pattern);
    }
  });
}
