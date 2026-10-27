import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('legacy order prototypes redirect to server-backed buyer flows', () {
    final router = File('lib/core/routing/app_router.dart').readAsStringSync();

    expect(router, isNot(contains('const OrderMatchedScreen()')));
    expect(router, isNot(contains('OrderDisputeScreen(order:')));
    expect(router, isNot(contains('const DisputeScreen()')));
    expect(router, isNot(contains('const WalletCouponDetailScreen()')));
    expect(router, isNot(contains('TrackingScreen(')));
    expect(router, isNot(contains('const ReserveScreen()')));
    expect(router, isNot(contains('const OrderPaymentDetailsScreen()')));
    expect(router, isNot(contains('const PaymentDetailScreen()')));
    expect(
      router,
      contains("path: '/orders/matched',\n        // Retired prototype"),
    );
    expect(router, contains("redirect: (_, __) => '/?tab=3'"));
    expect(
      RegExp(
        r"path: '/orders/(?:order-)?dispute',[\s\S]{0,240}"
        r"redirect: \(_, __\) => '/settings/after-sale'",
      ).allMatches(router),
      hasLength(2),
    );
    expect(
      RegExp(
        r"path: '/wallet/coupon-detail',[\s\S]{0,260}"
        r"redirect: \(_, __\) => '/wallet/coupons'",
      ).hasMatch(router),
      isTrue,
    );
    expect(
      RegExp(
        r"path: '/orders/tracking',[\s\S]{0,300}"
        r"redirect: \(_, __\) => '/\?tab=3'",
      ).hasMatch(router),
      isTrue,
    );
    expect(
      RegExp(
        r"path: '/orders/reserve',[\s\S]{0,220}"
        r"redirect: \(_, __\) => '/\?tab=1'",
      ).hasMatch(router),
      isTrue,
    );
    expect(
      RegExp(
        r"path: '/orders/(?:payment-details|payment)',[\s\S]{0,280}"
        r"redirect: \(_, __\) => '/\?tab=3'",
      ).allMatches(router),
      hasLength(2),
    );
  });
}
