import 'package:hawkr_seller/core/layout/app_media_query_policy.dart';
import 'package:hawkr_seller/shared/widgets/app_bottom_navigation_bar.dart';
import 'package:hawkr_seller/shared/widgets/seller_floating_navigation_layout.dart';
import 'package:hawkr_seller/shared/widgets/seller_gradient_canvas.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('continuous seller gradient fills the complete canvas', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(body: SellerGradientCanvas(child: SizedBox.expand())),
      ),
    );

    final canvas = tester.widget<DecoratedBox>(
      find.byKey(const ValueKey('seller-continuous-gradient-canvas')),
    );
    final decoration = canvas.decoration as BoxDecoration;

    expect(decoration.gradient, isA<LinearGradient>());
    expect(
      tester.getSize(find.byType(SellerGradientCanvas)),
      const Size(800, 600),
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'gradient remains behind the floating nav on phone and landscape',
    (tester) async {
      addTearDown(() => tester.binding.setSurfaceSize(null));

      for (final size in const [Size(375, 812), Size(812, 375)]) {
        await tester.binding.setSurfaceSize(size);
        await tester.pumpWidget(
          MaterialApp(
            home: MediaQuery(
              data: AppMediaQueryPolicy.apply(
                MediaQueryData(
                  size: size,
                  textScaler: const TextScaler.linear(2),
                ),
              ),
              child: const Scaffold(
                body: SellerFloatingNavigationLayout(
                  page: SellerGradientCanvas(child: SizedBox.expand()),
                  navigation: AppBottomNavigationBar(
                    currentIndex: NavIndex.dashboard,
                  ),
                ),
              ),
            ),
          ),
        );
        await tester.pump();

        expect(tester.getSize(find.byType(SellerGradientCanvas)), size);
        expect(
          tester.getSize(find.byKey(const ValueKey('seller-full-page-layer'))),
          size,
        );
        final navigationRect = tester.getRect(
          find.byKey(const ValueKey('seller-floating-navigation-layer')),
        );
        expect(navigationRect.bottom, size.height);
        expect(navigationRect.top, lessThan(size.height));
        expect(
          find.byKey(const ValueKey('seller-bottom-nav-surface')),
          findsOneWidget,
        );
        final pageContext = tester.element(find.byType(SellerGradientCanvas));
        final stackContext = tester.element(
          find.byKey(const ValueKey('seller-floating-navigation-stack')),
        );
        expect(MediaQuery.textScalerOf(pageContext).scale(10), 14);
        expect(
          MediaQuery.paddingOf(pageContext),
          MediaQuery.paddingOf(stackContext),
        );
        expect(tester.takeException(), isNull);
      }
    },
  );
}
