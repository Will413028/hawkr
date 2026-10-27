import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_order_lookup.dart';
import 'package:hawkr_buyer/features/order/presentation/after_widgets/after_sale_detail_screen.dart';
import 'package:hawkr_buyer/features/order/presentation/after_widgets/after_sale_view_screen.dart';
import 'package:hawkr_buyer/features/order/presentation/after_widgets/arbitration_view_screen.dart';
import 'package:hawkr_buyer/features/order/presentation/order_card_widgets/after_sale_expanded_section.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/production_layout_test_harness.dart';

const _sourceButtonCounts = <String, int>{
  'lib/features/order/presentation/after_widgets/after_sale_detail_screen.dart':
      5,
  'lib/features/order/presentation/after_widgets/after_sale_view_screen.dart':
      2,
  'lib/features/order/presentation/after_widgets/arbitration_view_screen.dart':
      1,
  'lib/features/order/presentation/order_card_widgets/after_sale_expanded_section.dart':
      3,
};

void main() {
  test(
    'production after-sale Material buttons use the shared label and 48dp',
    () {
      final materialButtonPattern = RegExp(
        r'\b(?:ElevatedButton|FilledButton|OutlinedButton|TextButton)\s*\(',
      );
      final sharedLabelPattern = RegExp(r'\bHkButtonLabel\s*\(');
      final explicitTapTargetPattern = RegExp(
        r'(?:height:\s*HkControl\.minTap|'
        r'minimumSize:\s*const Size\(0,\s*HkControl\.minTap\))',
      );

      for (final entry in _sourceButtonCounts.entries) {
        final source = File(entry.key).readAsStringSync();
        final materialButtons = materialButtonPattern.allMatches(source).length;
        expect(materialButtons, entry.value, reason: entry.key);
        expect(
          sharedLabelPattern.allMatches(source).length,
          materialButtons,
          reason: '${entry.key} has a Material button without HkButtonLabel',
        );
        expect(
          explicitTapTargetPattern.allMatches(source).length,
          materialButtons,
          reason:
              '${entry.key} has a Material button below an explicit 48dp target',
        );
      }
    },
  );

  testWidgets('after-sale detail action stays single-line at 320px / 1.4x', (
    tester,
  ) async {
    await _pumpAfterSaleSurface(
      tester,
      child: const AfterSaleDetailScreen(caseNo: 'CASE-1'),
    );

    await _expectAccessibleButton(tester, '取消售後', OutlinedButton);
  });

  testWidgets('after-sale view action stays single-line at 320px / 1.4x', (
    tester,
  ) async {
    await _pumpAfterSaleSurface(
      tester,
      child: const AfterSaleViewScreen(orderSn: 'ORDER-1'),
    );

    await _expectAccessibleButton(tester, '取消售後', ElevatedButton);
  });

  testWidgets('arbitration retry stays single-line at 320px / 1.4x', (
    tester,
  ) async {
    await _pumpAfterSaleSurface(
      tester,
      adapter: _AfterSaleAdapter(arbitrationFails: true),
      child: const ArbitrationViewScreen(caseNo: 'CASE-1'),
    );

    await _expectAccessibleButton(tester, '重試', OutlinedButton);
  });

  testWidgets('expanded after-sale action stays single-line at 320px / 1.4x', (
    tester,
  ) async {
    await _pumpAfterSaleSurface(
      tester,
      child: const Scaffold(
        body: SingleChildScrollView(
          child: AfterSaleExpandedSection(
            snapshot: AfterSaleCaseSnapshot(caseNo: 'CASE-1', status: '0'),
          ),
        ),
      ),
    );

    await _expectAccessibleButton(tester, '取消售後', OutlinedButton);
  });
}

Future<void> _pumpAfterSaleSurface(
  WidgetTester tester, {
  required Widget child,
  _AfterSaleAdapter? adapter,
}) async {
  final dio = Dio(BaseOptions(baseUrl: 'https://after-sale.test'))
    ..httpClientAdapter = adapter ?? _AfterSaleAdapter();
  addTearDown(dio.close);

  await pumpProductionLayoutTestApp(
    tester,
    size: const Size(320, 900),
    home: ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: child,
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump();

  final context = tester.element(find.byType(Scaffold).first);
  expect(MediaQuery.textScalerOf(context).scale(10), 14);
}

Future<void> _expectAccessibleButton(
  WidgetTester tester,
  String label,
  Type buttonType,
) async {
  final labelFinder = find.text(label);
  expect(labelFinder, findsOneWidget);
  await tester.ensureVisible(labelFinder);
  await tester.pump();

  final buttonFinder = find.ancestor(
    of: labelFinder,
    matching: find.byType(buttonType),
  );
  expect(buttonFinder, findsOneWidget);
  expect(tester.getSize(buttonFinder).height, greaterThanOrEqualTo(48));
  final text = tester.widget<Text>(labelFinder);
  expect(text.maxLines, 1);
  expect(text.softWrap, isFalse);
  final semantics = tester.ensureSemantics();
  try {
    await tester.pump();
    expect(find.bySemanticsLabel(label), findsOneWidget);
  } finally {
    semantics.dispose();
  }
  expect(tester.takeException(), isNull);
}

class _AfterSaleAdapter implements HttpClientAdapter {
  _AfterSaleAdapter({this.arbitrationFails = false});

  final bool arbitrationFails;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path == '/app/after_sales/CASE-1/arbitration' &&
        arbitrationFails) {
      return ResponseBody.fromString(
        jsonEncode({'code': 500, 'message': '暫時無法載入'}),
        500,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }

    final Object body = switch (options.path) {
      '/app/after_sales/CASE-1' => {'code': 0, 'data': _afterSaleCase},
      '/app/after_sales' => {
        'code': 0,
        'data': {
          'records': [_afterSaleCase],
          'total': 1,
        },
      },
      _ => {'code': 500, 'message': 'not used'},
    };
    final statusCode = options.path.startsWith('/app/orders/') ? 500 : 200;
    return ResponseBody.fromString(
      jsonEncode(body),
      statusCode,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

const _afterSaleCase = <String, Object>{
  'caseNo': 'CASE-1',
  'orderNo': 'ORDER-1',
  'status': '0',
  'reason': '7001',
  'description': '商品狀況與訂單不符',
  'createdAt': '',
  'refundableAmount': '0',
};
