import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/moderation/presentation/blocked_users_screen.dart';
import 'package:hawkr_buyer/shared/widgets/hk_empty_state.dart';
import 'package:hawkr_buyer/shared/widgets/truck_loading_overlay.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'moderation_fakes.dart';

Future<void> _pump(
  WidgetTester tester,
  FakeBuyerApp app, {
  bool settle = true,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [buyerAppClientProvider.overrideWithValue(app.client())],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: ThemeData(
            useMaterial3: true,
            colorScheme: ColorScheme.fromSeed(seedColor: AppColors.primary),
            textTheme: AppTypography.textTheme,
          ),
          home: const BlockedUsersScreen(),
        ),
      ),
    ),
  );
  if (settle) await tester.pumpAndSettle();
}

void main() {
  testWidgets('單頁兩段：「店家」在「買家」之前，各自列名稱', (tester) async {
    final app = FakeBuyerApp(
      storePages: [
        [blockedStore('7', name: '好店')],
      ],
      buyerPages: [
        [blockedBuyer('b1', name: '小明')],
      ],
    );
    await _pump(tester, app);

    final storeTitle = find.byKey(const ValueKey('blocked-stores-title'));
    final buyerTitle = find.byKey(const ValueKey('blocked-buyers-title'));
    expect(tester.widget<Text>(storeTitle).data, '店家');
    expect(tester.widget<Text>(buyerTitle).data, '買家');
    expect(
      tester.getTopLeft(storeTitle).dy,
      lessThan(tester.getTopLeft(buyerTitle).dy),
    );
    expect(find.text('好店'), findsOneWidget);
    expect(find.text('小明'), findsOneWidget);
    expect(app.storeListRequests.single.pageToken, '');
    expect(app.buyerListRequests.single.pageToken, '');
  });

  testWidgets('已刪除的買家顯示「已刪除的帳號」，仍可解除封鎖', (tester) async {
    final app = FakeBuyerApp(
      buyerPages: [
        [blockedBuyer('gone-1', name: '殘留的名稱', deleted: true)],
      ],
    );
    await _pump(tester, app);

    expect(find.text('已刪除的帳號'), findsOneWidget);
    expect(find.text('殘留的名稱'), findsNothing);
    await tester.tap(find.text('解除封鎖'));
    await tester.pump();
    await tester.pump();

    expect(app.buyerUnblocks.single.buyerId, 'gone-1');
    expect(find.text('已刪除的帳號'), findsNothing);
    expect(find.text('尚無封鎖的使用者'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('已關閉的店家照樣列出並標示狀態，解除送 UnblockStore', (tester) async {
    final app = FakeBuyerApp(
      storePages: [
        [
          blockedStore(
            '8',
            name: '歇業的店',
            status: StoreCardStatus.STORE_CARD_STATUS_CLOSED,
          ),
        ],
      ],
    );
    await _pump(tester, app);

    expect(find.text('歇業的店'), findsOneWidget);
    expect(find.text('已關閉'), findsOneWidget);
    await tester.tap(find.text('解除封鎖'));
    await tester.pump();
    await tester.pump();

    expect(app.storeUnblocks.single.storeId, '8');
    expect(app.buyerUnblocks, isEmpty);
    expect(find.text('歇業的店'), findsNothing);
    expect(find.text('已解除封鎖'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('每段各自分頁：店家「查看更多」只帶店家的 page_token', (tester) async {
    final app = FakeBuyerApp(
      storePages: [
        [blockedStore('1', name: '店一')],
        [blockedStore('2', name: '店二')],
      ],
      buyerPages: [
        [blockedBuyer('b1', name: '買家一')],
      ],
    );
    await _pump(tester, app);

    expect(find.text('店二'), findsNothing);
    expect(find.byKey(const ValueKey('blocked-buyers-more')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('blocked-stores-more')));
    await tester.pumpAndSettle();

    expect(app.storeListRequests.map((r) => r.pageToken), ['', 'page-1']);
    expect(app.buyerListRequests, hasLength(1));
    expect(find.text('店二'), findsOneWidget);
    expect(find.byKey(const ValueKey('blocked-stores-more')), findsNothing);
  });

  testWidgets('解除封鎖失敗時清單不變並顯示錯誤 toast', (tester) async {
    final app = FakeBuyerApp(
      storePages: [
        [blockedStore('7', name: '好店')],
      ],
      unblockError: ConnectException(Code.unavailable, 'down'),
    );
    await _pump(tester, app);

    await tester.tap(find.text('解除封鎖'));
    await tester.pump();
    await tester.pump();

    expect(find.text('好店'), findsOneWidget);
    expect(find.text('服務暫時不可用'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('沒有封鎖任何人時顯示空狀態', (tester) async {
    await _pump(tester, FakeBuyerApp());

    final empty = find.byType(HkEmptyState);
    expect(empty, findsOneWidget);
    expect(tester.widget<HkEmptyState>(empty).message, '尚無封鎖的使用者');
    expect(find.byType(TruckLoadingOverlay), findsNothing);
  });

  testWidgets('兩段都讀取失敗：錯誤狀態與空狀態分開，重試成功後顯示名單', (tester) async {
    final app = FakeBuyerApp(
      storePages: [
        [blockedStore('7', name: '好店')],
      ],
      listStoresError: ConnectException(Code.unavailable, 'down'),
      listBuyersError: ConnectException(Code.unavailable, 'down'),
    );
    await _pump(tester, app);

    expect(find.byType(HkEmptyState), findsNothing);
    expect(find.text('服務暫時不可用'), findsOneWidget);

    app.listStoresError = null;
    app.listBuyersError = null;
    await tester.tap(find.text('重試'));
    await tester.pumpAndSettle();

    expect(find.text('好店'), findsOneWidget);
    expect(find.text('服務暫時不可用'), findsNothing);
  });

  testWidgets('只有一段失敗：另一段照常顯示，失敗那段可單獨重試', (tester) async {
    final app = FakeBuyerApp(
      buyerPages: [
        [blockedBuyer('b1', name: '小明')],
      ],
      storePages: [
        [blockedStore('7', name: '好店')],
      ],
      listStoresError: ConnectException(Code.unavailable, 'down'),
    );
    await _pump(tester, app);

    expect(find.text('小明'), findsOneWidget);
    expect(find.byKey(const ValueKey('blocked-stores-retry')), findsOneWidget);

    app.listStoresError = null;
    await tester.tap(find.byKey(const ValueKey('blocked-stores-retry')));
    await tester.pumpAndSettle();

    expect(find.text('好店'), findsOneWidget);
    expect(app.buyerListRequests, hasLength(1));
  });

  testWidgets('解除整頁之後仍有下一頁：不顯示空狀態，保留「查看更多」並可繼續載入', (tester) async {
    final app = FakeBuyerApp(
      storePages: [
        [blockedStore('1', name: '店一')],
        [blockedStore('2', name: '店二')],
      ],
    );
    await _pump(tester, app);

    await tester.tap(find.text('解除封鎖'));
    await tester.pump();
    await tester.pump();

    expect(find.text('店一'), findsNothing);
    expect(find.text('尚無封鎖的使用者'), findsNothing);
    expect(find.byKey(const ValueKey('blocked-stores-more')), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
    await tester.tap(find.byKey(const ValueKey('blocked-stores-more')));
    await tester.pumpAndSettle();

    expect(find.text('店二'), findsOneWidget);
  });

  testWidgets('一段已讀完且沒有項目、另一段還在讀取：顯示 loader，不是空白頁', (tester) async {
    final gate = Completer<void>();
    final app = FakeBuyerApp(
      buyerPages: [
        [blockedBuyer('b1', name: '小明')],
      ],
    )..listBuyersGate = gate;
    await _pump(tester, app, settle: false);
    await tester.pump(const Duration(milliseconds: 50));

    expect(
      find.byKey(const ValueKey('blocked-buyers-loading')),
      findsOneWidget,
    );
    expect(find.byType(HkEmptyState), findsNothing);

    gate.complete();
    await tester.pumpAndSettle();
    expect(find.text('小明'), findsOneWidget);
    expect(find.byType(TruckLoadingOverlay), findsNothing);
  });

  testWidgets('一段失敗、另一段還在第一次讀取：失敗那段的重試與讀取中那段的 loader 同時在頁面上', (tester) async {
    final gate = Completer<void>();
    final app = FakeBuyerApp(
      listStoresError: ConnectException(Code.unavailable, 'down'),
    )..listBuyersGate = gate;
    await _pump(tester, app, settle: false);
    await tester.pump(const Duration(milliseconds: 50));

    expect(find.byKey(const ValueKey('blocked-stores-retry')), findsOneWidget);
    expect(
      find.byKey(const ValueKey('blocked-buyers-loading')),
      findsOneWidget,
    );
    expect(find.byType(TruckLoadingOverlay), findsNothing);

    gate.complete();
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('blocked-stores-retry')), findsOneWidget);
    expect(find.byKey(const ValueKey('blocked-buyers-loading')), findsNothing);
  });

  testWidgets('單獨重試一段、另一段已讀完且沒有項目：不換成整頁 loader，該段就地顯示 loader', (tester) async {
    final app = FakeBuyerApp(
      listStoresError: ConnectException(Code.unavailable, 'down'),
    );
    await _pump(tester, app);
    expect(find.byKey(const ValueKey('blocked-stores-retry')), findsOneWidget);

    final gate = Completer<void>();
    app
      ..listStoresError = null
      ..listStoresGate = gate;
    await tester.tap(find.byKey(const ValueKey('blocked-stores-retry')));
    await tester.pump();

    expect(find.byType(TruckLoadingOverlay), findsNothing);
    expect(find.byKey(const ValueKey('blocked-stores-title')), findsOneWidget);
    expect(
      find.byKey(const ValueKey('blocked-stores-loading')),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('blocked-stores-retry')), findsNothing);

    gate.complete();
    await tester.pumpAndSettle();
    expect(find.byType(HkEmptyState), findsOneWidget);
  });
}
