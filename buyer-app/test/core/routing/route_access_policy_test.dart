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
      '/account/orders/ORD-1',
      '/account/messages/store-1',
      '/orders/detail',
      '/settings/profile',
      '/wallet/coupons',
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
