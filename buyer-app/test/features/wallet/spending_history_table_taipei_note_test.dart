import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/wallet/presentation/spending_history_screen.dart';
import 'package:hawkr_buyer/l10n/gen/app_localizations.dart';
import 'package:hawkr_buyer/shared/time/taipei_display.dart';

const _records = <Map<String, dynamic>>[
  {'date': '2026/07/30 15:05', 'orderId': 'A1', 'amount': 100, 'refund': 0.0},
];

Future<void> _pump(WidgetTester tester, Duration offset) => tester.pumpWidget(
  ScreenUtilInit(
    designSize: const Size(414, 896),
    builder: (_, _) => MaterialApp(
      locale: const Locale('zh'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(
        body: TaipeiDisplayScope(
          deviceOffset: offset,
          child: const SingleChildScrollView(
            child: SpendingHistoryTable(records: _records),
          ),
        ),
      ),
    ),
  ),
);

void main() {
  testWidgets('shows one caption and no row label when not +8', (tester) async {
    await _pump(tester, const Duration(hours: -5));
    expect(find.text('時間為台北時間'), findsOneWidget);
    expect(find.text('2026/07/30 15:05'), findsOneWidget);
    expect(find.textContaining('(台北時間)'), findsNothing);
  });

  testWidgets('shows no caption when device is +8', (tester) async {
    await _pump(tester, const Duration(hours: 8));
    expect(find.textContaining('台北時間'), findsNothing);
    expect(find.text('2026/07/30 15:05'), findsOneWidget);
  });
}
