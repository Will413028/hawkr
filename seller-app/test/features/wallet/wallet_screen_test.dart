import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/features/seller/data/seller_status_refresh.dart';
import 'package:hawkr_seller/core/refresh/refresh_clock.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/coupon/data/coupon_model.dart';
import 'package:hawkr_seller/features/coupon/presentation/coupon_controller.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/payment_onboarding.dart';
import 'package:hawkr_seller/features/wallet/presentation/wallet_screen.dart';
import 'package:hawkr_seller/shared/widgets/truck_loading_overlay.dart';
import 'package:flutter/foundation.dart' show ValueListenable;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../support/fake_my_store.dart';
import '../../support/kyc_fixtures.dart';
import '../../support/my_store_fixtures.dart';
import '../../support/onboarding_fixtures.dart';

const _enabled = PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED;

PaymentAccount _account(
  PaymentAccountStatus status, {
  bool payoutsActive = true,
}) => PaymentAccount(status: status, payoutsActive: payoutsActive);

class _Server {
  _Server({
    KYCStatus review = KYCStatus.KYC_STATUS_APPROVED,
    Object? payment,
    this.certified = true,
    this.marketing = true,
  }) : _review = review {
    this.payment = review == KYCStatus.KYC_STATUS_APPROVED
        ? (payment ?? _account(_enabled))
        : payment;
    kyc = _kycResponse();
    store = FakeMyStoreServer(response: _storeResponse());
  }

  late Object kyc;
  final KYCStatus _review;

  /// `GetMyPaymentAccount` 的回應：[PaymentAccount]、例外，或 null（not_found：平台審核還沒核准或外部帳戶在建立）。
  Object? payment;

  GetMyKYCResponse _kycResponse() =>
      GetMyKYCResponse(application: individualApplication(kycStatus: _review));
  bool certified;
  bool marketing;
  late final FakeMyStoreServer store;
  Completer<void>? kycGate;
  Completer<void>? summaryGate;
  Completer<void>? launchGate;

  /// `GetMyOnboarding` 的回應：[GetMyOnboardingResponse] 或例外；null 時照目前的申請與收款帳戶推出一份同義的摘要
  /// （核准且收款帳戶 ENABLED 是全部完成，核准但帳戶還沒啟用是收款帳戶待開通，其餘是平台審核還沒完成）。
  Object? summary;
  int summaryCalls = 0;
  final calls = <String>[];

  GetMyOnboardingResponse _derivedSummary() {
    final answer = kyc;
    final approved =
        answer is GetMyKYCResponse &&
        answer.application.status == KYCStatus.KYC_STATUS_APPROVED;
    if (!approved) return reviewPendingSummary();
    final account = payment;
    return account is PaymentAccount && account.status == _enabled
        ? completedSummary()
        : paymentPendingSummary();
  }

  var now = DateTime(2026, 10, 9, 12);
  void advance(Duration d) => now = now.add(d);
  int kycCalls = 0;
  int paymentCalls = 0;
  final launched = <Uri>[];

  GetMyStoreResponse _storeResponse() => myStoreResponse(
    capabilities: marketing
        ? null
        : [SellerCapability.SELLER_CAPABILITY_STOREFRONT_WRITE],
    publicListing: publicListingMissing([
      if (!certified) SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE,
    ]),
  );

