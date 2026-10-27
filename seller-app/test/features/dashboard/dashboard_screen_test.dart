import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/refresh/refresh_clock.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/dashboard/data/dashboard_chat_unread_provider.dart';
import 'package:hawkr_seller/features/dashboard/data/dashboard_notification_unread_provider.dart';
import 'package:hawkr_seller/features/dashboard/presentation/dashboard_screen.dart';
import 'package:hawkr_seller/features/order/data/order_status_notifier.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/seller_status_refresh.dart';
import 'package:hawkr_seller/features/shipment_anomaly/data/shipment_anomaly_providers.dart';
import 'package:hawkr_seller/shared/widgets/fruit_refresh_indicator.dart';
import 'package:hawkr_seller/features/shipment_anomaly/data/shipment_anomaly_model.dart';
import 'package:flutter/foundation.dart' show ValueListenable, ValueNotifier;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/dashboard_fixtures.dart';
import '../../support/measure_fixtures.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _NoStatusRefresh extends SellerStatusRefresh {
  @override
  Future<bool> refresh({
    bool fresh = false,
    bool settledOnly = false,
    Duration? maxAge,
  }) async => true;
}

class _Server {
  int calls = 0;
  Object? failure;
  Completer<void>? gate;
  var now = DateTime.utc(2026, 10, 12, 8);

  /// `ListMyMeasures` 依 `state` 篩選回的措施（只有預定生效的終止）；記下每次問的狀態。
  List<Measure> scheduled = [];
  final measureStates = <MeasureState>[];

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.getMyDashboard, (_, _) async {
          final mine = ++calls;
          await gate?.future;
          final error = failure;
          if (error != null) throw error;
          return GetMyDashboardResponse(
            dashboard: dashboardFromJson(
              fullDashboardJson(pendingShipment: mine),
            ),
          );
        })
        .unary(SellerAppService.listMyMeasures, (request, _) async {
          measureStates.add(request.state);
          return ListMyMeasuresResponse(
            measures: request.state == MeasureState.MEASURE_STATE_SCHEDULED
                ? scheduled
                : const [],
          );
        })
        .build(),
  );
}

Future<void> _pump(
  WidgetTester tester,
  _Server server, {
  ValueListenable<bool>? tickers,
  bool chatUnreadFails = false,
  String? storeId = 'store-1',
}) async {
  tester.view.physicalSize = const Size(800, 2600);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        sellerClientProvider.overrideWithValue(server.client),
        refreshClockProvider.overrideWithValue(() => server.now),
        myStoreSnapshotProvider.overrideWithValue(null),
        sellerStoreIdProvider.overrideWithValue(storeId),
        sellerStatusRefreshProvider.overrideWith(_NoStatusRefresh.new),
        dashboardChatUnreadProvider.overrideWith(
          (ref) async => chatUnreadFails ? throw StateError('chat down') : 0,
        ),
        dashboardNotificationUnreadProvider.overrideWith((ref) async => 0),
        shipmentAnomalyStatsProvider.overrideWith(
          (ref) async => const ShipmentAnomalyStats(),
        ),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => MaterialApp(
          home: tickers == null
              ? const DashboardScreen()
              : ValueListenableBuilder<bool>(
                  valueListenable: tickers,
                  builder: (_, on, _) =>
                      TickerMode(enabled: on, child: const DashboardScreen()),
                ),
        ),
      ),
    ),
  );
}

Future<void> _settle(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));
}

