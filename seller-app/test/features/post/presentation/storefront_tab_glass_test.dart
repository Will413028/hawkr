import 'package:hawkr_seller/features/post/presentation/widgets/storefront_tab_glass.dart';
import 'package:hawkr_seller/shared/widgets/hk_frosted_card_surface.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('storefront empty state uses the shared frosted surface', (
    tester,
  ) async {
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(393, 852),
        minTextAdapt: true,
        builder: (_, __) => const MaterialApp(
          home: Scaffold(
            body: StorefrontTabCanvas(
              child: StorefrontGlassEmptyPanel(
                icon: HkIconName.chat,
                message: '暫無留言',
                description: '買家留言後會顯示在這裡。',
              ),
            ),
          ),
        ),
      ),
    );

    expect(find.byType(HkFrostedCardSurface), findsOneWidget);
    expect(find.text('暫無留言'), findsOneWidget);
    expect(find.text('買家留言後會顯示在這裡。'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('storefront tab stays transparent over the page gradient', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(body: StorefrontTabCanvas(child: SizedBox.expand())),
      ),
    );

    final layer = tester.widget<ColoredBox>(
      find.byKey(const ValueKey('storefront-transparent-tab-layer')),
    );
    expect(layer.color, Colors.transparent);
    expect(tester.takeException(), isNull);
  });

  testWidgets('storefront glass action keeps an icon and tap semantics', (
    tester,
  ) async {
    var tapped = false;
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Center(
            child: StorefrontGlassActionButton(
              icon: HkIconName.add,
              label: '新增商品',
              onTap: () => tapped = true,
            ),
          ),
        ),
      ),
    );

    expect(find.byType(HkIcon), findsOneWidget);
    expect(find.bySemanticsLabel('新增商品'), findsOneWidget);
    await tester.tap(find.text('新增商品'));
    expect(tapped, isTrue);
    expect(tester.takeException(), isNull);

    semantics.dispose();
  });
}
