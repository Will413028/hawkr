import 'package:hawkr_buyer/core/routing/floating_detail_page.dart';
import 'package:hawkr_buyer/features/product/presentation/merchant_item_detail_screen.dart';
import 'package:hawkr_buyer/shared/widgets/product_card.dart';
import 'package:hawkr_buyer/shared/widgets/product_card_hero.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('product Hero flight does not read size during build', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Column(
            children: [
              const SizedBox(
                width: 140,
                height: 180,
                child: Hero(
                  tag: 'product',
                  flightShuttleBuilder: productCardHeroFlightShuttleBuilder,
                  child: ColoredBox(color: Colors.red),
                ),
              ),
              Builder(
                builder: (context) => TextButton(
                  onPressed: () {
                    Navigator.of(context).push(
                      MaterialPageRoute<void>(
                        builder: (_) => const Scaffold(
                          body: Center(
                            child: SizedBox(
                              width: 360,
                              height: 600,
                              child: Hero(
                                tag: 'product',
                                flightShuttleBuilder:
                                    productCardHeroFlightShuttleBuilder,
                                child: ColoredBox(color: Colors.red),
                              ),
                            ),
                          ),
                        ),
                      ),
                    );
                  },
                  child: const Text('open'),
                ),
              ),
            ],
          ),
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(tester.takeException(), isNull);
    expect(find.byType(FittedBox), findsOneWidget);

    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });

  testWidgets('product media Hero stays interactive with iOS edge swipe', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    const heroTag = 'product-media:category:42';
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => Scaffold(
            body: Center(
              child: SizedBox(
                width: 160,
                height: 200,
                child: ProductCard(
                  name: '測試商品',
                  heroTag: heroTag,
                  onTap: () => context.push('/detail'),
                ),
              ),
            ),
          ),
        ),
        GoRoute(
          path: '/detail',
          pageBuilder: (context, state) => floatingDetailPage<void>(
            context: context,
            key: state.pageKey,
            child: const MerchantItemDetailScreen(
              title: '測試商品',
              heroTag: heroTag,
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

    await tester.tap(find.byKey(const ValueKey('product-card-surface')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 80));
    expect(tester.takeException(), isNull);
    expect(find.byType(Hero), findsWidgets);

    await tester.pumpAndSettle();
    final detailContext = tester.element(find.byType(MerchantItemDetailScreen));
    final detailRoute = ModalRoute.of(detailContext)! as PageRoute<dynamic>;
    expect(detailRoute.popGestureEnabled, isTrue);

    final gesture = await tester.startGesture(const Offset(5, 300));
    await gesture.moveBy(const Offset(180, 0));
    await tester.pump();
    expect(tester.takeException(), isNull);
    await gesture.moveBy(const Offset(210, 0));
    await gesture.up();
    await tester.pumpAndSettle();

    expect(find.byType(MerchantItemDetailScreen), findsNothing);
    expect(find.byKey(const ValueKey('product-card-surface')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
