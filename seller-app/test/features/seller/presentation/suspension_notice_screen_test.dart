import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/measures.dart';
import 'package:hawkr_seller/features/seller/presentation/seller_appeal_screen.dart'
    show MeasureCard;
import 'package:hawkr_seller/features/seller/presentation/suspension_notice_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/measure_fixtures.dart';
import '../../../support/my_store_fixtures.dart';

class _RecordingSession extends Session {
  int logouts = 0;

  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);

  @override
  Future<void> logout() async => logouts++;
}

Future<GoRouter> pumpWithRoutes(
  WidgetTester tester,
  MyStore store, {
  required AsyncValue<List<Measure>> measures,
}) async {
  final router = GoRouter(
    routes: [
      GoRoute(path: '/', builder: (_, _) => const SuspensionNoticeScreen()),
      GoRoute(
        path: SellerRoutes.measures,
        builder: (_, _) => const Scaffold(body: Text('MEASURES')),
      ),
      GoRoute(
        path: SellerRoutes.measureRequest,
        builder: (_, state) =>
            Scaffold(body: Text('FORM:${state.pathParameters['measureId']}')),
      ),
      GoRoute(
        path: SettingsRoutes.help,
        builder: (_, _) => const Scaffold(body: Text('HELP')),
      ),
    ],
  );
  addTearDown(router.dispose);
  tester.view.physicalSize = const Size(800, 2400);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        myStoreSnapshotProvider.overrideWithValue(store),
        currentMyMeasuresProvider(
          MeasureState.MEASURE_STATE_ACTIVE,
        ).overrideWithValue(measures),
      ],
      child: ScreenUtilInit(
        designSize: const Size(393, 852),
        builder: (_, _) => MaterialApp.router(routerConfig: router),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return router;
}