  void reloadStore() => store.response = _storeResponse();

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.getMyKYC, (_, _) async {
          kycCalls++;
          final gate = kycGate;
          if (gate != null) await gate.future;
          final answer = kyc;
          if (answer is ConnectException) throw answer;
          return answer as GetMyKYCResponse;
        })
        .unary(SellerAppService.getMyOnboarding, (_, _) async {
          summaryCalls++;
          calls.add('getMyOnboarding');
          final gate = summaryGate;
          if (gate != null) await gate.future;
          final answer = summary ?? _derivedSummary();
          if (answer is ConnectException) throw answer;
          return answer as GetMyOnboardingResponse;
        })
        .unary(SellerAppService.refreshMyPaymentAccount, (_, _) {
          calls.add('refreshMyPaymentAccount');
          return RefreshMyPaymentAccountResponse();
        })
        .unary(SellerAppService.getMyPaymentAccount, (_, _) {
          paymentCalls++;
          final answer = payment;
          if (answer is ConnectException) throw answer;
          if (answer == null) throw ConnectException(Code.notFound, 'none');
          return GetMyPaymentAccountResponse(account: answer as PaymentAccount);
        })
        .unary(
          SellerAppService.createPaymentOnboardingSession,
          (_, _) => CreatePaymentOnboardingSessionResponse(
            surface: OnboardingSurface.ONBOARDING_SURFACE_HOSTED,
            url: 'https://connect.example.test/setup',
          ),
        )
        .build(),
  );
}

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _CouponFixture extends CouponListController {
  @override
  FutureOr<CouponListState> build() => const CouponListState();
}

List<Override> _overrides(_Server server) => [
  refreshClockProvider.overrideWithValue(() => server.now),
  sessionProvider.overrideWith(_LoggedIn.new),
  sellerClientProvider.overrideWithValue(server.client),
  myStoreRepositoryProvider.overrideWithValue(server.store.repository),
  couponListControllerProvider.overrideWith(_CouponFixture.new),
  externalBrowserLauncherProvider.overrideWithValue((uri) async {
    server.launched.add(uri);
    final gate = server.launchGate;
    if (gate != null) await gate.future;
    return true;
  }),
];

Future<GoRouter> _pump(
  WidgetTester tester,
  _Server server, {
  ProviderContainer? container,
  ValueListenable<bool>? visible,
  ValueListenable<bool>? tickers,
}) async {
  addTearDown(tester.view.reset);
  tester.view
    ..devicePixelRatio = 1
    ..physicalSize = const Size(402, 1600);
  final router = GoRouter(
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) {
          const wallet = WalletScreen();
          Widget page = wallet;
          if (tickers != null) {
            page = ValueListenableBuilder<bool>(
              valueListenable: tickers,
              builder: (_, on, _) => TickerMode(enabled: on, child: wallet),
            );
          }
          if (visible == null) return page;
          return ValueListenableBuilder<bool>(
            valueListenable: visible,
            builder: (_, show, _) => show ? page : const SizedBox(),
          );
        },
      ),
      GoRoute(
        path: SellerRoutes.certificationUpload,
        builder: (_, state) => Scaffold(body: Text('wizard:${state.extra}')),
      ),
      GoRoute(
        path: MainRoutes.settings,
        builder: (_, _) => const Scaffold(body: Text('settings')),
      ),
      GoRoute(
        path: SettingsRoutes.help,
        builder: (_, _) => const Scaffold(body: Text('help')),
      ),
    ],
  );
  final app = ScreenUtilInit(
    designSize: const Size(414, 896),
    enableScaleWH: () => false,
    enableScaleText: () => false,
    builder: (_, _) => MaterialApp.router(
      theme: AppTheme.light,
      routerConfig: router,
      builder: (context, child) => MediaQuery(
        data: MediaQuery.of(context).copyWith(disableAnimations: true),
        child: child!,
      ),
    ),
  );
  await tester.pumpWidget(
    container != null
        ? UncontrolledProviderScope(container: container, child: app)
        : ProviderScope(overrides: _overrides(server), child: app),
  );
  await _settle(tester);
  return router;
}

Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 12; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

