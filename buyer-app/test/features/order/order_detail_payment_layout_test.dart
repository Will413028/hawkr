import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/features/order/presentation/order_detail_screen.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/production_layout_test_harness.dart';

const _longAmount = '123456789012';

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  final deviceSizes = <Size>[
    const Size(320, 568),
    const Size(390, 844),
    const Size(768, 1024),
  ];
  const textCases = [
    (name: 'App Large 1.15x', systemScale: 1.0, textSize: AppTextSize.large),
    (name: 'maximum 1.4x', systemScale: 2.0, textSize: AppTextSize.large),
  ];

  for (final textCase in textCases) {
    for (final size in deviceSizes) {
      testWidgets(
        'payment total fits ${size.width}x${size.height} at ${textCase.name}',
        (tester) async {
          final dio = Dio(BaseOptions(baseUrl: 'https://order-layout.test'))
            ..httpClientAdapter = _PaymentOrderAdapter();
          addTearDown(dio.close);

          await pumpProductionLayoutTestApp(
            tester,
            size: size,
            systemTextScale: textCase.systemScale,
            textSize: textCase.textSize,
            viewPadding: const EdgeInsets.only(top: 24, bottom: 16),
            home: ProviderScope(
              overrides: [dioProvider.overrideWithValue(dio)],
              child: const OrderDetailScreen(orderSn: 'ORDER-LAYOUT-1'),
            ),
          );
          await tester.pump(const Duration(milliseconds: 100));
          await tester.pump();

          await tester.dragUntilVisible(
            find.byKey(const ValueKey('order-detail-payment-total')),
            find.byType(ListView).first,
            const Offset(0, -220),
          );
          await tester.pump(const Duration(milliseconds: 1));

          _expectNoFlutterErrors(tester);
          final amount = find.byKey(
            const ValueKey('order-detail-payment-amount'),
          );
          expect(tester.widget<Text>(amount).data, _longAmount);
          final paragraph = tester.renderObject<RenderParagraph>(amount);
          expect(paragraph.didExceedMaxLines, isFalse);
          final method = find.text('ATM 轉帳');
          expect(
            tester.renderObject<RenderParagraph>(method).didExceedMaxLines,
            isFalse,
          );
          final totalLine = find.byKey(
            const ValueKey('order-detail-payment-total'),
          );
          final totalRect = tester.getRect(totalLine);
          expect(totalRect.left, greaterThanOrEqualTo(0));
          expect(totalRect.right, lessThanOrEqualTo(size.width));
          final inline = find.byKey(
            const ValueKey('order-detail-payment-total-inline'),
          );
          final stacked = find.byKey(
            const ValueKey('order-detail-payment-total-stacked'),
          );
          expect(inline.evaluate().length + stacked.evaluate().length, 1);
          if (size.width == 320) expect(stacked, findsOneWidget);
          expect(
            find.bySemanticsLabel('付款金額 NT\$ $_longAmount'),
            findsOneWidget,
          );
        },
      );
    }
  }
}

class _PaymentOrderAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final Object body;
    if (options.path == '/app/orders/ORDER-LAYOUT-1') {
      body = {
        'code': 0,
        'data': {
          'id': 1,
          'orderNo': 'ORDER-LAYOUT-1',
          'status': '10',
          'totalOriginalAmount': _longAmount,
          'totalActualAmount': _longAmount,
          'shippingFee': '0',
          'discountAmount': '0',
          'paymentMethod': '2004',
          'createTime': '2026-09-09T00:00:00+08:00',
          'items': <Object>[],
        },
      };
    } else if (options.path == '/app/after_sales') {
      body = {
        'code': 0,
        'data': {'records': <Object>[], 'total': 0},
      };
    } else {
      body = {'code': 0, 'data': <Object>[]};
    }
    return ResponseBody.fromString(
      jsonEncode(body),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void _expectNoFlutterErrors(WidgetTester tester) {
  final errors = <Object>[];
  Object? error;
  while ((error = tester.takeException()) != null) {
    errors.add(error!);
  }
  expect(
    errors,
    isEmpty,
    reason: errors
        .map(
          (error) => error is FlutterError
              ? error.diagnostics.map((node) => node.toString()).join('\n')
              : error.toString(),
        )
        .join('\n'),
  );
}
