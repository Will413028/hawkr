import 'dart:io';

import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final routes = <({String symbol, String path})>[
    (symbol: 'AuthRoutes.welcome', path: AuthRoutes.welcome),
    (symbol: 'AuthRoutes.auth', path: AuthRoutes.auth),
    (symbol: 'AuthRoutes.onboard', path: AuthRoutes.onboard),
    (symbol: 'MainRoutes.dashboard', path: MainRoutes.dashboard),
    (symbol: 'MainRoutes.category', path: MainRoutes.category),
    (symbol: 'MainRoutes.wallet', path: MainRoutes.wallet),
    (symbol: 'MainRoutes.orders', path: MainRoutes.orders),
    (symbol: 'MainRoutes.settings', path: MainRoutes.settings),
    (symbol: 'CustomerRoutes.customers', path: CustomerRoutes.customers),
    (symbol: 'CustomerRoutes.subscribers', path: CustomerRoutes.subscribers),
    (symbol: 'CustomerRoutes.detail', path: CustomerRoutes.detail),
    (
      symbol: 'CustomerRoutes.spendingHistory',
      path: CustomerRoutes.spendingHistory,
    ),
    (symbol: 'CustomerRoutes.orderDetail', path: CustomerRoutes.orderDetail),
    (symbol: 'OrderRoutes.details', path: OrderRoutes.details),
    (symbol: 'OrderRoutes.matched', path: OrderRoutes.matched),
    (symbol: 'OrderRoutes.paymentDetails', path: OrderRoutes.paymentDetails),
    (symbol: 'OrderRoutes.receipts', path: OrderRoutes.receipts),
    (symbol: 'OrderRoutes.receiptsDetail', path: OrderRoutes.receiptsDetail),
    (symbol: 'OrderRoutes.dispute', path: OrderRoutes.dispute),
    (symbol: 'OrderRoutes.logistics', path: OrderRoutes.logistics),
    (symbol: 'OrderRoutes.view', path: OrderRoutes.view),
    (symbol: 'OrderRoutes.matchedDetail', path: OrderRoutes.matchedDetail),
    (symbol: 'ProductRoutes.picker', path: ProductRoutes.picker),
    (symbol: 'ProductRoutes.info', path: ProductRoutes.info),
    (symbol: 'ProductRoutes.images', path: ProductRoutes.images),
    (symbol: 'ProductRoutes.tags', path: ProductRoutes.tags),
    (symbol: 'ProductRoutes.specs', path: ProductRoutes.specs),
    (symbol: 'ProductRoutes.mediaPicker', path: ProductRoutes.mediaPicker),
    (symbol: 'ProductRoutes.preview', path: ProductRoutes.preview),
    (symbol: 'ProductRoutes.reviews', path: ProductRoutes.reviews),
    (symbol: 'PostRoutes.mediaPicker', path: PostRoutes.mediaPicker),
    (symbol: 'PostRoutes.compose', path: PostRoutes.compose),
    (symbol: 'PostRoutes.edit', path: PostRoutes.edit),
    (symbol: 'PostRoutes.feed', path: PostRoutes.feed),
    (
      symbol: 'SellerRoutes.certificationUpload',
      path: SellerRoutes.certificationUpload,
    ),
    (symbol: 'SellerRoutes.photoTools', path: SellerRoutes.photoTools),
    (
      symbol: 'SellerRoutes.suspensionNotice',
      path: SellerRoutes.suspensionNotice,
    ),
    (symbol: 'SellerRoutes.measures', path: SellerRoutes.measures),
    (symbol: 'SellerRoutes.measureRequest', path: SellerRoutes.measureRequest),
    (
      symbol: 'SellerRoutes.storeUnavailable',
      path: SellerRoutes.storeUnavailable,
    ),
    (symbol: 'SellerRoutes.forceUpdate', path: SellerRoutes.forceUpdate),
    (symbol: 'CouponRoutes.editor', path: CouponRoutes.editor),
    (symbol: 'CouponRoutes.add', path: CouponRoutes.add),
    (symbol: 'CouponRoutes.detail', path: CouponRoutes.detail),
    (symbol: 'SettingsRoutes.profile', path: SettingsRoutes.profile),
    (symbol: 'SettingsRoutes.chat', path: SettingsRoutes.chat),
    (
      symbol: 'SettingsRoutes.chatAiAssistant',
      path: SettingsRoutes.chatAiAssistant,
    ),
    (symbol: 'SettingsRoutes.chatThread', path: SettingsRoutes.chatThread),
    (symbol: 'SettingsRoutes.shipping', path: SettingsRoutes.shipping),
    (
      symbol: 'SettingsRoutes.notifications',
      path: SettingsRoutes.notifications,
    ),
    (
      symbol: 'SettingsRoutes.notificationSettings',
      path: SettingsRoutes.notificationSettings,
    ),
    (symbol: 'SettingsRoutes.myReports', path: SettingsRoutes.myReports),
    (symbol: 'SettingsRoutes.blockList', path: SettingsRoutes.blockList),
    (symbol: 'SettingsRoutes.help', path: SettingsRoutes.help),
    (symbol: 'ToolRoutes.photoEditor', path: ToolRoutes.photoEditor),
    (symbol: 'ToolRoutes.richTextEditor', path: ToolRoutes.richTextEditor),
    (symbol: 'ToolRoutes.afterSales', path: ToolRoutes.afterSales),
    (symbol: 'ToolRoutes.afterSalesDetail', path: ToolRoutes.afterSalesDetail),
    (
      symbol: 'ShipmentAnomalyRoutes.dashboard',
      path: ShipmentAnomalyRoutes.dashboard,
    ),
    (
      symbol: 'ProcurementRoutes.sellerPublications',
      path: ProcurementRoutes.sellerPublications,
    ),
    (
      symbol: 'ProcurementRoutes.sellerQuote',
      path: ProcurementRoutes.sellerQuote,
    ),
  ];

  test('all 61 seller routes remain unique and declared', () {
    expect(routes, hasLength(61));
    expect(routes.map((route) => route.path).toSet(), hasLength(61));
    expect(routes.every((route) => route.path.startsWith('/')), isTrue);
  });

  test('every route constant is reachable from a GoRoute declaration', () {
    final source = File('lib/core/router/app_router.dart').readAsStringSync();
    for (final route in routes) {
      expect(
        source,
        contains('path: ${route.symbol}'),
        reason: '${route.symbol} (${route.path}) is not wired to GoRouter',
      );
    }
  });

  test('inventory covers every route constant in app_router.dart', () {
    final source = File('lib/core/router/app_router.dart').readAsStringSync();
    final declaredPaths = RegExp(
      r"static const \w+ = '(/[^']+)';",
    ).allMatches(source).map((match) => match.group(1)!).toSet();

    expect(declaredPaths, routes.map((route) => route.path).toSet());
  });

  test('route builders do not force-cast state.extra payloads', () {
    final source = File('lib/core/router/app_router.dart').readAsStringSync();

    expect(source, isNot(contains('state.extra as')));
    expect(
      RegExp(r"extra\['[^']+'\]\s+as").hasMatch(source),
      isFalse,
      reason:
          'Route payloads must go through typed adapters so malformed or '
          'restored routes cannot throw a cast error.',
    );
  });
}
