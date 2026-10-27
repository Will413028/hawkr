import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/features/order/presentation/order_card_widgets/pill_button.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final scale in <double>[0.9, 1, 1.15, 1.4]) {
    testWidgets('order pills keep 48dp targets and one-line copy at $scale', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      await tester.pumpWidget(
        MaterialApp(
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(scale)),
            child: child!,
          ),
          home: Scaffold(
            body: Center(
              child: SizedBox(
                width: 240,
                child: Wrap(
                  alignment: WrapAlignment.end,
                  children: const [
                    PillButton(
                      label: '填寫一段很長的退款與售後處理資訊',
                      color: AppColors.primary,
                      onTap: _noop,
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      );

      expect(tester.takeException(), isNull);
      expect(tester.getSize(find.byType(PillButton)).height, 48);
      final label = tester.widget<Text>(
        find.descendant(
          of: find.byType(HkButtonLabel),
          matching: find.byType(Text),
        ),
      );
      expect(label.maxLines, 1);
      expect(label.softWrap, isFalse);
      expect(label.overflow, TextOverflow.ellipsis);
      expect(find.bySemanticsLabel('填寫一段很長的退款與售後處理資訊'), findsOneWidget);
      semantics.dispose();
    });
  }
}

void _noop() {}