Future<void> _tap(WidgetTester tester, Finder finder) async {
  await tester.ensureVisible(finder);
  await tester.tap(finder);
  await _settle(tester);
}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  group('讀取', () {
    testWidgets('讀取中顯示載入樣式，完成後換成內容', (tester) async {
      final server = _Server()..kycGate = Completer<void>();
      await _pump(tester, server);
      expect(find.byType(TruckLoadingOverlay), findsOneWidget);
      server.kycGate!.complete();
      await _settle(tester);
      expect(find.byType(TruckLoadingOverlay), findsNothing);
      expect(find.text('營運錢包'), findsOneWidget);
    });

    testWidgets('GetMyKYC 失敗顯示重試，不當成未開始認證', (tester) async {
      final server = _Server()
        ..kyc = ConnectException(Code.unavailable, 'later');
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('尚未通過商家認證'), findsNothing);
      expect(find.text('重試'), findsOneWidget);

      server.kyc = server._kycResponse();
      await _tap(tester, find.text('重試'));
      expect(find.text('無法載入認證狀態'), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
    });

    testWidgets('GetMyPaymentAccount 失敗顯示重試，不當成還沒有收款帳戶', (tester) async {
      final server = _Server()
        ..payment = ConnectException(Code.unavailable, 'later');
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('尚未通過商家認證'), findsNothing);

      server.payment = _account(_enabled);
      await _tap(tester, find.text('重試'));
      expect(find.text('無法載入認證狀態'), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
    });

    testWidgets('切回錢包重讀認證流程與收款帳戶，重讀時繼續顯示上一次的內容', (tester) async {
      final server = _Server();
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      final tickers = ValueNotifier(true);
      addTearDown(tickers.dispose);
      await _pump(tester, server, container: container, tickers: tickers);
      expect(find.text('帳戶與認證'), findsOneWidget);

      tickers.value = false;
      await _settle(tester);
      final kycBefore = server.kycCalls;
      final paymentBefore = server.paymentCalls;
      server.kycGate = Completer<void>();
      server.advance(statusStaleTime + const Duration(seconds: 1));
      tickers.value = true;
      await _settle(tester);

      expect(server.kycCalls, kycBefore + 1);
      expect(server.paymentCalls, paymentBefore + 1);
      expect(find.byType(TruckLoadingOverlay), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
      server.kycGate!.complete();
      await _settle(tester);
      expect(find.text('帳戶與認證'), findsOneWidget);
    });

    testWidgets('申請狀態是 UNSPECIFIED（無法確認）：顯示重試，不當成任何有效狀態', (tester) async {
      final server = _Server()
        ..kyc = GetMyKYCResponse(
          application: individualApplication(
            kycStatus: KYCStatus.KYC_STATUS_UNSPECIFIED,
          ),
        );
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('帳戶與認證'), findsNothing);
    });

    testWidgets('App 回到前景時重讀認證流程與收款帳戶', (tester) async {
      final server = _Server();
      await _pump(tester, server);
      final kycBefore = server.kycCalls;
      final paymentBefore = server.paymentCalls;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(server.kycCalls, kycBefore + 1);
      expect(server.paymentCalls, paymentBefore + 1);
    });
  });

  group('重讀時機與等待', () {
    testWidgets('第一次開啟各讀一次；切分頁回來再讀一次', (tester) async {
      final server = _Server();
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      final tickers = ValueNotifier(true);
      addTearDown(tickers.dispose);
      await _pump(tester, server, container: container, tickers: tickers);
      expect(server.kycCalls, 1);
      expect(server.paymentCalls, 1);

      tickers.value = false;
      await _settle(tester);
      server.advance(statusStaleTime + const Duration(seconds: 1));
      tickers.value = true;
      await _settle(tester);
      expect(server.kycCalls, 2);
      expect(server.paymentCalls, 2);
    });

    testWidgets('重讀失敗但還留著上一次成功的值：繼續顯示錢包，不出現錯誤面板', (tester) async {
      final server = _Server();
      await _pump(tester, server);
      expect(find.text('帳戶與認證'), findsOneWidget);

      server.kyc = ConnectException(Code.unavailable, 'later');
      server.payment = ConnectException(Code.unavailable, 'later');
      server.store.failure = ConnectException(Code.unavailable, 'later');
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);

      expect(find.text('無法載入認證狀態'), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
    });

    testWidgets('成為可見的重讀：30 秒內讀過不讀，超過 30 秒讀一次（假時鐘）', (tester) async {
      final server = _Server();
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      final tickers = ValueNotifier(true);
      addTearDown(tickers.dispose);
      await _pump(tester, server, container: container, tickers: tickers);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      final before = server.kycCalls;

      tickers.value = false;
      await _settle(tester);
      server.advance(const Duration(seconds: 29));
      tickers.value = true;
      await _settle(tester);
      expect(server.kycCalls, before, reason: '29 秒前才讀過');

      tickers.value = false;
      await _settle(tester);
      server.advance(const Duration(seconds: 2));
      tickers.value = true;
      await _settle(tester);
      expect(server.kycCalls, before + 1, reason: '超過 30 秒');
    });

    testWidgets('錢包成為目前的分頁時重讀；回前景只在錢包是目前的分頁時重讀', (tester) async {
      final server = _Server();
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      final tickers = ValueNotifier(true);
      addTearDown(tickers.dispose);
      await _pump(tester, server, container: container, tickers: tickers);

      tickers.value = false;
      await _settle(tester);
      var before = server.kycCalls;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(server.kycCalls, before, reason: '錢包不是目前的分頁，回前景不重讀');

      server.advance(statusStaleTime + const Duration(seconds: 1));
      tickers.value = true;
      await _settle(tester);
      expect(server.kycCalls, before + 1, reason: '切回錢包就重讀');

      before = server.kycCalls;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(server.kycCalls, before + 1);
    });

    testWidgets('被認證流程蓋住時（TickerMode 關閉）回前景不重讀，回來才重讀', (tester) async {
      final server = _Server();
      final router = await _pump(tester, server);
      unawaited(router.push(SellerRoutes.certificationUpload));
      await _settle(tester);
      expect(find.text('wizard:null'), findsOneWidget);

      var before = server.kycCalls;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(server.kycCalls, before, reason: '錢包被蓋住，不重讀');

      server.advance(statusStaleTime + const Duration(seconds: 1));
      router.pop();
      await _settle(tester);
      expect(server.kycCalls, before + 1, reason: '回到錢包就重讀');
    });

    testWidgets('GetMyStore 失敗而且沒有快照：同樣的錯誤面板與重試', (tester) async {
      final server = _Server();
      server.store.failure = ConnectException(Code.unavailable, 'later');
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('帳戶與認證'), findsNothing);

      server.store.failure = null;
      await _tap(tester, find.text('重試'));
      expect(find.text('無法載入認證狀態'), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
    });

    testWidgets('第一次顯示也等第一次 GetMyStore，帳戶與認證、優惠券不會後跳出來', (tester) async {
      final server = _Server();
      final storeGate = Completer<void>();
      server.store.gate = storeGate.future;
      await _pump(tester, server);
      expect(find.byType(TruckLoadingOverlay), findsOneWidget);
      expect(find.text('帳戶與認證'), findsNothing);
      storeGate.complete();
      await _settle(tester);
      expect(find.byType(TruckLoadingOverlay), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
      expect(find.text('優惠券'), findsOneWidget);
    });

    testWidgets('GetMyPaymentAccount 失敗但平台審核還沒核准：仍是認證閘門（讀取失敗只在核准之後才有意義）', (
      tester,
    ) async {
      final server = _Server(
        review: KYCStatus.KYC_STATUS_SUBMITTED,
        payment: ConnectException(Code.unavailable, 'later'),
        certified: false,
      );
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsNothing);
      expect(find.text('尚未通過商家認證'), findsOneWidget);
    });

    testWidgets('第一次顯示等 GetMyKYC 與 GetMyPaymentAccount，不會先閃出閘門', (
      tester,
    ) async {
      final server = _Server(
        review: KYCStatus.KYC_STATUS_SUBMITTED,
        certified: false,
      )..kycGate = Completer<void>();
      await _pump(tester, server);
      expect(find.byType(TruckLoadingOverlay), findsOneWidget);
      expect(find.text('尚未通過商家認證'), findsNothing);
      server.kycGate!.complete();
      await _settle(tester);
      expect(find.byType(TruckLoadingOverlay), findsNothing);
      expect(find.text('尚未通過商家認證'), findsOneWidget);
    });
  });

  group('平台審核還沒核准：沒有收款帳戶，顯示認證閘門', () {
    final cases = {
      null: '未送出',
      KYCStatus.KYC_STATUS_SUBMITTED: '審核中',
      KYCStatus.KYC_STATUS_REJECTED: '需修改',
    };
    for (final entry in cases.entries) {
      testWidgets('${entry.key}：目前狀態取自目錄「${entry.value}」，按鈕進認證流程，回來後重讀', (
        tester,
      ) async {
        final server = _Server(
          review: entry.key ?? KYCStatus.KYC_STATUS_DRAFT,
          certified: false,
        );
        if (entry.key == null) {
          server.kyc = ConnectException(Code.notFound, 'none');
        }
        final router = await _pump(tester, server);
        expect(find.text('尚未通過商家認證'), findsOneWidget);
        expect(find.textContaining('通過平台審核並啟用收款帳戶後，就能啟用錢包與收款'), findsOneWidget);
        expect(find.byKey(const Key('wallet-gate-status')), findsOneWidget);
        if (entry.key != null) {
          expect(find.text('目前狀態：${entry.value}'), findsOneWidget);
        }

        final before = server.kycCalls;
        await _tap(tester, find.text('前往商家認證'));
        expect(find.text('wizard:null'), findsOneWidget);
        router.pop(true);
        await _settle(tester);
        expect(server.kycCalls, greaterThan(before));
      });
    }

    testWidgets('讀得到申請但沒有申請（not_found）：目前狀態是「未送出」', (tester) async {
      final server = _Server(certified: false)
        ..kyc = ConnectException(Code.notFound, 'none')
        ..payment = null;
      await _pump(tester, server);
      expect(find.text('目前狀態：未送出'), findsOneWidget);
    });
  });

  group('是否顯示閘門由開通摘要決定', () {
    testWidgets('摘要讀取失敗：錯誤面板與重試，不當成未通過也不當成通過', (tester) async {
      final server = _Server()
        ..summary = ConnectException(Code.unavailable, 'later');
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('尚未通過商家認證'), findsNothing);
      expect(find.byKey(const Key('wallet-payment-panel')), findsNothing);

      server.summary = null;
      await _tap(tester, find.text('重試'));
      expect(find.text('無法載入認證狀態'), findsNothing);
    });

    testWidgets('申請已核准但摘要說平台審核還沒完成：顯示閘門', (tester) async {
      final server = _Server(certified: false)
        ..summary = reviewPendingSummary();
      await _pump(tester, server);
      expect(find.text('尚未通過商家認證'), findsOneWidget);
      expect(find.byKey(const Key('wallet-payment-panel')), findsNothing);
    });

    testWidgets('申請不是核准但摘要說平台審核與收款帳戶都完成：沒有閘門也沒有面板', (tester) async {
      final server = _Server(review: KYCStatus.KYC_STATUS_SUBMITTED)
        ..summary = completedSummary();
      await _pump(tester, server);
      expect(find.text('尚未通過商家認證'), findsNothing);
      expect(find.byKey(const Key('wallet-payment-panel')), findsNothing);
    });

    testWidgets('收款帳戶任務由摘要決定：帳戶資料說審核中但摘要說完成，沒有面板', (tester) async {
      final server = _Server(
        payment: _account(
          PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW,
        ),
      )..summary = completedSummary();
      await _pump(tester, server);
      expect(find.byKey(const Key('wallet-payment-panel')), findsNothing);
    });

    testWidgets('摘要 UNKNOWN 的平台審核不算完成：顯示閘門', (tester) async {
      final server = _Server(certified: false)
        ..summary = onboardingSummary([
          serverTask(
            OnboardingTaskId.ONBOARDING_TASK_ID_PLATFORM_REVIEW,
            OnboardingTaskState.ONBOARDING_TASK_STATE_UNKNOWN,
          ),
        ]);
      await _pump(tester, server);
      expect(find.text('尚未通過商家認證'), findsOneWidget);
    });

    testWidgets('第一次顯示也等摘要，不會先閃出閘門', (tester) async {
      final server = _Server(certified: false)..summaryGate = Completer<void>();
      await _pump(tester, server);
      expect(find.byType(TruckLoadingOverlay), findsOneWidget);
      expect(find.text('尚未通過商家認證'), findsNothing);
      server.summaryGate!.complete();
      await _settle(tester);
      expect(find.byType(TruckLoadingOverlay), findsNothing);
    });

    testWidgets('App 回到前景時也重讀摘要', (tester) async {
      final server = _Server();
      await _pump(tester, server);
      final before = server.summaryCalls;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(server.summaryCalls, before + 1);
    });
  });

  group('收款帳戶狀態面板', () {
    testWidgets('待設定：目錄的標題、說明與動作；新帳戶不說逾期或停用，按鈕開始開通（與認證狀態畫面同一個動作）', (
      tester,
    ) async {
      final server = _Server(
        payment: PaymentAccount(
          status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED,
          onboardingSurface: OnboardingSurface.ONBOARDING_SURFACE_HOSTED,
        ),
        certified: false,
      );
      await _pump(tester, server);
      expect(
        find.byKey(const ValueKey('wallet-payment-panel')),
        findsOneWidget,
      );
      expect(find.text('待設定'), findsOneWidget);
      expect(find.text('提供身分與銀行資料後即可開始收款。資料由金流商收集，平台不經手。'), findsOneWidget);
      expect(find.textContaining('逾期'), findsNothing);
      expect(find.textContaining('停用'), findsNothing);
      expect(find.text('尚未通過商家認證'), findsNothing);

      await _tap(tester, find.text('前往設定收款帳戶'));
      expect(find.text('wizard:null'), findsNothing);
      expect(
        server.launched.single.toString(),
        'https://connect.example.test/setup',
      );
    });

    testWidgets('平台審核完成、讀不到收款帳戶（not_found）：照 website 顯示認證閘門，不自己當成「準備中」', (
      tester,
    ) async {
      final server = _Server(certified: false)..payment = null;
      await _pump(tester, server);
      expect(find.text('準備中'), findsNothing);
      expect(find.text('尚未通過商家認證'), findsOneWidget);
      expect(find.byKey(const Key('wallet-payment-panel')), findsNothing);
    });

    testWidgets('面板的「重新整理」：先 RefreshMyPaymentAccount 再重讀收款帳戶與摘要', (
      tester,
    ) async {
      final server = _Server(
        certified: false,
        payment: _account(
          PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW,
        ),
      );
      await _pump(tester, server);
      server.calls.clear();

      await _tap(tester, find.text('重新整理'));

      expect(server.calls.first, 'refreshMyPaymentAccount');
      expect(server.calls, contains('getMyOnboarding'));
    });

    testWidgets('審核中：沒有額外操作的說明', (tester) async {
      final server = _Server(
        payment: _account(
          PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW,
        ),
        certified: false,
      );
      await _pump(tester, server);
      expect(find.text('審核中'), findsOneWidget);
      expect(find.text('金流商正在審核你的資料，不需要額外操作。'), findsOneWidget);
    });

    testWidgets('無法啟用：動作是聯絡客服，進幫助中心', (tester) async {
      final server = _Server(
        payment: _account(PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_DISABLED),
        certified: false,
      );
      await _pump(tester, server);
      expect(find.text('無法啟用'), findsOneWidget);
      await _tap(tester, find.text('聯絡客服'));
      expect(find.text('help'), findsOneWidget);
    });

    testWidgets('不認得的狀態（UNSPECIFIED）：顯示重試，不當成任何有效狀態', (tester) async {
      final server = _Server(payment: PaymentAccount(), certified: false);
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.byKey(const ValueKey('wallet-payment-panel')), findsNothing);
    });

    testWidgets('同一個面板經過不認得的狀態再回到認得的狀態：不丟例外，進行中的動作仍停用按鈕', (tester) async {
      final recognised = PaymentAccount(
        status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED,
        onboardingSurface: OnboardingSurface.ONBOARDING_SURFACE_HOSTED,
      );
      final server = _Server(payment: recognised, certified: false)
        ..launchGate = Completer<void>();
      await _pump(tester, server);
      final button = find.widgetWithText(OutlinedButton, '前往設定收款帳戶');
      await tester.tap(button);
      await _settle(tester);
      expect(tester.widget<OutlinedButton>(button).onPressed, isNull);

      Future<void> resumeWith(PaymentAccount next) async {
        server.payment = next;
        tester.binding.handleAppLifecycleStateChanged(
          AppLifecycleState.resumed,
        );
        await _settle(tester);
      }

      await resumeWith(PaymentAccount());
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      await resumeWith(recognised);
      expect(tester.takeException(), isNull);
      expect(tester.widget<OutlinedButton>(button).onPressed, isNull);
      server.launchGate!.complete();
      await _settle(tester);
      expect(tester.widget<OutlinedButton>(button).onPressed, isNotNull);
    });

    testWidgets('已啟用：沒有面板，也沒有 PayUni 的前往提領', (tester) async {
      final server = _Server();
      await _pump(tester, server);
      expect(find.byKey(const ValueKey('wallet-payment-panel')), findsNothing);
      expect(find.text('尚未通過商家認證'), findsNothing);
      expect(find.text('前往提領'), findsNothing);
    });
  });

  group('核准後', () {
    testWidgets('店家已認證：顯示帳戶與認證，商家認證列可展開', (tester) async {
      final server = _Server();
      await _pump(tester, server);
      expect(find.text('帳戶與認證'), findsOneWidget);
      expect(find.text('商家認證'), findsOneWidget);
      await _tap(tester, find.byKey(const ValueKey('wallet-cert-row')));
      expect(find.text('商家認證已通過'), findsOneWidget);
    });

    testWidgets('店家未認證：不顯示帳戶與認證', (tester) async {
      final server = _Server(
        payment: _account(
          PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW,
        ),
        certified: false,
      );
      await _pump(tester, server);
      expect(find.text('帳戶與認證'), findsNothing);
      expect(find.byKey(const ValueKey('wallet-cert-row')), findsNothing);
    });
  });

  group('優惠券依 MARKETING 能力，與認證狀態無關', () {
    for (final review in [
      KYCStatus.KYC_STATUS_DRAFT,
      KYCStatus.KYC_STATUS_SUBMITTED,
      KYCStatus.KYC_STATUS_APPROVED,
    ]) {
      testWidgets('$review 有 MARKETING 顯示、沒有則不顯示', (tester) async {
        final withMarketing = _Server(review: review, certified: false);
        await _pump(tester, withMarketing);
        expect(find.text('優惠券'), findsOneWidget);
        await tester.pumpWidget(const SizedBox.shrink());

        final without = _Server(
          review: review,
          certified: false,
          marketing: false,
        );
        await _pump(tester, without);
        expect(find.text('優惠券'), findsNothing);
      });
    }

    testWidgets('讀取失敗頁也依 MARKETING 顯示', (tester) async {
      final server = _Server()
        ..kyc = ConnectException(Code.unavailable, 'later');
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('優惠券'), findsOneWidget);
    });
  });
}
