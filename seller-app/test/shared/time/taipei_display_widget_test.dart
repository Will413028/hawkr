import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/coupon/data/coupon_model.dart';
import 'package:hawkr_seller/features/coupon/presentation/coupon_card_widget.dart';
import 'package:hawkr_seller/l10n/gen/app_localizations.dart';
import 'package:hawkr_seller/shared/time/taipei_display.dart';

const _coupon = Coupon(templateId: 't1', endAt: '2026-07-30T07:05:00Z');

Future<void> _pumpCard(WidgetTester tester, Duration offset) =>
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
              child: const SingleChildScrollView(
                child: CouponCardWidget(coupon: _coupon),
              ),
            ),
          ),
        ),
      ),
    );

void main() {
  testWidgets('CouponCardWidget labels the Taipei date when device is not +8', (
    tester,
  ) async {
    await _pumpCard(tester, const Duration(hours: -5));
    expect(find.text('有效期至 2026/07/30 (台北時間)'), findsOneWidget);
  });

  testWidgets('CouponCardWidget shows no label when device is +8', (
    tester,
  ) async {
    await _pumpCard(tester, const Duration(hours: 8));
    expect(find.text('有效期至 2026/07/30'), findsOneWidget);
    expect(find.textContaining('台北時間'), findsNothing);
  });
}
