import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_order_lookup.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_status_notifier.dart';
import 'package:hawkr_buyer/features/order/data/refresh_coalescer.dart';
import 'package:hawkr_buyer/features/order/presentation/order_card_widgets/after_sale_expanded_section.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  testWidgets('N 張展開的售後卡片：一批提示不讓每張卡片各自重讀', (tester) async {
    final adapter = _CountingAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://after-sale.test'))
      ..httpClientAdapter = adapter;
    addTearDown(dio.close);

    await pumpProductionLayoutTestApp(
      tester,
      size: const Size(320, 900),
      home: ProviderScope(
        overrides: [dioProvider.overrideWithValue(dio)],
        child: Scaffold(
          body: SingleChildScrollView(
            child: Column(
              children: [
                for (final no in ['CASE-1', 'CASE-2', 'CASE-3'])
                  AfterSaleExpandedSection(
                    snapshot: AfterSaleCaseSnapshot(caseNo: no, status: '0'),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump(const Duration(seconds: 1));
    final initial = adapter.detailRequests;
    expect(initial, greaterThanOrEqualTo(3));

    AfterSaleStatusNotifier.instance.requestRefresh();
    AfterSaleStatusNotifier.instance.requestRefresh();
    await tester.pump(RefreshCoalescer.defaultWindow);
    await tester.pump(const Duration(milliseconds: 100));

    expect(adapter.detailRequests, initial);
  });
}

class _CountingAdapter implements HttpClientAdapter {
  var detailRequests = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path.startsWith('/app/after_sales/CASE-')) detailRequests++;
    final no = options.path.split('/').last;
    final body = options.path.startsWith('/app/after_sales/CASE-')
        ? {
            'code': 0,
            'data': {
              'caseNo': no,
              'orderNo': 'ORDER-1',
              'status': '0',
              'reason': '7001',
              'description': '商品狀況與訂單不符',
              'createdAt': '',
              'refundableAmount': '0',
            },
          }
        : {'code': 500, 'message': 'not used'};
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