/// 公告頁的文案與入口由 `GetMyStore` 的狀態與生效中的措施（`ListMyMeasures` 的 `state=ACTIVE`）推導；語意與文案照 website 的
/// `SellerSuspendedNotice`。
void main() {
  final activeKey = MeasureState.MEASURE_STATE_ACTIVE;

  Future<void> pumpScreen(
    WidgetTester tester,
    MyStore? store, {
    AsyncValue<List<Measure>>? measures,
  }) async {
    tester.view.physicalSize = const Size(800, 2400);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          myStoreSnapshotProvider.overrideWithValue(store),
          currentMyMeasuresProvider(
            activeKey,
          ).overrideWithValue(measures ?? const AsyncData(<Measure>[])),
        ],
        child: ScreenUtilInit(
          designSize: const Size(393, 852),
          builder: (_, __) => const MaterialApp(home: SuspensionNoticeScreen()),
        ),
      ),
    );
    await tester.pump();
    expect(tester.takeException(), isNull);
  }

  AsyncValue<List<Measure>> active(List<Measure> measures) =>
      AsyncData(measures);

  group('停權原因：生效中的措施', () {
    testWidgets('每一項生效中的措施一張卡：依據、說明、期間；標題與停權原因標題在', (tester) async {
      await pumpScreen(
        tester,
        suspendedStore(),
        measures: active([activeSuspension(id: 'a'), riskSuspension(id: 'b')]),
      );

      expect(find.text('商店已暫停營業'), findsOneWidget);
      expect(find.text('停權原因'), findsOneWidget);
      expect(find.byType(MeasureCard), findsNWidgets(2));
      expect(find.text('食品安全'), findsOneWidget);
      expect(find.text('商品標示與實品不符'), findsNWidgets(2));
      expect(find.text('停權中（至 2026/10/19 16:30）'), findsOneWidget);
      expect(find.text('風險：交易風險'), findsOneWidget);
      expect(find.text('停權中（未定結束時間，平台定期複審）'), findsOneWidget);
      expect(find.textContaining('商品已從市集下架'), findsOneWidget);
      expect(find.text('停權期間仍可使用'), findsOneWidget);
      expect(find.text('帳號已被平台停權'), findsNothing);
    });

    testWidgets('只有 can_appeal 為真的措施可按「提出申覆」，申覆開該措施的表單', (tester) async {
      final router = await pumpWithRoutes(
        tester,
        suspendedStore(),
        measures: active([
          activeSuspension(id: 'a', canAppeal: false),
          activeSuspension(id: 'b'),
        ]),
      );

      final enabled = find.byWidgetPredicate(
        (w) =>
            w is OutlinedButton &&
            w.key == const ValueKey('measure-appeal-button') &&
            w.onPressed != null,
      );
      expect(find.widgetWithText(OutlinedButton, '提出申覆'), findsNWidgets(2));
      expect(enabled, findsOneWidget);
      await tester.tap(enabled);
      await tester.pumpAndSettle();

      expect(router.state.uri.toString(), SellerRoutes.measureRequestFor('b'));
      expect(find.text('FORM:b'), findsOneWidget);
    });

    testWidgets('措施還在讀取：停權原因顯示載入中，仍有「措施與申覆」入口', (tester) async {
      await pumpScreen(
        tester,
        suspendedStore(),
        measures: const AsyncLoading<List<Measure>>(),
      );

      expect(find.text('載入中…'), findsOneWidget);
      // 還不知道是停權還是終止合作：不出現停權的字樣（終止合作的賣家不會先看到「停權」）。
      // 標題與失敗狀態同為「商店已暫停營業」，狀態列標題「帳號狀態」。
      expect(find.text('商店已暫停營業'), findsOneWidget);
      expect(find.text('帳號狀態'), findsOneWidget);
      expect(find.text('目前仍可使用'), findsOneWidget);
      expect(find.textContaining('停權'), findsNothing);
      expect(find.byType(MeasureCard), findsNothing);
      expect(find.widgetWithText(OutlinedButton, '措施與申覆'), findsOneWidget);
    });

    testWidgets('措施讀取失敗：不知道種類，只用中性文案（標題「商店已暫停營業」、沒有停權字樣與副標），仍有入口', (
      tester,
    ) async {
      await pumpScreen(
        tester,
        suspendedStore(),
        measures: AsyncError<List<Measure>>(
          StateError('down'),
          StackTrace.empty,
        ),
      );

      expect(find.text('商店已暫停營業'), findsNWidgets(2)); // 標題與狀態列
      expect(find.text('帳號狀態'), findsOneWidget);
      expect(find.text('目前仍可使用'), findsOneWidget);
      expect(find.textContaining('停權'), findsNothing);
      expect(find.textContaining('終止'), findsNothing);
      expect(find.byType(MeasureCard), findsNothing);
      expect(find.widgetWithText(OutlinedButton, '措施與申覆'), findsOneWidget);
    });

    testWidgets('讀得到、但沒有生效中的措施：通用的停權說明，仍有入口', (tester) async {
      await pumpScreen(tester, suspendedStore(), measures: active(const []));

      expect(find.text('帳號已被平台停權'), findsOneWidget);
      expect(find.byType(MeasureCard), findsNothing);
      expect(find.widgetWithText(OutlinedButton, '措施與申覆'), findsOneWidget);
    });

    testWidgets('沒有登入帳號（措施列表為 null）：通用說明', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            myStoreSnapshotProvider.overrideWithValue(suspendedStore()),
            currentMyMeasuresProvider(activeKey).overrideWithValue(null),
          ],
          child: ScreenUtilInit(
            designSize: const Size(393, 852),
            builder: (_, __) =>
                const MaterialApp(home: SuspensionNoticeScreen()),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('帳號已被平台停權'), findsOneWidget);
    });
  });

  group('疑義入口只有「措施與申覆」', () {
    void expectNoLegacyCopy() {
      expect(find.textContaining('停權僅能由平台管理員解除'), findsNothing);
      expect(find.textContaining('只能由管理員解除'), findsNothing);
      expect(find.textContaining('客服會協助你與平台治理團隊'), findsNothing);
      expect(find.textContaining('如認為此次停權有誤'), findsNothing);
      expect(find.text('前往幫助中心'), findsNothing);
      expect(find.text('請到幫助中心聯絡客服，說明你的疑問。'), findsNothing);
      expect(find.text('查看違規紀錄／提出申覆'), findsNothing);
    }

    testWidgets('依據風險的停權（過去的管理員停權）：到措施與申覆，沒有幫助中心', (tester) async {
      await pumpScreen(
        tester,
        suspendedStore(),
        measures: active([riskSuspension()]),
      );

      expect(find.text('對停權決定有疑義？'), findsOneWidget);
      expect(find.text('到措施與申覆查看措施內容與期間，並可對仍可申覆的措施提出申覆。'), findsOneWidget);
      expect(find.widgetWithText(OutlinedButton, '措施與申覆'), findsOneWidget);
      expectNoLegacyCopy();
      // 不承諾回覆時間（「盡快」「天內」「小時內」「回覆」）。
      for (final promise in ['盡快', '天內', '小時內', '回覆']) {
        expect(find.textContaining(promise), findsNothing, reason: promise);
      }
    });

    testWidgets('依據認定的停權：同一個入口', (tester) async {
      await pumpScreen(
        tester,
        suspendedStore(),
        measures: active([activeSuspension()]),
      );

      expect(find.widgetWithText(OutlinedButton, '措施與申覆'), findsOneWidget);
      expectNoLegacyCopy();
    });

    testWidgets('入口是 button semantics', (tester) async {
      final semantics = tester.ensureSemantics();
      await pumpScreen(
        tester,
        suspendedStore(),
        measures: active([activeSuspension()]),
      );
      final node = tester.getSemantics(
        find.widgetWithText(OutlinedButton, '措施與申覆'),
      );
      expect(node.flagsCollection.isButton, isTrue);
      semantics.dispose();
    });

    testWidgets('按鈕開措施與申覆列表，不經過幫助中心', (tester) async {
      await pumpWithRoutes(
        tester,
        suspendedStore(),
        measures: active([riskSuspension()]),
      );

      await tester.tap(find.widgetWithText(OutlinedButton, '措施與申覆'));
      await tester.pumpAndSettle();

      expect(find.text('MEASURES'), findsOneWidget);
      expect(find.text('HELP'), findsNothing);
    });
  });

  group('仍可使用的入口由能力推導', () {
    testWidgets('停權：商店設定、訂單管理、通知', (tester) async {
      await pumpScreen(tester, suspendedStore());

      expect(find.text('商店設定'), findsOneWidget);
      expect(find.text('檢視商店資料；需要時可以在這裡關閉商店。'), findsOneWidget);
      expect(find.text('訂單管理'), findsOneWidget);
      expect(find.text('通知'), findsOneWidget);
      expect(find.text('前往訂單'), findsNothing);
      expect(find.text('發票與稅務相關事項請至網站處理'), findsOneWidget);
    });

    testWidgets('沒有 ACCOUNT_CLOSE 就沒有商店設定入口', (tester) async {
      await pumpScreen(
        tester,
        myStore(
          suspended: true,
          capabilities: [
            SellerCapability.SELLER_CAPABILITY_ORDER_FULFILL,
            SellerCapability.SELLER_CAPABILITY_NOTIFICATION,
          ],
        ),
      );

      expect(find.text('商店設定'), findsNothing);
      expect(find.text('訂單管理'), findsOneWidget);
      expect(find.text('通知'), findsOneWidget);
    });

    testWidgets('沒有 ORDER_FULFILL 就沒有訂單入口', (tester) async {
      await pumpScreen(
        tester,
        myStore(
          suspended: true,
          capabilities: [SellerCapability.SELLER_CAPABILITY_NOTIFICATION],
        ),
      );

      expect(find.text('訂單管理'), findsNothing);
      expect(find.text('通知'), findsOneWidget);
    });
  });

  group('等待刪除（E2）', () {
    testWidgets('標題、狀態與提示文字；沒有停權說明與申覆', (tester) async {
      await pumpScreen(tester, pendingDeletionStore());

      expect(find.text('帳號刪除處理中'), findsOneWidget);
      expect(find.text('帳號狀態'), findsOneWidget);
      expect(find.text('帳號刪除處理中，等待未結訂單完成；期間只能處理訂單與售後'), findsOneWidget);
      expect(find.text('目前仍可使用'), findsOneWidget);
      expect(find.textContaining('商品已從市集下架'), findsNothing);
      expect(find.text('停權原因'), findsNothing);
      expect(find.text('提出申覆'), findsNothing);
      expect(find.text('對停權決定有疑義？'), findsNothing);
      expect(find.text('措施與申覆'), findsNothing);
      expect(find.text('前往幫助中心'), findsNothing);
    });

    testWidgets('入口只剩訂單與通知（沒有 ACCOUNT_CLOSE）', (tester) async {
      await pumpScreen(tester, pendingDeletionStore());

      expect(find.text('商店設定'), findsNothing);
      expect(find.text('訂單管理'), findsOneWidget);
      expect(find.text('通知'), findsOneWidget);
      expect(find.text('發票與稅務相關事項請至網站處理'), findsOneWidget);
    });

    testWidgets('同時停權與等待刪除 → 照停權顯示', (tester) async {
      await pumpScreen(
        tester,
        myStore(
          status: StoreStatus.STORE_STATUS_PENDING_DELETION,
          suspended: true,
        ),
      );

      expect(find.text('商店已暫停營業'), findsOneWidget);
      expect(find.text('停權原因'), findsOneWidget);
    });
  });

  testWidgets('還沒讀到店家 → 載入中，沒有入口；不知道停權來源，所以沒有任何疑義入口', (tester) async {
    await pumpScreen(tester, null);

    expect(find.text('載入中…'), findsOneWidget);
    expect(find.text('訂單管理'), findsNothing);
    expect(find.text('對停權決定有疑義？'), findsNothing);
    expect(find.text('措施與申覆'), findsNothing);
    expect(find.text('前往幫助中心'), findsNothing);
  });

  group('登出（停權與等待刪除都必須能登出）', () {
    Future<_RecordingSession> pumpWithRouter(
      WidgetTester tester,
      MyStore store,
    ) async {
      final session = _RecordingSession();
      final router = GoRouter(
        routes: [
          GoRoute(path: '/', builder: (_, _) => const SuspensionNoticeScreen()),
          GoRoute(
            path: '/auth',
            builder: (_, _) => const Scaffold(body: Text('AUTH')),
          ),
        ],
      );
      addTearDown(router.dispose);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            myStoreSnapshotProvider.overrideWithValue(store),
            sessionProvider.overrideWith(() => session),
          ],
          child: ScreenUtilInit(
            designSize: const Size(393, 852),
            builder: (_, _) => MaterialApp.router(routerConfig: router),
          ),
        ),
      );
      await tester.pumpAndSettle();
      return session;
    }

    for (final (name, store) in [
      ('停權', suspendedStore()),
      ('等待刪除', pendingDeletionStore()),
    ]) {
      testWidgets('$name：確認後登出並回到登入頁', (tester) async {
        final session = await pumpWithRouter(tester, store);
        final logout = find.widgetWithText(OutlinedButton, '登出');
        await tester.ensureVisible(logout);
        await tester.tap(logout);
        await tester.pumpAndSettle();

        expect(find.text('要登出賣家帳號嗎？'), findsOneWidget);
        expect(session.logouts, 0);

        await tester.tap(
          find.descendant(of: find.byType(Dialog), matching: find.text('登出')),
        );
        await tester.pumpAndSettle();

        expect(session.logouts, 1);
        expect(find.text('AUTH'), findsOneWidget);
      });
    }

    testWidgets('取消確認不登出', (tester) async {
      final session = await pumpWithRouter(tester, suspendedStore());
      final logout = find.widgetWithText(OutlinedButton, '登出');
      await tester.ensureVisible(logout);
      await tester.tap(logout);
      await tester.pumpAndSettle();
      await tester.tap(find.text('取消'));
      await tester.pumpAndSettle();

      expect(session.logouts, 0);
      expect(find.text('AUTH'), findsNothing);
    });
  });
}
