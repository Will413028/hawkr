import 'package:hawkr_seller/shared/widgets/hk_empty_state.dart';
import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _testApp(Widget child) => ScreenUtilInit(
  designSize: const Size(414, 896),
  minTextAdapt: true,
  splitScreenMode: true,
  builder: (_, __) => MaterialApp(home: Scaffold(body: child)),
);

void main() {
  for (final width in <double>[320, 390, 768]) {
    testWidgets('keeps the empty-state geometry at ${width.toInt()}px', (
      tester,
    ) async {
      await tester.binding.setSurfaceSize(Size(width, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      await tester.pumpWidget(
        _testApp(
          const HkEmptyState(
            key: ValueKey('empty-state'),
            iconKey: ValueKey('empty-state-icon'),
            icon: HkIconName.box,
            message: '暫無訂單',
            description: '買家下單後會自動出現在這裡。',
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 100));

      final state = find.byKey(const ValueKey('empty-state'));
      final icon = find.byKey(const ValueKey('empty-state-icon'));
      final iconWidget = tester.widget<HkIcon>(icon);

      expect(iconWidget.name, HkIconName.box);
      expect(iconWidget.size, HkEmptyState.iconSize);
      expect(iconWidget.color, AppIconColors.active);
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
            .icon
            ?.fontFamily,
        'MaterialIcons',
      );
      expect(
        find.descendant(of: state, matching: find.byType(SvgPicture)),
        findsNothing,
      );
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('exposes one concise semantic label', (tester) async {
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      _testApp(
        const HkEmptyState(icon: HkIconName.notification, message: '暫無通知'),
      ),
    );
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.bySemanticsLabel('暫無通知'), findsOneWidget);
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });
}
