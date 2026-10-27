import 'package:hawkr_seller/core/layout/app_media_query_policy.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/widgets/app_date_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('date picker uses a blurred translucent modal background', (
    tester,
  ) async {
    await _pumpProductionLayoutTestApp(
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
      await _pumpProductionLayoutTestApp(
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
        await _pumpProductionLayoutTestApp(
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

  testWidgets('390dp keeps 48dp date targets', (tester) async {
    final semantics = tester.ensureSemantics();
    try {
      await _pumpProductionLayoutTestApp(
        tester,
        size: const Size(390, 844),
        home: const _DatePickerLauncher(),
      );

      await tester.tap(find.text('選擇生日'));
      await tester.pumpAndSettle();

      final firstDay = find.bySemanticsLabel('民國 89 年 1 月 1 日');
      expect(firstDay, findsOneWidget);
      final firstDaySize = tester.getSize(firstDay);
      expect(firstDaySize.width, greaterThanOrEqualTo(48));
      expect(firstDaySize.height, greaterThanOrEqualTo(48));
      expect(tester.takeException(), isNull);
    } finally {
      semantics.dispose();
    }
  });
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

Future<void> _pumpProductionLayoutTestApp(
  WidgetTester tester, {
  required Size size,
  required Widget home,
  double systemTextScale = 2,
  EdgeInsets viewPadding = EdgeInsets.zero,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(414, 896),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      minTextAdapt: false,
      splitScreenMode: true,
      builder: (context, _) => MaterialApp(
        theme: ThemeData(
          useMaterial3: true,
          colorScheme: ColorScheme.fromSeed(
            seedColor: AppColors.brandPrimaryStrong,
          ),
        ),
        builder: (context, child) {
          final platformMediaQuery = MediaQuery.of(context).copyWith(
            size: size,
            padding: viewPadding,
            viewPadding: viewPadding,
            textScaler: TextScaler.linear(systemTextScale),
          );
          return MediaQuery(
            data: AppMediaQueryPolicy.apply(platformMediaQuery),
            child: child ?? const SizedBox.shrink(),
          );
        },
        home: home,
      ),
    ),
  );
  await tester.pump();
}
