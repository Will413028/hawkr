import 'dart:async';

import 'package:flutter/widgets.dart' show AppLifecycleState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code;
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/onboarding_summary.dart';
import 'package:hawkr_seller/features/seller/data/payment_account.dart';
import 'package:hawkr_seller/features/seller/data/payment_onboarding.dart';
import 'package:hawkr_seller/features/seller/data/seller_status_refresh.dart';

import '../../../support/fake_my_store.dart';
import '../../../support/connect_errors.dart';
import '../../../support/kyc_fixtures.dart';
import '../../../support/onboarding_fixtures.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

/// 假的伺服器：記下 GetMyPaymentAccount、GetMyOnboarding、RefreshMyPaymentAccount 與 CreatePaymentOnboardingSession 的呼叫。
class _Server {
  int paymentCalls = 0;
  int summaryCalls = 0;
  int refreshCalls = 0;

  /// 依序記下 RefreshMyPaymentAccount 與讀取的呼叫，驗證「先重讀再讀」的順序。
  final calls = <String>[];

  /// RefreshMyPaymentAccount 的失敗；null 是成功。
  Object? refreshFailure;
  final sessions = <CreatePaymentOnboardingSessionRequest>[];
  final launched = <Uri>[];
  bool launchResult = true;

  /// GetMyOnboarding 的回應。
  GetMyOnboardingResponse summary = paymentPendingSummary();

  /// 第 n 次（從 1 起）GetMyPaymentAccount 要等的門。
  final gates = <int, Completer<void>>{};
  CreatePaymentOnboardingSessionResponse sessionResponse =
      CreatePaymentOnboardingSessionResponse(
        hosted: HostedOnboarding(url: 'https://connect.example.test/setup/abc'),
      );

  late final store = FakeMyStoreServer();

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.getMyKYC, (_, _) {
          return GetMyKYCResponse(application: individualApplication());
        })
        .unary(SellerAppService.getMyOnboarding, (_, _) {
          summaryCalls++;
          calls.add('getMyOnboarding');
          return summary;
        })
        .unary(SellerAppService.refreshMyPaymentAccount, (_, _) {
          refreshCalls++;
          calls.add('refreshMyPaymentAccount');
          final failure = refreshFailure;
          if (failure != null) throw failure;
          return RefreshMyPaymentAccountResponse();
        })
        .unary(SellerAppService.getMyPaymentAccount, (_, _) async {
          calls.add('getMyPaymentAccount');
          final call = ++paymentCalls;
          await gates[call]?.future;
          return GetMyPaymentAccountResponse(
            account: PaymentAccount(
              status:
                  PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED,
            ),
          );
        })
        .unary(SellerAppService.createPaymentOnboardingSession, (req, _) {
          sessions.add(req);
          return sessionResponse;
        })
        .build(),
  );

  ProviderContainer container({bool allowInsecureUrl = false}) {
    final container = ProviderContainer(
      overrides: [
        allowInsecureOnboardingUrlProvider.overrideWithValue(allowInsecureUrl),
        sessionProvider.overrideWith(_LoggedIn.new),
        sellerClientProvider.overrideWithValue(client),
        myStoreRepositoryProvider.overrideWithValue(store.repository),
        externalBrowserLauncherProvider.overrideWithValue((uri) async {
          launched.add(uri);
          return launchResult;
        }),
      ],
    );
    addTearDown(container.dispose);
    final subs = [
      container.listen(currentStoreProvider, (_, _) {}),
      container.listen(currentKycProvider, (_, _) {}),
      container.listen(currentPaymentAccountProvider, (_, _) {}),
      container.listen(currentOnboardingProvider, (_, _) {}),
    ];
    for (final sub in subs) {
      addTearDown(sub.close);
    }
    return container;
  }
}

