import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/wallet/presentation/widgets/coupon_status_tabs.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final width in <double>[320, 390, 768]) {
    testWidgets(
      'coupon status tabs stay equal and overflow-free at ${width}px',
      (tester) async {
        var selectedIndex = 1;

        await _pumpTabs(
          tester,
          width: width,
          selectedIndex: selectedIndex,
          onSelected: (value) => selectedIndex = value,
        );

        expect(tester.takeException(), isNull);
        expect(
          tester
              .getSize(find.byKey(const ValueKey('coupon-status-tabs')))
              .height,
          HkControl.minTap,
        );
        final widths = List.generate(
          CouponStatusTabs.labels.length,
          (index) => tester
              .getSize(find.byKey(ValueKey('coupon-status-tab-$index')))
              .width,
        );
        expect(widths[0], closeTo(widths[1], 0.01));
        expect(widths[1], closeTo(widths[2], 0.01));

        final track = tester.widget<DecoratedBox>(
          find.byKey(const ValueKey('coupon-status-track')),
        );
        final trackDecoration = track.decoration as BoxDecoration;
        expect(trackDecoration.color, isNull);
        expect(trackDecoration.gradient, isNull);
        expect(trackDecoration.border, isNotNull);
        expect(find.byType(BackdropFilter), findsNothing);

        final indicatorFinder = find.byKey(
          const ValueKey('coupon-status-tab-indicator'),
        );
        final indicatorDecoration = tester.widget<DecoratedBox>(
          find
              .descendant(
                of: indicatorFinder,
                matching: find.byType(DecoratedBox),
              )
              .first,
        );
        final indicatorBoxDecoration =
            indicatorDecoration.decoration as BoxDecoration;
        expect(indicatorBoxDecoration.gradient, isNull);
        expect(indicatorBoxDecoration.color, AppColors.brandPrimaryStrong);
        expect(
          indicatorBoxDecoration.borderRadius,
          const BorderRadius.all(Radius.circular(HkRadius.pill)),
        );
        final expectedIndicatorWidth = (widths.first - HkSpace.section).clamp(
          HkSpace.xxl,
          widths.first,
        );
        expect(
          tester.getSize(indicatorFinder).width,
          closeTo(expectedIndicatorWidth, 0.01),
        );

        final selectedContainer = tester.widget<AnimatedContainer>(
          find
              .descendant(
                of: find.byKey(const ValueKey('coupon-status-tab-1')),
                matching: find.byType(AnimatedContainer),
              )
              .first,
        );
        final selectedDecoration =
            selectedContainer.decoration as BoxDecoration;
        expect(selectedDecoration.color, AppColors.primarySubtle);
        expect(find.bySemanticsLabel('已使用'), findsOneWidget);
        expect(find.text('可使用'), findsOneWidget);

        await tester.tap(find.byKey(const ValueKey('coupon-status-tab-2')));
        await tester.pump();
        expect(selectedIndex, 2);
      },
    );
  }
}

Future<void> _pumpTabs(
  WidgetTester tester, {
  required double width,
  required int selectedIndex,
  required ValueChanged<int> onSelected,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 200);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    MaterialApp(
      theme: ThemeData(splashFactory: NoSplash.splashFactory),
      home: Scaffold(
        body: Center(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 24),
            child: CouponStatusTabs(
              selectedIndex: selectedIndex,
              onSelected: onSelected,
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}
