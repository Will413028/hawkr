import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/core/layout/app_media_query_policy.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/features/dashboard/presentation/dashboard_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/dashboard_fixtures.dart';

void main() {
  test('dashboard header reserves height for accessible text sizes', () {
    expect(dashboardHeaderTextScaleExtra(TextScaler.noScaling), 0);
    expect(
      dashboardHeaderTextScaleExtra(const TextScaler.linear(1.3)),
      greaterThanOrEqualTo(20),
    );
    expect(
      dashboardHeaderTextScaleExtra(const TextScaler.linear(3)),
      lessThanOrEqualTo(32),
    );
  });

  test('dashboard grids stay useful on phone and tablet widths', () {
    expect(dashboardMetricColumnCount(320), 2);
    expect(dashboardMetricColumnCount(599), 2);
    expect(dashboardMetricColumnCount(600), 4);

    expect(dashboardQuickActionColumnCount(320), 2);
    expect(dashboardQuickActionColumnCount(390), 3);
  });

  testWidgets('complete overview fits a compact seller screen', (tester) async {
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => ProviderScope(
          child: MaterialApp(
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
                child: SizedBox(
                  width: 320,
                  child: DashboardOverviewContent(
                    dashboard: fullDashboard(),
                    onOpenOrderTab: (_) {},
                    onAddProduct: () {},
                    onOpenShipping: () {},
                    showAnomalyBanner: false,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );

    expect(find.text('今天有 7 件事待處理'), findsOneWidget);

    // 兩個期間都是最近 7 個完整的臺北日曆日（伺服器固定），標籤與百分比都明說「近 7 日／較前 7 日」。
    expect(find.text('近 7 日營收'), findsOneWidget);
    expect(find.text(r'NT$ 128,400'), findsOneWidget);
    // 五張 7 日數字卡（營收、訂單、顧客、商品瀏覽、新追蹤者）都帶同一段期間，同 website 的 buildStats。
    expect(find.text('9/30–10/6'), findsNWidgets(5));
    expect(find.text('關鍵數據與較前 7 日變化'), findsOneWidget);
    expect(find.text('累計營收'), findsNothing);
    expect(find.textContaining('週環比'), findsNothing);

    // 營收卡的 pill 與三張期間 tile 各自帶正負號；新追蹤者沒有比較值，變化欄寫「近 7 日」。
    expect(find.text('較前 7 日 +12%'), findsOneWidget); // revenue pill
    expect(find.text('較前 7 日 +3%'), findsOneWidget); // orders
    expect(find.text('較前 7 日 +6%'), findsOneWidget); // customers
    expect(find.text('較前 7 日 -3%'), findsOneWidget); // product views
    expect(find.text('近 7 日訂單'), findsOneWidget);
    expect(find.text('近 7 日顧客'), findsOneWidget);
    expect(find.text('商品瀏覽'), findsOneWidget);
    expect(find.text('新追蹤者'), findsOneWidget);
    expect(find.text('+5'), findsOneWidget);
    expect(find.text('近 7 日'), findsOneWidget);

    expect(
      tester.widget<Text>(find.text('較前 7 日 -3%')).style?.color,
      AppColors.dangerStrong,
    );
    expect(
      tester.widget<Text>(find.text('較前 7 日 +12%')).style?.color,
      AppColors.brandPrimaryStrong,
    );

    // 訂單進度四個桶：待出貨、進行中、已結束（含取消與全額退款，不是成交數）、售後。
    expect(find.text('23'), findsOneWidget);
    expect(find.text('8'), findsOneWidget);

    final dashboardContext = tester.element(
      find.byType(DashboardOverviewContent),
    );
    expect(MediaQuery.textScalerOf(dashboardContext).scale(10), 14);
    expect(find.text('營運快覽'), findsOneWidget);
    expect(find.text('訂單進度'), findsOneWidget);
    expect(find.text('常用功能'), findsOneWidget);
    expect(find.text('商品管理'), findsOneWidget);
    expect(tester.takeException(), isNull);
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    semantics.dispose();
  });

  Future<void> pumpOverview(
    WidgetTester tester, {
    Dashboard? dashboard,
    Widget? status,
    void Function(int)? onOpenOrderTab,
  }) => tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(390, 844),
      builder: (_, __) => ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(
              child: DashboardOverviewContent(
                dashboard: dashboard,
                status: status,
                onOpenOrderTab: onOpenOrderTab ?? (_) {},
                onAddProduct: () {},
                onOpenShipping: () {},
                showAnomalyBanner: false,
              ),
            ),
          ),
        ),
      ),
    ),
  );

  testWidgets('前期為 0 時沒有成長百分比，畫「—」不是 0%', (tester) async {
    await pumpOverview(
      tester,
      dashboard: dashboardFromJson(
        fullDashboardJson(previousRevenue: '0', previousViews: 0),
      ),
    );

    expect(find.text(r'NT$ 128,400'), findsOneWidget);
    expect(find.text('較前 7 日 0%'), findsNothing);
    // 營收與商品瀏覽兩處沒有基準。
    expect(find.text('—'), findsNWidgets(2));
    expect(tester.takeException(), isNull);
  });

  testWidgets('區塊沒設：該區塊每個欄位都是「—」，不是 0', (tester) async {
    final json = fullDashboardJson()
      ..remove('sales')
      ..remove('productViews')
      ..remove('followers');
    await pumpOverview(tester, dashboard: dashboardFromJson(json));

    expect(find.textContaining(r'NT$'), findsNothing);
    expect(find.text('0'), findsNothing);
    // 營收（值＋變化）、訂單、顧客、商品瀏覽、新追蹤者各自的值與變化欄：2 + 2 + 2 + 2 + 2。
    expect(find.text('—'), findsNWidgets(10));
    // 期間說明不因區塊沒設而消失（website 的 buildStats 一樣）。
    expect(find.text('9/30–10/6'), findsNWidgets(5));
    // 訂單狀態沒受影響。
    expect(find.text('今天有 7 件事待處理'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('訂單狀態沒設：今日待辦說明讀不到，按鈕與訂單進度都是「—」', (tester) async {
    final json = fullDashboardJson()..remove('orderStatusCounts');
    await pumpOverview(tester, dashboard: dashboardFromJson(json));

    expect(find.text('暫時無法取得待辦'), findsOneWidget);
    expect(find.text('下拉重新整理，或到訂單頁查看'), findsOneWidget);
    expect(find.text('今日待辦已清空'), findsNothing);
    expect(find.text('待出貨 —'), findsOneWidget);
    expect(find.text('售後 —'), findsOneWidget);
    // 訂單進度四個桶（待出貨、進行中、已結束、售後）。
    expect(find.text('—'), findsNWidgets(4));
    // 其他區塊不受影響。
    expect(find.text(r'NT$ 128,400'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('全部為 0 的訂單狀態是「今日待辦已清空」（有值的 0 不是沒設）', (tester) async {
    await pumpOverview(
      tester,
      dashboard: dashboardFromJson(
        fullDashboardJson(
          pendingShipment: 0,
          inProgress: 0,
          closed: 0,
          aftersales: 0,
        ),
      ),
    );

    expect(find.text('今日待辦已清空'), findsOneWidget);
    expect(find.text('待出貨 0'), findsOneWidget);
    expect(find.text('售後 0'), findsOneWidget);
  });

  testWidgets('沒有資料（讀取中或第一次失敗）：每一塊都是「—」，狀態區塊在最上方，常用功能照常可點', (tester) async {
    tester.view.physicalSize = const Size(800, 2400);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    var tapped = -1;
    await pumpOverview(
      tester,
      status: const Text('STATUS'),
      onOpenOrderTab: (index) => tapped = index,
    );

    expect(find.text('STATUS'), findsOneWidget);
    // 還沒有儀表板：狀況由狀態區塊說明，今日待辦不說「暫時無法取得待辦」。
    expect(find.text('暫時無法取得待辦'), findsNothing);
    expect(find.text('今日待辦'), findsOneWidget);
    expect(find.text('待出貨 —'), findsOneWidget);
    expect(find.textContaining(r'NT$'), findsNothing);
    expect(find.text('0'), findsNothing);
    expect(
      tester.getTopLeft(find.text('STATUS')).dy,
      lessThan(tester.getTopLeft(find.text('今日待辦')).dy),
    );
    // 常用功能一直可用。
    expect(find.text('商品管理'), findsOneWidget);
    expect(find.text('新增商品'), findsOneWidget);
    // 訂單進度的入口仍可點。
    await tester.tap(find.text('配送中'));
    expect(tapped, 1);
    expect(tester.takeException(), isNull);
  });
}
