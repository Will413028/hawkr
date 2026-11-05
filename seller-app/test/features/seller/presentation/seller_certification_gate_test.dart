import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/features/seller/data/version_conflict.dart'
    show storeStateUnavailableNotice;
import 'package:hawkr_seller/features/seller/presentation/seller_certification_gate.dart';
import 'package:hawkr_seller/features/seller/presentation/profile_other_options_card.dart';
import 'package:hawkr_seller/features/seller/presentation/suspension_notice_screen.dart'
    show pendingDeletionNotice;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/my_store_fixtures.dart';

MyStore _certified() => myStore(publicListing: publicListingMissing());

MyStore _uncertified() => myStore(
  publicListing: publicListingMissing([
    SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE,
  ]),
  shippingSetup: shippingSetupMissing([
    SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE,
  ]),
);

void main() {
  test('閘門文字與 website 的認證閘門相同：通過平台審核並啟用收款帳戶後才能設定物流與退貨', () {
    final copy = shippingCertificationGateCopy();
    expect(copy.title, '尚未通過商家認證');
    expect(copy.message, '通過平台審核並啟用收款帳戶後，就能設定物流與退貨。');
    expect(copy.confirmText, '前往商家認證');
  });

  testWidgets('unverified shipping entry explains the gate then opens KYC', (
    tester,
  ) async {
    final router = _routerWithShippingEntry(
      loadStore: () async => _uncertified(),
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.tap(find.text('配送設定'));
    await tester.pumpAndSettle();

    expect(find.text('尚未通過商家認證'), findsOneWidget);
    expect(find.textContaining('就能設定物流與退貨'), findsOneWidget);
    expect(find.text('配送頁'), findsNothing);
    expect(
      find.byWidgetPredicate(
        (widget) =>
            widget is ModalBarrier && widget.color == AppColors.bgOverlaySoft,
      ),
      findsOneWidget,
    );

    await tester.tap(find.text('前往商家認證'));
    await tester.pumpAndSettle();

    expect(find.text('實名認證流程'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('伺服器說物流設定缺停權條件：顯示停權說明、不讀 KYC、不開物流', (tester) async {
    final router = _routerWithShippingEntry(
      loadStore: () async => myStore(
        suspended: true,
        publicListing: publicListingMissing([
          SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED,
        ]),
        shippingSetup: shippingSetupMissing([
          SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED,
        ]),
      ),
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.tap(find.text('配送設定'));
    await tester.pumpAndSettle();

    expect(find.text('商店已暫停營業'), findsOneWidget);
    expect(find.text('尚未通過商家認證'), findsNothing);
    expect(find.text('配送頁'), findsNothing);
  });

  testWidgets('停權且等待刪除：對話框依 shippingGate 顯示等待刪除說明', (tester) async {
    final router = _routerWithShippingEntry(
      loadStore: () async => myStore(
        status: StoreStatus.STORE_STATUS_PENDING_DELETION,
        suspended: true,
        shippingSetup: shippingSetupMissing([
          SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED,
          SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION,
        ]),
      ),
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.tap(find.text('配送設定'));
    await tester.pumpAndSettle();

    expect(find.text('帳號刪除處理中'), findsOneWidget);
    expect(find.text(pendingDeletionNotice), findsOneWidget);
    expect(find.text('商店已暫停營業'), findsNothing);
  });

  testWidgets('停權且 SHIPPING_SETUP 只缺停權：即使店家也在等待刪除，仍顯示停權說明', (tester) async {
    final router = _routerWithShippingEntry(
      loadStore: () async => myStore(
        status: StoreStatus.STORE_STATUS_PENDING_DELETION,
        suspended: true,
        shippingSetup: shippingSetupMissing([
          SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED,
        ]),
      ),
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.tap(find.text('配送設定'));
    await tester.pumpAndSettle();

    expect(find.text('商店已暫停營業'), findsOneWidget);
    expect(find.text('帳號已被平台停權'), findsOneWidget);
    expect(find.text(pendingDeletionNotice), findsNothing);
  });

  testWidgets('伺服器說物流設定缺等待刪除條件：顯示刪除處理中說明', (tester) async {
    final router = _routerWithShippingEntry(
      loadStore: () async => pendingDeletionStore(),
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.tap(find.text('配送設定'));
    await tester.pumpAndSettle();

    expect(find.text('帳號刪除處理中'), findsOneWidget);
    expect(find.text(pendingDeletionNotice), findsOneWidget);
    expect(find.text('配送頁'), findsNothing);
  });

  testWidgets('PUBLIC_LISTING 不缺 KYC 但 SHIPPING_SETUP 缺：以 SHIPPING_SETUP 為準', (
    tester,
  ) async {
    final router = _routerWithShippingEntry(
      loadStore: () async => myStore(
        publicListing: publicListingMissing(),
        shippingSetup: shippingSetupMissing([
          SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE,
        ]),
      ),
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.tap(find.text('配送設定'));
    await tester.pumpAndSettle();

    expect(find.text('尚未通過商家認證'), findsOneWidget);
    expect(find.text('配送頁'), findsNothing);
  });

  for (final entry in {
    '店家讀不到': () async => null,
    'SHIPPING_SETUP 沒回報': () async => MyStore(
      GetMyStoreResponse(
        store: Store(id: 'store-1', onboardingCompleted: true),
      ),
    ),
    '缺 STORE_ACTIVE': () async => myStore(
      shippingSetup: shippingSetupMissing([
        SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE,
        SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE,
      ]),
    ),
    '缺項不認得': () async => myStore(
      shippingSetup: shippingSetupMissing([
        SellerRequirement.SELLER_REQUIREMENT_LOGO,
      ]),
    ),
  }.entries) {
    testWidgets('${entry.key}：不開放、不退回認證文案，提示無法確認商店狀態', (tester) async {
      final router = _routerWithShippingEntry(loadStore: entry.value);
      addTearDown(router.dispose);

      await tester.pumpWidget(MaterialApp.router(routerConfig: router));
      await tester.tap(find.text('配送設定'));
      await tester.pumpAndSettle();

      expect(find.text('配送頁'), findsNothing);
      expect(find.byType(Dialog), findsNothing);
      expect(find.text('尚未通過商家認證'), findsNothing);
      expect(find.text(storeStateUnavailableNotice), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
    });
  }

  testWidgets('approved shipping entry opens settings without a dialog', (
    tester,
  ) async {
    final router = _routerWithShippingEntry(
      loadStore: () async => _certified(),
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.tap(find.text('配送設定'));
    await tester.pumpAndSettle();

    expect(find.text('配送頁'), findsOneWidget);
    expect(find.byType(Dialog), findsNothing);
    expect(tester.takeException(), isNull);
  });

  for (final gate in [
    ShippingGate.suspended,
    ShippingGate.pendingDeletion,
    ShippingGate.unknown,
  ]) {
    testWidgets('設定頁物流入口在 $gate 顯示「未營業」', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(
              child: ProfileOtherOptionsCard(
                shippingGate: gate,
                onShippingTap: () {},
              ),
            ),
          ),
        ),
      );
      expect(find.text('未營業'), findsOneWidget);
      expect(find.text('需認證'), findsNothing);
      expect(find.text('管理宅配、超商與寄退件地址'), findsOneWidget);
    });
  }

  testWidgets(
    'settings keeps the locked shipping entry visible and explained',
    (tester) async {
      var tapped = false;
      // 正式畫面把這張卡放在 profile_screen 的 CustomScrollView 裡
      // （`HkPageSection('營運設定')`）。這裡不給捲動容器的話，營運入口每多
      // 一列就會撞一次 RenderFlex overflow——那是測試少了正式畫面本來就有的
      // 捲動，不是卡片壞了。
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(
              child: ProfileOtherOptionsCard(
                shippingGate: ShippingGate.certification,
                onShippingTap: () => tapped = true,
              ),
            ),
          ),
        ),
      );

      expect(find.text('物流與退貨'), findsOneWidget);
      expect(find.text('通過商家認證後才可設定配送'), findsOneWidget);
      expect(find.text('需認證'), findsOneWidget);

      await tester.ensureVisible(find.text('物流與退貨'));
      await tester.tap(find.text('物流與退貨'));
      await tester.pump();
      expect(tapped, isTrue);
      expect(tester.takeException(), isNull);
    },
  );
}

GoRouter _routerWithShippingEntry({required StoreLoader loadStore}) {
  return GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        builder: (context, _) => Scaffold(
          body: Center(
            child: TextButton(
              onPressed: () => openShippingSettingsWithCertificationGate(
                context,
                loadStore: loadStore,
                locale: UiLocale.zhTW,
              ),
              child: const Text('配送設定'),
            ),
          ),
        ),
      ),
      GoRoute(
        path: '/settings/shipping',
        builder: (_, __) => const Scaffold(body: Text('配送頁')),
      ),
      GoRoute(
        path: '/main/wallet',
        builder: (_, __) => const Scaffold(body: Text('實名認證流程')),
      ),
    ],
  );
}
