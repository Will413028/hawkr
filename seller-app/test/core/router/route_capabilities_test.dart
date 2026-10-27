import 'dart:io';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/router/route_capabilities.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/my_store_fixtures.dart';

/// 對照 website 的 `pageCapabilities.unit.test.ts`：每個路由都要宣告它需要的能力。
void main() {
  final declaredPaths = RegExp(r"static const \w+ = '(/[^']+)';")
      .allMatches(File('lib/core/router/app_router.dart').readAsStringSync())
      .map((match) => match.group(1)!)
      .toSet();

  test('app_router.dart 的每個路由常數都在能力表裡宣告', () {
    expect(declaredPaths, isNotEmpty);
    for (final path in declaredPaths) {
      expect(
        routeCapabilities.containsKey(path),
        isTrue,
        reason: '$path 沒有在 routeCapabilities 宣告需要的能力',
      );
    }
  });

  test('能力表沒有多餘的路由', () {
    expect(routeCapabilities.keys.toSet(), declaredPaths);
  });

  test('能力表不用 UNSPECIFIED', () {
    expect(
      routeCapabilities.values,
      isNot(contains(SellerCapability.SELLER_CAPABILITY_UNSPECIFIED)),
    );
  });

  test('對照 website 頁面能力表的選擇', () {
    const orderFulfill = SellerCapability.SELLER_CAPABILITY_ORDER_FULFILL;
    const productManage = SellerCapability.SELLER_CAPABILITY_PRODUCT_MANAGE;
    const marketing = SellerCapability.SELLER_CAPABILITY_MARKETING;
    expect(routeCapabilities[MainRoutes.orders], orderFulfill);
    expect(routeCapabilities[ToolRoutes.afterSales], orderFulfill);
    expect(routeCapabilities[ProductRoutes.info], productManage);
    expect(routeCapabilities[SettingsRoutes.shipping], productManage);
    expect(routeCapabilities[MainRoutes.dashboard], marketing);
    expect(routeCapabilities[CouponRoutes.add], marketing);
    expect(
      routeCapabilities[SettingsRoutes.chat],
      SellerCapability.SELLER_CAPABILITY_CHAT_READ,
    );
    expect(
      routeCapabilities[SettingsRoutes.notifications],
      SellerCapability.SELLER_CAPABILITY_NOTIFICATION,
    );
    expect(
      routeCapabilities[MainRoutes.wallet],
      SellerCapability.SELLER_CAPABILITY_STOREFRONT_WRITE,
    );
    expect(routeCapabilities[SellerRoutes.measures], isNull);
    expect(routeCapabilities.containsKey(SellerRoutes.measures), isTrue);
    // 措施與申覆是帳號層級：列表與表單都不需要能力（停權中也要能申覆）。
    expect(routeCapabilities[SellerRoutes.measureRequest], isNull);
    expect(routeCapabilities.containsKey(SellerRoutes.measureRequest), isTrue);
    expect(
      routeCapabilities[MainRoutes.settings],
      SellerCapability.SELLER_CAPABILITY_ACCOUNT_CLOSE,
    );
    expect(routeCapabilities[SellerRoutes.suspensionNotice], isNull);
    expect(
      routeCapabilities.containsKey(SellerRoutes.suspensionNotice),
      isTrue,
    );
  });

  test('公告頁自己在任何狀態都不被擋', () {
    for (final store in [
      suspendedStore(),
      pendingDeletionStore(),
      myStore(
        capabilities: [SellerCapability.SELLER_CAPABILITY_ACCOUNT_SELF_READ],
      ),
    ]) {
      expect(
        isBlockedByCapabilities(store, SellerRoutes.suspensionNotice),
        isFalse,
      );
    }
  });

  test('isRouteAvailable：沒宣告的路由不開放，不需要能力的路由一律開放', () {
    expect(isRouteAvailable('/not/declared', activeCapabilities), isFalse);
    expect(isRouteAvailable(SettingsRoutes.help, const []), isTrue);
    expect(isRouteAvailable(MainRoutes.orders, const []), isFalse);
  });
}
