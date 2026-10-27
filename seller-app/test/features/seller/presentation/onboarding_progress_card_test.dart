import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/router/pending_wallet_focus.dart';
import 'package:hawkr_seller/core/refresh/refresh_clock.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/onboarding_tasks.dart';
import 'package:hawkr_seller/features/seller/presentation/onboarding_progress_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/fake_my_store.dart';
import '../../../support/my_store_fixtures.dart';

const _name = SellerRequirement.SELLER_REQUIREMENT_STORE_NAME;
const _logo = SellerRequirement.SELLER_REQUIREMENT_LOGO;
const _kyc = SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED;
const _notSuspended = SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED;

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

ProviderContainer? _lastContainer;

Future<GoRouter> _pump(
  WidgetTester tester, {
  required List<SellerRequirement> missing,
  List<SellerRequirement> shippingMissing = const [],
  Object? kyc,
  OnboardingInputs inputs = const OnboardingInputs(),
  String description = '',
}) async {
  final server = FakeMyStoreServer(
    response: myStoreResponse(
      publicListing: publicListingMissing(missing),
      shippingSetup: shippingSetupMissing(shippingMissing),
      description: description,
    ),
  );
  final container = ProviderContainer(
    overrides: [
      sessionProvider.overrideWith(_LoggedIn.new),
      myStoreRepositoryProvider.overrideWithValue(server.repository),
      onboardingInputsProvider.overrideWith((ref) async => inputs),
      sellerClientProvider.overrideWithValue(
        SellerAppServiceClient(
          FakeTransportBuilder().unary(SellerAppService.getMyKYC, (_, _) {
            final answer = kyc;
            if (answer is ConnectException) throw answer;
            return answer as GetMyKYCResponse;
          }).build(),
        ),
      ),
    ],
  );
  addTearDown(container.dispose);
  _lastContainer = container;
  final sub = container.listen(currentStoreProvider, (_, _) {});
  addTearDown(sub.close);
  await container.read(currentStoreProvider.future);

  tester.view.physicalSize = const Size(900, 3000);
  tester.view.devicePixelRatio = 2;
  addTearDown(tester.view.reset);

  Widget page(String label) => Scaffold(body: Text(label));
  final router = GoRouter(
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) => const Scaffold(
          body: SingleChildScrollView(child: OnboardingProgressCard()),
        ),
      ),
      GoRoute(path: '/product/picker', builder: (_, _) => page('商品選擇頁')),
      GoRoute(
        path: '/main/wallet',
        builder: (_, state) => Scaffold(
          body: Column(children: [const Text('認證頁'), Text('uri:${state.uri}')]),
        ),
      ),
      GoRoute(
        path: '/seller/suspension-notice',
        builder: (_, _) => page('停權說明頁'),
      ),
      GoRoute(path: '/settings/shipping', builder: (_, _) => page('物流設定頁')),
    ],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp.router(routerConfig: router),
    ),
  );
  await tester.pumpAndSettle();
  return router;
}

GetMyKYCResponse _flow(KYCFlowStatus status) => GetMyKYCResponse(
  application: KYCApplication(flow: KYCFlow(status: status)),
);

