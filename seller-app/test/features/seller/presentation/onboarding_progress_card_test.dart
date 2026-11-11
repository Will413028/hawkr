import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
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

import '../../../support/connect_errors.dart';
import '../../../support/fake_my_store.dart';
import '../../../support/onboarding_fixtures.dart';
import '../../../support/my_store_fixtures.dart';

const _kyc = SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE;
const _logo = SellerRequirement.SELLER_REQUIREMENT_LOGO;

const _profile = OnboardingTaskId.ONBOARDING_TASK_ID_PROFILE;
const _review = OnboardingTaskId.ONBOARDING_TASK_ID_PLATFORM_REVIEW;
const _payment = OnboardingTaskId.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT;
const _completed = OnboardingTaskState.ONBOARDING_TASK_STATE_COMPLETED;
const _actionRequired =
    OnboardingTaskState.ONBOARDING_TASK_STATE_ACTION_REQUIRED;
const _inProgress = OnboardingTaskState.ONBOARDING_TASK_STATE_IN_PROGRESS;
const _locked = OnboardingTaskState.ONBOARDING_TASK_STATE_LOCKED;

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

Future<GoRouter> _pump(
  WidgetTester tester, {
  required List<SellerRequirement> missing,
  List<SellerRequirement> shippingMissing = const [],
  Object? summary,
  Object? refresh,
  List<String>? calls,
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
          FakeTransportBuilder()
              .unary(SellerAppService.getMyOnboarding, (_, _) {
                calls?.add('getMyOnboarding');
                final answer = summary;
                if (answer is ConnectException) throw answer;
                return answer as GetMyOnboardingResponse;
              })
              .unary(SellerAppService.refreshMyPaymentAccount, (_, _) {
                calls?.add('refreshMyPaymentAccount');
                final answer = refresh;
                if (answer is ConnectException) throw answer;
                return RefreshMyPaymentAccountResponse();
              })
              .build(),
        ),
      ),
    ],
  );
  addTearDown(container.dispose);
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
      GoRoute(path: '/settings/help', builder: (_, _) => page('說明頁')),
      GoRoute(path: '/main/settings', builder: (_, _) => page('商店設定頁')),
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

