import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_tab_bar.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
    'keeps the animated pill but suppresses the Material tap splash',
    (tester) async {
      await tester.pumpWidget(const MaterialApp(home: _TabBarHarness()));

      final initialLeft = tester
          .getTopLeft(find.byKey(const Key('merchant_tab_indicator')))
          .dx;
      final selectedStyle = tester
          .widget<Text>(find.byKey(const Key('merchant_tab_label_1')))
          .style!;
      final inactiveStyle = tester
          .widget<Text>(find.byKey(const Key('merchant_tab_label_0')))
          .style!;

      expect(selectedStyle.fontSize, inactiveStyle.fontSize);
      expect(selectedStyle.fontWeight, FontWeight.w700);
      expect(inactiveStyle.fontWeight, FontWeight.w500);
      expect(selectedStyle.fontSize, 18);

      final selectedLabel = tester.getRect(
        find.byKey(const Key('merchant_tab_label_1')),
      );
      final indicator = tester.getRect(
        find.byKey(const Key('merchant_tab_indicator')),
      );
      final indicatorBox = tester.widget<DecoratedBox>(
        find.byKey(const Key('merchant_tab_indicator')),
      );
      final decoration = indicatorBox.decoration as BoxDecoration;
      expect(indicator.height, 38);
      expect(selectedLabel.center.dy, closeTo(indicator.center.dy, 0.6));
      expect(decoration.color, isNull);
      expect(decoration.gradient, isA<LinearGradient>());
      expect(decoration.border, isNotNull);
      expect(decoration.boxShadow, isNotEmpty);
      expect(find.byType(BackdropFilter), findsOneWidget);

      final selectedInkWell = tester.widget<InkWell>(
        find.ancestor(
          of: find.byKey(const Key('merchant_tab_label_1')),
          matching: find.byType(InkWell),
        ),
      );
      expect(selectedInkWell.splashFactory, NoSplash.splashFactory);
      expect(
        selectedInkWell.overlayColor!.resolve({WidgetState.pressed}),
        Colors.transparent,
      );

      final tabDistance =
          tester.getCenter(find.text('留言')).dx -
          tester.getCenter(find.text('商城')).dx;
      await tester.tap(find.text('留言'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 80));

      final selectedLeft = tester
          .getTopLeft(find.byKey(const Key('merchant_tab_indicator')))
          .dx;
      final finalTargetLeft = initialLeft + tabDistance;
      expect(selectedLeft, greaterThan(initialLeft));
      expect(selectedLeft, lessThan(finalTargetLeft));

      await tester.pumpAndSettle();
      expect(
        tester.getTopLeft(find.byKey(const Key('merchant_tab_indicator'))).dx,
        closeTo(finalTargetLeft, 0.1),
      );
    },
  );

  testWidgets('merchant tabs use filled Material icons when selected', (
    tester,
  ) async {
    await tester.pumpWidget(const MaterialApp(home: _TabBarHarness()));
    await tester.pumpAndSettle();

    final postsFinder = find.byKey(const ValueKey('merchant-posts-tab-icon'));
    final shopFinder = find.byKey(const ValueKey('merchant-shop-tab-icon'));
    final messagesFinder = find.byKey(
      const ValueKey('merchant-messages-tab-icon'),
    );
    Icon glyph(Finder tab) => tester.widget<Icon>(
      find.descendant(of: tab, matching: find.byType(Icon)),
    );

    expect(tester.widget<HkIcon>(postsFinder).name, HkIconName.merchantPosts);
    expect(tester.widget<HkIcon>(shopFinder).name, HkIconName.merchantShop);
    expect(tester.widget<HkIcon>(messagesFinder).name, HkIconName.chat);
    expect(glyph(postsFinder).icon, AppIcons.articleOutlined);
    expect(glyph(postsFinder).color, AppIconColors.muted);
    expect(glyph(shopFinder).icon, AppIcons.storefront);
    expect(glyph(shopFinder).color, AppIconColors.strong);
    expect(glyph(messagesFinder).icon, AppIcons.chatBubbleOutline);

    await tester.tap(find.text('留言'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 120));
    expect(glyph(messagesFinder).icon, AppIcons.chatBubble);
    expect(glyph(shopFinder).icon, AppIcons.storefrontOutlined);
  });

  testWidgets('dragging the tab track previews and commits the nearest tab', (
    tester,
  ) async {
    await tester.pumpWidget(const MaterialApp(home: _TabBarHarness()));

    final start = tester.getCenter(
      find.byKey(const Key('merchant_tab_label_1')),
    );
    final target = tester.getCenter(
      find.byKey(const Key('merchant_tab_label_0')),
    );
    final initialIndicatorCenter = tester
        .getRect(find.byKey(const Key('merchant_tab_indicator')))
        .center
        .dx;
    final slotDistance = start.dx - target.dx;
    final firstSlotCenter = initialIndicatorCenter - slotDistance;
    final gesture = await tester.startGesture(start);
    await gesture.moveTo(target);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 180));
    final preview = tester.getRect(
      find.byKey(const Key('merchant_tab_indicator')),
    );
    expect(preview.center.dx, closeTo(firstSlotCenter, 8));
    final draggingDecoration =
        tester
                .widget<DecoratedBox>(
                  find.byKey(const Key('merchant_tab_indicator')),
                )
                .decoration
            as BoxDecoration;
    final draggingScale = tester.widget<AnimatedScale>(
      find.ancestor(
        of: find.byKey(const Key('merchant_tab_indicator')),
        matching: find.byType(AnimatedScale),
      ),
    );
    expect(draggingDecoration.boxShadow, isNotEmpty);
    expect(draggingScale.scale, 1.04);

    await gesture.up();
    await tester.pumpAndSettle();
    final indicator = tester.getRect(
      find.byKey(const Key('merchant_tab_indicator')),
    );
    expect(indicator.center.dx, closeTo(firstSlotCenter, 1));
    final selectedStyle = tester
        .widget<Text>(find.byKey(const Key('merchant_tab_label_0')))
        .style!;
    expect(selectedStyle.fontWeight, FontWeight.w700);
  });
}

class _TabBarHarness extends StatefulWidget {
  const _TabBarHarness();

  @override
  State<_TabBarHarness> createState() => _TabBarHarnessState();
}

class _TabBarHarnessState extends State<_TabBarHarness>
    with SingleTickerProviderStateMixin {
  late final TabController _controller = TabController(
    length: 3,
    initialIndex: 1,
    vsync: this,
  );

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Align(
        alignment: Alignment.topCenter,
        child: SizedBox(
          width: 390,
          child: MerchantTabBar(tabController: _controller),
        ),
      ),
    );
  }
}