void main() {
  final binding = TestWidgetsFlutterBinding.ensureInitialized();

  /// 把 App 切到背景再回前景（不經過任何畫面）。
  void backgroundAndResume() {
    binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
    binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
  }

  group('開通連結只收 https（dev 才收 http）', () {
    test('stage 與 prod：http 連結失敗，不開瀏覽器', () async {
      final server = _Server()
        ..sessionResponse = CreatePaymentOnboardingSessionResponse(
          hosted: HostedOnboarding(url: 'http://connect.example.test/setup'),
        );
      final container = server.container();
      expect(
        await container.read(paymentOnboardingProvider.notifier).start(),
        PaymentOnboardingOutcome.failed,
      );
      expect(server.launched, isEmpty);
    });

    test('dev：http 連結可以開（本機測試）', () async {
      final server = _Server()
        ..sessionResponse = CreatePaymentOnboardingSessionResponse(
          hosted: HostedOnboarding(url: 'http://localhost:4242/setup'),
        );
      final container = server.container(allowInsecureUrl: true);
      expect(
        await container.read(paymentOnboardingProvider.notifier).start(),
        PaymentOnboardingOutcome.launched,
      );
      expect(server.launched.single.toString(), 'http://localhost:4242/setup');
    });
  });

  group('start', () {
    test('只送 App 回跳目的地、不宣告內嵌執行環境（網址與語系由伺服器處理），在外部瀏覽器開啟連結', () async {
      final server = _Server();
      final container = server.container();

      final outcome = await container
          .read(paymentOnboardingProvider.notifier)
          .start();

      expect(outcome, PaymentOnboardingOutcome.launched);
      final request = server.sessions.single;
      expect(
        request.destination,
        OnboardingReturnDestination.ONBOARDING_RETURN_DESTINATION_SELLER_APP,
      );
      expect(request.embeddedRuntimes, isEmpty);
      expect(server.launched, [
        Uri.parse('https://connect.example.test/setup/abc'),
      ]);
    });

    test('回應不是託管連結（伺服器回了內嵌）：失敗，不開瀏覽器', () async {
      final server = _Server()
        ..sessionResponse = CreatePaymentOnboardingSessionResponse(
          embedded: EmbeddedOnboarding(
            stripe: StripeEmbeddedOnboarding(
              clientSecret: 'secret',
              publishableKey: 'pk_test_x',
            ),
          ),
        );
      final container = server.container();
      expect(
        await container.read(paymentOnboardingProvider.notifier).start(),
        PaymentOnboardingOutcome.failed,
      );
      expect(server.launched, isEmpty);
    });

    test('兩支都沒設，或託管連結為空：失敗，不開瀏覽器', () async {
      final server = _Server()
        ..sessionResponse = CreatePaymentOnboardingSessionResponse();
      final container = server.container();
      expect(
        await container.read(paymentOnboardingProvider.notifier).start(),
        PaymentOnboardingOutcome.failed,
      );

      server.sessionResponse = CreatePaymentOnboardingSessionResponse(
        hosted: HostedOnboarding(),
      );
      expect(
        await container.read(paymentOnboardingProvider.notifier).start(),
        PaymentOnboardingOutcome.failed,
      );
      expect(server.launched, isEmpty);
    });

    test('瀏覽器開不起來：失敗，之後回到前景不當成從瀏覽器回來', () async {
      final server = _Server()
        ..launchResult = false
        ..summary = completedSummary();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      await container.read(accountOnboardingProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);

      expect(await notifier.start(), PaymentOnboardingOutcome.failed);

      final gate = Completer<void>();
      server.gates[server.paymentCalls + 1] = gate;
      final first = container
          .read(sellerStatusRefreshProvider.notifier)
          .refresh();
      await Future<void>.delayed(Duration.zero);
      final resumed = notifier.onResumed();
      gate.complete();
      await Future.wait([first, resumed]);
      expect(
        server.paymentCalls,
        2,
        reason: '進行中的一輪共用：沒有第二輪（開瀏覽器失敗，沒有「從瀏覽器回來」）',
      );
    });
  });

  group('onResumed：回到前景重讀', () {
    test('一般回前景：任務還沒完成就先 RefreshMyPaymentAccount，再重讀收款帳戶與開通摘要', () async {
      final server = _Server();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      await container.read(accountOnboardingProvider('acc-1').future);
      final before = server.paymentCalls;
      final summaryBefore = server.summaryCalls;
      server.calls.clear();

      await container.read(paymentOnboardingProvider.notifier).onResumed();

      expect(server.refreshCalls, 1);
      expect(server.calls.first, 'refreshMyPaymentAccount');
      expect(server.paymentCalls, before + 1);
      expect(server.summaryCalls, summaryBefore + 1);
    });

    test('一般回前景：收款帳戶任務已完成或鎖住就不呼叫 RefreshMyPaymentAccount，只重讀', () async {
      for (final summary in [completedSummary(), reviewPendingSummary()]) {
        final server = _Server()..summary = summary;
        final container = server.container();
        await container.read(accountPaymentAccountProvider('acc-1').future);
        await container.read(accountOnboardingProvider('acc-1').future);
        final before = server.paymentCalls;

        await container.read(paymentOnboardingProvider.notifier).onResumed();

        expect(server.refreshCalls, 0);
        expect(server.paymentCalls, before + 1);
      }
    });

    test('還沒有開通摘要（手上沒有資訊）：照樣先 RefreshMyPaymentAccount', () async {
      final server = _Server();
      final container = server.container();

      await container.read(paymentOnboardingProvider.notifier).autoRefresh();

      expect(server.refreshCalls, 1);
    });

    test('從外部瀏覽器回來：先 RefreshMyPaymentAccount，再讀收款帳戶與開通摘要（同一輪）', () async {
      final server = _Server();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);
      await notifier.start();
      server.calls.clear();

      await notifier.onResumed();

      expect(server.refreshCalls, 1);
      expect(server.calls.first, 'refreshMyPaymentAccount');
      expect(
        server.calls.skip(1),
        containsAll(['getMyPaymentAccount', 'getMyOnboarding']),
      );
    });

    test('從外部瀏覽器回來而 RefreshMyPaymentAccount 失敗：照樣讀，失敗不顯示、不丟出', () async {
      final server = _Server()
        ..refreshFailure = reasonError(
          Code.resourceExhausted,
          'RATE_LIMITED',
          message: '請稍後再試',
        );
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);
      await notifier.start();
      server.calls.clear();

      await notifier.onResumed();

      expect(server.calls.first, 'refreshMyPaymentAccount');
      expect(
        server.calls,
        containsAll(['getMyPaymentAccount', 'getMyOnboarding']),
      );
    });

    test(
      'confirmPaymentAccount 失敗（含 resource_exhausted、permission_denied）：回傳讀取結果，不丟出',
      () async {
        for (final code in [
          Code.unavailable,
          Code.resourceExhausted,
          Code.permissionDenied,
        ]) {
          final server = _Server()..refreshFailure = reasonError(code, 'DOWN');
          final container = server.container();
          final notifier = container.read(paymentOnboardingProvider.notifier);

          final read = await notifier.confirmPaymentAccount();

          expect(read, isTrue, reason: '$code');
          expect(server.refreshCalls, 1);
          expect(server.summaryCalls, greaterThan(0));
        }
      },
    );

    test('從外部瀏覽器回來帶 fresh: true：進行中那一輪可能在狀態改變之前就開始讀了，之後再讀一輪', () async {
      final server = _Server();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);
      expect(await notifier.start(), PaymentOnboardingOutcome.launched);

      // 賣家在瀏覽器裡的時候，已經有一輪重讀在進行（還沒讀完）。
      final gate = Completer<void>();
      server.gates[server.paymentCalls + 1] = gate;
      final inFlight = container
          .read(sellerStatusRefreshProvider.notifier)
          .refresh();
      await Future<void>.delayed(Duration.zero);
      final callsDuringRound = server.paymentCalls;

      final resumed = notifier.onResumed();
      gate.complete();
      await Future.wait([inFlight, resumed]);

      expect(
        server.paymentCalls,
        callsDuringRound + 1,
        reason: 'fresh: true：進行中那輪結束後，另起一輪',
      );
    });

    test('不是從瀏覽器回來而且任務已完成（一般回前景）共用進行中的一輪', () async {
      final server = _Server()..summary = completedSummary();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      await container.read(accountOnboardingProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);

      final gate = Completer<void>();
      server.gates[server.paymentCalls + 1] = gate;
      final inFlight = container
          .read(sellerStatusRefreshProvider.notifier)
          .refresh();
      await Future<void>.delayed(Duration.zero);
      final callsDuringRound = server.paymentCalls;

      final resumed = notifier.onResumed();
      gate.complete();
      await Future.wait([inFlight, resumed]);

      expect(server.paymentCalls, callsDuringRound);
      expect(server.refreshCalls, 0);
    });

    test('「從瀏覽器回來」只算一次：第二次回前景不再帶 fresh', () async {
      final server = _Server()..summary = completedSummary();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      await container.read(accountOnboardingProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);
      await notifier.start();
      await notifier.onResumed();

      final gate = Completer<void>();
      server.gates[server.paymentCalls + 1] = gate;
      final inFlight = container
          .read(sellerStatusRefreshProvider.notifier)
          .refresh();
      await Future<void>.delayed(Duration.zero);
      final callsDuringRound = server.paymentCalls;
      final resumed = notifier.onResumed();
      gate.complete();
      await Future.wait([inFlight, resumed]);

      expect(server.paymentCalls, callsDuringRound);
    });

    test('沒有任何畫面在監看時，從瀏覽器回來的那次回前景也帶 fresh: true，而且只一次', () async {
      final server = _Server();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);
      expect(await notifier.start(), PaymentOnboardingOutcome.launched);

      Future<int> callsAfterResumeDuringRound() async {
        final gate = Completer<void>();
        server.gates[server.paymentCalls + 1] = gate;
        final inFlight = container
            .read(sellerStatusRefreshProvider.notifier)
            .refresh();
        await Future<void>.delayed(Duration.zero);
        final duringRound = server.paymentCalls;
        backgroundAndResume();
        gate.complete();
        await inFlight;
        await Future<void>.delayed(const Duration(milliseconds: 20));
        return server.paymentCalls - duringRound;
      }

      expect(
        await callsAfterResumeDuringRound(),
        1,
        reason: '從瀏覽器回來：進行中那輪結束後另起一輪',
      );
      expect(
        await callsAfterResumeDuringRound(),
        0,
        reason: '記號已消耗：第二次回前景不再帶 fresh（也不自己重讀）',
      );
    });

    test('一次回前景最多一次 RefreshMyPaymentAccount：有畫面登記且剛從瀏覽器回來，也只一次', () async {
      final server = _Server();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);
      final release = notifier.watchForeground();
      final releaseAgain = notifier.watchForeground();

      expect(await notifier.start(), PaymentOnboardingOutcome.launched);
      server.refreshCalls = 0;
      backgroundAndResume();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(server.refreshCalls, 1, reason: '從瀏覽器回來，兩個畫面登記');

      server.refreshCalls = 0;
      backgroundAndResume();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(server.refreshCalls, 1, reason: '一般回前景，兩個畫面登記');

      release();
      release();
      server.refreshCalls = 0;
      backgroundAndResume();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(server.refreshCalls, 1, reason: '還剩一個登記（重複取消無效）');

      releaseAgain();
      server.refreshCalls = 0;
      backgroundAndResume();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(server.refreshCalls, 0, reason: '沒有畫面登記也不是從瀏覽器回來：不重讀');
    });
  });
}