void main() {
  testWidgets('兩組任務：公開前必須的標題、狀態句、按鈕取自伺服器與目錄，建議完成由 client 組', (tester) async {
    await _pump(
      tester,
      missing: [_logo, _kyc],
      shippingMissing: [_kyc],
      summary: onboardingSummary([
        serverTask(
          _profile,
          _actionRequired,
          action: OnboardingAction.ONBOARDING_ACTION_EDIT_PROFILE,
          title: '商店資料',
          status: '缺少：logo',
        ),
        serverTask(
          _review,
          _inProgress,
          action: OnboardingAction.ONBOARDING_ACTION_NONE,
          title: '平台審核',
          status: '審核中',
        ),
        serverTask(_payment, _locked, title: '收款帳戶', status: '平台審核通過後開放設定'),
      ]),
    );

    expect(find.text('賣家開店任務'), findsOneWidget);
    expect(find.text('開店進度'), findsOneWidget);
    expect(find.text('0/3'), findsOneWidget);
    expect(find.text('公開前必須'), findsOneWidget);
    expect(find.text('建議完成'), findsOneWidget);
    expect(find.text('缺少：logo'), findsOneWidget);
    expect(find.text('店名、logo 與所在地。'), findsOneWidget);
    expect(find.text('商家資料與統編的平台審核。'), findsOneWidget);
    expect(find.text('向金流商設定身分與銀行資料。'), findsOneWidget);
    expect(find.text('審核中'), findsWidgets);
    expect(find.text('平台審核通過後開放設定'), findsOneWidget);
    expect(find.text('需先通過認證'), findsOneWidget);
    expect(find.text('收合'), findsOneWidget);
    expect(
      find.text('前往商家認證'),
      findsNothing,
      reason: 'IN_PROGRESS 且 action 是 NONE 沒有按鈕',
    );
  });

  testWidgets('沒有摘要（讀取失敗）：沒有必要任務，也不自己推導', (tester) async {
    await _pump(
      tester,
      missing: [_logo, _kyc],
      summary: ConnectException(Code.unavailable, 'later'),
    );

    expect(find.text('公開前必須'), findsNothing);
    expect(find.text('平台審核'), findsNothing);
    expect(find.text('建議完成'), findsOneWidget);
  });

  testWidgets('公開前必須全部完成時預設收合，可以展開', (tester) async {
    await _pump(tester, missing: const [], summary: completedSummary());

    expect(find.text('任務完成'), findsOneWidget);
    expect(find.text('展開任務'), findsOneWidget);
    expect(find.text('公開前必須'), findsNothing);

    await tester.tap(find.text('展開任務'));
    await tester.pumpAndSettle();
    expect(find.text('公開前必須'), findsOneWidget);
    expect(find.text('建議完成'), findsOneWidget);
    expect(find.text('收合'), findsOneWidget);
  });

  testWidgets('摘要是 UNKNOWN 的任務不算完成', (tester) async {
    await _pump(
      tester,
      missing: const [],
      summary: onboardingSummary([
        serverTask(_profile, OnboardingTaskState.ONBOARDING_TASK_STATE_UNKNOWN),
        serverTask(_review, _completed),
      ]),
    );

    expect(find.text('任務完成'), findsNothing);
    expect(find.text('1/2'), findsOneWidget);
  });

  testWidgets('SHIPPING_SETUP 沒有啟用時物流設定鎖定，按鈕點不了', (tester) async {
    await _pump(
      tester,
      missing: [_kyc],
      shippingMissing: [_kyc],
      summary: reviewPendingSummary(),
    );

    final button = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '設定物流'),
    );
    expect(button.onPressed, isNull);
  });

  testWidgets('PUBLIC_LISTING 還缺 KYC 但 SHIPPING_SETUP 啟用：物流設定可以進入', (
    tester,
  ) async {
    await _pump(tester, missing: [_kyc], summary: reviewPendingSummary());
    final button = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '設定物流'),
    );
    expect(button.onPressed, isNotNull);
  });

  testWidgets('SHIPPING_SETUP 啟用時物流設定可以進入', (tester) async {
    await _pump(tester, missing: const [], summary: completedSummary());
    await tester.tap(find.text('展開任務'));
    await tester.pumpAndSettle();

    expect(find.text('待設定'), findsOneWidget);
    await tester.tap(find.text('設定物流'));
    await tester.pumpAndSettle();
    expect(find.text('物流設定頁'), findsOneWidget);
  });

  testWidgets('還沒通過認證也能新增商品，不經過認證流程', (tester) async {
    await _pump(tester, missing: [_kyc], summary: reviewPendingSummary());

    await tester.tap(find.text('新增商品'));
    await tester.pumpAndSettle();
    expect(find.text('商品選擇頁'), findsOneWidget);
    expect(find.text('認證頁'), findsNothing);
  });

  testWidgets('SUBMIT_CERTIFICATION 的按鈕進入認證頁；鎖定的收款帳戶沒有按鈕', (tester) async {
    await _pump(tester, missing: [_kyc], summary: reviewPendingSummary());

    expect(find.text('前往設定收款帳戶'), findsNothing);
    await tester.tap(find.text('前往商家認證'));
    await tester.pumpAndSettle();
    expect(find.text('認證頁'), findsOneWidget);
  });

  testWidgets('START_PAYMENT_ONBOARDING 的按鈕進入認證頁；已完成的平台審核沒有按鈕', (tester) async {
    await _pump(tester, missing: [_kyc], summary: paymentPendingSummary());

    expect(find.text('前往商家認證'), findsNothing);
    await tester.tap(find.text('前往設定收款帳戶'));
    await tester.pumpAndSettle();
    expect(find.text('認證頁'), findsOneWidget);
  });

  testWidgets('EDIT_PROFILE 的按鈕去商店設定頁；CONTACT_SUPPORT 的按鈕去說明頁', (tester) async {
    await _pump(
      tester,
      missing: [_logo],
      summary: onboardingSummary([
        serverTask(
          _profile,
          _actionRequired,
          action: OnboardingAction.ONBOARDING_ACTION_EDIT_PROFILE,
        ),
        serverTask(
          _review,
          _actionRequired,
          action: OnboardingAction.ONBOARDING_ACTION_CONTACT_SUPPORT,
        ),
      ]),
    );

    await tester.tap(find.text('聯絡客服'));
    await tester.pumpAndSettle();
    expect(find.text('說明頁'), findsOneWidget);
  });

  testWidgets('REFRESH_PAYMENT_ACCOUNT：原地先呼叫 RefreshMyPaymentAccount 再重讀摘要', (
    tester,
  ) async {
    final calls = <String>[];
    await _pump(
      tester,
      missing: [_kyc],
      calls: calls,
      summary: onboardingSummary([
        serverTask(_profile, _completed),
        serverTask(_review, _completed),
        serverTask(
          _payment,
          _inProgress,
          action: OnboardingAction.ONBOARDING_ACTION_REFRESH_PAYMENT_ACCOUNT,
        ),
      ]),
    );
    calls.clear();

    await tester.tap(find.text('重新整理'));
    await tester.pumpAndSettle();

    expect(calls.first, 'refreshMyPaymentAccount');
    expect(calls, contains('getMyOnboarding'));
    expect(find.text('認證頁'), findsNothing, reason: '原地處理，不換頁');
  });

  testWidgets('REFRESH_PAYMENT_ACCOUNT 重讀失敗：照樣重讀摘要，並顯示伺服器的句子', (tester) async {
    final calls = <String>[];
    await _pump(
      tester,
      missing: [_kyc],
      calls: calls,
      refresh: reasonError(
        Code.resourceExhausted,
        'RATE_LIMITED',
        message: '請稍後再試',
      ),
      summary: onboardingSummary([
        serverTask(_review, _completed),
        serverTask(
          _payment,
          _inProgress,
          action: OnboardingAction.ONBOARDING_ACTION_REFRESH_PAYMENT_ACCOUNT,
        ),
      ]),
    );
    calls.clear();

    await tester.tap(find.text('重新整理'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(calls, contains('getMyOnboarding'));
    expect(find.text('請稍後再試'), findsOneWidget);
    await tester.pumpAndSettle(const Duration(seconds: 5));
  });

  testWidgets('商品數與商店介紹讓建議項目完成', (tester) async {
    await _pump(
      tester,
      missing: const [],
      summary: completedSummary(),
      inputs: const OnboardingInputs(productCount: 2),
      description: '產地直送',
    );
    await tester.tap(find.text('展開任務'));
    await tester.pumpAndSettle();

    expect(find.text('已上架'), findsOneWidget);
    expect(find.text('待完成'), findsNothing);
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
            FakeTransportBuilder()
                .unary(SellerAppService.getMyKYC, (_, _) {
                  throw ConnectException(Code.notFound, 'none');
                })
                .unary(SellerAppService.getMyPaymentAccount, (_, _) {
                  throw ConnectException(Code.notFound, 'none');
                })
                .unary(SellerAppService.getMyOnboarding, (_, _) {
                  return reviewPendingSummary();
                })
                .build(),
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

    await tester.tap(find.text('前往商家認證').first);
    await tester.pumpAndSettle();
    expect(find.text('錢包頁'), findsOneWidget);
    expect(server.calls, storeBefore, reason: '離開時不重讀');
    expect(inputsBuilds, inputsBefore);

    router.go('/main/dashboard');
    await tester.pumpAndSettle();
    expect(server.calls, storeBefore + 1, reason: '回來重讀一次');
    expect(inputsBuilds, inputsBefore + 1);

    await tester.tap(find.text('前往商家認證').first);
    await tester.pumpAndSettle();
    now = now.add(const Duration(seconds: 29));
    router.go('/main/dashboard');
    await tester.pumpAndSettle();
    expect(server.calls, storeBefore + 1, reason: '店家與認證 30 秒內讀過，不再讀');
    expect(inputsBuilds, inputsBefore + 2, reason: '物流與商品數每次回來都重讀');

    await tester.tap(find.text('前往商家認證').first);
    await tester.pumpAndSettle();
    now = now.add(const Duration(seconds: 2));
    router.go('/main/dashboard');
    await tester.pumpAndSettle();
    expect(server.calls, storeBefore + 2, reason: '超過 30 秒，再讀一次');
    expect(inputsBuilds, inputsBefore + 3);
  });
}
