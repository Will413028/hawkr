import 'package:hawkr_buyer/core/routing/route_access_policy.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('public discovery routes remain guest accessible', () {
    for (final route in <String>[
      '/',
      '/marketplace',
      '/merchant/2683382637',
      '/merchant-product',
      '/category',
      '/search',
      '/social/post/1',
      // 強制更新是攔截頁，未登入者也必須看得到（54 號設計書 §2.3）——
      // 若它需要登入，router 會先把訪客導去 /login，強更畫面永遠出不來。
      '/force-update',
    ]) {
      expect(
        requiresLoginForRoute(route),
        isFalse,
        reason: '$route should match the public website',
      );
    }
  });

  test('account-owned and mutating routes still require login', () {
    for (final route in <String>[
      '/cart',
      '/chat',
      '/chat/room',
      '/orders/detail',
      '/settings/profile',
      '/wallet/coupons',
      '/payment/webview',
      '/review/product',
    ]) {
      expect(
        requiresLoginForRoute(route),
        isTrue,
        reason: '$route exposes account-owned or mutating state',
      );
    }
  });
}
