import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/l10n/gen/app_localizations.dart';
import 'package:hawkr_buyer/shared/time/taipei_display.dart';
import 'package:hawkr_buyer/shared/widgets/chat_order_bubble.dart';

const _message =
    '{"messageType":"order_prev","orderNo":"A1","createTime":"2026-07-30T07:05:00Z"}';

Future<void> _pumpBubble(WidgetTester tester, Duration offset) =>
    tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(414, 896),
        builder: (_, _) => MaterialApp(
          locale: const Locale('zh'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: TaipeiDisplayScope(
              deviceOffset: offset,
              child: const ChatOrderBubble(
                messageContent: _message,
                isSelf: true,
              ),
            ),
          ),
        ),
      ),
    );

void main() {
  testWidgets('ChatOrderBubble labels the Taipei date when device is not +8', (
    tester,
  ) async {
    await _pumpBubble(tester, const Duration(hours: -5));
    expect(find.text('2026/07/30 (台北時間)'), findsOneWidget);
  });

  testWidgets('ChatOrderBubble shows no label when device is +8', (
    tester,
  ) async {
    await _pumpBubble(tester, const Duration(hours: 8));
    expect(find.text('2026/07/30'), findsOneWidget);
    expect(find.textContaining('台北時間'), findsNothing);
  });
}
