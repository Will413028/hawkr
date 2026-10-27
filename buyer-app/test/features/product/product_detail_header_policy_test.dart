import 'dart:io';

import 'package:hawkr_buyer/core/routing/floating_detail_page.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/features/product/presentation/merchant_item_detail_screen.dart';
import 'package:hawkr_buyer/features/product/presentation/product_detail_preview_layout.dart';
import 'package:hawkr_buyer/features/product/presentation/widgets/product_purchase_panel.dart';
import 'package:hawkr_buyer/shared/widgets/hk_liquid_glass.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('product detail uses one solid card for content and purchase', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      const ProviderScope(
        child: ScreenUtilInit(
          designSize: Size(390, 844),
          child: MaterialApp(home: MerchantItemDetailScreen(title: '測試商品')),
        ),
      ),
    );
    await tester.pump(const Duration(milliseconds: 400));

    expect(
      find.byKey(const ValueKey('product-detail-floating-shell')),
      findsOneWidget,
    );
    expect(
      tester
          .widget<Scaffold>(
            find.byKey(const ValueKey('product-detail-floating-shell')),
          )
          .backgroundColor,
      Colors.transparent,
    );
    expect(
      find.byKey(const ValueKey('product-detail-backdrop')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('product-detail-preview-window')),
      findsOneWidget,
    );
    expect(find.byType(AppBar), findsNothing);
    expect(find.byKey(const ValueKey('product-detail-close')), findsOneWidget);
    expect(
      find.byKey(const ValueKey('product-detail-favorite')),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('product-detail-share')), findsOneWidget);
    expect(find.byType(HkLiquidGlassSurface), findsNWidgets(3));
    expect(find.byType(HkLiquidGlassIconButton), findsNWidgets(3));
    expect(
      find.byKey(const ValueKey('product-detail-floating-motion')),
      findsNothing,
    );

    final previewRect = tester.getRect(
      find.byKey(const ValueKey('product-detail-preview-window')),
    );
    final purchaseDock = find.byKey(
      const ValueKey('product-detail-purchase-dock'),
    );
    final purchaseRect = tester.getRect(purchaseDock);
    expect(
      previewRect.left,
      closeTo(ProductDetailPreviewLayout.previewHorizontalInset, 0.01),
    );
    expect(
      previewRect.right,
      closeTo(390 - ProductDetailPreviewLayout.previewHorizontalInset, 0.01),
    );
    expect(
      purchaseRect.left,
      closeTo(ProductDetailPreviewLayout.previewHorizontalInset, 0.01),
    );
    expect(
      purchaseRect.right,
      closeTo(390 - ProductDetailPreviewLayout.previewHorizontalInset, 0.01),
    );
    expect(
      previewRect.bottom,
      closeTo(844 - ProductDetailPreviewLayout.purchaseBottomInset, 0.01),
    );
    expect(purchaseRect.bottom, closeTo(previewRect.bottom, 0.01));
    expect(
      find.ancestor(
        of: purchaseDock,
        matching: find.byKey(const ValueKey('product-detail-preview-window')),
      ),
      findsOneWidget,
    );
    expect(
      tester
          .widget<Material>(
            find.byKey(const ValueKey('product-detail-preview-window')),
          )
          .color,
      AppColors.bgCard,
    );

    for (final key in const <String>[
      'product-detail-merchant-section',
      'product-detail-info-section',
      'product-detail-purchase-selector-card',
      'product-detail-description-section',
      'product-detail-review-section',
    ]) {
      expect(
        find.ancestor(
          of: find.byKey(ValueKey(key)),
          matching: find.byKey(const ValueKey('product-detail-preview-window')),
        ),
        findsOneWidget,
      );
    }

    final footer = tester.widget<ProductPurchaseFooter>(
      find.byType(ProductPurchaseFooter),
    );
    expect(footer.floating, isFalse);
    expect(footer.integrated, isTrue);
    expect(
      find.byKey(const ValueKey('product-purchase-floating-surface')),
      findsNothing,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('single-card detail stays stable at 320pt and 1.4x text', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 720);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          child: MaterialApp(
            home: Builder(
              builder: (context) => MediaQuery(
                data: MediaQuery.of(
                  context,
                ).copyWith(textScaler: const TextScaler.linear(1.4)),
                child: const MerchantItemDetailScreen(title: '窄螢幕測試商品'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump(const Duration(milliseconds: 400));

    expect(
      find.byKey(const ValueKey('product-detail-preview-window')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('product-detail-purchase-dock')),
      findsOneWidget,
    );
    expect(tester.takeException(), isNull);
  });

  test('product route uses the shared transparent detail overlay', () {
    final source = File('lib/core/routing/app_router.dart').readAsStringSync();
    expect(
      source,
      matches(
        RegExp(
          r"path: '/merchant-product',[\s\S]*?pageBuilder:[\s\S]*?"
          r'floatingDetailPage<Map<String, dynamic>\?>[\s\S]*?'
          r"barrierLabel: '關閉商品詳情'",
        ),
      ),
    );
    final categorySource = File(
      'lib/features/category/presentation/category_landing_screen.dart',
    ).readAsStringSync();
    expect(categorySource, contains('extra: MerchantItemDetailScreen('));
  });

  testWidgets(
    'actual iOS product detail keeps its edge-swipe drill-down behavior',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(390, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      Map<String, dynamic>? gestureResult;
      final router = GoRouter(
        routes: [
          GoRoute(
            path: '/',
            builder: (_, __) => const Scaffold(body: Text('商品列表')),
          ),
          GoRoute(
            path: '/merchant-product',
            pageBuilder: (context, state) =>
                floatingDetailPage<Map<String, dynamic>?>(
                  context: context,
                  key: state.pageKey,
                  child: MerchantItemDetailScreen(
                    title: '測試商品',
                    onResultChanged: (result) => gestureResult = result,
                  ),
                ),
          ),
        ],
      );
      addTearDown(router.dispose);

      await tester.pumpWidget(
        ProviderScope(
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            child: MaterialApp.router(
              theme: ThemeData(platform: TargetPlatform.iOS),
              routerConfig: router,
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final result = router.push<Map<String, dynamic>?>('/merchant-product');
      await tester.pump();
      await tester.pumpAndSettle();

      final detailContext = tester.element(
        find.byType(MerchantItemDetailScreen),
      );
      final detailRoute = ModalRoute.of(detailContext)! as PageRoute<dynamic>;
      expect(detailRoute.animation!.status, AnimationStatus.completed);
      expect(detailRoute.popGestureEnabled, isTrue);

      final gesture = await tester.startGesture(const Offset(5, 300));
      await gesture.moveBy(const Offset(390, 0));
      await gesture.up();
      await tester.pumpAndSettle();

      expect(await result, isNull);
      expect(gestureResult, <String, dynamic>{
        'productId': null,
        'isFavorite': false,
        'isRestockSubscribed': false,
      });
      expect(find.text('商品列表'), findsOneWidget);
      expect(find.byType(MerchantItemDetailScreen), findsNothing);
    },
  );
}
