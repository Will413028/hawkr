import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';
import 'package:hawkr_buyer/features/order/presentation/order_card_widgets/atm_info_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  const order = Order(
    atmBankCode: '812',
    atmVirtualAccount: '81212345678901234567',
    atmExpireDate: '2026-09-09T23:59:59+08:00',
    totalActualAmount: '123456789',
  );

  for (final scenario in <({Size size, double scale, AppTextSize textSize})>[
    (size: const Size(320, 568), scale: 1, textSize: AppTextSize.large),
    (size: const Size(320, 568), scale: 2, textSize: AppTextSize.standard),
    (size: const Size(390, 844), scale: 2, textSize: AppTextSize.standard),
    (size: const Size(768, 1024), scale: 2, textSize: AppTextSize.standard),
  ]) {
    testWidgets(
      'ATM details fit and remain reachable at ${scenario.size.width.toInt()}dp',
      (tester) async {
        await pumpProductionLayoutTestApp(
          tester,
          size: scenario.size,
          systemTextScale: scenario.scale,
          textSize: scenario.textSize,
          viewPadding: const EdgeInsets.only(bottom: 16),
          home: Scaffold(
            body: Builder(
              builder: (context) => TextButton(
                onPressed: () => showAtmInfoSheet(context, order: order),
                child: const Text('開啟'),
              ),
            ),
          ),
        );

        await tester.tap(find.text('開啟'));
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        for (final value in <String>[
          order.atmBankCode,
          order.atmVirtualAccount,
          order.atmExpireDate,
          'NT\$${order.totalActualAmount}',
        ]) {
          expect(find.text(value), findsOneWidget);
        }
        final guidance = find.text('請於期限內完成匯款，逾時訂單將自動取消。');
        expect(guidance, findsOneWidget);
        await tester.ensureVisible(guidance);
        await tester.pump();
        expect(guidance.hitTestable(), findsOneWidget);

        final accountLabel = tester.getRect(find.text('虛擬帳號'));
        final accountValue = tester.getRect(
          find.byKey(const ValueKey('atm-info-value-虛擬帳號')),
        );
        if (scenario.size.width < 768) {
          expect(accountValue.top, greaterThan(accountLabel.bottom));
        } else {
          expect((accountValue.top - accountLabel.top).abs(), lessThan(1));
        }
        expect(tester.takeException(), isNull);
      },
    );
  }
}
