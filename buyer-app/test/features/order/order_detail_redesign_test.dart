import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/features/order/presentation/order_detail_screen.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  const statusLabels = <String, String>{
    '0': '待付款',
    '10': '已付款',
    '20': '待出貨',
    '30': '配送中',
    '32': '待取貨',
    '35': '物流異常',
    '40': '已送達',
    '50': '已完成',
    '60': '部分退款',
    '70': '全額退款',
    '80': '已取消',
  };
  const statusIcons = <String, HkIconName>{
    '0': HkIconName.orderWaitingPayment,
    '10': HkIconName.orderPaid,
    '20': HkIconName.orderPreparing,
    '30': HkIconName.orderShipping,
    '32': HkIconName.orderPickup,
    '35': HkIconName.orderLogisticsIssue,
    '40': HkIconName.orderDelivered,
    '50': HkIconName.orderCompleted,
    '60': HkIconName.orderPartialRefund,
    '70': HkIconName.orderRefunded,
    '80': HkIconName.orderCancelled,
  };

  for (final entry in statusLabels.entries) {
    testWidgets('買家新版訂單細節顯示 ${entry.value} 狀態', (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(390, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = _OrderDetailAdapter(status: entry.key);
      addTearDown(dio.close);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [dioProvider.overrideWithValue(dio)],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, __) => const MaterialApp(
              home: OrderDetailScreen(
                orderSn: 'O125369895',
                heroTag: 'order-detail-card:O125369895',
              ),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      await tester.pump();
      // 詳情資料建立 ListView 後，付款歷程卡才會發出第二個 request。
      await tester.pump(const Duration(milliseconds: 1));

      expect(find.text('訂單細節'), findsOneWidget);
      final floatingShell = tester.widget<Scaffold>(
        find.byKey(const ValueKey('order-detail-floating-shell')),
      );
      expect(floatingShell.backgroundColor, Colors.transparent);
      final previewRect = tester.getRect(
        find.byKey(const ValueKey('order-detail-preview-window')),
      );
      expect(previewRect.left, 8);
      expect(previewRect.top, 8);
      expect(previewRect.right, 382);
      expect(previewRect.bottom, 836);
      expect(
        tester.widget<Hero>(find.byType(Hero)).tag,
        'order-detail-card:O125369895',
      );
      expect(find.text(entry.value), findsOneWidget);
      expect(
        find.byWidgetPredicate(
          (widget) => widget is HkIcon && widget.name == statusIcons[entry.key],
        ),
        findsOneWidget,
      );
      expect(find.text('商品'), findsOneWidget);
      expect(find.text('交易總覽'), findsNothing);
      expect(find.text('商品明細'), findsNothing);
      expect(find.text('目前沒有售後案件'), findsNothing);
      await tester.dragUntilVisible(
        find.text('付款'),
        find.byType(ListView).first,
        const Offset(0, -220),
      );
      // 滾動後付款歷程卡才建立，等待該次 mock Dio request 與 FutureBuilder 完成。
      await tester.pumpAndSettle(const Duration(milliseconds: 1));
      expect(find.text('收件與配送'), findsOneWidget);
      expect(find.text('付款'), findsOneWidget);
      expect(find.text('付款詳情'), findsNothing);
      expect(find.text('付款方式'), findsNothing);
      expect(find.text('物流進度'), findsNothing);
    });
  }

  testWidgets('買家新版訂單細節在 320px 小螢幕不會溢位', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 760);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
      ..httpClientAdapter = _OrderDetailAdapter(status: '30');
    addTearDown(dio.close);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [dioProvider.overrideWithValue(dio)],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) =>
              const MaterialApp(home: OrderDetailScreen(orderSn: 'O125369895')),
        ),
      ),
    );
    await tester.pumpAndSettle(const Duration(milliseconds: 100));

    expect(find.byKey(const ValueKey('order-detail-content')), findsOneWidget);
    expect(tester.takeException(), isNull);

    await tester.fling(
      find.byType(ListView).first,
      const Offset(0, -1600),
      2000,
    );
    await tester.pumpAndSettle();

    expect(find.text('付款'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}

class _OrderDetailAdapter implements HttpClientAdapter {
  _OrderDetailAdapter({required this.status});

  final String status;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final Object data;
    if (options.path == '/app/orders/O125369895') {
      data = _orderPayload(status);
    } else if (options.path == '/app/after_sales') {
      data = {
        'code': 0,
        'data': {'records': <Object>[], 'total': 0},
      };
    } else {
      data = {'code': 0, 'data': <Object>[]};
    }
    return ResponseBody.fromString(
      jsonEncode(data),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Map<String, dynamic> _orderPayload(String status) => {
  'code': 0,
  'data': {
    'id': 125369895,
    'orderNo': 'O125369895',
    'status': status,
    'statusSummary': '請查看訂單最新進度。',
    'totalOriginalAmount': '260',
    'totalActualAmount': '250',
    'createTime': '2026-07-16T06:38:00+08:00',
    'storeId': '88',
    'sellerName': '金湖水產',
    'logoUrl': '',
    'recipientName': '陳小日',
    'recipientPhone': '0912345678',
    'shippingAddress': '桃園市桃園區鎮撫街1號',
    'shippingMethod': '3001',
    'shippingStatus': '31',
    'shippingFee': '60',
    'discountAmount': '10',
    'items': [
      {
        'id': '1',
        'productName': '蟹味棒',
        'skuId': '1001',
        'quantity': '1',
        'unitPrice': '200',
        'listPrice': '210',
        'actualPayAmount': '200',
        'imageUrl': '',
      },
    ],
  },
};
