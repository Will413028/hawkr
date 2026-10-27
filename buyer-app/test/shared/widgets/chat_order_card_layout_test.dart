import 'dart:convert';

import 'package:hawkr_buyer/shared/widgets/chat_order_bubble.dart';
import 'package:hawkr_buyer/shared/widgets/chat_order_picker_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

const _orderNo = 'HK202605070001';
const _amount = '123456789';
const _sellerName = '範例生鮮友善農產合作超級長名稱店舖';
const _order = <String, dynamic>{
  'messageType': 'order_check',
  'orderNo': _orderNo,
  'status': '60',
  'totalActualAmount': _amount,
  'sellerName': _sellerName,
  'createTime': '2026-09-08T04:00:00Z',
  'items': [
    {'productName': '台灣產地直送超長名稱當令蔬果箱', 'quantity': 2},
  ],
};

void main() {
  testWidgets(
    'chat order bubble reflows real order data at 320dp and effective 1.4x',
    (tester) async {
      var tapCount = 0;
      final semantics = tester.ensureSemantics();

      await pumpProductionLayoutTestApp(
        tester,
        size: const Size(320, 568),
        systemTextScale: 2,
        home: Scaffold(
          body: Align(
            alignment: Alignment.topLeft,
            child: Padding(
              padding: const EdgeInsets.only(left: 40, right: 8),
              child: ChatOrderBubble(
                messageContent: jsonEncode(_order),
                isSelf: false,
                onTap: () => tapCount += 1,
              ),
            ),
          ),
        ),
      );

      final card = find.byKey(const ValueKey('chat-order-bubble-$_orderNo'));
      expect(card, findsOneWidget);
      _expectInsideViewport(tester, card);
      _expectPrimaryValuesComplete(tester);
      final semanticData = tester.getSemantics(card).getSemanticsData();
      expect(semanticData.label, contains(_orderNo));
      expect(semanticData.label, contains(_amount));
      expect(semanticData.label, contains(_sellerName));
      expect(tester.takeException(), isNull);

      await tester.tap(card);
      expect(tapCount, 1);
      semantics.dispose();
    },
  );

  testWidgets(
    'chat order picker reflows and returns the selected order at 320dp 1.4x',
    (tester) async {
      Map<String, dynamic>? selected;
      final semantics = tester.ensureSemantics();

      await pumpProductionLayoutTestApp(
        tester,
        size: const Size(320, 568),
        systemTextScale: 2,
        home: Scaffold(
          body: Builder(
            builder: (context) => FilledButton(
              onPressed: () async {
                selected = await showOrderPickerSheet(
                  context: context,
                  orders: const [_order],
                  isSeller: false,
                );
              },
              child: const Text('選擇訂單'),
            ),
          ),
        ),
      );

      await tester.tap(find.text('選擇訂單'));
      await tester.pumpAndSettle();

      final card = find.byKey(
        const ValueKey('chat-order-picker-card-$_orderNo'),
      );
      expect(card, findsOneWidget);
      _expectInsideViewport(tester, card);
      _expectPrimaryValuesComplete(tester);
      final semanticData = tester.getSemantics(card).getSemanticsData();
      expect(semanticData.label, contains(_orderNo));
      expect(semanticData.label, contains(_amount));
      expect(semanticData.label, contains(_sellerName));
      expect(tester.takeException(), isNull);

      await tester.tap(card);
      await tester.pumpAndSettle();
      expect(selected?['orderNo'], _orderNo);
      semantics.dispose();
    },
  );
}

void _expectInsideViewport(WidgetTester tester, Finder card) {
  final rect = tester.getRect(card);
  expect(rect.left, greaterThanOrEqualTo(0));
  expect(rect.right, lessThanOrEqualTo(320));
  expect(rect.width, lessThanOrEqualTo(300));
}

void _expectPrimaryValuesComplete(WidgetTester tester) {
  final orderNumber = find.text('#$_orderNo');
  expect(orderNumber, findsOneWidget);
  expect(
    tester.renderObject<RenderParagraph>(orderNumber).didExceedMaxLines,
    isFalse,
  );

  for (final value in const ['部分退款', '\$$_amount']) {
    final finder = find.text(value);
    expect(finder, findsOneWidget);
    final text = tester.widget<Text>(finder);
    expect(text.maxLines, 1);
    expect(text.softWrap, isFalse);
    expect(
      tester.renderObject<RenderParagraph>(finder).didExceedMaxLines,
      isFalse,
      reason: '$value must remain visually complete',
    );
  }
}
