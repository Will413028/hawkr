import 'package:hawkr_buyer/shared/widgets/app_date_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  testWidgets('date picker uses a blurred translucent modal background', (
    tester,
  ) async {
    await pumpProductionLayoutTestApp(
      tester,
      size: const Size(390, 844),
      home: const _DatePickerLauncher(),
    );

    await tester.tap(find.text('選擇生日'));
    await tester.pump(const Duration(milliseconds: 250));

    expect(find.byType(BackdropFilter), findsOneWidget);
    expect(find.byType(AppDatePicker), findsOneWidget);
    final blurSize = tester.getSize(find.byType(BackdropFilter));
    final viewSize = tester.view.physicalSize / tester.view.devicePixelRatio;
    expect(blurSize.width, closeTo(viewSize.width, 1));
    expect(blurSize.height, closeTo(viewSize.height, 1));
    expect(AppDatePicker.barrierOpacity, greaterThan(0));
    expect(AppDatePicker.barrierOpacity, lessThan(0.2));
    expect(tester.takeException(), isNull);
  });

  testWidgets('date picker scrolls within 844x390 safe area at 1.4x text', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    try {
      await pumpProductionLayoutTestApp(
        tester,
        size: const Size(844, 390),
        viewPadding: const EdgeInsets.fromLTRB(59, 0, 59, 21),
        home: const _DatePickerLauncher(),
      );

      await tester.tap(find.text('選擇生日'));
      await tester.pumpAndSettle();

      expect(find.byType(AppDatePicker), findsOneWidget);
      expect(find.byType(SingleChildScrollView), findsOneWidget);
      expect(
        tester.getSize(find.byType(Dialog)).height,
        lessThanOrEqualTo(390 - 21),
      );
      expect(
        tester.getSize(find.byType(SingleChildScrollView)).height,
        lessThanOrEqualTo(390 - 21 - 16),
      );
      expect(
        tester.getSize(find.bySemanticsLabel(RegExp(r'切換年月選擇'))).height,
        greaterThanOrEqualTo(48),
      );
      expect(tester.getSize(find.text('日')).height, greaterThan(20));

      await tester.ensureVisible(find.text('完成'));
      await tester.pump();
      final finishButton = find.ancestor(
        of: find.text('完成'),
        matching: find.byType(TextButton),
      );
      expect(tester.getSize(finishButton).height, greaterThanOrEqualTo(48));
      final finishLabel = tester.widget<Text>(find.text('完成'));
      expect(finishLabel.maxLines, 1);
      expect(finishLabel.softWrap, isFalse);
      expect(tester.takeException(), isNull);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets(
    '320dp keeps full month semantics and platform-safe date targets',
    (tester) async {
      final semantics = tester.ensureSemantics();
      try {
        await pumpProductionLayoutTestApp(
          tester,
          size: const Size(320, 568),
          home: const _DatePickerLauncher(),
        );

        await tester.tap(find.text('選擇生日'));
        await tester.pumpAndSettle();

        final monthControl = find.bySemanticsLabel('民國 89 年 1 月，切換年月選擇');
        expect(monthControl, findsOneWidget);
        expect(tester.getSize(monthControl).height, greaterThanOrEqualTo(48));

        final firstDay = find.bySemanticsLabel('民國 89 年 1 月 1 日');
        expect(firstDay, findsOneWidget);
        final firstDaySize = tester.getSize(firstDay);
        expect(firstDaySize.width, greaterThanOrEqualTo(44));
        expect(firstDaySize.height, greaterThanOrEqualTo(48));

        await tester.tap(find.bySemanticsLabel('下個月'));
        await tester.pump();
        expect(find.bySemanticsLabel('民國 89 年 2 月，切換年月選擇'), findsOneWidget);
        expect(tester.takeException(), isNull);
      } finally {
        semantics.dispose();
      }
    },
  );
}

class _DatePickerLauncher extends StatelessWidget {
  const _DatePickerLauncher();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Builder(
        builder: (context) => TextButton(
          onPressed: () =>
              AppDatePicker.show(context, initial: DateTime(2000, 1, 1)),
          child: const Text('選擇生日'),
        ),
      ),
    );
  }
}
