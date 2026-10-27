import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/app_bottom_navigation_bar.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('seller bottom navigation is floating glass with five labels', (
    tester,
  ) async {
    var tappedIndex = -1;
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: const SizedBox.expand(),
          bottomNavigationBar: AppBottomNavigationBar(
            currentIndex: NavIndex.order,
            onTap: (index) => tappedIndex = index,
          ),
        ),
      ),
    );
    await tester.pump();

    expect(
      tester.getSize(find.byKey(const ValueKey('seller-bottom-nav-surface'))),
      const Size(560, 68),
    );
    for (final label in const ['總覽', '商店', '錢包', '訂單', '設定']) {
      expect(find.text(label), findsOneWidget);
      expect(find.bySemanticsLabel(label), findsOneWidget);
    }
    expect(find.byType(BackdropFilter), findsWidgets);
    final surface = tester.widget<Container>(
      find.byKey(const ValueKey('seller-bottom-nav-surface')),
    );
    final surfaceDecoration = surface.decoration! as BoxDecoration;
    final surfaceGradient = surfaceDecoration.gradient! as LinearGradient;
    expect(
      surfaceGradient.colors.first.a,
      closeTo(HkGlass.navBarHighlightOpacity, 0.001),
    );
    expect(
      surfaceGradient.colors.last.a,
      closeTo(HkGlass.navBarSurfaceOpacity, 0.001),
    );
    final navBackground = tester.widget<ColoredBox>(
      find
          .ancestor(
            of: find.byKey(const ValueKey('seller-bottom-nav-surface')),
            matching: find.byType(ColoredBox),
          )
          .first,
    );
    expect(navBackground.color, Colors.transparent);
    expect(find.byType(HkIcon), findsNWidgets(5));

    HkIcon iconFor(String label) => tester.widget<HkIcon>(
      find.descendant(
        of: find.bySemanticsLabel(label),
        matching: find.byType(HkIcon),
      ),
    );
    expect(iconFor('訂單').selected, isTrue);
    expect(iconFor('商店').selected, isFalse);
    IconData glyphFor(String label) => tester
        .widget<Icon>(
          find.descendant(
            of: find.bySemanticsLabel(label),
            matching: find.byType(Icon),
          ),
        )
        .icon!;
    expect(glyphFor('訂單'), AppIcons.shoppingBag, reason: '選取態用 filled，不只靠顏色');
    expect(glyphFor('商店'), AppIcons.categoryOutlined);

    await tester.tap(find.text('商店'));
    await tester.pump();
    expect(tappedIndex, NavIndex.category.value);
    expect(iconFor('商店').selected, isTrue);
    expect(glyphFor('商店'), AppIcons.category);
    await tester.pumpAndSettle();
    expect(
      tester
          .getCenter(find.byKey(const ValueKey('seller-bottom-nav-indicator')))
          .dx,
      closeTo(tester.getCenter(find.text('商店')).dx, 1),
    );

    semantics.dispose();
  });

  testWidgets('dragging across icons previews then commits the selected tab', (
    tester,
  ) async {
    final selectedIndexes = <int>[];

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: const SizedBox.expand(),
          bottomNavigationBar: AppBottomNavigationBar(
            currentIndex: NavIndex.dashboard,
            onTap: selectedIndexes.add,
          ),
        ),
      ),
    );
    await tester.pump();

    final start = tester.getCenter(find.text('總覽'));
    final end = tester.getCenter(find.text('訂單'));
    await tester.dragFrom(start, end - start, touchSlopX: 4);
    await tester.pumpAndSettle();

    expect(selectedIndexes, [NavIndex.order.value]);
    expect(
      tester
          .getCenter(find.byKey(const ValueKey('seller-bottom-nav-indicator')))
          .dx,
      closeTo(end.dx, 1),
    );
    expect(tester.takeException(), isNull);
  });
}
