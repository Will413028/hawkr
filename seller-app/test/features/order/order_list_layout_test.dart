import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_seller/core/layout/app_media_query_policy.dart';
import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/order/presentation/order_card.dart';
import 'package:hawkr_seller/features/order/presentation/order_list_screen.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final device in const [
    (size: Size(320, 568), scale: 1.4, top: 20.0),
    (size: Size(402, 874), scale: 1.0, top: 62.0),
    (size: Size(440, 956), scale: 1.4, top: 62.0),
    (size: Size(874, 402), scale: 1.4, top: 0.0),
  ]) {
    testWidgets(
      'order list remains scrollable and filters work at ${device.size}, ${device.scale}x',
      (tester) async {
        tester.view
          ..devicePixelRatio = 1
          ..physicalSize = device.size
          ..padding = FakeViewPadding(top: device.top, bottom: 34)
          ..viewPadding = FakeViewPadding(top: device.top, bottom: 34);
        addTearDown(tester.view.reset);
        final adapter = _OrdersAdapter();
        final dio = Dio(BaseOptions(baseUrl: 'https://fixture.invalid'))
          ..httpClientAdapter = adapter;
        addTearDown(() => dio.close(force: true));

        await tester.pumpWidget(
          ProviderScope(
            overrides: [dioProvider.overrideWithValue(dio)],
            child: ScreenUtilInit(
              designSize: const Size(414, 896),
              enableScaleWH: () => false,
              enableScaleText: () => false,
              builder: (_, _) => MaterialApp(
                theme: AppTheme.light,
                builder: (context, child) => MediaQuery(
                  data: AppMediaQueryPolicy.apply(
                    MediaQuery.of(context).copyWith(
                      textScaler: TextScaler.linear(device.scale),
                      disableAnimations: true,
                    ),
                  ),
                  child: child!,
                ),
                home: const OrderListScreen(),
              ),
            ),
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 300));

        expect(find.text('全部 (2)'), findsOneWidget);
        final first = tester.getRect(find.byType(OrderCard).first);
        expect(first.width, lessThanOrEqualTo(688));
        expect(first.left, greaterThanOrEqualTo(16));
        expect(first.right, lessThanOrEqualTo(device.size.width - 16));
        final filter = find.ancestor(
          of: find.text('待處理 (2)'),
          matching: find.byType(InkWell),
        );
        expect(tester.getSize(filter).height, greaterThanOrEqualTo(48));
        await tester.tap(filter);
        await tester.pump(const Duration(milliseconds: 200));
        expect(find.byKey(const ValueKey('QA-PENDING-1')), findsOneWidget);
        await tester.scrollUntilVisible(
          find.text('訂單 QA-PENDING-2'),
          180,
          scrollable: find.byType(Scrollable).first,
        );
        await tester.pump(const Duration(milliseconds: 200));
        expect(find.text('訂單 QA-PENDING-2').hitTestable(), findsOneWidget);
        expect(tester.takeException(), isNull);

        // Tabs still use the server status contract. Clear the action filter
        // before checking a delivery order that needs no seller action.
        await tester.tap(find.text('全部 (2)'));
        await tester.tap(find.byKey(const ValueKey('seller_order_tab_1')));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 300));
        expect(find.text('全部 (1)'), findsOneWidget);
        expect(find.text('售後處理 (0)'), findsOneWidget);
        expect(find.byKey(const ValueKey('QA-TRANSIT')), findsOneWidget);
        await tester.tap(find.text('售後處理 (0)'));
        await tester.pump(const Duration(milliseconds: 200));
        expect(find.text('暫無售後處理訂單'), findsOneWidget);
        await tester.tap(find.byKey(const ValueKey('seller_order_tab_2')));
        await tester.pump();
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('QA-ENDED')), findsOneWidget);
        expect(find.text('售後處理 (0)'), findsNothing);
        expect(
          adapter.statuses,
          containsAll(['10,20', '30,32,35,40,20', '50,60,70,80']),
        );
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pump();
      },
    );
  }
}

class _OrdersAdapter implements HttpClientAdapter {
  final statuses = <String>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.method != 'GET') {
      throw StateError('Fixture rejects ${options.method} ${options.path}');
    }
    final List<Object> records;
    if (options.path == '/api/v1/app/orders') {
      final status = options.queryParameters['status'].toString();
      statuses.add(status);
      records = switch (status) {
        '10,20' => [_order('QA-PENDING-1', 20), _order('QA-PENDING-2', 10)],
        '30,32,35,40,20' => [_order('QA-TRANSIT', 30)],
        '50,60,70,80' => [_order('QA-ENDED', 50)],
        _ => throw StateError('Unknown fixture status $status'),
      };
    } else if (options.path == '/api/v1/app/seller/after_sales' ||
        options.path == '/api/v1/app/shipments/anomalies') {
      records = [];
    } else {
      throw StateError('Unknown fixture GET ${options.path}');
    }
    return ResponseBody.fromString(
      jsonEncode({
        'code': 0,
        'data': {'records': records, 'total': records.length},
      }),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Map<String, Object> _order(String id, int status) => {
  'orderNo': id,
  'status': '$status',
  'createTime': '2026-09-22T08:45:00+08:00',
  'recipientName': '訂單列表測試買家',
  'shippingAddress': '臺北市南港區經貿二路1號12樓之3（測試地址）',
  'shippingMethod': '3001',
  'deliveryTemp': '3203',
  'shippingStatus': status == 30 ? '31' : '10',
  'totalActualAmount': '1860',
  'items': [
    {'productName': '產地直送・屏東金鑽鳳梨家庭分享箱', 'unitPrice': '1860', 'quantity': 1},
  ],
};