void main() {
  testWidgets('首頁終止預告提示卡：有預定生效的終止時顯示伺服器給的生效時間，沒有時不顯示', (tester) async {
    final server = _Server()
      ..scheduled = [terminationOf(start: DateTime.utc(2026, 11, 20, 4, 30))];
    await _pump(tester, server);
    await _settle(tester);

    expect(
      server.measureStates,
      contains(MeasureState.MEASURE_STATE_SCHEDULED),
    );
    expect(
      find.text('平台已通知將於 2026/11/20 12:30 終止合作。在此之前店家照常營業；如有異議，可提出申覆。'),
      findsOneWidget,
    );

    final none = _Server();
    await _pump(tester, none);
    await _settle(tester);
    expect(find.textContaining('終止合作'), findsNothing);
  });

  testWidgets('下拉重新整理：預告卡重讀預定生效的措施，伺服器已撤銷的預告消失', (tester) async {
    final server = _Server()
      ..scheduled = [terminationOf(start: DateTime.utc(2026, 11, 20, 4, 30))];
    await _pump(tester, server);
    await _settle(tester);
    const notice = '平台已通知將於 2026/11/20 12:30 終止合作。在此之前店家照常營業；如有異議，可提出申覆。';
    expect(find.text(notice), findsOneWidget);
    int scheduledReads() => server.measureStates
        .where((s) => s == MeasureState.MEASURE_STATE_SCHEDULED)
        .length;
    final readsBefore = scheduledReads();

    // 下拉手勢在有預告卡時內容比視窗高，測試環境的捲動物理不會產生負的 pixels；直接呼叫下拉元件的 onRefresh，
    // 與手勢放手後呼叫的是同一個函式。
    server.scheduled = [];
    await tester
        .widget<FruitRefreshIndicator>(find.byType(FruitRefreshIndicator))
        .onRefresh();
    await _settle(tester);

    expect(scheduledReads(), readsBefore + 1);
    expect(find.text(notice), findsNothing);
  });

  testWidgets('讀取中：資料區顯示讀取中與「—」，常用功能照常可用', (tester) async {
    final server = _Server()..gate = Completer<void>();
    await _pump(tester, server);
    await tester.pump();

    expect(find.text('正在整理今日營運資料'), findsOneWidget);
    expect(find.text('暫時無法取得待辦'), findsNothing);
    expect(find.text('今日待辦'), findsOneWidget);
    expect(find.text('待出貨 —'), findsOneWidget);
    expect(find.text('商品管理'), findsOneWidget);
    expect(server.calls, 1);

    server.gate!.complete();
    await _settle(tester);
    expect(find.text('正在整理今日營運資料'), findsNothing);
    expect(find.text('待出貨 1'), findsOneWidget);
  });

  testWidgets('店家還沒讀到（沒有 storeId）：和第一次讀取一樣顯示讀取中，不讀儀表板；下拉沒有東西可讀，不提示失敗', (
    tester,
  ) async {
    final server = _Server();
    await _pump(tester, server, storeId: null);
    await _settle(tester);

    expect(find.text('正在整理今日營運資料'), findsOneWidget);
    expect(server.calls, 0);

    await tester.fling(
      find.byType(SingleChildScrollView).first,
      const Offset(0, 400),
      1000,
    );
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
    await tester.pump(const Duration(milliseconds: 500));

    expect(server.calls, 0);
    expect(find.text(dashboardRefreshFailedNotice), findsNothing);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('第一次讀取失敗：頂端錯誤區塊加重試，每塊「—」，常用功能照常可用；重試成功後恢復', (tester) async {
    final server = _Server()..failure = ConnectException(Code.unavailable, 'x');
    await _pump(tester, server);
    await _settle(tester);

    expect(find.text('暫時無法載入總覽'), findsOneWidget);
    expect(find.text('請檢查網路後再試一次'), findsOneWidget);
    expect(find.text('暫時無法取得待辦'), findsNothing);
    expect(find.text('今日待辦'), findsOneWidget);
    expect(find.text('待出貨 —'), findsOneWidget);
    expect(find.text('新增商品'), findsOneWidget);
    expect(server.calls, 1);

    server.failure = null;
    await tester.tap(find.text('重新載入'));
    await _settle(tester);

    expect(find.text('暫時無法載入總覽'), findsNothing);
    expect(find.text('待出貨 2'), findsOneWidget);
    expect(server.calls, 2);
  });

  testWidgets('回前景：重讀一次（不看 30 秒）', (tester) async {
    final server = _Server();
    await _pump(tester, server);
    await _settle(tester);
    expect(server.calls, 1);

    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await _settle(tester);
    expect(server.calls, 2);
    expect(find.text('待出貨 2'), findsOneWidget);
  });

  testWidgets('從看不到變成看得到：30 秒內讀過就跳過，超過 30 秒才重讀', (tester) async {
    final server = _Server();
    final tickers = ValueNotifier(true);
    addTearDown(tickers.dispose);
    await _pump(tester, server, tickers: tickers);
    await _settle(tester);
    expect(server.calls, 1);

    tickers.value = false;
    await tester.pump();
    server.now = server.now.add(const Duration(seconds: 29));
    tickers.value = true;
    await _settle(tester);
    expect(server.calls, 1);

    tickers.value = false;
    await tester.pump();
    server.now = server.now.add(const Duration(seconds: 2));
    tickers.value = true;
    await _settle(tester);
    expect(server.calls, 2);
    expect(find.text('待出貨 2'), findsOneWidget);
  });

  testWidgets('收到訂單推播：重讀一次', (tester) async {
    final server = _Server();
    await _pump(tester, server);
    await _settle(tester);
    expect(server.calls, 1);

    OrderStatusNotifier.instance.handlePushNotification({
      'eventType': 'ORDER_PAID',
      'orderSn': 'HK1',
    });
    await _settle(tester);

    expect(server.calls, 2);
    expect(find.text('待出貨 2'), findsOneWidget);
  });

  testWidgets('背景重讀失敗：繼續顯示上一次的值，沒有錯誤區塊也沒有提示', (tester) async {
    final server = _Server();
    await _pump(tester, server);
    await _settle(tester);
    expect(find.text('待出貨 1'), findsOneWidget);

    server.failure = ConnectException(Code.unavailable, 'down');
    OrderStatusNotifier.instance.handlePushNotification({
      'eventType': 'ORDER_PAID',
    });
    await _settle(tester);

    expect(server.calls, 2);
    expect(find.text('待出貨 1'), findsOneWidget);
    expect(find.text('暫時無法載入總覽'), findsNothing);
    expect(find.text(dashboardRefreshFailedNotice), findsNothing);
  });

  testWidgets('下拉重新整理失敗：保留上一次的值並以 toast 提示', (tester) async {
    final server = _Server();
    await _pump(tester, server);
    await _settle(tester);
    server.failure = ConnectException(Code.unavailable, 'down');

    await tester.fling(
      find.byType(SingleChildScrollView).first,
      const Offset(0, 400),
      1000,
    );
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
    await tester.pump(const Duration(milliseconds: 500));

    expect(server.calls, 2);
    expect(find.text('待出貨 1'), findsOneWidget);
    expect(find.text(dashboardRefreshFailedNotice), findsOneWidget);
    expect(find.text('暫時無法載入總覽'), findsNothing);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('下拉：其他資料與儀表板都讀不到，照樣提示、不丟例外', (tester) async {
    final server = _Server();
    await _pump(tester, server, chatUnreadFails: true);
    await _settle(tester);
    server.failure = ConnectException(Code.unavailable, 'down');

    await tester.fling(
      find.byType(SingleChildScrollView).first,
      const Offset(0, 400),
      1000,
    );
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
    await tester.pump(const Duration(milliseconds: 500));

    expect(tester.takeException(), isNull);
    expect(find.text(dashboardRefreshFailedNotice), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('隱藏時收到訂單推播：不立刻讀，變成看得到時讀最新的（不套用 30 秒）', (tester) async {
    final server = _Server();
    final tickers = ValueNotifier(true);
    addTearDown(tickers.dispose);
    await _pump(tester, server, tickers: tickers);
    await _settle(tester);
    expect(server.calls, 1);

    tickers.value = false;
    await tester.pump();
    OrderStatusNotifier.instance.handlePushNotification({
      'eventType': 'ORDER_PAID',
      'orderSn': 'HK1',
    });
    await _settle(tester);
    expect(server.calls, 1);

    // 30 秒內：沒有推播時會跳過，有推播就一定讀。
    server.now = server.now.add(const Duration(seconds: 5));
    tickers.value = true;
    await _settle(tester);
    expect(server.calls, 2);
    expect(find.text('待出貨 2'), findsOneWidget);

    // 旗標只用一次：再隱藏、30 秒內再顯示就照舊跳過。
    tickers.value = false;
    await tester.pump();
    server.now = server.now.add(const Duration(seconds: 5));
    tickers.value = true;
    await _settle(tester);
    expect(server.calls, 2);
  });
}
