import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/customer/data/customer_model.dart';
import 'package:hawkr_seller/features/customer/presentation/customer_order_card_widget.dart';
import 'package:hawkr_seller/l10n/gen/app_localizations.dart';
import 'package:hawkr_seller/shared/time/taipei_display.dart';

Future<void> _pump(WidgetTester tester, String date, Duration offset) =>
    tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(800, 600),
        builder: (_, _) => MaterialApp(
          locale: const Locale('zh'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: TaipeiDisplayScope(
              deviceOffset: offset,
              child: CustomerOrderCardWidget(
                customerName: 'c',
                order: OrderRecord(
                  orderId: 'o1',
                  date: date,
                  items: 'i',
                  amount: 1,
                ),
              ),
            ),
          ),
        ),
      ),
    );

void main() {
  testWidgets('the wire time string is shown as a Taipei time, labeled', (
    tester,
  ) async {
    await _pump(tester, '2026-07-30T07:05:00Z', const Duration(hours: -5));
    expect(find.text('2026/07/30 15:05 (台北時間)'), findsOneWidget);
  });

  testWidgets('a 13-digit millisecond string parses', (tester) async {
    await _pump(tester, '1785398400000', const Duration(hours: 8));
    expect(find.text('2026/07/30 16:00'), findsOneWidget);
  });

  testWidgets('an unparseable string is shown as is, without a label', (
    tester,
  ) async {
    await _pump(tester, 'soon', const Duration(hours: -5));
    expect(find.text('soon'), findsOneWidget);
  });
}