void main() {
  testWidgets('兩組任務：公開前必須與建議完成，文字照 website', (tester) async {
    await _pump(
      tester,
      missing: [_logo, _kyc],
      shippingMissing: [_kyc],
      kyc: _flow(KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED),
    );

    expect(find.text('賣家開店任務'), findsOneWidget);
    expect(find.text('開店進度'), findsOneWidget);
    expect(find.text('0/2'), findsOneWidget);
    expect(find.text('公開前必須'), findsOneWidget);
    expect(find.text('建議完成'), findsOneWidget);
    expect(find.text('缺少：logo'), findsOneWidget);
    expect(find.text('平台審核中'), findsOneWidget);
    expect(find.text('查看進度'), findsOneWidget);
    expect(find.text('需先通過認證'), findsOneWidget);
    expect(find.text('食品加工登錄字號'), findsOneWidget);
    expect(find.text('非必要'), findsOneWidget);
    expect(find.text('停權處分'), findsNothing);
    expect(find.text('收合'), findsOneWidget);
  });

  testWidgets('缺停權條件時多一列停權處分，導向停權說明', (tester) async {
    await _pump(
      tester,
      missing: [_notSuspended],
      kyc: _flow(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED),
    );

    expect(find.text('停權處分'), findsOneWidget);
    expect(find.text('停權中'), findsOneWidget);
    await tester.tap(find.text('查看停權說明'));
    await tester.pumpAndSettle();
    expect(find.text('停權說明頁'), findsOneWidget);
  });

  testWidgets('公開前必須全部完成時預設收合，可以展開', (tester) async {
    await _pump(
      tester,
      missing: const [],
      kyc: _flow(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED),
    );

    expect(find.text('任務完成'), findsOneWidget);
    expect(find.text('展開任務'), findsOneWidget);
    expect(find.text('公開前必須'), findsNothing);

    await tester.tap(find.text('展開任務'));
    await tester.pumpAndSettle();
    expect(find.text('公開前必須'), findsOneWidget);
    expect(find.text('建議完成'), findsOneWidget);
    expect(find.text('收合'), findsOneWidget);
  });

  testWidgets('SHIPPING_SETUP 沒有啟用時物流設定鎖定，按鈕點不了', (tester) async {
    await _pump(
      tester,
      missing: [_kyc],
      shippingMissing: [_kyc],
      kyc: _flow(KYCFlowStatus.KYC_FLOW_STATUS_DRAFT),
    );

    final button = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '設定物流'),
    );
    expect(button.onPressed, isNull);
  });

  testWidgets('PUBLIC_LISTING 還缺 KYC 但 SHIPPING_SETUP 啟用：物流設定可以進入', (
    tester,
  ) async {
    await _pump(
      tester,
      missing: [_kyc],
      kyc: _flow(KYCFlowStatus.KYC_FLOW_STATUS_DRAFT),
    );
    final button = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '設定物流'),
    );
    expect(button.onPressed, isNotNull);
  });

  testWidgets('SHIPPING_SETUP 啟用時物流設定可以進入', (tester) async {
    await _pump(
      tester,
      missing: const [],
      kyc: _flow(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED),
    );
    await tester.tap(find.text('展開任務'));
    await tester.pumpAndSettle();

    expect(find.text('待設定'), findsOneWidget);
    await tester.tap(find.text('設定物流'));
    await tester.pumpAndSettle();
    expect(find.text('物流設定頁'), findsOneWidget);
  });

  testWidgets('還沒通過認證也能新增商品，不經過認證流程', (tester) async {
    await _pump(
      tester,
      missing: [_kyc],
      kyc: ConnectException(Code.notFound, 'none'),
    );

    expect(find.text('待認證'), findsOneWidget);
    await tester.tap(find.text('新增商品'));
    await tester.pumpAndSettle();
    expect(find.text('商品選擇頁'), findsOneWidget);
    expect(find.text('認證頁'), findsNothing);
  });

  testWidgets('認證列的按鈕依流程狀態，進入認證頁', (tester) async {
    await _pump(
      tester,
      missing: [_name, _kyc],
      kyc: _flow(KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO),
    );

    expect(find.text('需補件'), findsOneWidget);
    expect(find.text('前往補件'), findsOneWidget);
    await tester.tap(find.text('前往補件').first);
    await tester.pumpAndSettle();
    expect(find.text('認證頁'), findsOneWidget);
  });

  testWidgets('讀不到認證狀態時顯示「未知」，不顯示成待認證', (tester) async {
    await _pump(
      tester,
      missing: [_kyc],
      kyc: ConnectException(Code.unavailable, 'later'),
    );

    expect(find.text('待認證'), findsNothing);
    expect(find.text('未知'), findsOneWidget);
    expect(find.text('開始認證'), findsWidgets);
  });

  testWidgets('商品數與商店介紹讓建議項目完成', (tester) async {
    await _pump(
      tester,
      missing: const [],
      kyc: _flow(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED),
      inputs: const OnboardingInputs(productCount: 2),
      description: '產地直送',
    );
    await tester.tap(find.text('展開任務'));
    await tester.pumpAndSettle();

    expect(find.text('已上架'), findsOneWidget);
    expect(find.text('待完成'), findsNothing);
    expect(find.text('已完成'), findsNWidgets(2));
  });

  testWidgets('食品登錄字號：核准前後都導向錢包的加工食品列', (tester) async {
    await _pump(
      tester,
      missing: const [],
      kyc: _flow(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED),
    );
    await tester.tap(find.text('展開任務'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('填寫資料'));
    await tester.pumpAndSettle();
    expect(find.text('認證頁'), findsOneWidget);
    expect(find.text('uri:/main/wallet'), findsOneWidget);
    expect(
      _lastContainer!.read(pendingWalletFocusProvider),
      WalletSections.foodRegistration,
      reason: '錢包取走之前，意圖還在',
    );

    await tester.pumpWidget(const SizedBox.shrink());
    await _pump(
      tester,
      missing: [_kyc],
      kyc: _flow(KYCFlowStatus.KYC_FLOW_STATUS_DRAFT),
    );
    await tester.tap(find.text('填寫資料'));
    await tester.pumpAndSettle();
    expect(find.text('uri:/main/wallet'), findsOneWidget);
    expect(
      _lastContainer!.read(pendingWalletFocusProvider),
      WalletSections.foodRegistration,
    );
  });

  testWidgets('點任務去錢包：離開時不重讀，回到儀表板（成為可見）時重讀一次', (tester) async {
    final server = FakeMyStoreServer(
      response: myStoreResponse(
        publicListing: publicListingMissing([_kyc]),
        shippingSetup: shippingSetupMissing(),
      ),
    );
    var inputsBuilds = 0;
    var now = DateTime(2026, 10, 9, 12);
    final container = ProviderContainer(
      overrides: [
        refreshClockProvider.overrideWithValue(() => now),
        sessionProvider.overrideWith(_LoggedIn.new),
        myStoreRepositoryProvider.overrideWithValue(server.repository),
        onboardingInputsProvider.overrideWith((ref) async {
          inputsBuilds++;
          return const OnboardingInputs();
        }),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyKYC, (_, _) {
              return _flow(KYCFlowStatus.KYC_FLOW_STATUS_DRAFT);
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final storeSub = container.listen(currentStoreProvider, (_, _) {});
    final inputsSub = container.listen(onboardingInputsProvider, (_, _) {});
    addTearDown(storeSub.close);
    addTearDown(inputsSub.close);
    await container.read(currentStoreProvider.future);

    tester.view.physicalSize = const Size(900, 3000);
    tester.view.devicePixelRatio = 2;
    addTearDown(tester.view.reset);
    final router = GoRouter(
      initialLocation: '/main/dashboard',
      routes: [
        StatefulShellRoute.indexedStack(
          builder: (_, _, shell) => Scaffold(body: shell),
          branches: [
            StatefulShellBranch(
              routes: [
                GoRoute(
                  path: '/main/dashboard',
                  builder: (_, _) => const SingleChildScrollView(
                    child: OnboardingProgressCard(),
                  ),
                ),
              ],
            ),
            StatefulShellBranch(
              routes: [
                GoRoute(
                  path: '/main/wallet',
                  builder: (_, _) => const Text('錢包頁'),
                ),
              ],
            ),
          ],
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    final storeBefore = server.calls;
    final inputsBefore = inputsBuilds;

    await tester.tap(find.text('開始認證').first);
    await tester.pumpAndSettle();
    expect(find.text('錢包頁'), findsOneWidget);
    expect(server.calls, storeBefore, reason: '離開時不重讀');
    expect(inputsBuilds, inputsBefore);

    router.go('/main/dashboard');
    await tester.pumpAndSettle();
    expect(server.calls, storeBefore + 1, reason: '回來重讀一次');
    expect(inputsBuilds, inputsBefore + 1);

    await tester.tap(find.text('開始認證').first);
    await tester.pumpAndSettle();
    now = now.add(const Duration(seconds: 29));
    router.go('/main/dashboard');
    await tester.pumpAndSettle();
    expect(server.calls, storeBefore + 1, reason: '店家與認證 30 秒內讀過，不再讀');
    expect(inputsBuilds, inputsBefore + 2, reason: '物流與商品數每次回來都重讀');

    await tester.tap(find.text('開始認證').first);
    await tester.pumpAndSettle();
    now = now.add(const Duration(seconds: 2));
    router.go('/main/dashboard');
    await tester.pumpAndSettle();
    expect(server.calls, storeBefore + 2, reason: '超過 30 秒，再讀一次');
    expect(inputsBuilds, inputsBefore + 3);
  });
}
