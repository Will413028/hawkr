import 'package:hawkr_seller/features/settings/presentation/help_center_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

const _lineId = '@support-line';
const _lineUrl = 'https://line.example/support';

Future<void> _pump(WidgetTester tester, HelpCenterScreen screen) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(393, 1600);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(393, 852),
      builder: (context, child) => MaterialApp(home: screen),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('help center shows the configured LINE entry points', (
    tester,
  ) async {
    await _pump(
      tester,
      const HelpCenterScreen(lineId: _lineId, lineUrl: _lineUrl),
    );

    expect(find.byKey(const ValueKey('help-line-contact')), findsOneWidget);
    expect(find.byKey(const ValueKey('help-line-cta')), findsOneWidget);
    expect(find.text('ID：$_lineId'), findsOneWidget);
    expect(find.text(helpCenterEmail), findsOneWidget);
  });

  for (final scenario in <({String name, String lineId, String lineUrl})>[
    (name: 'no LINE settings', lineId: '', lineUrl: ''),
    (name: 'LINE id without url', lineId: _lineId, lineUrl: ''),
  ]) {
    testWidgets('help center hides LINE entry points with ${scenario.name}', (
      tester,
    ) async {
      await _pump(
        tester,
        HelpCenterScreen(lineId: scenario.lineId, lineUrl: scenario.lineUrl),
      );

      expect(find.byKey(const ValueKey('help-line-contact')), findsNothing);
      expect(find.byKey(const ValueKey('help-line-cta')), findsNothing);
      expect(find.text('線上客服'), findsNothing);
      expect(find.text(helpCenterEmail), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }

  test('LINE entry points are disabled unless defined at build time', () {
    expect(helpCenterLineId, '');
    expect(helpCenterLineUrl, '');
  });
}
