import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_seller/core/layout/app_media_query_policy.dart';
import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:hawkr_seller/features/order/presentation/order_card.dart';
import 'package:hawkr_seller/features/order/presentation/order_card_widgets/pill_button.dart';
import 'package:hawkr_seller/shared/widgets/hk_tap_target.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('賣家列表卡片與詳情共用狀態、商品與配送風格', (tester) async {
    final semantics = tester.ensureSemantics();
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 760);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
      ..httpClientAdapter = _EmptyApiAdapter();
    addTearDown(dio.close);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [dioProvider.overrideWithValue(dio)],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => MaterialApp(
            theme: AppTheme.light,
            builder: (context, child) => MediaQuery(
              data: AppMediaQueryPolicy.apply(
                MediaQuery.of(
                  context,
                ).copyWith(textScaler: const TextScaler.linear(2)),
              ),
              child: child!,
            ),
            home: Scaffold(
              body: SingleChildScrollView(
                padding: const EdgeInsets.all(12),
                child: OrderCard(
                  order: _order,
                  isPending: false,
                  isLoading: false,
                  onSetLoading: (_, __) {},
                  onReload: () {},
                  onReloadAndSwitchTab: (_) {},
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 20));

    expect(find.text('配送中'), findsWidgets);
    final orderCardContext = tester.element(find.byType(OrderCard));
    expect(MediaQuery.textScalerOf(orderCardContext).scale(10), 14);
    expect(find.text('蟹味棒'), findsOneWidget);
    expect(find.text('共 1 件'), findsOneWidget);
    expect(find.text('黑貓宅急便 · 配送中'), findsOneWidget);
    expect(find.text('訂單 O125369895'), findsOneWidget);
    expect(find.text('細節'), findsOneWidget);
    expect(find.bySemanticsLabel('細節'), findsOneWidget);
    expect(find.byTooltip('複製訂單編號'), findsOneWidget);

    expect(find.bySemanticsLabel('展開訂單 O125369895'), findsOneWidget);
    expect(
      tester.getSemantics(find.bySemanticsLabel('展開訂單 O125369895')),
      isSemantics(
        label: '展開訂單 O125369895',
        isButton: true,
        hasExpandedState: true,
        isExpanded: false,
        hasTapAction: true,
      ),
    );

    // 內層操作要贏得手勢競爭，不能連帶觸發卡片展開。
    await tester.tap(find.byTooltip('複製訂單編號'));
    await tester.pump();
    expect(find.bySemanticsLabel('展開訂單 O125369895'), findsOneWidget);
    expect(
      tester.getSemantics(find.bySemanticsLabel('展開訂單 O125369895')),
      isSemantics(
        label: '展開訂單 O125369895',
        isButton: true,
        hasExpandedState: true,
        isExpanded: false,
        hasTapAction: true,
      ),
    );
    await tester.pump(const Duration(seconds: 2));

    await tester.tap(find.text('配送中').first);
    await tester.pump(const Duration(milliseconds: 300));
    expect(find.bySemanticsLabel('收合訂單 O125369895'), findsOneWidget);
    expect(
      tester.getSemantics(find.bySemanticsLabel('收合訂單 O125369895')),
      isSemantics(
        label: '收合訂單 O125369895',
        isButton: true,
        hasExpandedState: true,
        isExpanded: true,
        hasTapAction: true,
      ),
    );
    expect(find.bySemanticsLabel('細節'), findsOneWidget);
    expect(find.byTooltip('複製訂單編號'), findsOneWidget);

    await tester.tap(find.text('配送中').first);
    await tester.pump(const Duration(milliseconds: 300));
    expect(find.bySemanticsLabel('展開訂單 O125369895'), findsOneWidget);
    expect(tester.takeException(), isNull);
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    semantics.dispose();
  });

  testWidgets('訂單膠囊保留 ripple 且一次實體點擊只觸發一次', (tester) async {
    var tapCount = 0;
    final semantics = tester.ensureSemantics();
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp(
          theme: AppTheme.light,
          home: Scaffold(
            body: Center(
              child: PillButton(
                label: '細節',
                color: Colors.green,
                onTap: () => tapCount++,
              ),
            ),
          ),
        ),
      ),
    );

    expect(find.byType(HkTapTarget), findsOneWidget);
    expect(find.byType(InkWell), findsOneWidget);
    expect(find.bySemanticsLabel('細節'), findsOneWidget);
    final targetSize = tester.getSize(find.byType(PillButton));
    expect(targetSize.width, greaterThanOrEqualTo(48));
    expect(targetSize.height, greaterThanOrEqualTo(48));

    await tester.tap(find.byType(PillButton));
    await tester.pump();
    expect(tapCount, 1);
    semantics.dispose();
  });
}

const _order = OrderSummary(
  id: 'O125369895',
  orderSn: 'O125369895',
  statusCode: 30,
  title: '訂單',
  subtitle: '30',
  dateTime: '2026-07-16T06:38:00+08:00',
  productName: '蟹味棒',
  price: '250',
  totalAmount: 250,
  buyerName: '陳小日',
  recipientName: '陳小日',
  shippingAddress: '桃園市桃園區鎮撫街1號',
  shippingMethod: '3001',
  shipmentStatus: '31',
  shippingStatus: '31',
  items: [
    OrderItem(
      name: '蟹味棒',
      price: '125',
      unitPrice: '125',
      actualPayAmount: '250',
      quantity: 2,
    ),
  ],
);

class _EmptyApiAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString(
      jsonEncode({
        'code': 0,
        'data': {'records': <Object>[], 'total': 0},
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
