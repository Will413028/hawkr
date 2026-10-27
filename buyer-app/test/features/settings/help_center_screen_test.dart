import 'package:hawkr_buyer/features/settings/presentation/settings_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

const _lineId = '@support-line';
const _lineUrl = 'https://line.example/support';

void main() {
  for (final scenario in <({double width, double textScale})>[
    (width: 320, textScale: 1.4),
    (width: 390, textScale: 1.4),
  ]) {
    testWidgets('help center keeps contact layout readable at '
        '${scenario.width.toInt()}px and ${scenario.textScale}x text', (
      tester,
    ) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = Size(scenario.width, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ScreenUtilInit(
          designSize: const Size(393, 852),
          builder: (context, child) => MaterialApp(
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(
                context,
              ).copyWith(textScaler: TextScaler.linear(scenario.textScale)),
              child: child!,
            ),
            home: const HelpCenterScreen(lineId: _lineId, lineUrl: _lineUrl),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('幫助中心'), findsOneWidget);
      expect(find.text('常見問題'), findsOneWidget);
      expect(tester.takeException(), isNull);

      await tester.scrollUntilVisible(
        find.byKey(const ValueKey('help-line-contact')),
        240,
        scrollable: find.byType(Scrollable).first,
      );
      await tester.pumpAndSettle();

      expect(find.text(_lineId), findsOneWidget);
      expect(find.text(helpCenterEmail), findsOneWidget);
      expect(find.text('透過 LINE 聯絡客服'), findsOneWidget);
      expect(find.text('服務時間：週一至週五 09:00–18:00'), findsOneWidget);
      expect(find.textContaining('freshmarket'), findsNothing);
      final emailText = tester.widget<Text>(find.text(helpCenterEmail));
      expect(emailText.maxLines, 1);
      expect(emailText.softWrap, isFalse);
      final emailBounds = tester.getRect(
        find.byKey(ValueKey('help-contact-value-$helpCenterEmail')),
      );
      final emailRowBounds = tester.getRect(
        find.byKey(const ValueKey('help-email-contact')),
      );
      expect(emailBounds.height, lessThan(emailRowBounds.height / 2));
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('help center FAQ expands with its answer', (tester) async {
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(393, 852),
        builder: (context, child) =>
            const MaterialApp(home: HelpCenterScreen()),
      ),
    );

    await tester.tap(find.text('如何取消訂單？'));
    await tester.pumpAndSettle();

    expect(find.text('訂單成立後請直接與賣家聯繫協助取消，買家無法自行取消訂單。'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  for (final scenario in <({String name, String lineId, String lineUrl})>[
    (name: 'no LINE settings', lineId: '', lineUrl: ''),
    (name: 'LINE id without url', lineId: _lineId, lineUrl: ''),
  ]) {
    testWidgets('help center hides LINE entry points with ${scenario.name}', (
      tester,
    ) async {
      await tester.pumpWidget(
        ScreenUtilInit(
          designSize: const Size(393, 852),
          builder: (context, child) => MaterialApp(
            home: HelpCenterScreen(
              lineId: scenario.lineId,
              lineUrl: scenario.lineUrl,
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.scrollUntilVisible(
        find.byKey(const ValueKey('help-email-contact')),
        240,
        scrollable: find.byType(Scrollable).first,
      );

      expect(find.byKey(const ValueKey('help-line-contact')), findsNothing);
      expect(find.byKey(const ValueKey('help-line-cta')), findsNothing);
      expect(find.text('透過 LINE 聯絡客服'), findsNothing);
      expect(find.text(helpCenterEmail), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }

  test('LINE entry points are disabled unless defined at build time', () {
    expect(helpCenterLineId, '');
    expect(helpCenterLineUrl, '');
  });
}
