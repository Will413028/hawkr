import 'dart:ui' show SemanticsAction, Tristate;

import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';
import 'package:hawkr_buyer/features/order/presentation/order_card_widgets/retry_payment_sheet.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:hawkr_buyer/shared/widgets/swipe_to_pay.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

const _longOrder = Order(
  totalOriginalAmount: '123456789012345678901234567890',
  shippingFee: '12345678901234567890',
  totalActualAmount: '123456789012345678901234567890',
  availablePaymentMethods: [
    {'paymentType': '2002', 'eligible': false, 'ineligibleReason': '2101'},
    {'paymentType': '2004', 'eligible': true, 'ineligibleReason': ''},
    {'paymentType': '2001', 'eligible': false, 'ineligibleReason': '2102'},
  ],
);

void main() {
  final deviceSizes = <Size>[
    const Size(320, 568),
    const Size(390, 844),
    const Size(768, 1024),
  ];
  const textCases = [
    (name: 'standard 1.0x', systemScale: 1.0, textSize: AppTextSize.standard),
    (name: 'App Large 1.15x', systemScale: 1.0, textSize: AppTextSize.large),
    (name: 'maximum 1.4x', systemScale: 2.0, textSize: AppTextSize.large),
  ];

  for (final textCase in textCases) {
    for (final size in deviceSizes) {
      testWidgets(
        'retry payment stays reachable at ${size.width}x${size.height} and ${textCase.name}',
        (tester) async {
          String? selectedPayType;
          final semantics = tester.ensureSemantics();

          await _pumpSheetHost(
            tester,
            size: size,
            systemTextScale: textCase.systemScale,
            textSize: textCase.textSize,
            onPaymentSelected: (payType) async {
              selectedPayType = payType;
            },
          );
          await tester.tap(find.byKey(const ValueKey('open-retry-payment')));
          await tester.pumpAndSettle();

          final scroll = find.byKey(
            const PageStorageKey('retry-payment-sheet-scroll'),
          );
          expect(scroll, findsOneWidget);
          expect(
            find.ancestor(of: scroll, matching: find.byType(SafeArea)),
            findsWidgets,
          );
          _expectNoFlutterErrors(tester);

          for (final value in const [
            'NT\$ 123456789012345678901234567890',
            'NT\$ 12345678901234567890',
          ]) {
            final paragraph = tester.renderObject<RenderParagraph>(
              find.text(value).first,
            );
            expect(
              paragraph.didExceedMaxLines,
              isFalse,
              reason: '$value must remain complete at ${textCase.name}',
            );
          }

          final atmNote = find.text('虛擬帳號只能用網路銀行跟ATM轉帳，不能臨櫃匯款');
          expect(atmNote, findsOneWidget);
          expect(
            tester.renderObject<RenderParagraph>(atmNote).didExceedMaxLines,
            isFalse,
          );
          for (final label in const ['Apple Pay', 'ATM 虛擬轉帳', '信用卡']) {
            final labelFinder = find.text(label);
            final text = tester.widget<Text>(labelFinder);
            final paragraph = tester.renderObject<RenderParagraph>(labelFinder);
            expect(text.maxLines, 1);
            expect(text.softWrap, isFalse);
            expect(
              paragraph.didExceedMaxLines,
              isFalse,
              reason:
                  '$label must remain complete at ${textCase.name}; '
                  'size=${tester.getSize(labelFinder)}, '
                  'text=${paragraph.text.toPlainText()}, '
                  'intrinsic=${paragraph.getMaxIntrinsicWidth(double.infinity)}',
            );
          }
          for (final reason in const ['金額未達此付款方式下限', '金額超過此付款方式上限']) {
            expect(
              tester
                  .renderObject<RenderParagraph>(find.text(reason))
                  .didExceedMaxLines,
              isFalse,
            );
          }

          if (size.width == 320) {
            expect(
              find.byKey(const ValueKey('retry-payment-info-商品總金額-stacked')),
              findsOneWidget,
            );
            expect(
              find.byKey(const ValueKey('retry-payment-info-總付款金額-stacked')),
              findsOneWidget,
            );
          }
          final atmMethod = find.byKey(
            const ValueKey('retry-payment-method-2004'),
          );
          await tester.ensureVisible(atmMethod);
          await tester.pump();
          expect(tester.getRect(atmMethod).height, greaterThanOrEqualTo(48));
          expect(atmMethod.hitTestable(), findsOneWidget);
          final atmData = tester
              .getSemantics(find.bySemanticsLabel('ATM 虛擬轉帳'))
              .getSemanticsData();
          expect(atmData.flagsCollection.isButton, isTrue);
          expect(atmData.flagsCollection.isSelected, Tristate.isTrue);
          expect(atmData.flagsCollection.isEnabled, Tristate.isTrue);
          expect(atmData.hasAction(SemanticsAction.tap), isTrue);

          final swipe = find.byType(SwipeToPay);
          await tester.ensureVisible(swipe);
          await tester.pump();
          final viewportRect = tester.getRect(scroll);
          final swipeRect = tester.getRect(swipe);
          expect(swipeRect.height, greaterThanOrEqualTo(48));
          expect(swipeRect.left, greaterThanOrEqualTo(viewportRect.left));
          expect(swipeRect.right, lessThanOrEqualTo(viewportRect.right));
          expect(swipeRect.top, greaterThanOrEqualTo(viewportRect.top));
          expect(swipeRect.bottom, lessThanOrEqualTo(viewportRect.bottom));
          expect(swipe.hitTestable(), findsOneWidget);

          final swipeLabel = find.descendant(
            of: find.byType(HkButtonLabel),
            matching: find.text('向右滑結帳'),
          );
          final text = tester.widget<Text>(swipeLabel);
          final paragraph = tester.renderObject<RenderParagraph>(swipeLabel);
          expect(text.maxLines, 1);
          expect(text.softWrap, isFalse);
          expect(paragraph.didExceedMaxLines, isFalse);

          final swipeData = tester
              .getSemantics(find.bySemanticsLabel('向右滑結帳'))
              .getSemanticsData();
          expect(swipeData.flagsCollection.isButton, isTrue);
          expect(swipeData.flagsCollection.isEnabled, Tristate.isTrue);
          expect(swipeData.hasAction(SemanticsAction.tap), isTrue);

          await tester.drag(swipe, const Offset(500, 0));
          await tester.pump();
          expect(selectedPayType, '2004');
          _expectNoFlutterErrors(tester);
          semantics.dispose();
        },
      );
    }
  }
}

Future<void> _pumpSheetHost(
  WidgetTester tester, {
  required Size size,
  required double systemTextScale,
  required AppTextSize textSize,
  required Future<void> Function(String payType) onPaymentSelected,
}) {
  return pumpProductionLayoutTestApp(
    tester,
    size: size,
    systemTextScale: systemTextScale,
    textSize: textSize,
    viewPadding: const EdgeInsets.only(top: 24, bottom: 16),
    home: Scaffold(
      body: Builder(
        builder: (context) => Center(
          child: FilledButton(
            key: const ValueKey('open-retry-payment'),
            onPressed: () => showRetryPaymentSheet(
              context,
              order: _longOrder,
              currentPayType: null,
              onPaymentSelected: onPaymentSelected,
            ),
            child: const Text('開啟再次付款'),
          ),
        ),
      ),
    ),
  );
}

void _expectNoFlutterErrors(WidgetTester tester) {
  final errors = <Object>[];
  Object? error;
  while ((error = tester.takeException()) != null) {
    errors.add(error!);
  }
  expect(errors, isEmpty);
}
