import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_empty_state.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';

void main() {
  for (final width in <double>[320, 390, 768]) {
    testWidgets('keeps the empty-state icon aligned at ${width.toInt()}px', (
      tester,
    ) async {
      await tester.binding.setSurfaceSize(Size(width, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: HkEmptyState(
              key: ValueKey('empty-state'),
              iconKey: ValueKey('empty-state-icon'),
              icon: HkIconName.emptyOrders,
              message: '暫無訂單',
            ),
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 100));

      final state = find.byKey(const ValueKey('empty-state'));
      final icon = find.byKey(const ValueKey('empty-state-icon'));
      final iconWidget = tester.widget<HkIcon>(icon);

      expect(iconWidget.name, HkIconName.emptyOrders);
      expect(iconWidget.size, HkEmptyState.iconSize);
      expect(tester.getSize(icon), const Size.square(HkEmptyState.iconSize));
      expect(
        tester.getTopLeft(icon).dy - tester.getTopLeft(state).dy,
        HkEmptyState.topInset +
            (HkEmptyState.haloSize - HkEmptyState.iconSize) / 2,
      );
      expect(tester.getCenter(icon).dx, closeTo(width / 2, 0.01));
      expect(
        tester
            .widget<Icon>(
              find.descendant(of: icon, matching: find.byType(Icon)),
            )
            .icon!
            .fontFamily,
        'MaterialIcons',
      );
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('supports the coupon icon and one concise semantic label', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: HkEmptyState(icon: HkIconName.emptyCoupon, message: '暫無優惠券'),
        ),
      ),
    );
    await tester.pump(const Duration(milliseconds: 100));

    final icon = tester.widget<HkIcon>(find.byType(HkIcon));
    expect(icon.name, HkIconName.emptyCoupon);
    expect(icon.color, AppIconColors.active);
    expect(
      tester.widget<Icon>(find.byType(Icon)).icon,
      AppIcons.confirmationNumberOutlined,
    );
    expect(find.bySemanticsLabel('暫無優惠券'), findsOneWidget);
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });

  testWidgets('uses distinct icons for the two wallet empty states', (
    tester,
  ) async {
    Future<IconData?> glyph(HkIconName name) async {
      await tester.pumpWidget(MaterialApp(home: HkIcon(name)));
      return tester.widget<Icon>(find.byType(Icon)).icon;
    }

    expect(
      await glyph(HkIconName.emptyMemberCard),
      isNot(await glyph(HkIconName.emptyCoupon)),
    );
  });
}
